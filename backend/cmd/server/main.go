package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aiops-mvp/internal/app"
	"aiops-mvp/internal/embed"
	"aiops-mvp/internal/httpapi"
	"aiops-mvp/internal/knowledge"
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
	index, err := knowledge.LoadMarkdownFiles(paths)
	if err != nil {
		log.Printf("知识库加载失败，继续以空索引启动: %v", err)
	} else {
		log.Printf("知识库已加载: %d 个分块（来自 %d 个文件）", index.Size(), len(paths))
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
	toolService := tools.NewService(index, alertProvider, logProvider)

	// 可选：为知识库构建向量索引（BM25 + 向量 RRF 混合检索）。未配置或构建失败则退回 BM25。
	embedder := &embed.Client{BaseURL: os.Getenv("EMBED_BASE_URL"), APIKey: os.Getenv("EMBED_API_KEY"), Model: os.Getenv("EMBED_MODEL")}
	if embedder.Enabled() && index != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		if vecs, e := embedder.Embed(ctx, index.ChunkTexts()); e != nil {
			log.Printf("向量索引构建失败，知识库仅用 BM25: %v", e)
		} else {
			index.SetVectors(vecs)
			toolService.Embed = func(q string) ([]float32, error) {
				c2, c2cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer c2cancel()
				vs, err2 := embedder.Embed(c2, []string{q})
				if err2 != nil || len(vs) == 0 {
					return nil, err2
				}
				return vs[0], nil
			}
			log.Printf("向量索引已构建: %d 个分块 (embed=%s)", index.Size(), embedder.Model)
		}
		cancel()
	} else {
		log.Printf("知识检索: 仅 BM25（未配置 EMBED_BASE_URL）")
	}

	llmClient := &llm.Client{BaseURL: os.Getenv("LLM_BASE_URL"), APIKey: os.Getenv("LLM_API_KEY"), Model: os.Getenv("LLM_MODEL")}
	service := app.New(toolService, llmClient, repo)

	// 主动巡检调度器：进程内定时跑到期的巡检任务。
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	service.StartInspectionScheduler(schedCtx)
	log.Printf("主动巡检调度器已启动")

	router := httpapi.New(service, indexSize(index), storageMode)
	addr := env("APP_ADDR", ":8080")
	log.Printf("AIOps API listening on %s (storage=%s)", addr, storageMode)
	if err = router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
func indexSize(i *knowledge.Index) int {
	if i == nil {
		return 0
	}
	return i.Size()
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
