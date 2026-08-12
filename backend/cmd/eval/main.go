// 命令 eval：AIOps 诊断质量评测。
// 对同一批带“标准答案”的故障案例，在多种配置下运行诊断，计算指标并产出对照报告。
// 配置：
//   full     —— Agent 工具循环 + 向量 RAG（BM25+向量 RRF）
//   bm25     —— Agent 工具循环 + 仅 BM25 知识检索（关闭向量）
//   no-agent —— 纯 LLM 单轮作答，不调用任何工具（无证据基线）
// 指标：根因命中率、证据召回率、高风险审批标记正确率、平均置信度。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"aiops-mvp/internal/app"
	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/embed"
	"aiops-mvp/internal/knowledge"
	"aiops-mvp/internal/llm"
	"aiops-mvp/internal/tools"
	"github.com/joho/godotenv"
)

type Case struct {
	ID                   string   `json:"id"`
	Product              string   `json:"product"`
	Question             string   `json:"question"`
	ExpectCauseAny       []string `json:"expect_cause_any"`
	ExpectEvidenceSource []string `json:"expect_evidence_source"`
	ExpectHighRisk       bool     `json:"expect_high_risk"`
}

type result struct {
	hitCause  bool
	recall    float64
	highRisk  bool // 实际是否标记了高风险需审批
	highRatOK bool // 与期望是否一致
	conf      float64
	ms        int64
}

func main() {
	_ = godotenv.Load()
	datasetPath := flag.String("dataset", "eval/dataset.json", "评测数据集路径")
	reportPath := flag.String("report", "eval/report.md", "报告输出路径")
	limit := flag.Int("limit", 0, "只跑前 N 个案例（0=全部）")
	flag.Parse()

	raw, err := os.ReadFile(*datasetPath)
	if err != nil {
		fmt.Println("读取数据集失败:", err)
		os.Exit(1)
	}
	var cases []Case
	if err := json.Unmarshal(raw, &cases); err != nil {
		fmt.Println("解析数据集失败:", err)
		os.Exit(1)
	}
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
	}

	kpath := env("KNOWLEDGE_PATH", "../README-原始需求.md")
	index, err := knowledge.LoadMarkdown(kpath)
	if err != nil {
		fmt.Println("知识库加载失败:", err)
		os.Exit(1)
	}

	// 嵌入器 + 向量索引（供 full 配置使用）
	embedder := &embed.Client{BaseURL: os.Getenv("EMBED_BASE_URL"), APIKey: os.Getenv("EMBED_API_KEY"), Model: os.Getenv("EMBED_MODEL")}
	var embedFn func(string) ([]float32, error)
	if embedder.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		if vecs, e := embedder.Embed(ctx, index.ChunkTexts()); e == nil {
			index.SetVectors(vecs)
			embedFn = func(q string) ([]float32, error) {
				c2, c2c := context.WithTimeout(context.Background(), 20*time.Second)
				defer c2c()
				vs, err2 := embedder.Embed(c2, []string{q})
				if err2 != nil || len(vs) == 0 {
					return nil, err2
				}
				return vs[0], nil
			}
			fmt.Printf("向量索引已构建: %d 分块 (embed=%s)\n", index.Size(), embedder.Model)
		} else {
			fmt.Println("向量构建失败，full 配置退化为 BM25:", e)
		}
		cancel()
	}

	llmClient := &llm.Client{BaseURL: os.Getenv("LLM_BASE_URL"), APIKey: os.Getenv("LLM_API_KEY"), Model: os.Getenv("LLM_MODEL")}
	if !llmClient.Enabled() {
		fmt.Println("警告：未配置 LLM，评测无意义。请在 backend/.env 配置 DeepSeek。")
		os.Exit(1)
	}

	// 各配置的诊断函数
	fullSvc := app.New(tools.NewService(index, tools.DemoAlertProvider{}, tools.DemoLogProvider{}), llmClient)
	fullSvc.Tools.Embed = embedFn
	bm25Svc := app.New(tools.NewService(index, tools.DemoAlertProvider{}, tools.DemoLogProvider{}), llmClient) // Embed 为 nil → 走 BM25

	configs := []struct {
		name string
		run  func(context.Context, Case) domain.DiagnosisResult
	}{
		{"full", func(ctx context.Context, c Case) domain.DiagnosisResult {
			return fullSvc.Diagnose(ctx, req(c))
		}},
		{"bm25", func(ctx context.Context, c Case) domain.DiagnosisResult {
			return bm25Svc.Diagnose(ctx, req(c))
		}},
		{"no-agent", func(ctx context.Context, c Case) domain.DiagnosisResult {
			return noAgent(ctx, llmClient, c)
		}},
	}

	type agg struct {
		n      int
		hit    int
		recall float64
		riskOK int
		conf   float64
		ms     int64
	}
	sums := map[string]*agg{}
	detail := map[string][]result{}

	for _, cfg := range configs {
		sums[cfg.name] = &agg{}
		fmt.Printf("\n==== 配置 %s ====\n", cfg.name)
		for _, c := range cases {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			t0 := time.Now()
			dr := cfg.run(ctx, c)
			ms := time.Since(t0).Milliseconds()
			cancel()
			res := score(c, dr, ms)
			detail[cfg.name] = append(detail[cfg.name], res)
			a := sums[cfg.name]
			a.n++
			if res.hitCause {
				a.hit++
			}
			a.recall += res.recall
			if res.highRatOK {
				a.riskOK++
			}
			a.conf += res.conf
			a.ms += res.ms
			fmt.Printf("  %-12s 根因=%v 证据召回=%.0f%% 高危标记正确=%v 置信=%.2f %dms\n",
				c.ID, res.hitCause, res.recall*100, res.highRatOK, res.conf, res.ms)
		}
	}

	// 生成报告
	var b strings.Builder
	b.WriteString("# AIOps 诊断评测报告\n\n")
	b.WriteString(fmt.Sprintf("案例数：%d ｜ 模型：%s ｜ 知识检索：%s\n\n", len(cases), llmClient.Model, knowledgeMode(index)))
	b.WriteString("## 配置对照\n\n")
	b.WriteString("| 配置 | 根因命中率 | 证据召回率 | 高危标记正确率 | 平均置信度 | 平均用时 |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	order := []string{"full", "bm25", "no-agent"}
	desc := map[string]string{"full": "Agent+工具+向量RAG", "bm25": "Agent+工具+仅BM25", "no-agent": "纯LLM无取证"}
	for _, name := range order {
		a := sums[name]
		if a == nil || a.n == 0 {
			continue
		}
		n := float64(a.n)
		b.WriteString(fmt.Sprintf("| **%s**<br><sub>%s</sub> | %.0f%% | %.0f%% | %.0f%% | %.2f | %.1fs |\n",
			name, desc[name],
			float64(a.hit)/n*100, a.recall/n*100, float64(a.riskOK)/n*100, a.conf/n, float64(a.ms)/n/1000))
	}
	b.WriteString("\n> 指标说明：**根因命中率**=诊断结论/假设命中标准根因关键词的比例；**证据召回率**=应引用的证据来源被实际引用的比例；**高危标记正确率**=对高风险动作是否需审批的判断与期望一致的比例。\n")
	b.WriteString("\n> 说明：当前基于内置多产品演示数据集，用于验证方法与产出初步数字；正式投稿前建议替换为更大/更真实的案例集。\n")

	if err := os.WriteFile(*reportPath, []byte(b.String()), 0644); err != nil {
		fmt.Println("写报告失败:", err)
	} else {
		fmt.Println("\n报告已写入:", *reportPath)
	}
	fmt.Println("\n" + b.String())
}

func req(c Case) domain.DiagnosisRequest {
	return domain.DiagnosisRequest{ProductID: c.Product, Question: c.Question, WindowMinute: 30}
}

// noAgent 基线：不调用任何工具、无证据，纯靠 LLM 凭经验直接给出诊断（json 格式）。
func noAgent(ctx context.Context, c *llm.Client, cs Case) domain.DiagnosisResult {
	sys := "你是运维故障诊断助手。现在没有任何工具和证据，只能根据用户描述凭经验推断最可能的诊断。以 json 格式只输出一个对象：{\"summary\":\"一句话根因\",\"confidence\":0到1的小数,\"hypotheses\":[{\"rank\":1,\"cause\":\"根因\",\"confidence\":0.7}],\"actions\":[{\"name\":\"处置\",\"risk\":\"low|medium|high\",\"requires_approval\":false}]}。回滚/重启/删库/扩容等高风险动作必须 risk=high 且 requires_approval=true。"
	msg, err := c.Chat(ctx, []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: cs.Question + "（产品：" + cs.Product + "）"}}, nil, true)
	if err != nil || msg == nil {
		return domain.DiagnosisResult{}
	}
	var dr domain.DiagnosisResult
	if json.Unmarshal([]byte(msg.Content), &dr) != nil {
		return domain.DiagnosisResult{}
	}
	for i := range dr.Actions {
		if strings.EqualFold(dr.Actions[i].Risk, "high") {
			dr.Actions[i].RequiresApproval = true
		}
	}
	return dr
}

func score(c Case, dr domain.DiagnosisResult, ms int64) result {
	text := strings.ToLower(dr.Summary)
	for _, h := range dr.Hypotheses {
		text += " " + strings.ToLower(h.Cause)
	}
	hit := false
	for _, kw := range c.ExpectCauseAny {
		if kw != "" && strings.Contains(text, strings.ToLower(kw)) {
			hit = true
			break
		}
	}
	found := 0
	for _, src := range c.ExpectEvidenceSource {
		for _, e := range dr.Evidence {
			if strings.Contains(strings.ToLower(e.Source), strings.ToLower(src)) {
				found++
				break
			}
		}
	}
	recall := 0.0
	if len(c.ExpectEvidenceSource) > 0 {
		recall = float64(found) / float64(len(c.ExpectEvidenceSource))
	}
	flagged := false
	for _, a := range dr.Actions {
		if strings.EqualFold(a.Risk, "high") && a.RequiresApproval {
			flagged = true
			break
		}
	}
	return result{hitCause: hit, recall: recall, highRisk: flagged, highRatOK: flagged == c.ExpectHighRisk, conf: dr.Confidence, ms: ms}
}

func knowledgeMode(i *knowledge.Index) string {
	if i != nil && i.HasVectors() {
		return "BM25+向量(RRF)"
	}
	return "BM25"
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
