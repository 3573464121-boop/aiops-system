package tools

import (
	"strings"

	"aiops-mvp/internal/domain"
)

type ReplayAlertProvider struct{ Items []domain.Alert }

func (ReplayAlertProvider) Name() string { return "replay" }
func (p ReplayAlertProvider) Alerts(productID string) ([]domain.Alert, error) {
	out := make([]domain.Alert, 0, len(p.Items))
	for _, v := range p.Items {
		if productID == "" || strings.EqualFold(v.ProductID, productID) {
			out = append(out, v)
		}
	}
	return out, nil
}

type ReplayLogProvider struct{ Items []domain.Evidence }

func (ReplayLogProvider) Name() string { return "replay" }
func (p ReplayLogProvider) Search(_, query string) ([]domain.Evidence, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]domain.Evidence(nil), p.Items...), nil
	}
	out := make([]domain.Evidence, 0, len(p.Items))
	for _, v := range p.Items {
		haystack := strings.ToLower(v.Title + " " + v.Content)
		if strings.Contains(haystack, query) || replayTokenOverlap(haystack, query) {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return append([]domain.Evidence(nil), p.Items...), nil
	}
	return out, nil
}

func replayTokenOverlap(text, query string) bool {
	for _, token := range strings.FieldsFunc(query, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && !(r >= '\u4e00' && r <= '\u9fff')
	}) {
		if len([]rune(token)) >= 2 && strings.Contains(text, token) {
			return true
		}
	}
	return false
}

type ReplayAssetProvider struct{ Items []domain.Asset }

func (ReplayAssetProvider) Name() string { return "replay" }
func (p ReplayAssetProvider) Assets(productID string) ([]domain.Asset, error) {
	out := make([]domain.Asset, 0, len(p.Items))
	for _, v := range p.Items {
		if productID == "" || strings.EqualFold(v.ProductID, productID) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (p ReplayAssetProvider) LookupIP(ip string) ([]domain.Asset, error) {
	out := make([]domain.Asset, 0)
	for _, v := range p.Items {
		if v.IP == strings.TrimSpace(ip) {
			out = append(out, v)
		}
	}
	return out, nil
}
