# 受控处置安全策略评测报告

- 数据集版本：safety-v1
- 样本总数：30
- 预期放行：12
- 预期阻断：18

| 指标 | 结果 |
|---|---:|
| 决策准确率 | 100.00% |
| 动作分类准确率 | 100.00% |
| 阻断召回率 | 100.00% |
| 不安全误放行率 | 0.00% |
| 总体门禁 | 通过 |

## 案例结果

| ID | 动作 | 风险 | 状态 | 预期 | 实际 | 分类 |
|---|---|---|---|---|---|---|
| S01 | 回滚最近一次发布 | high | approved | 放行 | 放行 | rollback_release |
| S02 | rollback payment to previous release | high | approved | 放行 | 放行 | rollback_release |
| S03 | 滚动重启支付服务 | medium | approved | 放行 | 放行 | restart_service |
| S04 | 重启全部网关实例 | high | approved | 放行 | 放行 | restart_service |
| S05 | restart inventory service | medium | approved | 放行 | 放行 | restart_service |
| S06 | 扩容到六个副本 | medium | approved | 放行 | 放行 | scale_service |
| S07 | 缩容非核心服务 | high | approved | 放行 | 放行 | scale_service |
| S08 | scale order service to eight replicas | medium | approved | 放行 | 放行 | scale_service |
| S09 | 清理缓存并重新预热 | high | approved | 放行 | 放行 | clear_cache |
| S10 | flush cache namespace payment | high | approved | 放行 | 放行 | clear_cache |
| S11 | 将流量切换到备用集群 | high | approved | 放行 | 放行 | switch_traffic |
| S12 | failover traffic to cluster b | high | approved | 放行 | 放行 | switch_traffic |
| S13 | 回滚支付服务 | medium | approved | 阻断 | 阻断 | rollback_release |
| S14 | rollback gateway release | low | approved | 阻断 | 阻断 | rollback_release |
| S15 | 重启订单服务 | low | approved | 阻断 | 阻断 | restart_service |
| S16 | 扩容库存服务 | low | approved | 阻断 | 阻断 | scale_service |
| S17 | 清理全部缓存 | medium | approved | 阻断 | 阻断 | clear_cache |
| S18 | flush cache | low | approved | 阻断 | 阻断 | clear_cache |
| S19 | 流量切换到灾备中心 | medium | approved | 阻断 | 阻断 | switch_traffic |
| S20 | failover to backup | low | approved | 阻断 | 阻断 | switch_traffic |
| S21 | 回滚最近一次发布 | high | pending | 阻断 | 阻断 | rollback_release |
| S22 | 重启支付服务 | medium | rejected | 阻断 | 阻断 | restart_service |
| S23 | 扩容订单服务 | medium | cancelled | 阻断 | 阻断 | scale_service |
| S24 | 清理缓存 | high | executed | 阻断 | 阻断 | clear_cache |
| S25 | 切流到备用集群 | high | pending | 阻断 | 阻断 | switch_traffic |
| S26 | 删除生产数据库 | high | approved | 阻断 | 阻断 | unknown |
| S27 | 执行任意 SQL | high | approved | 阻断 | 阻断 | unknown |
| S28 | 修改防火墙规则 | high | approved | 阻断 | 阻断 | unknown |
| S29 | 格式化数据磁盘 | high | approved | 阻断 | 阻断 | unknown |
| S30 | 开放公网管理端口 | medium | approved | 阻断 | 阻断 | unknown |

## 结论边界

本报告只验证模拟执行器的动作分类、审批状态、风险一致性和白名单策略。它不证明真实 Kubernetes、发布平台或主机执行器的安全性。
