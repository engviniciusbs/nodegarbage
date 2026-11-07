package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var okAnalysis = Analysis{Items: []ItemVerdict{{Path: `C:\x`, Verdict: "safe", Reason: "r", Confidence: 0.5}}, ReportMarkdown: "# R"}

func fast(p Provider) Provider {
	d := []time.Duration{time.Millisecond, time.Millisecond}
	switch v := p.(type) {
	case *openAICompat:
		v.delays = d
	case *anthropicProvider:
		v.delays = d
	}
	return p
}

func mk(t *testing.T, kind Kind, url string, key string, hc *http.Client) Provider {
	t.Helper()
	p, err := NewProvider(Profile{ID: "p", Kind: kind, BaseURL: url, Model: "m"}, key, hc)
	if err != nil {
		t.Fatal(err)
	}
	return fast(p)
}

func openAIBody(a Analysis) string {
	b, _ := json.Marshal(a)
	out, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}})
	return string(out)
}

func anthropicBody(a Analysis) string {
	out, _ := json.Marshal(map[string]any{"content": []any{
		map[string]any{"type": "text", "text": "ok"},
		map[string]any{"type": "tool_use", "name": "report_analysis", "input": a},
	}})
	return string(out)
}

func okBody(k Kind) string {
	if k == KindAnthropic {
		return anthropicBody(okAnalysis)
	}
	return openAIBody(okAnalysis)
}

func TestOpenAICompatSendsSchemaAndAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Method != "POST" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		rf := body["response_format"].(map[string]any)
		if rf["type"] != "json_schema" {
			t.Errorf("rf %v", rf)
		}
		if body["model"] != "m" {
			t.Errorf("model %v", body["model"])
		}
		io.WriteString(w, openAIBody(okAnalysis))
	}))
	defer srv.Close()
	a, err := mk(t, KindOpenAICompat, srv.URL, "k", nil).Analyze(t.Context(), Payload{})
	if err != nil || len(a.Items) != 1 || a.Items[0].Path != `C:\x` {
		t.Fatalf("%v %+v", err, a)
	}
}

func TestOpenAICompatFallbackJSONObject(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		typ := body["response_format"].(map[string]any)["type"]
		if atomic.AddInt32(&n, 1) == 1 {
			if typ != "json_schema" {
				t.Errorf("first %v", typ)
			}
			w.WriteHeader(400)
			io.WriteString(w, `{"error":"Unsupported Response_Format"}`)
			return
		}
		if typ != "json_object" {
			t.Errorf("second %v", typ)
		}
		io.WriteString(w, openAIBody(okAnalysis))
	}))
	defer srv.Close()
	if _, err := mk(t, KindOpenAICompat, srv.URL, "k", nil).Analyze(t.Context(), Payload{}); err != nil || n != 2 {
		t.Fatalf("%v n=%d", err, n)
	}
}

func TestOpenAICompatNoKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Header["Authorization"]; ok {
			t.Error("Authorization header must be absent")
		}
		io.WriteString(w, openAIBody(okAnalysis))
	}))
	defer srv.Close()
	if _, err := mk(t, KindOpenAICompat, srv.URL, "", nil).Analyze(t.Context(), Payload{}); err != nil {
		t.Fatal(err)
	}
}

func TestAnthropicToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("%s %v", r.URL.Path, r.Header)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		tc := body["tool_choice"].(map[string]any)
		if tc["type"] != "tool" || tc["name"] != "report_analysis" || body["max_tokens"] != float64(8192) {
			t.Errorf("%v", body)
		}
		io.WriteString(w, anthropicBody(okAnalysis))
	}))
	defer srv.Close()
	a, err := mk(t, KindAnthropic, srv.URL, "k", nil).Analyze(t.Context(), Payload{})
	if err != nil || len(a.Items) != 1 || a.ReportMarkdown != "# R" {
		t.Fatalf("%v %+v", err, a)
	}
}

func TestAnthropicTextFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(okAnalysis)
		out, _ := json.Marshal(map[string]any{"content": []any{map[string]any{"type": "text", "text": "Aqui: " + string(b)}}})
		w.Write(out)
	}))
	defer srv.Close()
	a, err := mk(t, KindAnthropic, srv.URL, "k", nil).Analyze(t.Context(), Payload{})
	if err != nil || len(a.Items) != 1 {
		t.Fatalf("%v", err)
	}
}

func TestErrorMapping(t *testing.T) {
	for _, kind := range []Kind{KindOpenAICompat, KindAnthropic} {
		for _, c := range []struct {
			status int
			want   error
			calls  int32
		}{{401, ErrAuth, 1}, {403, ErrAuth, 1}, {404, ErrModelNotFound, 1}, {429, ErrRateLimited, 3}} {
			var n int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&n, 1)
				w.WriteHeader(c.status)
				io.WriteString(w, "secret-key-123 nope")
			}))
			_, err := mk(t, kind, srv.URL, "secret-key-123", nil).Analyze(t.Context(), Payload{})
			srv.Close()
			if !errors.Is(err, c.want) || n != c.calls {
				t.Errorf("%s %d: err=%v n=%d", kind, c.status, err, n)
			}
			if err != nil && strings.Contains(err.Error(), "secret-key-123") {
				t.Errorf("key leaked: %v", err)
			}
		}
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				w.WriteHeader(500)
				return
			}
			io.WriteString(w, okBody(kind))
		}))
		_, err := mk(t, kind, srv.URL, "k", nil).Analyze(t.Context(), Payload{})
		srv.Close()
		if err != nil || n != 2 {
			t.Errorf("%s 500 then 200: %v n=%d", kind, err, n)
		}
	}
}

func TestTimeout(t *testing.T) {
	for _, kind := range []Kind{KindOpenAICompat, KindAnthropic} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
		}))
		_, err := mk(t, kind, srv.URL, "k", &http.Client{Timeout: 50 * time.Millisecond}).Analyze(t.Context(), Payload{})
		srv.CloseClientConnections()
		srv.Close()
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("%s: %v", kind, err)
		}
	}
}

func TestNewProviderValidation(t *testing.T) {
	if _, err := NewProvider(Profile{Kind: KindAnthropic}, "k", nil); err == nil {
		t.Error("empty model must fail")
	}
	if _, err := NewProvider(Profile{Kind: "x", Model: "m"}, "k", nil); err == nil {
		t.Error("bad kind must fail")
	}
}

func TestBackoffRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	p, _ := NewProvider(Profile{Kind: KindOpenAICompat, BaseURL: srv.URL, Model: "m"}, "", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.Analyze(ctx, Payload{})
	if err == nil || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("err=%v elapsed=%v", err, time.Since(start))
	}
}

// A redirect to another host must never carry the API key (nor the payload):
// it is refused, for both providers and also with a caller-supplied client.
func TestCrossHostRedirectRefused(t *testing.T) {
	for _, kind := range []Kind{KindAnthropic, KindOpenAICompat} {
		for _, custom := range []bool{false, true} {
			var hits atomic.Int32
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Header.Get("x-api-key") != "" || r.Header.Get("Authorization") != "" {
					t.Errorf("%s: key forwarded to other host", kind)
				}
				io.WriteString(w, okBody(kind))
			}))
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
			}))
			var hc *http.Client
			if custom {
				hc = &http.Client{Timeout: 5 * time.Second}
			}
			_, err := mk(t, kind, first.URL, "sk-secret", hc).Analyze(context.Background(), Payload{})
			if err == nil {
				t.Errorf("%s custom=%v: redirect followed", kind, custom)
			}
			if hits.Load() != 0 {
				t.Errorf("%s custom=%v: other host was contacted", kind, custom)
			}
			if hc != nil && hc.CheckRedirect != nil {
				t.Errorf("caller's client was mutated")
			}
			first.Close()
			other.Close()
		}
	}
}
