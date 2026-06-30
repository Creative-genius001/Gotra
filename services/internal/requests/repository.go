// Package requests implements the request-capture, inspection and replay
// features (Backend Bible — Request Capture Pipeline & Replay Engine). The
// gateway writes captures here; the API reads and replays them.
package requests

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates the request does not exist or is not accessible.
var ErrNotFound = errors.New("requests: not found")

// Capture is a captured request/response pair to persist.
type Capture struct {
	TunnelID    uuid.UUID
	ProjectID   uuid.UUID
	Method      string
	Path        string
	Query       string
	ReqHeaders  map[string][]string
	ReqBody     []byte
	Status      int
	RespHeaders map[string][]string
	RespBody    []byte
	DurationMs  int
}

// Summary is a row in the request list.
type Summary struct {
	ID         uuid.UUID `json:"id"`
	TunnelID   uuid.UUID `json:"tunnel_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     *int      `json:"status,omitempty"`
	DurationMs *int      `json:"duration_ms,omitempty"`
	ReceivedAt time.Time `json:"received_at"`
}

// ResponseDetail is the captured response.
type ResponseDetail struct {
	Status     int                 `json:"status"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	DurationMs int                 `json:"duration_ms"`
}

// Detail is a full captured request with its response.
type Detail struct {
	ID         uuid.UUID           `json:"id"`
	TunnelID   uuid.UUID           `json:"tunnel_id"`
	ProjectID  uuid.UUID           `json:"project_id"`
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	Query      string              `json:"query,omitempty"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	Response   *ResponseDetail     `json:"response,omitempty"`
	ReceivedAt time.Time           `json:"received_at"`
}

// ReplayContext holds what's needed to replay a request through the tunnel.
type ReplayContext struct {
	RequestID       uuid.UUID
	ProjectID       uuid.UUID
	TunnelPublicURL string
	Method          string
	Path            string
	Query           string
	Headers         map[string][]string
	Body            []byte
}

// Repository provides data access for captured requests.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// SaveCapture persists a request and its response in one transaction.
func (r *Repository) SaveCapture(ctx context.Context, c Capture) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	reqHeaders, _ := json.Marshal(c.ReqHeaders)
	var requestID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO requests (tunnel_id, project_id, method, path, query, headers, body)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		c.TunnelID, c.ProjectID, c.Method, c.Path, c.Query, reqHeaders, c.ReqBody,
	).Scan(&requestID); err != nil {
		return uuid.Nil, err
	}

	respHeaders, _ := json.Marshal(c.RespHeaders)
	if _, err := tx.Exec(ctx,
		`INSERT INTO responses (request_id, status_code, headers, body, duration_ms)
		 VALUES ($1, $2, $3, $4, $5)`,
		requestID, c.Status, respHeaders, c.RespBody, c.DurationMs,
	); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return requestID, nil
}

// ListForProject returns recent requests for a project the user can access.
func (r *Repository) ListForProject(ctx context.Context, userID, projectID uuid.UUID, limit int) ([]Summary, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT req.id, req.tunnel_id, req.method, req.path, resp.status_code, resp.duration_ms, req.received_at
		 FROM requests req
		 JOIN project_members pm ON pm.project_id = req.project_id AND pm.user_id = $1
		 LEFT JOIN responses resp ON resp.request_id = req.id
		 WHERE req.project_id = $2
		 ORDER BY req.received_at DESC
		 LIMIT $3`, userID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Summary
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.ID, &s.TunnelID, &s.Method, &s.Path, &s.Status, &s.DurationMs, &s.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetForUser returns a full captured request if the user can access it.
func (r *Repository) GetForUser(ctx context.Context, userID, requestID uuid.UUID) (*Detail, error) {
	var (
		d           Detail
		reqHeaders  []byte
		reqBody     []byte
		status      *int
		respHeaders []byte
		respBody    []byte
		durationMs  *int
	)
	err := r.pool.QueryRow(ctx,
		`SELECT req.id, req.tunnel_id, req.project_id, req.method, req.path, COALESCE(req.query,''),
		        req.headers, req.body, req.received_at,
		        resp.status_code, resp.headers, resp.body, resp.duration_ms
		 FROM requests req
		 JOIN project_members pm ON pm.project_id = req.project_id AND pm.user_id = $1
		 LEFT JOIN responses resp ON resp.request_id = req.id
		 WHERE req.id = $2`, userID, requestID,
	).Scan(&d.ID, &d.TunnelID, &d.ProjectID, &d.Method, &d.Path, &d.Query,
		&reqHeaders, &reqBody, &d.ReceivedAt,
		&status, &respHeaders, &respBody, &durationMs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(reqHeaders, &d.Headers)
	d.Body = string(reqBody)

	if status != nil {
		resp := &ResponseDetail{Status: *status, Body: string(respBody)}
		if durationMs != nil {
			resp.DurationMs = *durationMs
		}
		_ = json.Unmarshal(respHeaders, &resp.Headers)
		d.Response = resp
	}
	return &d, nil
}

// GetReplayContext returns the original request plus its tunnel's public URL.
func (r *Repository) GetReplayContext(ctx context.Context, userID, requestID uuid.UUID) (*ReplayContext, error) {
	var (
		rc         ReplayContext
		reqHeaders []byte
	)
	err := r.pool.QueryRow(ctx,
		`SELECT req.id, req.project_id, t.public_url, req.method, req.path, COALESCE(req.query,''), req.headers, req.body
		 FROM requests req
		 JOIN project_members pm ON pm.project_id = req.project_id AND pm.user_id = $1
		 JOIN tunnels t ON t.id = req.tunnel_id
		 WHERE req.id = $2`, userID, requestID,
	).Scan(&rc.RequestID, &rc.ProjectID, &rc.TunnelPublicURL, &rc.Method, &rc.Path, &rc.Query, &reqHeaders, &rc.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(reqHeaders, &rc.Headers)
	return &rc, nil
}

// SaveReplay records a replay result.
func (r *Repository) SaveReplay(ctx context.Context, originalRequestID, projectID uuid.UUID, modified []byte, status int, body []byte, durationMs int) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO replays (original_request_id, project_id, modified_request, result_status_code, result_body, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		originalRequestID, projectID, modified, status, body, durationMs,
	).Scan(&id)
	return id, err
}
