package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

type Kind string

const (
	KindOpenAICompat Kind = "openai-compat"
	KindAnthropic    Kind = "anthropic"
)

type Profile struct {
	ID, Name string
	Kind     Kind
	BaseURL  string
	Model    string
}

type ItemVerdict struct {
	Path       string        `json:"path"`
	Verdict    rules.Verdict `json:"verdict"`
	Reason     string        `json:"reason"`
	Confidence float64       `json:"confidence"`
}

type Analysis struct {
	Items          []ItemVerdict `json:"items"`
	ReportMarkdown string        `json:"report_markdown"`
}

// Provider asks a model for advice. Its answer is untrusted data.
type Provider interface {
	Analyze(ctx context.Context, p Payload) (Analysis, error)
}

var (
	ErrAuth            = errors.New("ai: authentication failed")
	ErrModelNotFound   = errors.New("ai: model or endpoint not found")
	ErrRateLimited     = errors.New("ai: rate limited")
	ErrInvalidResponse = errors.New("ai: invalid response")
	ErrTimeout         = errors.New("ai: timeout")
	ErrNetwork         = errors.New("ai: network error")
)

const (
	defaultTimeout   = 120 * time.Second
	maxResponseBytes = 16 << 20
)

var defaultDelays = []time.Duration{1 * time.Second, 3 * time.Second}

// NewProvider builds the provider for pr. hc nil means a client with a 120 s timeout.
func NewProvider(pr Profile, apiKey string, hc *http.Client) (Provider, error) {
	if strings.TrimSpace(pr.Model) == "" {
		return nil, errors.New("ai: profile has no model")
	}
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	// Never follow a redirect to another origin: x-api-key (and, between
	// ports of one host, Authorization) and the payload would go with it.
	// The caller's client is copied, not mutated.
	c := *hc
	c.CheckRedirect = sameOriginRedirect
	hc = &c
	switch pr.Kind {
	case KindOpenAICompat:
		base := strings.TrimRight(pr.BaseURL, "/")
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		return &openAICompat{base: base, model: pr.Model, key: apiKey, hc: hc, delays: defaultDelays}, nil
	case KindAnthropic:
		base := strings.TrimRight(pr.BaseURL, "/")
		if base == "" {
			base = "https://api.anthropic.com"
		}
		return &anthropicProvider{base: base, model: pr.Model, key: apiKey, hc: hc, delays: defaultDelays}, nil
	}
	return nil, fmt.Errorf("ai: unknown provider kind %q", pr.Kind)
}

// errCrossOriginRedirect refuses a redirect that changes scheme or host:port.
var errCrossOriginRedirect = errors.New("ai: redirect to another origin refused")

// sameOriginRedirect is the providers' http.Client.CheckRedirect: a redirect
// is followed only within the original scheme and host:port, at most 10 times.
func sameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("ai: too many redirects")
	}
	orig := via[0].URL
	if !strings.EqualFold(req.URL.Scheme, orig.Scheme) || !strings.EqualFold(req.URL.Host, orig.Host) {
		return errCrossOriginRedirect
	}
	return nil
}

// analysisSchema is the JSON schema of the expected answer, shared by both providers.
func analysisSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":       map[string]any{"type": "string"},
						"verdict":    map[string]any{"type": "string", "enum": []string{"safe", "caution", "keep"}},
						"reason":     map[string]any{"type": "string"},
						"confidence": map[string]any{"type": "number"},
					},
					"required":             []string{"path", "verdict", "reason", "confidence"},
					"additionalProperties": false,
				},
			},
			"report_markdown": map[string]any{"type": "string"},
		},
		"required":             []string{"items", "report_markdown"},
		"additionalProperties": false,
	}
}

// userMessage is the scan data sent as the user turn.
func userMessage(p Payload) (string, error) {
	b, err := p.Compact()
	if err != nil {
		return "", err
	}
	return "Dados da varredura (JSON compacto, caminhos anonimizados):\n" + string(b), nil
}

// statusError is a non-2xx HTTP answer; it unwraps to the matching sentinel.
type statusError struct {
	Status   int
	Body     string
	sentinel error
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%v: HTTP %d: %s", e.sentinel, e.Status, e.Body)
}
func (e *statusError) Unwrap() error { return e.sentinel }

func snippet(b []byte, key string) string {
	s := string(b)
	if key != "" {
		s = strings.ReplaceAll(s, key, "***")
	}
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200])
	}
	return s
}

func sentinelFor(status int) (error, bool) {
	switch {
	case status == 401 || status == 403:
		return ErrAuth, false
	case status == 404:
		return ErrModelNotFound, false
	case status == 429:
		return ErrRateLimited, true
	case status >= 500:
		return ErrNetwork, true
	}
	return ErrInvalidResponse, false
}

// ctxError keeps cancellation distinguishable (errors.Is context.Canceled).
func ctxError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return transportError(err)
}

func transportError(err error) error {
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return fmt.Errorf("%w: %v", ErrTimeout, err)
	}
	return fmt.Errorf("%w: %v", ErrNetwork, err)
}

// post sends the JSON body, retrying on 429, 5xx and network errors with the
// given backoff delays. A timeout or context end is not retried.
func post(ctx context.Context, hc *http.Client, url string, headers map[string]string, key string, body []byte, delays []time.Duration) ([]byte, error) {
	var last error
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		retry := false
		resp, err := hc.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctxError(ctx.Err())
			}
			last = transportError(err)
			retry = errors.Is(last, ErrNetwork)
		} else {
			data, rerr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
			resp.Body.Close()
			switch {
			case rerr != nil:
				last = transportError(rerr)
				retry = errors.Is(last, ErrNetwork)
			case resp.StatusCode/100 == 2:
				return data, nil
			default:
				s, r := sentinelFor(resp.StatusCode)
				last = &statusError{Status: resp.StatusCode, Body: snippet(data, key), sentinel: s}
				retry = r
			}
		}
		if !retry || attempt >= len(delays) {
			return nil, last
		}
		t := time.NewTimer(delays[attempt])
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctxError(ctx.Err())
		case <-t.C:
		}
	}
}
