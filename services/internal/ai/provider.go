package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gotra/gotra/internal/config"
)

// CompletionRequest is a provider-agnostic LLM request.
type CompletionRequest struct {
	System    string
	User      string
	MaxTokens int
}

// CompletionResponse is a provider-agnostic LLM response.
type CompletionResponse struct {
	Text      string
	Provider  string
	TokensIn  int
	TokensOut int
}

// Provider is one LLM backend.
type Provider interface {
	Name() string
	Available() bool
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// Orchestrator tries providers in order (primary → secondary → stub fallback),
// implementing the LLM Orchestration Layer's failover behavior.
type Orchestrator struct {
	providers []Provider
	log       *slog.Logger
}

// NewOrchestrator builds the provider chain from config. Per the AI Bible,
// Gemini is primary and Claude is secondary; a local stub is always appended so
// the pipeline works without API keys.
func NewOrchestrator(cfg *config.Config, log *slog.Logger) *Orchestrator {
	byName := map[string]Provider{
		"gemini": newGeminiProvider(cfg.AI.GeminiAPIKey),
		"claude": newAnthropicProvider(cfg.AI.AnthropicAPIKey),
	}

	var chain []Provider
	for _, name := range []string{cfg.AI.PrimaryProvider, cfg.AI.SecondaryProvider} {
		if p, ok := byName[name]; ok && p.Available() {
			chain = append(chain, p)
		}
	}
	chain = append(chain, newStubProvider()) // always-available fallback

	return &Orchestrator{providers: chain, log: log}
}

// Complete runs the request against the provider chain, falling back on error.
func (o *Orchestrator) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	var lastErr error
	for _, p := range o.providers {
		resp, err := p.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		o.log.Warn("ai provider failed, falling back", "provider", p.Name(), "error", err)
		lastErr = err
	}
	return CompletionResponse{}, fmt.Errorf("all ai providers failed: %w", lastErr)
}

// --- Gemini (primary) -------------------------------------------------------

const geminiModel = "gemini-2.0-flash"

type geminiProvider struct {
	apiKey string
	client *http.Client
}

func newGeminiProvider(apiKey string) *geminiProvider {
	return &geminiProvider{apiKey: apiKey, client: &http.Client{Timeout: 60 * time.Second}}
}

func (g *geminiProvider) Name() string    { return "gemini" }
func (g *geminiProvider) Available() bool  { return g.apiKey != "" }

func (g *geminiProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	body := map[string]any{
		"systemInstruction": map[string]any{"parts": []map[string]string{{"text": req.System}}},
		"contents":          []map[string]any{{"role": "user", "parts": []map[string]string{{"text": req.User}}}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"maxOutputTokens":  req.MaxTokens,
			"temperature":      0,
		},
	}
	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return CompletionResponse{}, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return CompletionResponse{}, err
	}
	var text strings.Builder
	if len(parsed.Candidates) > 0 {
		for _, p := range parsed.Candidates[0].Content.Parts {
			text.WriteString(p.Text)
		}
	}
	return CompletionResponse{
		Text:      text.String(),
		Provider:  g.Name(),
		TokensIn:  parsed.UsageMetadata.PromptTokenCount,
		TokensOut: parsed.UsageMetadata.CandidatesTokenCount,
	}, nil
}

// --- Stub (always-available fallback) ---------------------------------------

// stubProvider produces a deterministic, heuristic analysis so the AI pipeline
// is fully functional and testable without any provider API keys configured.
type stubProvider struct{}

func newStubProvider() *stubProvider { return &stubProvider{} }

func (s *stubProvider) Name() string   { return "stub" }
func (s *stubProvider) Available() bool { return true }

func (s *stubProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	severity := "low"
	switch {
	case strings.Contains(req.User, "status\": 5"), strings.Contains(req.User, "Status: 5"), strings.Contains(req.User, "5xx"):
		severity = "high"
	case strings.Contains(req.User, "status\": 4"), strings.Contains(req.User, "Status: 4"), strings.Contains(req.User, "4xx"):
		severity = "medium"
	}

	// A superset object so it parses into either AnalysisResult or IncidentResult.
	out := map[string]any{
		"explanation":         "Heuristic analysis (no AI provider configured). Set GEMINI_API_KEY or ANTHROPIC_API_KEY to enable model-backed analysis.",
		"root_cause":          "Derived from captured status code and headers; configure a provider for a precise root cause.",
		"suggested_fix":       "Inspect the failing endpoint and its upstream dependencies; reproduce via the Replay Center.",
		"severity":            severity,
		"confidence":          35,
		"summary":             "Automated incident summary generated without an AI provider.",
		"timeline":            []string{"Request captured", "Non-2xx response observed", "Incident generated"},
		"affected_services":   []string{"tunnel", "local-app"},
		"recommended_actions": []string{"Configure an AI provider", "Review recent deploys", "Add request validation"},
	}
	payload, _ := json.Marshal(out)
	return CompletionResponse{Text: string(payload), Provider: s.Name(), TokensIn: 0, TokensOut: 0}, nil
}
