package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const anthropicToolName = "report_analysis"

type anthropicProvider struct {
	base, model, key string
	hc               *http.Client
	delays           []time.Duration
}

func (a *anthropicProvider) Analyze(ctx context.Context, p Payload) (Analysis, error) {
	msg, err := userMessage(p)
	if err != nil {
		return Analysis{}, err
	}
	body, err := json.Marshal(map[string]any{
		"model":      a.model,
		"max_tokens": 8192,
		"system":     systemPrompt,
		"messages":   []map[string]string{{"role": "user", "content": msg}},
		"tools": []map[string]any{{
			"name":         anthropicToolName,
			"description":  "Entrega a análise dos itens e o relatório em Markdown.",
			"input_schema": analysisSchema(),
		}},
		"tool_choice": map[string]any{"type": "tool", "name": anthropicToolName},
	})
	if err != nil {
		return Analysis{}, err
	}
	headers := map[string]string{"x-api-key": a.key, "anthropic-version": "2023-06-01"}
	data, err := post(ctx, a.hc, a.base+"/v1/messages", headers, a.key, body, a.delays)
	if err != nil {
		return Analysis{}, err
	}
	var resp struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return Analysis{}, fmt.Errorf("%w: unexpected messages body", ErrInvalidResponse)
	}
	var text strings.Builder
	for _, c := range resp.Content {
		switch c.Type {
		case "tool_use":
			var out Analysis
			if err := json.Unmarshal(c.Input, &out); err != nil {
				return Analysis{}, fmt.Errorf("%w: tool input: %v", ErrInvalidResponse, err)
			}
			if len(out.Items) == 0 && out.ReportMarkdown == "" {
				return Analysis{}, fmt.Errorf("%w: empty analysis", ErrInvalidResponse)
			}
			return out, nil
		case "text":
			text.WriteString(c.Text)
		}
	}
	return ParseAnalysis(text.String())
}
