package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aiops-mvp/internal/executor"
	"aiops-mvp/internal/safetyeval"
)

func main() {
	jsonPath := flag.String("json", "eval/security-policy-report-v1.json", "JSON report path")
	markdownPath := flag.String("markdown", "eval/security-policy-report-v1.md", "Markdown report path")
	flag.Parse()

	report, err := safetyeval.RunDefault(executor.NewSimulator())
	if err != nil {
		fatal(err)
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err = writeFile(*jsonPath, append(raw, '\n')); err != nil {
		fatal(err)
	}
	if err = writeFile(*markdownPath, []byte(markdown(report))); err != nil {
		fatal(err)
	}
	fmt.Printf("safety evaluation: total=%d accuracy=%.2f block_recall=%.2f unsafe_escape_rate=%.2f passed=%t\n",
		report.Total, report.DecisionAccuracy, report.BlockRecall, report.UnsafeEscapeRate, report.Passed)
	if !report.Passed {
		os.Exit(1)
	}
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func markdown(report safetyeval.Report) string {
	var b strings.Builder
	b.WriteString("# 受控处置安全策略评测报告\n\n")
	b.WriteString("- 数据集版本：" + report.DatasetVersion + "\n")
	b.WriteString(fmt.Sprintf("- 样本总数：%d\n", report.Total))
	b.WriteString(fmt.Sprintf("- 预期放行：%d\n", report.ExpectedAllowed))
	b.WriteString(fmt.Sprintf("- 预期阻断：%d\n\n", report.ExpectedBlocked))
	b.WriteString("| 指标 | 结果 |\n|---|---:|\n")
	b.WriteString(fmt.Sprintf("| 决策准确率 | %.2f%% |\n", report.DecisionAccuracy*100))
	b.WriteString(fmt.Sprintf("| 动作分类准确率 | %.2f%% |\n", report.ClassificationAccuracy*100))
	b.WriteString(fmt.Sprintf("| 阻断召回率 | %.2f%% |\n", report.BlockRecall*100))
	b.WriteString(fmt.Sprintf("| 不安全误放行率 | %.2f%% |\n", report.UnsafeEscapeRate*100))
	b.WriteString(fmt.Sprintf("| 总体门禁 | %s |\n\n", passText(report.Passed)))
	b.WriteString("## 案例结果\n\n")
	b.WriteString("| ID | 动作 | 风险 | 状态 | 预期 | 实际 | 分类 |\n|---|---|---|---|---|---|---|\n")
	for _, result := range report.Results {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
			result.ID, escape(result.Action), result.Risk, result.Status,
			allowText(result.ExpectedAllowed), allowText(result.ActualAllowed), result.ActualKind))
	}
	b.WriteString("\n## 结论边界\n\n")
	b.WriteString("本报告只验证模拟执行器的动作分类、审批状态、风险一致性和白名单策略。")
	b.WriteString("它不证明真实 Kubernetes、发布平台或主机执行器的安全性。\n")
	return b.String()
}

func passText(value bool) string {
	if value {
		return "通过"
	}
	return "未通过"
}

func allowText(value bool) string {
	if value {
		return "放行"
	}
	return "阻断"
}

func escape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
