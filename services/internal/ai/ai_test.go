package ai

import (
	"context"
	"strings"
	"testing"
)

func TestParseJSONStripsCodeFence(t *testing.T) {
	var r AnalysisResult
	raw := "```json\n{\"explanation\":\"x\",\"severity\":\"high\",\"confidence\":80}\n```"
	if err := parseJSON(raw, &r); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Severity != "high" || r.Confidence != 80 {
		t.Errorf("got %+v", r)
	}
}

func TestApplyConfidence(t *testing.T) {
	cases := []struct {
		model int
		comp  float64
		want  int
	}{
		{80, 1.0, 80},
		{80, 0.5, 40},
		{120, 1.0, 100}, // clamps high
		{-5, 1.0, 0},    // clamps low
		{50, 0.8, 40},
	}
	for _, c := range cases {
		if got := applyConfidence(c.model, c.comp); got != c.want {
			t.Errorf("applyConfidence(%d, %v) = %d, want %d", c.model, c.comp, got, c.want)
		}
	}
}

func TestStubProviderSeverityFromStatus(t *testing.T) {
	p := newStubProvider()
	resp, err := p.Complete(context.Background(), CompletionRequest{User: "Status: 503 error"})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	var r AnalysisResult
	if err := parseJSON(resp.Text, &r); err != nil {
		t.Fatalf("parse stub output: %v", err)
	}
	if r.Severity != "high" {
		t.Errorf("5xx should yield high severity, got %q", r.Severity)
	}
	if resp.Provider != "stub" {
		t.Errorf("provider = %q", resp.Provider)
	}
}

func TestCostUSD(t *testing.T) {
	// claude: 1M in @ $5 + 1M out @ $25 = $30
	if got := costUSD("claude", 1_000_000, 1_000_000); got != 30 {
		t.Errorf("claude cost = %v, want 30", got)
	}
	if got := costUSD("stub", 100, 100); got != 0 {
		t.Errorf("stub cost = %v, want 0", got)
	}
}

func TestStubAlwaysAvailable(t *testing.T) {
	if !newStubProvider().Available() {
		t.Fatal("stub must always be available")
	}
}

func TestTruncate(t *testing.T) {
	long := strings.Repeat("a", maxBodyChars+500)
	out := truncate(long)
	if !strings.Contains(out, "truncated") {
		t.Error("expected truncation marker")
	}
	if len(out) >= len(long) {
		t.Error("expected shorter output")
	}
}
