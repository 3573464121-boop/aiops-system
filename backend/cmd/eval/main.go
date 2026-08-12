// 命令 eval：AIOps 诊断质量评测。
// 对同一批带“标准答案”的故障案例，在多种配置下运行诊断，计算指标并产出对照报告。
// 配置：
//   full     —— Agent 工具循环 + 向量 RAG（BM25+向量 RRF）
//   bm25     —— Agent 工具循环 + 仅 BM25 知识检索（关闭向量）
//   no-agent —— 纯 LLM 单轮作答，不调用任何工具（无证据基线）
// 指标：根因准确率(LLM 判官)、证据召回率、知识命中率、忠实度(LLM 判官)、幻觉率(LLM 判官)、平均置信度。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	GoldCause            string   `json:"gold_cause"`
	ExpectCauseAny       []string `json:"expect_cause_any"`
	ExpectEvidenceSource []string `json:"expect_evidence_source"`
	ExpectKnowledge      string   `json:"expect_knowledge"`
	ExpectHighRisk       bool     `json:"expect_high_risk"`
}

type result struct {
	causeOK bool
	recall  float64
	kbHit   bool
	kbEval  bool
	conf    float64
	ms      int64
	faith   float64
	halluc  bool
	judged  bool
}

func main() {
	_ = godotenv.Load()
	datasetPath := flag.String("dataset", "eval/dataset.json", "评测数据集路径")
	reportPath := flag.String("report", "eval/report.md", "报告输出路径")
	limit := flag.Int("limit", 0, "只跑前 N 个案例（0=全部）")
	judgeFlag := flag.Bool("judge", true, "用 LLM 判官评估根因准确率/忠实度/幻觉率")
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

	paths := []string{env("KNOWLEDGE_PATH", "../README-原始需求.md")}
	if extra, _ := filepath.Glob(filepath.Join(env("KNOWLEDGE_DIR", "knowledge-base"), "*.md")); len(extra) > 0 {
		paths = append(paths, extra...)
	}
	index, err := knowledge.LoadMarkdownFiles(paths)
	if err != nil {
		fmt.Println("知识库加载失败:", err)
		os.Exit(1)
	}

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

	fullSvc := app.New(tools.NewService(index, tools.DemoAlertProvider{}, tools.DemoLogProvider{}), llmClient)
	fullSvc.Tools.Embed = embedFn
	bm25Svc := app.New(tools.NewService(index, tools.DemoAlertProvider{}, tools.DemoLogProvider{}), llmClient) // Embed 为 nil → 走 BM25

	configs := []struct {
		name string
		run  func(context.Context, Case) domain.DiagnosisResult
	}{
		{"full", func(ctx context.Context, c Case) domain.DiagnosisResult { return fullSvc.Diagnose(ctx, req(c)) }},
		{"bm25", func(ctx context.Context, c Case) domain.DiagnosisResult { return bm25Svc.Diagnose(ctx, req(c)) }},
		{"no-agent", func(ctx context.Context, c Case) domain.DiagnosisResult { return noAgent(ctx, llmClient, c) }},
	}

	type agg struct {
		n, kbN, kb     int
		recall, conf   float64
		ms             int64
		faithN, halluc int
		causeOK        int
		faith          float64
	}
	sums := map[string]*agg{}

	for _, cfg := range configs {
		sums[cfg.name] = &agg{}
		fmt.Printf("\n==== 配置 %s ====\n", cfg.name)
		for _, c := range cases {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			t0 := time.Now()
			dr := cfg.run(ctx, c)
			ms := time.Since(t0).Milliseconds()
			res := score(c, dr, ms)
			if *judgeFlag {
				if f, h, cc, ok := judge(ctx, llmClient, c.Question, c.GoldCause, dr); ok {
					res.faith, res.halluc, res.causeOK, res.judged = f, h, cc, true
				}
			}
			cancel()
			a := sums[cfg.name]
			a.n++
			a.recall += res.recall
			if res.kbEval {
				a.kbN++
				if res.kbHit {
					a.kb++
				}
			}
			a.conf += res.conf
			a.ms += res.ms
			if res.judged {
				a.faithN++
				a.faith += res.faith
				if res.halluc {
					a.halluc++
				}
				if res.causeOK {
					a.causeOK++
				}
			}
			fmt.Printf("  %-12s 根因正确=%v 证据召回=%.0f%% 知识命中=%v 忠实=%.2f 幻觉=%v 置信=%.2f %dms\n",
				c.ID, res.causeOK, res.recall*100, res.kbHit, res.faith, res.halluc, res.conf, res.ms)
		}
	}

	var b strings.Builder
	b.WriteString("# AIOps 诊断评测报告\n\n")
	b.WriteString(fmt.Sprintf("案例数：%d ｜ 模型：%s ｜ 知识检索：%s ｜ 判官：%s\n\n", len(cases), llmClient.Model, knowledgeMode(index), judgeLabel(*judgeFlag, llmClient.Model)))
	b.WriteString("## 配置对照\n\n")
	b.WriteString("| 配置 | 根因准确率 | 证据召回率 | 知识命中率 | 忠实度↑ | 幻觉率↓ | 平均置信度 | 平均用时 |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	order := []string{"full", "bm25", "no-agent"}
	desc := map[string]string{"full": "Agent+工具+向量RAG", "bm25": "Agent+工具+仅BM25", "no-agent": "纯LLM无取证"}
	for _, name := range order {
		a := sums[name]
		if a == nil || a.n == 0 {
			continue
		}
		n := float64(a.n)
		kbStr, faithStr, hallStr, causeStr := "—", "—", "—", "—"
		if a.kbN > 0 {
			kbStr = fmt.Sprintf("%.0f%%", float64(a.kb)/float64(a.kbN)*100)
		}
		if a.faithN > 0 {
			faithStr = fmt.Sprintf("%.2f", a.faith/float64(a.faithN))
			hallStr = fmt.Sprintf("%.0f%%", float64(a.halluc)/float64(a.faithN)*100)
			causeStr = fmt.Sprintf("%.0f%%", float64(a.causeOK)/float64(a.faithN)*100)
		}
		b.WriteString(fmt.Sprintf("| **%s**<br><sub>%s</sub> | %s | %.0f%% | %s | %s | %s | %.2f | %.1fs |\n",
			name, desc[name], causeStr, a.recall/n*100, kbStr, faithStr, hallStr, a.conf/n, float64(a.ms)/n/1000))
	}
	b.WriteString("\n> **根因准确率**=LLM 判官严格判定诊断是否识别出标准根因（核心机理一致才算对）。\n")
	b.WriteString("> **知识命中率**=诊断是否检索并引用了对应的历史故障复盘（考察向量 RAG 语义召回，用于对比 full 与 bm25）。\n")
	b.WriteString("> **忠实度**=诊断论断被所引证据支撑的比例（判官，0-1，越高越好）；**幻觉率**=存在无证据支撑断言的案例占比（越低越好）；**证据召回率**=应引用日志证据被实际引用的比例。\n")
	b.WriteString("> 判官与被测为同一模型，存在自评偏差，仅作相对参考；正式投稿建议用更强/独立判官 + 更大更真实的数据集。\n")

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

// judge 用 LLM 判官依据"已收集证据"与"标准根因"评估：忠实度、是否幻觉、根因是否正确。
func judge(ctx context.Context, c *llm.Client, question, goldCause string, dr domain.DiagnosisResult) (float64, bool, bool, bool) {
	var ev strings.Builder
	for i, e := range dr.Evidence {
		ev.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, e.Title, e.Content))
	}
	if ev.Len() == 0 {
		ev.WriteString("（无任何证据）")
	}
	var hyp strings.Builder
	for _, h := range dr.Hypotheses {
		hyp.WriteString("- " + h.Cause + "\n")
	}
	sys := "你是严格的运维诊断评审。依据【可用证据】与【标准根因】评估【诊断结论/根因假设】。以 json 只输出一个对象：{\"faithfulness\":0到1的小数（结论被可用证据支撑的比例）,\"has_hallucination\":true或false（是否存在无证据支撑却断言的内容）,\"cause_correct\":true或false（是否正确识别出标准根因，核心机理一致才算对，方向对但关键机理缺失或搞错算错）}。若可用证据为空却给出具体断言，faithfulness 应很低、has_hallucination=true。"
	user := fmt.Sprintf("【用户问题】%s\n\n【标准根因】%s\n\n【可用证据】\n%s\n【诊断结论】%s\n【根因假设】\n%s", question, goldCause, ev.String(), dr.Summary, hyp.String())
	msg, err := c.Chat(ctx, []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}, nil, true)
	if err != nil || msg == nil {
		return 0, false, false, false
	}
	var v struct {
		Faithfulness     float64 `json:"faithfulness"`
		HasHallucination bool    `json:"has_hallucination"`
		CauseCorrect     bool    `json:"cause_correct"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &v) != nil {
		return 0, false, false, false
	}
	if v.Faithfulness < 0 {
		v.Faithfulness = 0
	}
	if v.Faithfulness > 1 {
		v.Faithfulness = 1
	}
	return v.Faithfulness, v.HasHallucination, v.CauseCorrect, true
}

func score(c Case, dr domain.DiagnosisResult, ms int64) result {
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
	kbHit, kbEval := false, c.ExpectKnowledge != ""
	if kbEval {
		for _, e := range dr.Evidence {
			if strings.Contains(e.Source, c.ExpectKnowledge) {
				kbHit = true
				break
			}
		}
	}
	return result{recall: recall, kbHit: kbHit, kbEval: kbEval, conf: dr.Confidence, ms: ms}
}

func knowledgeMode(i *knowledge.Index) string {
	if i != nil && i.HasVectors() {
		return "BM25+向量(RRF)"
	}
	return "BM25"
}

func judgeLabel(on bool, model string) string {
	if on {
		return model + "(自评)"
	}
	return "关闭"
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
