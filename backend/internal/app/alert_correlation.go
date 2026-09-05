package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"aiops-mvp/internal/domain"
)

const (
	alertCorrelationVersion = "rule-v1"
	alertCorrelationWindow  = 20 * time.Minute
	alertCorrelationCutoff  = 0.65
)

type correlationLink struct {
	left    int
	right   int
	score   float64
	reasons []string
}

func (s *Service) CorrelateAlertEvents(productID string) ([]domain.AlertIncident, domain.AlertCorrelationMetrics, error) {
	events, err := s.Repo.ListAlertEvents("open", strings.TrimSpace(productID), maxAlertEvents)
	metrics := domain.AlertCorrelationMetrics{
		OpenEventCount:   len(events),
		AlgorithmVersion: alertCorrelationVersion,
		WindowMinutes:    int(alertCorrelationWindow / time.Minute),
		Threshold:        alertCorrelationCutoff,
	}
	if err != nil {
		return nil, metrics, err
	}
	if len(events) == 0 {
		return []domain.AlertIncident{}, metrics, nil
	}

	parents := make([]int, len(events))
	for i := range parents {
		parents[i] = i
	}
	find := func(index int) int { return index }
	var findRoot func(int) int
	findRoot = func(index int) int {
		if parents[index] != index {
			parents[index] = findRoot(parents[index])
		}
		return parents[index]
	}
	find = findRoot
	union := func(left, right int) {
		leftRoot, rightRoot := find(left), find(right)
		if leftRoot != rightRoot {
			parents[rightRoot] = leftRoot
		}
	}

	links := make([]correlationLink, 0)
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			metrics.PairComparisons++
			score, reasons := scoreAlertPair(events[i], events[j])
			if score < alertCorrelationCutoff {
				continue
			}
			metrics.LinkedPairs++
			links = append(links, correlationLink{left: i, right: j, score: score, reasons: reasons})
			union(i, j)
		}
	}

	groups := make(map[int][]domain.AlertEvent)
	reasonsByRoot := make(map[int]map[string]struct{})
	for i, event := range events {
		root := find(i)
		groups[root] = append(groups[root], event)
	}
	for _, link := range links {
		root := find(link.left)
		if reasonsByRoot[root] == nil {
			reasonsByRoot[root] = make(map[string]struct{})
		}
		for _, reason := range link.reasons {
			reasonsByRoot[root][reason] = struct{}{}
		}
	}

	incidents := make([]domain.AlertIncident, 0, len(groups))
	for root, group := range groups {
		reasons := sortedSet(reasonsByRoot[root])
		if len(group) == 1 {
			reasons = []string{"未发现达到关联阈值的其他事件"}
		}
		incident := buildAlertIncident(group, reasons)
		incidents = append(incidents, incident)
		if len(group) > 1 {
			metrics.CorrelatedEventCount += len(group)
		} else {
			metrics.SingletonCount++
		}
	}
	sort.Slice(incidents, func(i, j int) bool {
		if incidents[i].Severity != incidents[j].Severity {
			return incidents[i].Severity < incidents[j].Severity
		}
		return incidents[i].LastSeenAt.After(incidents[j].LastSeenAt)
	})
	metrics.IncidentCount = len(incidents)
	metrics.CompressionRate = float64(len(events)-len(incidents)) / float64(len(events))
	return incidents, metrics, nil
}

func (s *Service) DiagnoseAlertIncident(ctx context.Context, id, productID string) (domain.AlertIncident, domain.DiagnosisResult, error) {
	incidents, _, err := s.CorrelateAlertEvents(productID)
	if err != nil {
		return domain.AlertIncident{}, domain.DiagnosisResult{}, err
	}
	var incident *domain.AlertIncident
	for i := range incidents {
		if incidents[i].ID == strings.TrimSpace(id) {
			incident = &incidents[i]
			break
		}
	}
	if incident == nil {
		return domain.AlertIncident{}, domain.DiagnosisResult{}, fmt.Errorf("alert incident not found: %s", id)
	}

	evidence := make([]domain.Evidence, 0, len(incident.Events))
	alerts := make([]domain.Alert, 0, len(incident.Events))
	for _, event := range incident.Events {
		evidence = append(evidence, domain.Evidence{
			Type: "alert", Title: event.Rule,
			Content: fmt.Sprintf("product=%s target=%s value=%s occurrences=%d first_seen=%s last_seen=%s", event.ProductID, event.Target, event.Value, event.Occurrences, event.FirstSeenAt.Format(time.RFC3339), event.LastSeenAt.Format(time.RFC3339)),
			Score:   1, Source: "alert-event/" + event.ID,
		})
		alerts = append(alerts, domain.Alert{ID: event.ID, ProductID: event.ProductID, Rule: event.Rule, Severity: event.Severity, Target: event.Target, Value: event.Value, Triggered: event.LastSeenAt})
	}
	question := fmt.Sprintf("分析关联故障簇：%s。该故障簇包含 %d 个事件、%d 条原始信号，关联依据为%s。请结合全部事件给出共同根因、证据和处置顺序。", incident.Title, incident.EventCount, incident.SignalCount, strings.Join(incident.Reasons, "、"))
	result := s.Diagnose(ctx, domain.DiagnosisRequest{
		ProductID: incident.ProductID, Question: question,
		WindowMinute: int(alertCorrelationWindow / time.Minute),
		SeedEvidence: evidence, SeedAlerts: alerts,
	})
	s.addAudit(ctx, "diagnose_alert_incident", incident.ProductID, "success", 0)
	return *incident, result, nil
}

func scoreAlertPair(left, right domain.AlertEvent) (float64, []string) {
	if !strings.EqualFold(strings.TrimSpace(left.ProductID), strings.TrimSpace(right.ProductID)) {
		return 0, nil
	}
	delta := left.LastSeenAt.Sub(right.LastSeenAt)
	if delta < 0 {
		delta = -delta
	}
	if delta > alertCorrelationWindow {
		return 0, nil
	}
	score := 0.60
	reasons := []string{
		"同属产品 " + left.ProductID,
		fmt.Sprintf("最近出现时间相差不超过 %d 分钟", int(alertCorrelationWindow/time.Minute)),
	}
	if strings.EqualFold(strings.TrimSpace(left.Target), strings.TrimSpace(right.Target)) {
		score += 0.30
		reasons = append(reasons, "目标完全相同")
	} else if sharedMeaningfulToken(left.Target, right.Target) {
		score += 0.20
		reasons = append(reasons, "目标命名存在共同服务标识")
	}
	if sharedMeaningfulToken(left.Rule, right.Rule) {
		score += 0.15
		reasons = append(reasons, "告警规则存在共同关键词")
	}
	if score > 1 {
		score = 1
	}
	return score, reasons
}

func buildAlertIncident(events []domain.AlertEvent, reasons []string) domain.AlertIncident {
	ordered := append([]domain.AlertEvent(nil), events...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Severity != ordered[j].Severity {
			return ordered[i].Severity < ordered[j].Severity
		}
		return ordered[i].LastSeenAt.After(ordered[j].LastSeenAt)
	})
	incident := domain.AlertIncident{
		ID: incidentID(ordered), ProductID: ordered[0].ProductID,
		Severity: ordered[0].Severity, EventCount: len(ordered),
		FirstSeenAt: ordered[0].FirstSeenAt, LastSeenAt: ordered[0].LastSeenAt,
		Events: ordered, Reasons: reasons,
	}
	targets, rules := make(map[string]struct{}), make(map[string]struct{})
	for _, event := range ordered {
		incident.SignalCount += event.Occurrences
		if event.FirstSeenAt.Before(incident.FirstSeenAt) {
			incident.FirstSeenAt = event.FirstSeenAt
		}
		if event.LastSeenAt.After(incident.LastSeenAt) {
			incident.LastSeenAt = event.LastSeenAt
		}
		targets[event.Target] = struct{}{}
		rules[event.Rule] = struct{}{}
	}
	incident.Targets = sortedSet(targets)
	incident.Rules = sortedSet(rules)
	incident.Title = ordered[0].Rule
	if len(ordered) > 1 {
		incident.Title += fmt.Sprintf(" 等 %d 项告警", len(ordered))
	}
	return incident
}

func incidentID(events []domain.AlertEvent) string {
	keys := make([]string, 0, len(events))
	for _, event := range events {
		key := event.Fingerprint
		if key == "" {
			key = event.ID
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sum := sha256.Sum256([]byte(strings.Join(keys, "\x00")))
	return "INC-" + hex.EncodeToString(sum[:6])
}

func sharedMeaningfulToken(left, right string) bool {
	leftTokens := correlationTokens(left)
	for token := range correlationTokens(right) {
		if _, ok := leftTokens[token]; ok {
			return true
		}
	}
	return false
}

func correlationTokens(value string) map[string]struct{} {
	ignored := map[string]bool{"api": true, "service": true, "server": true, "node": true, "prod": true, "the": true}
	out := make(map[string]struct{})
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, part := range parts {
		if len([]rune(part)) < 2 || ignored[part] || allDigits(part) {
			continue
		}
		out[part] = struct{}{}
	}
	return out
}

func allDigits(value string) bool {
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return value != ""
}

func sortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
