package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aiops-mvp/internal/domain"
)

const maxAlertEvents = 1000

func (s *Service) IngestAlertEvents(ctx context.Context, inputs []domain.AlertEventInput, source string) (domain.AlertIngestResult, error) {
	result := domain.AlertIngestResult{Received: len(inputs), Items: make([]domain.AlertEvent, 0, len(inputs))}
	if len(inputs) == 0 {
		return result, fmt.Errorf("告警载荷中没有可处理的事件")
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "manual"
	}

	prepared := make([]domain.AlertEvent, 0, len(inputs))
	for _, input := range inputs {
		v, err := normalizeAlertEvent(input, source)
		if err != nil {
			return result, err
		}
		prepared = append(prepared, v)
	}

	for _, v := range prepared {
		stored, created, err := s.Repo.UpsertAlertEvent(v)
		if err != nil {
			s.addAudit(ctx, "ingest_alert_event", v.ProductID, "error", 0)
			return result, err
		}
		if created {
			result.Created++
		} else {
			result.Merged++
		}
		result.Items = append(result.Items, stored)
	}
	s.addAudit(ctx, "ingest_alert_events", "", "success", 0)
	return result, nil
}

func (s *Service) SyncAlertEvents(ctx context.Context) (domain.AlertIngestResult, error) {
	alerts, err := s.Tools.Alerts("")
	if err != nil {
		s.addAudit(ctx, "sync_alert_events", "", "error", 0)
		return domain.AlertIngestResult{}, err
	}
	inputs := make([]domain.AlertEventInput, 0, len(alerts))
	for _, a := range alerts {
		inputs = append(inputs, domain.AlertEventInput{
			ExternalID: a.ID,
			ProductID:  a.ProductID,
			Rule:       a.Rule,
			Severity:   a.Severity,
			Target:     a.Target,
			Value:      a.Value,
			Status:     "open",
			OccurredAt: a.Triggered,
		})
	}
	result, err := s.IngestAlertEvents(ctx, inputs, s.Tools.AlertProviderName())
	if err == nil {
		s.addAudit(ctx, "sync_alert_events", "", "success", 0)
	}
	return result, err
}

func (s *Service) ListAlertEvents(status, productID string, limit int) ([]domain.AlertEvent, domain.AlertEventMetrics, error) {
	status = normalizeAlertStatus(status)
	if status != "" && status != "open" && status != "resolved" {
		return nil, domain.AlertEventMetrics{}, fmt.Errorf("status must be open or resolved")
	}
	all, err := s.Repo.ListAlertEvents(status, strings.TrimSpace(productID), maxAlertEvents)
	if err != nil {
		return nil, domain.AlertEventMetrics{}, err
	}
	metrics := alertEventMetrics(all)
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, metrics, nil
}

func (s *Service) ResolveAlertEvent(ctx context.Context, id string) (domain.AlertEvent, error) {
	v, err := s.Repo.GetAlertEvent(strings.TrimSpace(id))
	if err != nil {
		return domain.AlertEvent{}, err
	}
	v.Status = "resolved"
	if err := s.Repo.UpdateAlertEvent(v); err != nil {
		s.addAudit(ctx, "resolve_alert_event", v.ProductID, "error", 0)
		return domain.AlertEvent{}, err
	}
	s.addAudit(ctx, "resolve_alert_event", v.ProductID, "success", 0)
	return v, nil
}

func (s *Service) ReopenAlertEvent(ctx context.Context, id string) (domain.AlertEvent, error) {
	v, err := s.Repo.GetAlertEvent(strings.TrimSpace(id))
	if err != nil {
		return domain.AlertEvent{}, err
	}
	v.Status = "open"
	if err := s.Repo.UpdateAlertEvent(v); err != nil {
		s.addAudit(ctx, "reopen_alert_event", v.ProductID, "error", 0)
		return domain.AlertEvent{}, err
	}
	s.addAudit(ctx, "reopen_alert_event", v.ProductID, "success", 0)
	return v, nil
}

func (s *Service) DiagnoseAlertEvent(ctx context.Context, id string) (domain.AlertEvent, domain.DiagnosisResult, error) {
	v, err := s.Repo.GetAlertEvent(strings.TrimSpace(id))
	if err != nil {
		return domain.AlertEvent{}, domain.DiagnosisResult{}, err
	}
	question := fmt.Sprintf("分析告警事件：规则=%s，目标=%s，当前值=%s，已聚合%d次。请给出根因、证据和处置建议。", v.Rule, v.Target, v.Value, v.Occurrences)
	result := s.Diagnose(ctx, domain.DiagnosisRequest{
		ProductID:    v.ProductID,
		Question:     question,
		WindowMinute: 30,
		SeedEvidence: []domain.Evidence{{
			Type: "alert", Title: v.Rule,
			Content: fmt.Sprintf("product=%s target=%s value=%s status=%s occurrences=%d first_seen=%s last_seen=%s", v.ProductID, v.Target, v.Value, v.Status, v.Occurrences, v.FirstSeenAt.Format(time.RFC3339), v.LastSeenAt.Format(time.RFC3339)),
			Score:   1, Source: "alert-event/" + v.ID,
		}},
		SeedAlerts: []domain.Alert{{ID: v.ID, ProductID: v.ProductID, Rule: v.Rule, Severity: v.Severity, Target: v.Target, Value: v.Value, Triggered: v.LastSeenAt}},
	})
	v.DiagnosisSummary = result.Summary
	v.DiagnosisConfidence = result.Confidence
	v.DiagnosedAt = time.Now()
	if err := s.Repo.UpdateAlertEvent(v); err != nil {
		s.addAudit(ctx, "diagnose_alert_event", v.ProductID, "error", 0)
		return domain.AlertEvent{}, result, err
	}
	s.addAudit(ctx, "diagnose_alert_event", v.ProductID, "success", 0)
	return v, result, nil
}

func normalizeAlertEvent(input domain.AlertEventInput, source string) (domain.AlertEvent, error) {
	input.ProductID = strings.TrimSpace(input.ProductID)
	input.Rule = strings.TrimSpace(input.Rule)
	input.Target = strings.TrimSpace(input.Target)
	if input.ProductID == "" {
		input.ProductID = "unassigned"
	}
	if input.Rule == "" || input.Target == "" {
		return domain.AlertEvent{}, fmt.Errorf("告警规则和目标不能为空")
	}
	if input.Severity < 1 || input.Severity > 4 {
		input.Severity = 3
	}
	status := normalizeAlertStatus(input.Status)
	if status == "" {
		status = "open"
	}
	if status != "open" && status != "resolved" {
		return domain.AlertEvent{}, fmt.Errorf("不支持的告警状态: %s", input.Status)
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now()
	}
	fingerprint := alertFingerprint(source, input.ProductID, input.Rule, input.Target)
	return domain.AlertEvent{
		ID:          newAlertEventID(),
		Fingerprint: fingerprint,
		ExternalID:  strings.TrimSpace(input.ExternalID),
		ProductID:   input.ProductID,
		Rule:        input.Rule,
		Severity:    input.Severity,
		Target:      input.Target,
		Value:       strings.TrimSpace(input.Value),
		Status:      status,
		Source:      source,
		Occurrences: 1,
		FirstSeenAt: input.OccurredAt,
		LastSeenAt:  input.OccurredAt,
	}, nil
}

func alertFingerprint(parts ...string) string {
	normalized := make([]string, len(parts))
	for i, part := range parts {
		normalized[i] = strings.ToLower(strings.TrimSpace(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return hex.EncodeToString(sum[:])
}

func newAlertEventID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return "AEV-" + hex.EncodeToString(b)
	}
	return fmt.Sprintf("AEV-%d", time.Now().UnixNano())
}

func normalizeAlertStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "open", "firing", "triggered", "active", "alerting":
		return "open"
	case "resolved", "recovered", "recovery", "closed", "ok":
		return "resolved"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func alertEventMetrics(items []domain.AlertEvent) domain.AlertEventMetrics {
	m := domain.AlertEventMetrics{EventCount: len(items)}
	for _, v := range items {
		m.RawSignals += v.Occurrences
		if v.Status == "resolved" {
			m.ResolvedCount++
		} else {
			m.OpenCount++
		}
	}
	if m.RawSignals > 0 {
		m.ReductionRate = float64(m.RawSignals-m.EventCount) / float64(m.RawSignals)
	}
	return m
}

// ParseNightingaleAlertPayload accepts raw events and the common event/events wrappers.
func ParseNightingaleAlertPayload(data []byte) ([]domain.AlertEventInput, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("Nightingale Webhook JSON 无效: %w", err)
	}
	maps := flattenAlertPayload(root)
	if len(maps) == 0 {
		return nil, fmt.Errorf("Nightingale Webhook 中没有事件")
	}
	out := make([]domain.AlertEventInput, 0, len(maps))
	for _, item := range maps {
		input := domain.AlertEventInput{
			ExternalID: firstString(item, "id", "event_id"),
			ProductID:  firstString(item, "product_id", "group_name", "group"),
			Rule:       firstString(item, "rule_name", "rule", "rule_id"),
			Severity:   severityValue(firstValue(item, "severity", "priority")),
			Target:     firstString(item, "target_ident", "target", "ident"),
			Value:      firstString(item, "trigger_value", "value"),
			Status:     firstString(item, "status", "state"),
			OccurredAt: timeValue(firstValue(item, "trigger_time", "event_time", "timestamp", "created_at")),
		}
		if input.ProductID == "" {
			input.ProductID = productFromTags(firstValue(item, "tags", "tag_map"))
		}
		if boolValue(firstValue(item, "is_recovered", "recovered")) || positiveNumber(firstValue(item, "recover_time")) {
			input.Status = "resolved"
		}
		if input.Status == "" {
			input.Status = "open"
		}
		out = append(out, input)
	}
	return out, nil
}

func flattenAlertPayload(v any) []map[string]any {
	switch value := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			out = append(out, flattenAlertPayload(item)...)
		}
		return out
	case map[string]any:
		for _, key := range []string{"events", "event"} {
			if nested, ok := value[key]; ok {
				return flattenAlertPayload(nested)
			}
		}
		return []map[string]any{value}
	default:
		return nil
	}
}

func firstValue(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := item[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstString(item map[string]any, keys ...string) string {
	return stringValue(firstValue(item, keys...))
}

func stringValue(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	default:
		return ""
	}
}

func severityValue(v any) int {
	text := strings.ToLower(stringValue(v))
	if n, err := strconv.Atoi(text); err == nil {
		return n
	}
	switch text {
	case "critical", "p1", "emergency":
		return 1
	case "warning", "warn", "p2":
		return 2
	case "info", "p3":
		return 3
	default:
		return 3
	}
}

func timeValue(v any) time.Time {
	text := stringValue(v)
	if text == "" {
		return time.Time{}
	}
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		if n > 1_000_000_000_000 {
			return time.UnixMilli(n)
		}
		return time.Unix(n, 0)
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func boolValue(v any) bool {
	switch value := v.(type) {
	case bool:
		return value
	case json.Number:
		return value.String() != "0"
	case string:
		value = strings.ToLower(strings.TrimSpace(value))
		return value == "true" || value == "1" || value == "yes"
	default:
		return false
	}
}

func positiveNumber(v any) bool {
	n, err := strconv.ParseFloat(stringValue(v), 64)
	return err == nil && n > 0
}

func productFromTags(v any) string {
	if tags, ok := v.(map[string]any); ok {
		return firstString(tags, "product_id", "product", "group_name")
	}
	values := make([]string, 0)
	switch tags := v.(type) {
	case []any:
		for _, tag := range tags {
			if m, ok := tag.(map[string]any); ok {
				if product := firstString(m, "product_id", "product", "group_name"); product != "" {
					return product
				}
			} else {
				values = append(values, stringValue(tag))
			}
		}
	case string:
		values = strings.FieldsFunc(tags, func(r rune) bool { return r == ',' || r == ';' || unicode.IsSpace(r) })
	}
	for _, tag := range values {
		parts := strings.FieldsFunc(tag, func(r rune) bool { return r == '=' || r == ':' })
		if len(parts) == 2 && (strings.EqualFold(parts[0], "product_id") || strings.EqualFold(parts[0], "product")) {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}
