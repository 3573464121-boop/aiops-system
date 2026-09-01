package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

func (s *Service) recordDiagnosisRun(ctx context.Context, req domain.DiagnosisRequest, result domain.DiagnosisResult, duration time.Duration) {
	_, username, _ := actorFromContext(ctx)
	model := ""
	if s.LLM != nil {
		model = s.LLM.Model
	}
	sources := make([]string, 0, len(result.Evidence))
	toolsUsed := make([]string, 0, len(result.Trace))
	sourceSeen := map[string]bool{}
	toolSeen := map[string]bool{}
	failed := 0
	knowledgeHit, memoryHit, assetHit := false, false, false
	for _, e := range result.Evidence {
		if e.Source != "" && !sourceSeen[e.Source] {
			sources = append(sources, e.Source)
			sourceSeen[e.Source] = true
		}
		switch strings.ToLower(e.Type) {
		case "knowledge":
			knowledgeHit = true
		case "memory":
			memoryHit = true
		case "asset":
			assetHit = true
		}
	}
	for _, trace := range result.Trace {
		if trace.Status == "error" {
			failed++
		}
		if strings.HasPrefix(trace.Tool, "llm_") || trace.Tool == "" || toolSeen[trace.Tool] {
			continue
		}
		toolsUsed = append(toolsUsed, trace.Tool)
		toolSeen[trace.Tool] = true
	}
	run := domain.DiagnosisRun{
		ID: fmt.Sprintf("RUN-%d", time.Now().UnixNano()), ProductID: req.ProductID, Question: req.Question,
		Mode: result.Mode, Model: model, Summary: result.Summary, Confidence: result.Confidence,
		EvidenceCount: len(result.Evidence), AlertCount: len(result.Alerts), ToolCallCount: len(toolsUsed), FailedToolCount: failed,
		KnowledgeHit: knowledgeHit, MemoryHit: memoryHit, AssetHit: assetHit, DurationMS: duration.Milliseconds(),
		AlertProvider: s.Tools.AlertProviderName(), LogProvider: s.Tools.LogProviderName(), KnowledgeMode: s.Tools.KnowledgeMode(),
		EvidenceSources: sources, Tools: toolsUsed, Username: username, CreatedAt: time.Now(),
	}
	_ = s.Repo.CreateDiagnosisRun(run)
}

func (s *Service) ListDiagnosisRuns(limit int) ([]domain.DiagnosisRun, error) {
	return s.Repo.ListDiagnosisRuns(limit)
}

func (s *Service) ReviewDiagnosisRun(ctx context.Context, id string, req domain.DiagnosisRunReviewRequest) (domain.DiagnosisRun, error) {
	v, err := s.Repo.GetDiagnosisRun(strings.TrimSpace(id))
	if err != nil {
		return domain.DiagnosisRun{}, err
	}
	goldCause := strings.TrimSpace(req.GoldCause)
	if req.Included && goldCause == "" {
		return domain.DiagnosisRun{}, fmt.Errorf("纳入论文数据集时必须填写标准根因")
	}
	_, username, _ := actorFromContext(ctx)
	v.Included = req.Included
	v.GoldCause = goldCause
	v.ReviewerNote = strings.TrimSpace(req.Note)
	v.ReviewedBy = username
	v.ReviewedAt = time.Now()
	if err := s.Repo.UpdateDiagnosisRunReview(v); err != nil {
		s.addAudit(ctx, "review_diagnosis_run", v.ProductID, "error", 0)
		return domain.DiagnosisRun{}, err
	}
	s.addAudit(ctx, "review_diagnosis_run", v.ProductID, "success", 0)
	return v, nil
}
