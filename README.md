# 智能运维诊断助手（AIOps）

一个基于大语言模型的运维故障诊断系统。给定一条告警或故障描述，系统会自主调用一组只读工具（查询告警、检索日志、检索知识库、查询资产、召回历史记忆）收集证据，再由模型综合出带根因分析、处置建议和证据链的结构化诊断，推理过程可流式展示，诊断结果可一键沉淀为工单。除人工提问外，还可配置主动巡检定时自动诊断。

设计上坚持一个原则：先给出有证据支撑的辅助判断，再谈自动处置。因此所有工具都是只读的，回滚、重启、扩容等高风险操作只会被标记为“需人工审批”，不会自动执行。

## 功能

- 工具调用循环。模型自主决定调用哪些工具、最多五轮，逐步取证，全过程记录调用轨迹。
- 流式诊断。通过 SSE 把每一步工具调用与结果实时推送到前端，而不是等待整段结果返回。
- 知识检索。Markdown 文档按标题分块，BM25 关键词检索与向量检索（bge-m3）经 RRF 融合，回答附带引用来源。
- 资产管理（CMDB）。服务器、数据库等资产按产品归档，支持 IP 反查；诊断时作为 `get_assets` / `lookup_ip` 工具供模型取证。
- 主动巡检。为产品配置定时巡检任务，进程内调度器按周期自动跑诊断并沉淀巡检报告，按风险分级（正常/关注/高风险）标记。
- 记忆空间。跨对话沉淀长期经验与环境事实，诊断时按相关度自动召回并作为背景注入，模型也可主动调用 `recall_memory`；支持从诊断结论用模型提炼记忆草稿。
- 认证与权限。用户名密码登录，bcrypt 存哈希，HMAC 签名令牌鉴权；分管理员与只读两种角色，管理类操作（配置、增删）仅管理员可执行。
- 可插拔数据源。告警与日志通过 Provider 接口接入，内置夜莺（Nightingale）与 Loki 适配器，未配置时使用内置演示数据；可扩展 Elasticsearch、ClickHouse 等。
- 数据源管理。统一展示告警、日志、知识、资产、模型与存储的运行模式、脱敏地址和连接检测结果。
- 实验留档。每次诊断自动记录配置、工具、证据来源和运行指标；人工复核标准根因后才能纳入并导出论文数据集。
- 可审计。工具调用轨迹、证据来源和操作人身份均写入审计日志；高风险动作通过审批中心完成人工批准、驳回和执行确认。
- 多模型。OpenAI 兼容接口，默认对接 DeepSeek，也可切换到本机 Ollama。
- 控制台。Vue 3 + Ant Design，包含登录、仪表盘、运维助手、告警、知识库、工单、审计、记忆、巡检、资产、审批、数据源、实验记录和故障回放等页面。
- 故障回放。导入版本化、脱敏的告警、日志与资产快照，使用完整系统、仅 BM25 和无 Agent 三组配置回放，持久化诊断结果与判官指标。
- 独立判官。回放支持通过 `JUDGE_BASE_URL`、`JUDGE_API_KEY` 和 `JUDGE_MODEL` 配置独立评审模型，并记录判官来源；未配置时明确标记为自评。
- 实验质量控制。自动标记判官指标冲突，只有经人工复核采纳的回放结果才能从页面导出为 JSON 或 CSV。
- 实验批次。可在页面选择多个案例、对照配置和重复次数，后台串行执行并持久化进度、失败数、模型与判官快照。
- 持久化。工单、审计、巡检、记忆、用户、审批单、诊断实验记录和回放结果均写入 MySQL，不可用时自动回退内存。

## 架构

```
运维助手对话页 (Vue 3 + Ant Design, SSE)  ←  登录鉴权(角色: 管理员/只读)
        │
        ▼
Agent 工具调用循环 (Go)  →  get_alerts / search_logs / search_knowledge
        │  最多 5 轮，模型自主编排、逐步取证、记录轨迹     get_assets / lookup_ip / recall_memory
        ▼
LLM (OpenAI 兼容) 基于收集到的证据 + 召回的长期记忆产出结构化诊断
        │
        ▼
根因分析 · 处置建议(含风险分级/审批) · 证据链 · 一键工单 (MySQL)
        ▲
        │  主动巡检调度器 (进程内定时) 按周期自动触发诊断并沉淀巡检报告
```

```
backend/
  cmd/server/        服务入口，按环境变量选择数据源与存储
  cmd/eval/          诊断质量评测程序
  cmd/casegen/       将评测集转换为可批量导入的回放案例
  internal/
    app/             诊断编排：工具循环、流式、审批、巡检、记忆、认证、实验留档
    auth/            HMAC 签名登录令牌的签发与校验
    domain/          领域模型
    httpapi/         HTTP 路由（含 SSE 流式接口、登录与角色鉴权中间件）
    knowledge/       Markdown 分块、BM25、向量检索、RRF
    llm/             OpenAI 兼容客户端（工具调用、限流退避重试）
    embed/           文本向量化客户端
    storage/         Repository：内存 / MySQL（业务数据与实验记录）
    tools/           Provider 接口、夜莺/Loki 适配器、资产源、演示数据
  knowledge-base/    历史故障复盘与运维手册
frontend/
  src/               App.vue（导航/登录态）、router（守卫）、api（带令牌）、views/（含故障回放）
docs/
  research-protocol.md  研究问题、实验设计与投稿前检查
```

## 技术栈

后端 Go + Gin，前端 Vue 3 + Ant Design Vue + Vite，模型走 OpenAI 兼容接口（DeepSeek 或 Ollama），向量检索用 bge-m3，持久化用 MySQL。

## 运行

后端：

```bash
cd backend
cp .env.example .env      # 按需填写，见下方“配置”
go run ./cmd/server        # 默认 http://localhost:8080
```

前端：

```bash
cd frontend
npm install
npm run dev                # 默认 http://localhost:5173
```

## 配置

编辑 `backend/.env`，各项均可留空（留空时走内置演示数据或内存存储）。

| 变量 | 说明 |
|---|---|
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | 诊断模型。DeepSeek 为 `https://api.deepseek.com/v1` + `deepseek-chat`，需自备 API Key |
| `EMBED_BASE_URL` / `EMBED_MODEL` | 向量检索嵌入模型，例如本机 Ollama 的 `bge-m3`；留空则只用 BM25 |
| `N9E_BASE_URL` / `N9E_TOKEN` | 夜莺告警源，留空用演示数据 |
| `LOG_BASE_URL` / `LOG_TOKEN` | Loki 日志源，留空用演示数据 |
| `MYSQL_DSN` | 业务数据和诊断实验记录的持久化，留空用内存 |
| `AUTH_SECRET` | 登录令牌签名密钥，生产环境务必改为随机串；留空用内置开发密钥并告警 |
| `AUTH_ADMIN_PASSWORD` | 首次启动无用户时自动创建的管理员 `admin` 初始口令，留空默认 `admin123` |

`.env` 含密钥，已在 `.gitignore` 中排除，不要提交。

**首次登录**：系统启动时若数据库中没有任何用户，会自动创建管理员账号 `admin`（口令取自 `AUTH_ADMIN_PASSWORD`，默认 `admin123`），初始口令也会打印在启动日志里。登录后管理员可在后台新增只读账号。

## 评估

`backend/cmd/eval` 是一个可复现的评测程序，在同一批带标准答案的故障案例上对比三种配置：完整的智能体（工具循环 + 向量 RAG）、仅用 BM25 的智能体、以及不调用任何工具的纯模型问答。10 个案例的一次结果：

回放平台的独立判官实验见 `backend/eval/replay-report-v1.md`；固定输入数据为 `backend/eval/replay-dataset-v1.json`。

| 配置 | 根因准确率 | 证据召回 | 知识命中 | 忠实度 | 幻觉率 |
|---|---|---|---|---|---|
| 智能体 + 向量 RAG | 100% | 100% | 100% | 0.93 | 0% |
| 智能体 + 仅 BM25 | 100% | 100% | 90% | 0.96 | 0% |
| 纯模型问答（无取证） | 0% | 0% | 0% | 0.00 | 100% |

其中根因准确率、忠实度、幻觉率由 LLM 判官评定。可以看到，接入证据的智能体在根因准确率和忠实度上明显优于无证据的纯模型问答，后者结论虽然听起来合理，却缺乏可核验的证据；向量检索相比纯关键词在语义化查询上略有优势。

需要说明其局限：当前判官与被测为同一模型，存在自评偏差；数据集为演示规模（10 例），结论仅作方法可行性的初步验证。更严谨的评测需要独立判官与更大、更真实的数据集。后续实验的纳入标准、消融配置和统计方法见 `docs/research-protocol.md`。

## 主要接口

除 `/healthz` 与 `/api/v1/auth/login` 外，所有 `/api/v1` 接口都需在请求头带 `Authorization: Bearer <token>`；标注「仅管理员」的还要求管理员角色。

```
GET  /healthz
POST /api/v1/auth/login           登录，换取令牌
GET  /api/v1/auth/me              当前登录用户
GET  /api/v1/system/status
GET  /api/v1/data-sources                         数据源状态
POST /api/v1/data-sources/:name/test              连接检测（仅管理员）
GET  /api/v1/alerts/active?product_id=payment
GET  /api/v1/logs/search?product_id=payment&query=timeout
GET  /api/v1/knowledge/search?query=告警检索
GET  /api/v1/assets?product_id=payment          资产列表
GET  /api/v1/assets/lookup?ip=10.0.0.1          IP 反查资产
POST /api/v1/diagnoses            一次性诊断
POST /api/v1/diagnoses/stream     流式诊断（SSE）
GET  /api/v1/issues   POST /api/v1/issues
GET  /api/v1/inspections          巡检任务列表
POST /api/v1/inspections                        新建巡检任务（仅管理员）
POST /api/v1/inspections/:id/toggle             启用/停用（仅管理员）
POST /api/v1/inspections/:id/run                立即巡检（仅管理员）
DELETE /api/v1/inspections/:id                  删除（仅管理员）
GET  /api/v1/inspection-reports?task_id=&limit= 巡检报告
GET  /api/v1/memories             记忆列表
POST /api/v1/memories                           新增记忆（仅管理员）
DELETE /api/v1/memories/:id                     删除记忆（仅管理员）
POST /api/v1/memories/extract                   从文本提炼记忆草稿（仅管理员）
GET  /api/v1/users   POST /api/v1/users         用户管理（仅管理员）
GET  /api/v1/approvals                         审批单列表，可按 status 筛选
POST /api/v1/approvals                         提交审批申请
POST /api/v1/approvals/:id/review              批准或驳回（仅管理员）
POST /api/v1/approvals/:id/execute             确认已人工执行（仅管理员）
POST /api/v1/approvals/:id/cancel              撤销待审批申请
GET  /api/v1/diagnosis-runs                    诊断实验记录（仅管理员）
POST /api/v1/diagnosis-runs/:id/review         复核并标注标准根因（仅管理员）
GET  /api/v1/fault-cases                       故障案例列表（仅管理员）
POST /api/v1/fault-cases                       导入故障案例（仅管理员）
POST /api/v1/fault-cases/bulk                  批量导入故障案例（仅管理员，最多 500 条）
GET  /api/v1/fault-cases/:id                   故障案例详情（仅管理员）
DELETE /api/v1/fault-cases/:id                 删除故障案例（仅管理员）
POST /api/v1/fault-cases/:id/replay            运行对照回放（仅管理员）
GET  /api/v1/replay-results?case_id=           回放结果（仅管理员）
POST /api/v1/replay-results/:id/review         人工复核回放结果（仅管理员）
GET  /api/v1/experiment-batches                实验批次与进度（仅管理员）
POST /api/v1/experiment-batches                创建并后台运行实验批次（仅管理员）
GET  /api/v1/audits
```

## 现状与后续

已实现：工具调用循环、流式诊断、多页控制台、夜莺/Loki 适配器、数据源检测、向量 RAG、评测程序、诊断实验留档与人工标注、故障案例导入、对照回放与实验批次、资产管理（CMDB）、主动巡检、记忆空间、认证与角色权限、操作人审计、高风险处置审批闭环。

计划中：

- 记忆的个人 / 团队作用域（当前支持全局 / 产品两级，个人 / 团队待接入用户身份后细分）；
- 真实处置执行器与审批策略（当前审批通过后仍由管理员线下执行并确认，不直接操作生产环境）；
- 更大规模、更真实的评测数据集与独立判官。

## 许可

暂未确定。如需引用或商用请先联系作者。
