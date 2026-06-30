package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/internal/requests"
	"github.com/gotra/gotra/pkg/database"
)

// Service-level errors.
var (
	ErrNoData = errors.New("ai: no data to analyze")
)

// maxAnalysisTokens bounds output for a single analysis (Cost Controller).
const maxAnalysisTokens = 1024

// Service implements the AI Debugging Service use cases (Backend Bible).
type Service struct {
	repo   *Repository
	ctxb   *ContextBuilder
	orch   *Orchestrator
	log    *slog.Logger
}

// NewService wires the AI service: context builder over captured requests, the
// provider orchestrator, and the AI persistence layer.
func NewService(cfg *config.Config, db *database.DB, log *slog.Logger) *Service {
	return &Service{
		repo: NewRepository(db.Pool),
		ctxb: NewContextBuilder(requests.NewRepository(db.Pool)),
		orch: NewOrchestrator(cfg, log),
		log:  log,
	}
}

// StoredAnalysis is a persisted analysis returned to the caller.
type StoredAnalysis struct {
	ID           uuid.UUID      `json:"id"`
	Provider     string         `json:"provider"`
	AnalysisType string         `json:"analysis_type"`
	PromptVersion string        `json:"prompt_version"`
	Result       AnalysisResult `json:"result"`
}

// StoredIncident is a persisted incident returned to the caller.
type StoredIncident struct {
	ID       uuid.UUID      `json:"id"`
	Provider string         `json:"provider"`
	Result   IncidentResult `json:"result"`
}

// ExplainError runs the Explain Error flow for a captured request.
func (s *Service) ExplainError(ctx context.Context, userID, requestID uuid.UUID) (*StoredAnalysis, error) {
	return s.analyzeRequest(ctx, userID, requestID, "explain_error", userForExplainError)
}

// AnalyzeRequest runs general request analysis.
func (s *Service) AnalyzeRequest(ctx context.Context, userID, requestID uuid.UUID) (*StoredAnalysis, error) {
	return s.analyzeRequest(ctx, userID, requestID, "analyze_request", userForAnalyzeRequest)
}

// AnalyzeReplay runs replay-outcome analysis.
func (s *Service) AnalyzeReplay(ctx context.Context, userID, requestID uuid.UUID) (*StoredAnalysis, error) {
	return s.analyzeRequest(ctx, userID, requestID, "analyze_replay", userForAnalyzeReplay)
}

func (s *Service) analyzeRequest(ctx context.Context, userID, requestID uuid.UUID, analysisType string, userFn func(string) string) (*StoredAnalysis, error) {
	block, projectID, completeness, err := s.ctxb.ForRequest(ctx, userID, requestID)
	if err != nil {
		return nil, err
	}

	resp, err := s.orch.Complete(ctx, CompletionRequest{
		System: systemFor(analysisType), User: userFn(block), MaxTokens: maxAnalysisTokens,
	})
	if err != nil {
		return nil, err
	}

	var result AnalysisResult
	if err := parseJSON(resp.Text, &result); err != nil {
		return nil, fmt.Errorf("parse ai response: %w", err)
	}
	result.Confidence = applyConfidence(result.Confidence, completeness)
	if result.Severity == "" {
		result.Severity = "medium"
	}

	resultJSON, _ := json.Marshal(result)
	id, err := s.repo.SaveAnalysis(ctx, projectID, &requestID, analysisType, resp.Provider, result.Confidence, result.Severity, resultJSON)
	if err != nil {
		return nil, err
	}
	_ = s.repo.RecordUsage(ctx, projectID, resp.Provider, resp.TokensIn, resp.TokensOut, costUSD(resp.Provider, resp.TokensIn, resp.TokensOut))

	return &StoredAnalysis{ID: id, Provider: resp.Provider, AnalysisType: analysisType, PromptVersion: promptVersion, Result: result}, nil
}

// ExplainLogs runs the Explain Logs flow on user-supplied log text.
func (s *Service) ExplainLogs(ctx context.Context, userID, projectID uuid.UUID, logs string) (*StoredAnalysis, error) {
	if err := s.ensureMember(ctx, userID, projectID); err != nil {
		return nil, err
	}

	resp, err := s.orch.Complete(ctx, CompletionRequest{
		System: systemFor("explain_logs"), User: userForExplainLogs(logs), MaxTokens: maxAnalysisTokens,
	})
	if err != nil {
		return nil, err
	}

	var result AnalysisResult
	if err := parseJSON(resp.Text, &result); err != nil {
		return nil, fmt.Errorf("parse ai response: %w", err)
	}
	result.Confidence = applyConfidence(result.Confidence, 0.8)
	if result.Severity == "" {
		result.Severity = "medium"
	}

	resultJSON, _ := json.Marshal(result)
	id, err := s.repo.SaveAnalysis(ctx, projectID, nil, "explain_logs", resp.Provider, result.Confidence, result.Severity, resultJSON)
	if err != nil {
		return nil, err
	}
	_ = s.repo.RecordUsage(ctx, projectID, resp.Provider, resp.TokensIn, resp.TokensOut, costUSD(resp.Provider, resp.TokensIn, resp.TokensOut))

	return &StoredAnalysis{ID: id, Provider: resp.Provider, AnalysisType: "explain_logs", PromptVersion: promptVersion, Result: result}, nil
}

// GenerateIncident builds an incident report from a project's recent failures.
func (s *Service) GenerateIncident(ctx context.Context, userID, projectID uuid.UUID) (*StoredIncident, error) {
	if err := s.ensureMember(ctx, userID, projectID); err != nil {
		return nil, err
	}

	block, count, err := s.ctxb.ForRecentFailures(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrNoData
	}

	resp, err := s.orch.Complete(ctx, CompletionRequest{
		System: systemFor("generate_incident"), User: userForIncident(block), MaxTokens: maxAnalysisTokens * 2,
	})
	if err != nil {
		return nil, err
	}

	var result IncidentResult
	if err := parseJSON(resp.Text, &result); err != nil {
		return nil, fmt.Errorf("parse ai response: %w", err)
	}
	completeness := 0.7
	if count >= 3 {
		completeness = 1.0
	}
	result.Confidence = applyConfidence(result.Confidence, completeness)

	reportJSON, _ := json.Marshal(result)
	id, err := s.repo.SaveIncident(ctx, projectID, result.Summary, result.RootCause, result.Confidence, reportJSON)
	if err != nil {
		return nil, err
	}
	_ = s.repo.RecordUsage(ctx, projectID, resp.Provider, resp.TokensIn, resp.TokensOut, costUSD(resp.Provider, resp.TokensIn, resp.TokensOut))

	return &StoredIncident{ID: id, Provider: resp.Provider, Result: result}, nil
}

// ListAnalyses returns recent analyses for a project the user can access.
func (s *Service) ListAnalyses(ctx context.Context, userID, projectID uuid.UUID, limit int) ([]AnalysisRow, error) {
	if err := s.ensureMember(ctx, userID, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListAnalyses(ctx, projectID, limit)
}

// ListIncidents returns recent incidents for a project the user can access.
func (s *Service) ListIncidents(ctx context.Context, userID, projectID uuid.UUID, limit int) ([]IncidentRow, error) {
	if err := s.ensureMember(ctx, userID, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListIncidents(ctx, projectID, limit)
}

func (s *Service) ensureMember(ctx context.Context, userID, projectID uuid.UUID) error {
	ok, err := s.repo.UserInProject(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// costUSD estimates request cost from provider pricing (Cost Controller).
func costUSD(provider string, tokensIn, tokensOut int) float64 {
	switch provider {
	case "claude": // claude-opus-4-8: $5 / $25 per MTok
		return float64(tokensIn)/1e6*5 + float64(tokensOut)/1e6*25
	case "gemini": // gemini-2.0-flash: ~$0.075 / $0.30 per MTok
		return float64(tokensIn)/1e6*0.075 + float64(tokensOut)/1e6*0.30
	default:
		return 0
	}
}
