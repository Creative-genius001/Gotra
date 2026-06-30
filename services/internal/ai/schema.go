package ai

import (
	"encoding/json"
	"strings"
)

// AnalysisResult is the structured output for explain/analyze operations.
// Per the AI Debugging Service Bible, every output is structured JSON with a
// 0–100 confidence score.
type AnalysisResult struct {
	Explanation  string `json:"explanation"`
	RootCause    string `json:"root_cause"`
	SuggestedFix string `json:"suggested_fix"`
	Severity     string `json:"severity"` // low | medium | high | critical
	Confidence   int    `json:"confidence"`
}

// IncidentResult is the structured output for incident generation.
type IncidentResult struct {
	Summary            string   `json:"summary"`
	Timeline           []string `json:"timeline"`
	AffectedServices   []string `json:"affected_services"`
	RootCause          string   `json:"root_cause"`
	RecommendedActions []string `json:"recommended_actions"`
	Severity           string   `json:"severity"`
	Confidence         int      `json:"confidence"`
}

// parseJSON tolerantly unmarshals a model response into v, stripping common
// markdown code-fence wrappers some providers emit around JSON.
func parseJSON(raw string, v any) error {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	return json.Unmarshal([]byte(s), v)
}

// clampConfidence keeps a confidence score within [0, 100].
func clampConfidence(c int) int {
	if c < 0 {
		return 0
	}
	if c > 100 {
		return 100
	}
	return c
}

// applyConfidence is the Confidence Engine: it tempers the model's self-reported
// confidence by how complete the input context was (Backend Bible — Confidence
// Engine). completeness is a 0.0–1.0 factor.
func applyConfidence(modelConfidence int, completeness float64) int {
	adjusted := float64(clampConfidence(modelConfidence)) * completeness
	return clampConfidence(int(adjusted))
}
