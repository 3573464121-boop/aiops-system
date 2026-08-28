package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

const (
	minInspectInterval = 30  // 巡检最短周期（秒），防止过于频繁地触发大模型
	defaultInspectSecs = 300 // 默认周期：5 分钟
	scanInterval       = 15 * time.Second
	runTimeout         = 90 * time.Second
)

// CreateInspectionTask 新建一条巡检任务。周期过小会被抬到下限；问题留空时给一个通用健康巡检提示。
func (s *Service) CreateInspectionTask(req domain.InspectionTaskRequest) (domain.InspectionTask, error) {
	q := strings.TrimSpace(req.Question)
	if q == "" {
		q = "对该产品做一次例行巡检：检查当前是否存在活跃告警或异常，如有请给出根因与处置建议。"
	}
	interval := req.IntervalSec
	if interval <= 0 {
		interval = defaultInspectSecs
	}
	if interval < minInspectInterval {
		interval = minInspectInterval
	}
	t := domain.InspectionTask{
		ID:          fmt.Sprintf("INS-%d", time.Now().UnixNano()),
		ProductID:   strings.TrimSpace(req.ProductID),
		Question:    q,
		IntervalSec: interval,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}
	if err := s.Repo.CreateInspectionTask(t); err != nil {
		return domain.InspectionTask{}, err
	}
	return t, nil
}

func (s *Service) ListInspectionTasks() ([]domain.InspectionTask, error) {
	return s.Repo.ListInspectionTasks()
}

// ToggleInspectionTask 开关一条巡检任务。
func (s *Service) ToggleInspectionTask(id string, enabled bool) error {
	t, err := s.Repo.GetInspectionTask(id)
	if err != nil {
		return err
	}
	t.Enabled = enabled
	return s.Repo.UpdateInspectionTask(t)
}

func (s *Service) DeleteInspectionTask(id string) error {
	return s.Repo.DeleteInspectionTask(id)
}

func (s *Service) ListInspectionReports(taskID string, limit int) ([]domain.InspectionReport, error) {
	return s.Repo.ListInspectionReports(taskID, limit)
}

// RunInspectionNow 手动立即触发一次巡检并返回生成的报告。
func (s *Service) RunInspectionNow(id string) (domain.InspectionReport, error) {
	t, err := s.Repo.GetInspectionTask(id)
	if err != nil {
		return domain.InspectionReport{}, err
	}
	return s.runInspection(context.Background(), t), nil
}

// runInspection 跑一次诊断，沉淀成巡检报告，并回写任务的上次运行状态。整体串行执行。
func (s *Service) runInspection(ctx context.Context, t domain.InspectionTask) domain.InspectionReport {
	s.inspectMu.Lock()
	defer s.inspectMu.Unlock()

	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	res := s.Diagnose(runCtx, domain.DiagnosisRequest{ProductID: t.ProductID, Question: t.Question})
	risk := riskLevel(res)
	report := domain.InspectionReport{
		ID:         fmt.Sprintf("INR-%d", time.Now().UnixNano()),
		TaskID:     t.ID,
		ProductID:  t.ProductID,
		Question:   t.Question,
		Summary:    res.Summary,
		Confidence: res.Confidence,
		Risk:       risk,
		DurationMS: time.Since(started).Milliseconds(),
		CreatedAt:  time.Now(),
	}
	if err := s.Repo.AddInspectionReport(report); err != nil {
		log.Printf("巡检报告写入失败 task=%s: %v", t.ID, err)
	}

	t.LastRunAt = report.CreatedAt
	t.LastStatus = risk
	if err := s.Repo.UpdateInspectionTask(t); err != nil {
		log.Printf("巡检任务状态回写失败 task=%s: %v", t.ID, err)
	}
	s.addAudit("inspection", t.ProductID, "success", report.DurationMS)
	return report
}

// riskLevel 从诊断结果推断巡检风险等级：有高风险处置动作=high，有活跃告警=warn，否则=ok。
func riskLevel(r domain.DiagnosisResult) string {
	for _, a := range r.Actions {
		if strings.EqualFold(a.Risk, "high") {
			return "high"
		}
	}
	if len(r.Alerts) > 0 {
		return "warn"
	}
	return "ok"
}

// StartInspectionScheduler 启动进程内调度器：每 scanInterval 扫描一次，跑到期的启用任务。ctx 取消时退出。
func (s *Service) StartInspectionScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(scanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.scanAndRun(ctx)
			}
		}
	}()
}

func (s *Service) scanAndRun(ctx context.Context) {
	tasks, err := s.Repo.ListInspectionTasks()
	if err != nil {
		log.Printf("巡检调度扫描失败: %v", err)
		return
	}
	now := time.Now()
	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		due := t.LastRunAt.IsZero() || now.Sub(t.LastRunAt) >= time.Duration(t.IntervalSec)*time.Second
		if !due {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.runInspection(ctx, t)
	}
}
