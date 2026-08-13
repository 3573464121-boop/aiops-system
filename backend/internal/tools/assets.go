package tools

import (
	"strings"

	"aiops-mvp/internal/domain"
)

// AssetProvider 隔离 CMDB / 资产数据源，后续可替换为真实 CMDB 或夜莺目标列表。
type AssetProvider interface {
	Name() string
	Assets(productID string) ([]domain.Asset, error)
	LookupIP(ip string) ([]domain.Asset, error)
}

type DemoAssetProvider struct{}

func (DemoAssetProvider) Name() string { return "demo" }

// demoAssets 与演示告警/日志中的主机名保持一致（如 payment-api-01、inventory-03）。
var demoAssets = []domain.Asset{
	{ID: "srv-p01", ProductID: "payment", Kind: "server", Name: "payment-api-01", IP: "10.0.1.11", Detail: "支付接口服务", Env: "prod", Status: "online"},
	{ID: "srv-p02", ProductID: "payment", Kind: "server", Name: "payment-api-02", IP: "10.0.1.12", Detail: "支付接口服务", Env: "prod", Status: "online"},
	{ID: "db-p01", ProductID: "payment", Kind: "db", Name: "payment-db", IP: "10.0.1.30", Detail: "MySQL 主库 :3306", Env: "prod", Status: "online"},
	{ID: "srv-i01", ProductID: "inventory", Kind: "server", Name: "inventory-01", IP: "10.0.2.11", Detail: "库存服务", Env: "prod", Status: "online"},
	{ID: "srv-i03", ProductID: "inventory", Kind: "server", Name: "inventory-03", IP: "10.0.2.13", Detail: "库存服务", Env: "prod", Status: "offline"},
	{ID: "db-i01", ProductID: "inventory", Kind: "db", Name: "inventory-db", IP: "10.0.2.30", Detail: "MySQL :3306", Env: "prod", Status: "online"},
	{ID: "srv-o02", ProductID: "order", Kind: "server", Name: "order-api-02", IP: "10.0.3.12", Detail: "订单接口服务", Env: "prod", Status: "online"},
	{ID: "srv-oc01", ProductID: "order", Kind: "server", Name: "order-consumer-01", IP: "10.0.3.21", Detail: "订单消息消费者", Env: "prod", Status: "online"},
	{ID: "db-o01", ProductID: "order", Kind: "db", Name: "order-db", IP: "10.0.3.30", Detail: "MySQL :3306", Env: "prod", Status: "online"},
	{ID: "srv-g01", ProductID: "gateway", Kind: "server", Name: "gateway-01", IP: "10.0.0.11", Detail: "API 网关", Env: "prod", Status: "online"},
	{ID: "srv-g02", ProductID: "gateway", Kind: "server", Name: "gateway-02", IP: "10.0.0.12", Detail: "API 网关", Env: "prod", Status: "online"},
	{ID: "srv-u03", ProductID: "user", Kind: "server", Name: "user-api-03", IP: "10.0.4.13", Detail: "用户服务", Env: "prod", Status: "online"},
	{ID: "cache-u01", ProductID: "user", Kind: "db", Name: "session-redis", IP: "10.0.4.30", Detail: "Redis 会话缓存 :6379", Env: "prod", Status: "online"},
}

func (DemoAssetProvider) Assets(productID string) ([]domain.Asset, error) {
	out := make([]domain.Asset, 0)
	for _, a := range demoAssets {
		if productID == "" || strings.EqualFold(a.ProductID, productID) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (DemoAssetProvider) LookupIP(ip string) ([]domain.Asset, error) {
	ip = strings.TrimSpace(ip)
	out := make([]domain.Asset, 0)
	for _, a := range demoAssets {
		if a.IP == ip {
			out = append(out, a)
		}
	}
	return out, nil
}
