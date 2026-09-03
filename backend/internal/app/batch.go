package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

func (s *Service) CreateExperimentBatch(ctx context.Context, req domain.ExperimentBatchRequest) (domain.ExperimentBatch, error) {
	if s.LLM == nil || !s.LLM.Enabled() {
		return domain.ExperimentBatch{}, fmt.Errorf("实验批次需要配置可用的大模型")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domain.ExperimentBatch{}, fmt.Errorf("批次名称不能为空")
	}
	if req.Repeats == 0 {
		req.Repeats = 1
	}
	if req.Repeats < 1 || req.Repeats > 10 {
		return domain.ExperimentBatch{}, fmt.Errorf("重复次数必须在 1 到 10 之间")
	}
	configs, err := normalizeReplayConfigs(req.Configs)
	if err != nil {
		return domain.ExperimentBatch{}, err
	}
	caseIDs := uniqueStrings(req.CaseIDs)
	if len(caseIDs) < 1 || len(caseIDs) > 100 {
		return domain.ExperimentBatch{}, fmt.Errorf("每个批次必须选择 1 到 100 个案例")
	}
	for _, id := range caseIDs {
		if _, err := s.GetFaultCase(id); err != nil {
			return domain.ExperimentBatch{}, fmt.Errorf("案例 %s 不存在", id)
		}
	}
	_, username, _ := actorFromContext(ctx)
	judgeModel, judgeSource := s.judgeInfo()
	now := time.Now()
	v := domain.ExperimentBatch{
		ID: fmt.Sprintf("BATCH-%d", now.UnixNano()), Name: name, CaseIDs: caseIDs,
		Configs: configs, Repeats: req.Repeats, Model: s.LLM.Model,
		JudgeModel: judgeModel, JudgeSource: judgeSource, KnowledgeMode: s.Tools.KnowledgeMode(),
		Status: "pending", TotalRuns: len(caseIDs) * len(configs) * req.Repeats,
		CreatedBy: username, CreatedAt: now,
	}
	if err := s.Repo.CreateExperimentBatch(v); err != nil {
		return domain.ExperimentBatch{}, err
	}
	userID, _, role := actorFromContext(ctx)
	go s.runExperimentBatch(WithActor(context.Background(), userID, username, role), v.ID)
	s.addAudit(ctx, "create_experiment_batch", "", "success", 0)
	return v, nil
}

func uniqueStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func (s *Service) ListExperimentBatches(limit int) ([]domain.ExperimentBatch, error) {
	return s.Repo.ListExperimentBatches(limit)
}

func (s *Service) GetExperimentBatch(id string) (domain.ExperimentBatch, error) {
	return s.Repo.GetExperimentBatch(strings.TrimSpace(id))
}

func (s *Service) runExperimentBatch(ctx context.Context, id string) {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()

	v, err := s.Repo.GetExperimentBatch(id)
	if err != nil {
		return
	}
	v.Status = "running"
	if err = s.Repo.UpdateExperimentBatch(v); err != nil {
		return
	}
	for trial := 1; trial <= v.Repeats; trial++ {
		for _, caseID := range v.CaseIDs {
			faultCase, caseErr := s.GetFaultCase(caseID)
			for _, config := range v.Configs {
				if caseErr != nil {
					v.FailedRuns++
					v.Error = caseErr.Error()
				} else if _, runErr := s.executeReplay(ctx, faultCase, config, v.ID, trial, v.CreatedBy); runErr != nil {
					v.FailedRuns++
					v.Error = runErr.Error()
				}
				v.CompletedRuns++
				if err = s.Repo.UpdateExperimentBatch(v); err != nil {
					return
				}
			}
		}
	}
	v.Status = "completed"
	v.CompletedAt = time.Now()
	if v.CompletedRuns == v.FailedRuns {
		v.Status = "failed"
	}
	_ = s.Repo.UpdateExperimentBatch(v)
	status := "success"
	if v.FailedRuns > 0 {
		status = "error"
	}
	s.addAudit(ctx, "run_experiment_batch", "", status, 0)
}
