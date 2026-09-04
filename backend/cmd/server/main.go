package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aiops-mvp/internal/app"
	"aiops-mvp/internal/auth"
	"aiops-mvp/internal/embed"
	"aiops-mvp/internal/httpapi"
	"aiops-mvp/internal/llm"
	"aiops-mvp/internal/storage"
	"aiops-mvp/internal/tools"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	paths := []string{env("KNOWLEDGE_PATH", "../README-原始需求.md")}
	if extra, _ := filepath.Glob(filepath.Join(env("KNOWLEDGE_DIR", "knowledge-base"), "*.md")); len(extra) > 0 {
		paths = append(paths, extra...)
	}
	repo := storage.Repository(storage.NewMemory())
	storageMode := "memory"
	if dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN")); dsn != "" {
		mysqlRepo, dbErr := storage.NewMySQL(context.Background(), dsn)
		if dbErr != nil {
			log.Printf("MySQL 初始化失败，降级为内存存储: %v", dbErr)
		} else {
			repo, storageMode = mysqlRepo, "mysql"
			defer repo.Close()
		}
	}
	var alertProvider tools.AlertProvider = tools.DemoAlertProvider{}
	if u := strings.TrimSpace(os.Getenv("N9E_BASE_URL")); u != "" {
		alertProvider = tools.N9EAlertProvider{BaseURL: u, Token: os.Getenv("N9E_TOKEN")}
		log.Printf("告警数据源: Nightingale (%s)", u)
	} else {
		log.Printf("告警数据源: demo（未配置 N9E_BASE_URL）")
	}
	var logProvider tools.LogProvider = tools.DemoLogProvider{}
	if u := strings.TrimSpace(os.Getenv("LOG_BASE_URL")); u != "" {
		logProvider = tools.LokiLogProvider{BaseURL: u, Token: os.Getenv("LOG_TOKEN")}
		log.Printf("日志数据源: Loki (%s)", u)
	} else {
		log.Printf("日志数据源: demo（未配置 LOG_BASE_URL）")
	}
	toolService := tools.NewService(nil, alertProvider, logProvider)

	// 可选：为知识库构建向量索引（BM25 + 向量 RRF 混合检索）。未配置或构建失败则退回 BM25。
	embedder := &embed.Client{BaseURL: os.Getenv("EMBED_BASE_URL"), APIKey: os.Getenv("EMBED_API_KEY"), Model: os.Getenv("EMBED_MODEL")}
	if embedder.Enabled() {
		toolService.EmbedDocuments = func(texts []string) ([][]float32, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			return embedder.Embed(ctx, texts)
		}
		toolService.Embed = func(q string) ([]float32, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			vectors, err := embedder.Embed(ctx, []string{q})
			if err != nil || len(vectors) == 0 {
				return nil, err
			}
			return vectors[0], nil
		}
	} else {
		log.Printf("知识检索: 仅 BM25（未配置 EMBED_BASE_URL）")
	}

	llmClient := &llm.Client{BaseURL: os.Getenv("LLM_BASE_URL"), APIKey: os.Getenv("LLM_API_KEY"), Model: os.Getenv("LLM_MODEL")}
	service := app.New(toolService, llmClient, repo)
	knowledgeStatus, knowledgeErr := service.InitializeKnowledge(context.Background(), paths, env("KNOWLEDGE_MANAGED_DIR", "knowledge-managed"))
	if knowledgeErr != nil {
		log.Printf("知识库初始化失败，继续以空索引启动: %v", knowledgeErr)
	} else {
		log.Printf("知识库已加载: %d 个文档，%d 个分块，模式 %s", knowledgeStatus.EnabledCount, knowledgeStatus.ChunkCount, knowledgeStatus.Mode)
		if knowledgeStatus.Warning != "" {
			log.Printf("知识库提示: %s", knowledgeStatus.Warning)
		}
	}
	if judgeURL := strings.TrimSpace(os.Getenv("JUDGE_BASE_URL")); judgeURL != "" {
		service.Judge = &llm.Client{BaseURL: judgeURL, APIKey: os.Getenv("JUDGE_API_KEY"), Model: env("JUDGE_MODEL", llmClient.Model)}
		log.Printf("独立判官已配置: %s", service.Judge.Model)
	} else {
		log.Printf("独立判官未配置，回放将使用被测模型自评；正式实验建议配置 JUDGE_BASE_URL")
	}

	// 认证：HMAC 令牌签发器 + 首次启动时播种管理员账号。
	secret := strings.TrimSpace(os.Getenv("AUTH_SECRET"))
	if secret == "" {
		secret = "aiops-dev-secret-change-me"
		log.Printf("警告: 未配置 AUTH_SECRET，正在使用内置开发密钥，生产环境务必在 .env 中设置")
	}
	signer := auth.NewSigner(secret, 12*time.Hour)
	adminPass := env("AUTH_ADMIN_PASSWORD", "admin123")
	if created, err := service.EnsureSeedAdmin("admin", adminPass); err != nil {
		log.Printf("播种管理员账号失败: %v", err)
	} else if created {
		log.Printf("已创建初始管理员账号: admin / %s（首次登录后请尽快修改）", adminPass)
	}

	// 主动巡检调度器：进程内定时跑到期的巡检任务。
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	service.StartInspectionScheduler(schedCtx)
	log.Printf("主动巡检调度器已启动")

	router := httpapi.New(service, signer, toolService.KnowledgeSize(), storageMode)
	addr := env("APP_ADDR", ":8080")
	log.Printf("AIOps API listening on %s (storage=%s)", addr, storageMode)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
