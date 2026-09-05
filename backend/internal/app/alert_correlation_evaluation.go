package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

const maxCorrelationLabels = 200

func (s *Service) SaveAlertCorrelationLabels(ctx context.Context, req domain.AlertCorrelationLabelRequest) ([]domain.AlertCorrelationLabel, error) {
	if len(req.Items) == 0 || len(req.Items) > maxCorrelationLabels {
		return nil, fmt.Errorf("关联标注数量必须在 1 到 %d 之间", maxCorrelationLabels)
	}
	_, username, _, _ := actorFromContext(ctx)
	now := time.Now()
	upserts := make([]domain.AlertCorrelationLabel, 0, len(req.Items))
	deletes := make([]string, 0)
	seen := make(map[string]bool, len(req.Items))
	productID := ""
	for _, item := range req.Items {
		eventID := strings.TrimSpace(item.EventID)
		faultKey := strings.TrimSpace(item.FaultKey)
		note := strings.TrimSpace(item.Note)
		if eventID == "" || seen[eventID] {
			return nil, fmt.Errorf("event_id 不能为空且不能重复")
		}
		seen[eventID] = true
		event, err := s.Repo.GetAlertEvent(eventID)
		if err != nil {
			return nil, fmt.Errorf("告警事件不存在: %s", eventID)
		}
		if event.Status != "open" {
			return nil, fmt.Errorf("只能标注开放事件: %s", eventID)
		}
		if productID == "" {
			productID = event.ProductID
		} else if !strings.EqualFold(productID, event.ProductID) {
			return nil, fmt.Errorf("一次只能标注同一产品的事件")
		}
		if len([]rune(faultKey)) > 128 || len([]rune(note)) > 512 {
			return nil, fmt.Errorf("故障组不能超过 128 字，说明不能超过 512 字")
		}
		if faultKey == "" {
			deletes = append(deletes, eventID)
			continue
		}
		upserts = append(upserts, domain.AlertCorrelationLabel{
			EventID: eventID, ProductID: event.ProductID, FaultKey: faultKey,
			Note: note, LabeledBy: username, UpdatedAt: now,
		})
	}
	if err := s.Repo.SaveAlertCorrelationLabels(upserts, deletes); err != nil {
		s.addAudit(ctx, "label_alert_correlation", productID, "error", 0)
		return nil, err
	}
	s.addAudit(ctx, "label_alert_correlation", productID, "success", 0)
	return s.Repo.ListAlertCorrelationLabels(productID)
}

func (s *Service) EvaluateAlertCorrelation(productID string) (domain.AlertCorrelationEvaluation, []domain.AlertCorrelationLabel, error) {
	incidents, _, err := s.CorrelateAlertEvents(productID)
	if err != nil {
		return domain.AlertCorrelationEvaluation{}, nil, err
	}
	labels, err := s.Repo.ListAlertCorrelationLabels(strings.TrimSpace(productID))
	if err != nil {
		return domain.AlertCorrelationEvaluation{}, nil, err
	}
	predicted := make(map[string]string)
	events := make(map[string]domain.AlertEvent)
	for _, incident := range incidents {
		for _, event := range incident.Events {
			predicted[event.ID] = incident.ID
			events[event.ID] = event
		}
	}
	result := domain.AlertCorrelationEvaluation{EligibleEventCount: len(events)}
	activeLabels := make([]domain.AlertCorrelationLabel, 0, len(labels))
	for _, label := range labels {
		if _, ok := events[label.EventID]; ok && label.FaultKey != "" {
			activeLabels = append(activeLabels, label)
		}
	}
	result.LabeledEventCount = len(activeLabels)
	result.Coverage = ratio(result.LabeledEventCount, result.EligibleEventCount)
	for i := 0; i < len(activeLabels); i++ {
		for j := i + 1; j < len(activeLabels); j++ {
			left, right := activeLabels[i], activeLabels[j]
			if !strings.EqualFold(left.ProductID, right.ProductID) {
				continue
			}
			result.EvaluatedPairCount++
			truthSame := strings.EqualFold(left.FaultKey, right.FaultKey)
			predictedSame := predicted[left.EventID] == predicted[right.EventID]
			switch {
			case truthSame && predictedSame:
				result.TruePositive++
			case !truthSame && predictedSame:
				result.FalsePositive++
			case truthSame && !predictedSame:
				result.FalseNegative++
			default:
				result.TrueNegative++
			}
		}
	}
	result.PairPrecision = ratio(result.TruePositive, result.TruePositive+result.FalsePositive)
	result.PairRecall = ratio(result.TruePositive, result.TruePositive+result.FalseNegative)
	result.PairF1 = harmonicMean(result.PairPrecision, result.PairRecall)
	result.PairAccuracy = ratio(result.TruePositive+result.TrueNegative, result.EvaluatedPairCount)
	result.FalseLinkRate = ratio(result.FalsePositive, result.TruePositive+result.FalsePositive)
	result.MissedLinkRate = ratio(result.FalseNegative, result.TruePositive+result.FalseNegative)
	return result, activeLabels, nil
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func harmonicMean(left, right float64) float64 {
	if left+right == 0 {
		return 0
	}
	return 2 * left * right / (left + right)
}
