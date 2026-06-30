package ai

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrForbidden indicates the user cannot access the project.
var ErrForbidden = errors.New("ai: forbidden")

// Repository persists AI analyses, incidents and usage.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// UserInProject reports whether the user is a member of the project.
func (r *Repository) UserInProject(ctx context.Context, userID, projectID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`,
		projectID, userID,
	).Scan(&exists)
	return exists, err
}

// AnalysisRow is a stored analysis summary.
type AnalysisRow struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	RequestID    *uuid.UUID `json:"request_id,omitempty"`
	AnalysisType string    `json:"analysis_type"`
	Provider     string    `json:"provider"`
	Confidence   int             `json:"confidence_score"`
	Severity     string          `json:"severity,omitempty"`
	Result       json.RawMessage `json:"result"`
	CreatedAt    time.Time       `json:"created_at"`
}

// SaveAnalysis inserts an ai_analyses row.
func (r *Repository) SaveAnalysis(ctx context.Context, projectID uuid.UUID, requestID *uuid.UUID, analysisType, provider string, confidence int, severity string, result []byte) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO ai_analyses (project_id, request_id, analysis_type, provider, confidence_score, severity, result_json)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		projectID, requestID, analysisType, provider, confidence, severity, result,
	).Scan(&id)
	return id, err
}

// ListAnalyses returns recent analyses for a project.
func (r *Repository) ListAnalyses(ctx context.Context, projectID uuid.UUID, limit int) ([]AnalysisRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, request_id, analysis_type, provider, confidence_score, COALESCE(severity,''), result_json, created_at
		 FROM ai_analyses WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AnalysisRow
	for rows.Next() {
		var a AnalysisRow
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.RequestID, &a.AnalysisType, &a.Provider, &a.Confidence, &a.Severity, &a.Result, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// IncidentRow is a stored incident summary.
type IncidentRow struct {
	ID         uuid.UUID `json:"id"`
	ProjectID  uuid.UUID `json:"project_id"`
	Summary    string    `json:"summary"`
	RootCause  string    `json:"root_cause,omitempty"`
	Confidence int             `json:"confidence_score"`
	Status     string          `json:"status"`
	Report     json.RawMessage `json:"report"`
	CreatedAt  time.Time       `json:"created_at"`
}

// SaveIncident inserts an ai_incidents row.
func (r *Repository) SaveIncident(ctx context.Context, projectID uuid.UUID, summary, rootCause string, confidence int, report []byte) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO ai_incidents (project_id, summary, root_cause, confidence_score, report_json)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		projectID, summary, rootCause, confidence, report,
	).Scan(&id)
	return id, err
}

// ListIncidents returns recent incidents for a project.
func (r *Repository) ListIncidents(ctx context.Context, projectID uuid.UUID, limit int) ([]IncidentRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, summary, COALESCE(root_cause,''), confidence_score, status, report_json, created_at
		 FROM ai_incidents WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IncidentRow
	for rows.Next() {
		var i IncidentRow
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.Summary, &i.RootCause, &i.Confidence, &i.Status, &i.Report, &i.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// RecordUsage logs token/cost usage for the Cost Controller.
func (r *Repository) RecordUsage(ctx context.Context, projectID uuid.UUID, provider string, tokensIn, tokensOut int, costUSD float64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ai_usage (project_id, provider, tokens_in, tokens_out, cost_usd)
		 VALUES ($1, $2, $3, $4, $5)`,
		projectID, provider, tokensIn, tokensOut, costUSD,
	)
	return err
}
