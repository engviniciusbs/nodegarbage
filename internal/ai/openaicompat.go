package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type openAICompat struct {
	base, model, key string
	hc               *http.Client
	delays           []time.Duration
}

func (o *openAICompat) Analyze(ctx context.Context, p Payload) (Analysis, error) {
	msg, err := userMessage(p)
	if err != nil {
		return Analysis{}, err
	}
	schemaFmt := map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "nodegarbage_analysis",
			"strict": true,
			"schema": analysisSchema(),
		},
	}
	data, err := o.send(ctx, msg, schemaFmt)
	var se *statusError
	if errors.As(err, &se) && se.Status == http.StatusBadRequest && strings.Contains(strings.ToLower(se.Body), "response_format") {
		data, err = o.send(ctx, msg, map[string]any{"type": "json_object"})
	}
	if err != nil {
		return Analysis{}, err
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || len(resp.Choices) == 0 {
		return Analysis{}, fmt.Errorf("%w: unexpected chat completion body", ErrInvalidResponse)
	}
	return ParseAnalysis(resp.Choices[0].Message.Content)
}

func (o *openAICompat) send(ctx context.Context, userMsg string, format map[string]any) ([]byte, error) {
	body, err := json.Marshal(map[string]any{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMsg},
		},
		"response_format": format,
	})
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if o.key != "" {
		headers["Authorization"] = "Bearer " + o.key
	}
	return post(ctx, o.hc, o.base+"/chat/completions", headers, o.key, body, o.delays)
}
