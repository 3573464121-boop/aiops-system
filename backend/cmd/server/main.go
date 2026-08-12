package main

import (
	"context"
	"log"
	"os"
	"strings"

	"aiops-mvp/internal/app"
	"aiops-mvp/internal/httpapi"
	"aiops-mvp/internal/knowledge"
	"aiops-mvp/internal/llm"
	"aiops-mvp/internal/storage"
	"aiops-mvp/internal/tools"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	path := env("KNOWLEDGE_PATH", "../README-原始需求.md")
	index, err := knowledge.LoadMarkdown(path)
	if err != nil {
		log.Printf("知识库加载失败，继续以空索引启动: %v", err)
	} else {
		log.Printf("知识库已加载: %d 个分块", index.Size())
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
	llmClient := &llm.Client{BaseURL: os.Getenv("LLM_BASE_URL"), APIKey: os.Getenv("LLM_API_KEY"), Model: os.Getenv("LLM_MODEL")}
	service := app.New(toolService, llmClient, repo)
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
