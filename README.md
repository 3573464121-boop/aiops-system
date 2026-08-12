# 智能运维（AIOps）诊断助手

面向告警 / 故障，由一个**只读、有证据、可审计**的 AI Agent 自主调用工具收集证据（告警 / 日志 / 知识库），**流式**产出带根因假设、处置建议（含风险分级与审批）、证据链的结构化诊断，并可一键沉淀为工单。

> 设计原则：**先做到「有证据地辅助判断」，再谈「自动处置」。**

---

## ✨ 核心特性

- **真正的 LLM Agent 工具调用循环** —— 模型自主决定调用哪些工具、最多 5 轮取证，全程记录调用轨迹（非固定流程 / 非套壳问答）
- **流式诊断（SSE）** —— 推理过程（每一步工具调用与结果）实时推送到前端
- **RAG 知识检索** —— Markdown 自动分块 + BM25，回答携带引用来源（向量 + RRF 规划中）
- **可插拔数据源** —— `Provider` 接口隔离；内置 **夜莺(Nightingale) 告警适配器**与 **Loki 日志适配器**，通过环境变量即插即用，未配置时使用内置演示数据；可无缝扩展 Elasticsearch / ClickHouse
- **可审计 & 安全** —— 只读工具、工具调用轨迹、证据溯源、**高风险动作强制「需审批」**、操作审计日志
- **多模型** —— OpenAI 兼容接口，默认 **DeepSeek**，可切本机 Ollama
- **控制台** —— Vue 3 + Ant Design 多页界面：仪表盘 / 运维助手对话 / 告警管理 / 知识库 / 问题工单 / 审计日志
- **持久化** —— 工单与审计写入 MySQL，不可用时自动降级内存
- **测试** —— 覆盖 Agent 工具循环、流式事件、适配器映射、知识检索、HTTP 接口

## 🏗️ 架构

```
用户提问 ──► 运维助手对话页(Vue3+Antd, SSE)
                     │
                     ▼
         Agent 工具调用循环 (Go)  ──► get_alerts / search_logs / search_knowledge
                     │  (≤5 轮，模型自主编排、逐步取证、全程审计)
                     ▼
         LLM (DeepSeek, OpenAI 兼容) 基于证据产出结构化诊断
                     │
                     ▼
   根因假设 · 处置建议(风险分级/审批) · 证据链 · 一键工单(MySQL)
```

```
backend/
├─ cmd/server/main.go          # 启动、按 env 选择数据源与存储
└─ internal/
   ├─ app/        # 诊断编排：Agent 工具循环 / 流式 / 工单 / 审计
   ├─ domain/     # 领域模型
   ├─ httpapi/    # Gin 路由（含 SSE 流式接口）
   ├─ knowledge/  # Markdown 分块 + BM25
   ├─ llm/        # OpenAI 兼容客户端（Chat + 工具调用 + 强制 JSON）
   ├─ storage/    # Repository：内存 / MySQL
   └─ tools/      # Provider 接口、夜莺/Loki 适配器、演示数据
frontend/
└─ src/{App.vue(导航), router.js, api.js, views/*}
```

## 🧰 技术栈

后端 **Go + Gin** ｜ 前端 **Vue 3 + Ant Design Vue + Vite** ｜ LLM **OpenAI 兼容（DeepSeek / Ollama）** ｜ 存储 **MySQL** ｜ 检索 **BM25（内存）** ｜ 流式 **SSE**

## 🚀 快速开始

### 后端

```bash
cd backend
cp .env.example .env      # 填入你自己的 DeepSeek API Key（见下）
go run ./cmd/server        # 默认 http://localhost:8080
```

### 前端

```bash
cd frontend
npm install
npm run dev                # 默认 http://localhost:5173
```

### 配置（`backend/.env`）

| 变量 | 说明 |
|---|---|
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | 大模型。DeepSeek：`https://api.deepseek.com/v1` + `deepseek-chat`（需到 platform.deepseek.com 自备 key） |
| `N9E_BASE_URL` / `N9E_TOKEN` | 夜莺告警源。留空 = 演示数据 |
| `LOG_BASE_URL` / `LOG_TOKEN` | Loki 日志源。留空 = 演示数据 |
| `MYSQL_DSN` | 工单/审计持久化。留空 = 内存 |

> ⚠️ `.env` 含密钥，已被 `.gitignore` 忽略，**请勿提交**。

## 📡 主要 API

```
GET  /healthz
GET  /api/v1/system/status
GET  /api/v1/alerts/active?product_id=payment
GET  /api/v1/logs/search?product_id=payment&query=timeout
GET  /api/v1/knowledge/search?query=告警检索
POST /api/v1/diagnoses            # 一次性诊断
POST /api/v1/diagnoses/stream     # 流式诊断（SSE）
GET  /api/v1/issues  ·  POST /api/v1/issues
GET  /api/v1/audits
```

## 🗺️ 路线图

- [x] LLM Agent 工具调用循环、SSE 流式、多页控制台
- [x] 夜莺 / Loki 数据源适配器（即插即用）
- [ ] 向量检索 + RRF（真 RAG）
- [ ] 评估数据集 + 指标（根因命中率 / 证据引用正确率 / 幻觉率）+ 对照实验
- [ ] 认证 / 多租户、记忆空间、主动巡检、工作流

## 📄 许可

暂定（如需开源建议 MIT；如作商业用途请保留版权）。
