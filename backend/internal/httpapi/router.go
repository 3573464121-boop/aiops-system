package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"aiops-mvp/internal/app"
	"aiops-mvp/internal/auth"
	"aiops-mvp/internal/domain"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func New(s *app.Service, signer *auth.Signer, knowledgeCount int, storageModes ...string) *gin.Engine {
	storageMode := "memory"
	if len(storageModes) > 0 && storageModes[0] != "" {
		storageMode = storageModes[0]
	}
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{AllowOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"}, AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Origin", "Content-Type", "Authorization", "X-User-ID"}, MaxAge: 12 * time.Hour}))
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "time": time.Now(), "knowledge_chunks": knowledgeCount, "storage": storageMode})
	})
	api := r.Group("/api/v1")

	// 登录：公开接口，用户名 + 密码换取令牌。
	api.POST("/auth/login", func(c *gin.Context) {
		var req domain.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请填写用户名与密码"})
			return
		}
		u, err := s.Authenticate(req.Username, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		token, err := signer.Issue(u.ID, u.Username, u.Role, time.Now())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "签发令牌失败"})
			return
		}
		c.JSON(200, domain.LoginResponse{Token: token, User: u})
	})

	// requireAdmin 用作管理类接口的附加中间件：仅 admin 角色放行。
	requireAdmin := func(c *gin.Context) {
		if role, _ := c.Get("role"); role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "该操作需要管理员权限"})
			return
		}
		c.Next()
	}

	// 以下所有接口都要求携带有效令牌。
	api.Use(func(c *gin.Context) {
		h := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		claims, err := signer.Verify(token, time.Now())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.Set("uid", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Request = c.Request.WithContext(app.WithActor(c.Request.Context(), claims.UserID, claims.Username, claims.Role))
		c.Next()
	})

	// 当前登录用户信息。
	api.GET("/auth/me", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"id":       c.GetString("uid"),
			"username": c.GetString("username"),
			"role":     c.GetString("role"),
		})
	})

	api.GET("/system/status", func(c *gin.Context) {
		c.JSON(200, gin.H{"backend": "online", "alert_provider": s.Tools.AlertProviderName(), "log_provider": s.Tools.LogProviderName(), "asset_provider": s.Tools.AssetProviderName(), "llm_provider": mode("LLM_BASE_URL"), "knowledge_provider": s.Tools.KnowledgeMode(), "knowledge_chunks": knowledgeCount, "storage_provider": storageMode, "safe_mode": true})
	})
	api.GET("/data-sources", func(c *gin.Context) {
		items := s.Tools.DataSources()
		llmConfigured := strings.TrimSpace(os.Getenv("LLM_BASE_URL")) != ""
		llmStatus, llmMessage := "demo", "未配置大模型服务"
		if llmConfigured {
			llmStatus, llmMessage = "ready", "大模型服务已配置"
		}
		items = append(items,
			domain.DataSourceStatus{Name: "llm", Kind: "model", Mode: mode("LLM_BASE_URL"), Configured: llmConfigured, Status: llmStatus, Message: llmMessage},
			domain.DataSourceStatus{Name: "storage", Kind: "storage", Mode: storageMode, Configured: storageMode == "mysql", Status: "ready", Message: "持久化存储可用"},
		)
		c.JSON(200, gin.H{"items": items, "total": len(items)})
	})
	api.POST("/data-sources/:name/test", requireAdmin, func(c *gin.Context) {
		name := strings.ToLower(strings.TrimSpace(c.Param("name")))
		if name == "llm" {
			configured := strings.TrimSpace(os.Getenv("LLM_BASE_URL")) != ""
			status, message := "error", "未配置 LLM_BASE_URL"
			if configured {
				status, message = "ready", "大模型服务已配置；实际可用性由下一次诊断验证"
			}
			c.JSON(200, domain.DataSourceStatus{Name: name, Kind: "model", Mode: mode("LLM_BASE_URL"), Configured: configured, Status: status, Message: message})
			return
		}
		if name == "storage" {
			c.JSON(200, domain.DataSourceStatus{Name: name, Kind: "storage", Mode: storageMode, Configured: storageMode == "mysql", Status: "ready", Message: "当前存储连接正常"})
			return
		}
		result := s.Tools.TestDataSource(name)
		if result.Status == "error" {
			c.JSON(http.StatusBadGateway, result)
			return
		}
		c.JSON(200, result)
	})
	api.GET("/tools", func(c *gin.Context) {
		c.JSON(200, gin.H{"items": []gin.H{{"name": "get_alerts", "mode": s.Tools.AlertProviderName(), "readonly": true}, {"name": "search_logs", "mode": s.Tools.LogProviderName(), "readonly": true}, {"name": "search_knowledge", "mode": s.Tools.KnowledgeMode(), "readonly": true}, {"name": "get_assets", "mode": s.Tools.AssetProviderName(), "readonly": true}, {"name": "lookup_ip", "mode": s.Tools.AssetProviderName(), "readonly": true}, {"name": "recall_memory", "mode": "memory", "readonly": true}}})
	})
	api.GET("/alerts/active", func(c *gin.Context) {
		v, err := s.Tools.Alerts(strings.TrimSpace(c.Query("product_id")))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v), "mode": s.Tools.AlertProviderName()})
	})
	api.GET("/logs/search", func(c *gin.Context) {
		pid := strings.TrimSpace(c.Query("product_id"))
		if pid == "" {
			c.JSON(400, gin.H{"error": "product_id is required"})
			return
		}
		v, err := s.Tools.Logs(pid, c.Query("query"))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v), "mode": s.Tools.LogProviderName()})
	})
	api.GET("/knowledge/search", func(c *gin.Context) {
		q := strings.TrimSpace(c.Query("query"))
		if q == "" {
			c.JSON(400, gin.H{"error": "query is required"})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
		if limit < 1 || limit > 20 {
			limit = 5
		}
		v := s.Tools.SearchKnowledge(q, limit)
		c.JSON(200, gin.H{"items": v, "total": len(v), "mode": s.Tools.KnowledgeMode()})
	})
	api.GET("/assets", func(c *gin.Context) {
		v, err := s.Tools.Assets(strings.TrimSpace(c.Query("product_id")))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v), "mode": s.Tools.AssetProviderName()})
	})
	api.GET("/assets/lookup", func(c *gin.Context) {
		ip := strings.TrimSpace(c.Query("ip"))
		if ip == "" {
			c.JSON(400, gin.H{"error": "ip is required"})
			return
		}
		v, err := s.Tools.LookupIP(ip)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.POST("/diagnoses", func(c *gin.Context) {
		var req domain.DiagnosisRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		req.ProductID, req.Question = strings.TrimSpace(req.ProductID), strings.TrimSpace(req.Question)
		if req.ProductID == "" || req.Question == "" {
			c.JSON(400, gin.H{"error": "product_id and question cannot be blank"})
			return
		}
		c.JSON(200, s.Diagnose(c.Request.Context(), req))
	})
	api.POST("/diagnoses/stream", func(c *gin.Context) {
		var req domain.DiagnosisRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		req.ProductID, req.Question = strings.TrimSpace(req.ProductID), strings.TrimSpace(req.Question)
		if req.ProductID == "" || req.Question == "" {
			c.JSON(400, gin.H{"error": "product_id and question cannot be blank"})
			return
		}
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(500, gin.H{"error": "streaming unsupported"})
			return
		}
		h := c.Writer.Header()
		h.Set("Content-Type", "text/event-stream; charset=utf-8")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		emit := func(ev domain.StreamEvent) {
			b, _ := json.Marshal(ev)
			fmt.Fprintf(c.Writer, "data: %s\n\n", b)
			flusher.Flush()
		}
		result := s.DiagnoseStream(c.Request.Context(), req, emit)
		emit(domain.StreamEvent{Type: "result", Result: &result})
		emit(domain.StreamEvent{Type: "done"})
	})
	api.GET("/issues", func(c *gin.Context) {
		v, err := s.Issues()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.POST("/issues", func(c *gin.Context) {
		var req domain.IssueRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.ProductID, req.Title, req.Diagnosis = strings.TrimSpace(req.ProductID), strings.TrimSpace(req.Title), strings.TrimSpace(req.Diagnosis)
		if req.ProductID == "" || req.Title == "" || req.Diagnosis == "" {
			c.JSON(400, gin.H{"error": "product_id, title and diagnosis cannot be blank"})
			return
		}
		v, err := s.CreateIssue(c.Request.Context(), req)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, v)
	})
	api.GET("/inspections", func(c *gin.Context) {
		v, err := s.ListInspectionTasks()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.POST("/inspections", requireAdmin, func(c *gin.Context) {
		var req domain.InspectionTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.ProductID = strings.TrimSpace(req.ProductID)
		if req.ProductID == "" {
			c.JSON(400, gin.H{"error": "product_id cannot be blank"})
			return
		}
		v, err := s.CreateInspectionTask(c.Request.Context(), req)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, v)
	})
	api.POST("/inspections/:id/toggle", requireAdmin, func(c *gin.Context) {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := s.ToggleInspectionTask(c.Request.Context(), c.Param("id"), body.Enabled); err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
	api.POST("/inspections/:id/run", requireAdmin, func(c *gin.Context) {
		v, err := s.RunInspectionNow(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, v)
	})
	api.DELETE("/inspections/:id", requireAdmin, func(c *gin.Context) {
		if err := s.DeleteInspectionTask(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
	api.GET("/inspection-reports", func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		v, err := s.ListInspectionReports(strings.TrimSpace(c.Query("task_id")), limit)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.GET("/memories", func(c *gin.Context) {
		v, err := s.ListMemories()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.POST("/memories", requireAdmin, func(c *gin.Context) {
		var req domain.MemoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		v, err := s.CreateMemory(c.Request.Context(), req)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, v)
	})
	api.DELETE("/memories/:id", requireAdmin, func(c *gin.Context) {
		if err := s.DeleteMemory(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
	api.POST("/memories/extract", requireAdmin, func(c *gin.Context) {
		var req domain.MemoryExtractRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		draft, err := s.ExtractMemory(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"draft": draft})
	})
	api.GET("/users", requireAdmin, func(c *gin.Context) {
		v, err := s.ListUsers()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.POST("/users", requireAdmin, func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		u, err := s.CreateUser(c.Request.Context(), req.Username, req.Password, req.Role)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, u)
	})
	api.GET("/approvals", func(c *gin.Context) {
		v, err := s.ListApprovals(c.Query("status"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.GET("/approvals/:id", func(c *gin.Context) {
		v, err := s.GetApproval(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, v)
	})
	api.POST("/approvals", func(c *gin.Context) {
		var req domain.ApprovalRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		v, err := s.CreateApproval(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, v)
	})
	api.POST("/approvals/:id/review", requireAdmin, func(c *gin.Context) {
		var req domain.ApprovalDecisionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		v, err := s.ReviewApproval(c.Request.Context(), c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, v)
	})
	api.POST("/approvals/:id/execute", requireAdmin, func(c *gin.Context) {
		var req domain.ApprovalExecutionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		v, err := s.ExecuteApproval(c.Request.Context(), c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, v)
	})
	api.POST("/approvals/:id/cancel", func(c *gin.Context) {
		v, err := s.CancelApproval(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, v)
	})
	api.GET("/diagnosis-runs", requireAdmin, func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))
		v, err := s.ListDiagnosisRuns(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	api.POST("/diagnosis-runs/:id/review", requireAdmin, func(c *gin.Context) {
		var req domain.DiagnosisRunReviewRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		v, err := s.ReviewDiagnosisRun(c.Request.Context(), c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, v)
	})
	api.GET("/fault-cases", requireAdmin, func(c *gin.Context) {
		v, err := s.ListFaultCases()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": v, "total": len(v)})
	})
	api.POST("/fault-cases", requireAdmin, func(c *gin.Context) {
		var req domain.FaultCaseRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		v, err := s.CreateFaultCase(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, v)
	})
	api.GET("/fault-cases/:id", requireAdmin, func(c *gin.Context) {
		v, err := s.GetFaultCase(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})
	api.DELETE("/fault-cases/:id", requireAdmin, func(c *gin.Context) {
		if err := s.DeleteFaultCase(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	api.POST("/fault-cases/:id/replay", requireAdmin, func(c *gin.Context) {
		var req domain.ReplayRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		v, err := s.ReplayFaultCase(c.Request.Context(), c.Param("id"), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": v, "total": len(v)})
	})
	api.GET("/replay-results", requireAdmin, func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "1000"))
		v, err := s.ListReplayResults(c.Query("case_id"), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": v, "total": len(v)})
	})
	api.GET("/audits", func(c *gin.Context) {
		v, err := s.Audits()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": v, "total": len(v)})
	})
	return r
}
func mode(key string) string {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return "demo"
	}
	return "configured"
}
