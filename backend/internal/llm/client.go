package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

type Client struct {
	BaseURL, APIKey, Model string
	HTTP                   *http.Client
}

func (c *Client) Enabled() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Model) != ""
}

// ---------- Tool Calling 基础类型（OpenAI 兼容） ----------

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 120 * time.Second}
}

// Chat 执行一次对话补全。tools 非空时开启工具调用；jsonMode 为 true 时强制 JSON 输出。
// 返回助手消息（可能携带 tool_calls，或携带最终 content）。
func (c *Client) Chat(ctx context.Context, messages []Message, tools []Tool, jsonMode bool) (*Message, error) {
	if !c.Enabled() {
		return nil, errors.New("LLM 未配置")
	}
	payload := map[string]any{
		"model":       c.Model,
		"temperature": 0.2,
		"messages":    messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	raw, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	var completion struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err = json.Unmarshal(data, &completion); err != nil {
		return nil, err
	}
	if len(completion.Choices) == 0 {
		return nil, errors.New("LLM 未返回 choices")
	}
	return &completion.Choices[0].Message, nil
}

// Synthesize 单轮强制 JSON 综合：把已收集的证据交给模型，产出结构化诊断。用作 Agent 的兜底。
func (c *Client) Synthesize(ctx context.Context, req domain.DiagnosisRequest, evidence []domain.Evidence, alerts []domain.Alert) (*domain.DiagnosisResult, error) {
	if !c.Enabled() {
		return nil, errors.New("LLM 未配置")
	}
	contextJSON, _ := json.Marshal(map[string]any{"question": req.Question, "product_id": req.ProductID, "alerts": alerts, "evidence": evidence})
	system := "你是只读AIOps诊断助手。只能根据证据判断，不得捏造。返回纯 json 对象：summary、confidence、hypotheses([{rank,cause,confidence}])、actions([{name,risk,requires_approval}])。高风险动作必须审批。"
	msg, err := c.Chat(ctx, []Message{{Role: "system", Content: system}, {Role: "user", Content: string(contextJSON)}}, nil, true)
	if err != nil {
		return nil, err
	}
	var result domain.DiagnosisResult
	if err = json.Unmarshal([]byte(msg.Content), &result); err != nil {
		return nil, fmt.Errorf("LLM JSON解析失败: %w", err)
	}
	if strings.TrimSpace(result.Summary) == "" {
		return nil, errors.New("LLM结果缺少 summary")
	}
	for i := range result.Actions {
		if result.Actions[i].Risk == "high" {
			result.Actions[i].RequiresApproval = true
		}
	}
	return &result, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
