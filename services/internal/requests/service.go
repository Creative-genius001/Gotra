package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/config"
)

// Service implements request inspection and replay.
type Service struct {
	cfg    *config.Config
	repo   *Repository
	client *http.Client
}

// NewService constructs a requests Service.
func NewService(cfg *config.Config, repo *Repository) *Service {
	return &Service{cfg: cfg, repo: repo, client: &http.Client{Timeout: 35 * time.Second}}
}

// List returns recent captured requests for a project.
func (s *Service) List(ctx context.Context, userID, projectID uuid.UUID, limit int) ([]Summary, error) {
	return s.repo.ListForProject(ctx, userID, projectID, limit)
}

// Get returns a full captured request.
func (s *Service) Get(ctx context.Context, userID, requestID uuid.UUID) (*Detail, error) {
	return s.repo.GetForUser(ctx, userID, requestID)
}

// ReplayInput carries optional overrides applied to the original request.
type ReplayInput struct {
	Method  *string              `json:"method"`
	Path    *string              `json:"path"`
	Headers map[string][]string  `json:"headers"`
	Body    *string              `json:"body"`
}

// ReplayResult is the outcome of a replay.
type ReplayResult struct {
	ReplayID   uuid.UUID `json:"replay_id"`
	Status     int       `json:"status"`
	Body       string    `json:"body"`
	DurationMs int       `json:"duration_ms"`
}

// Replay re-issues a stored request through its tunnel (the agent must be
// connected), captures the new response and records a replay row.
func (s *Service) Replay(ctx context.Context, userID, requestID uuid.UUID, in ReplayInput) (*ReplayResult, error) {
	rc, err := s.repo.GetReplayContext(ctx, userID, requestID)
	if err != nil {
		return nil, err
	}

	// Apply overrides over the original request.
	method := rc.Method
	if in.Method != nil && *in.Method != "" {
		method = *in.Method
	}
	path := rc.Path
	if rc.Query != "" {
		path += "?" + rc.Query
	}
	if in.Path != nil && *in.Path != "" {
		path = *in.Path
	}
	headers := rc.Headers
	if in.Headers != nil {
		headers = in.Headers
	}
	body := rc.Body
	if in.Body != nil {
		body = []byte(*in.Body)
	}

	// Build the request against the gateway, routed by the tunnel's Host.
	host := hostFromPublicURL(rc.TunnelPublicURL)
	url := strings.TrimRight(s.cfg.GatewayInternalURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Host = host
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	durationMs := int(time.Since(start).Milliseconds())

	modified, _ := json.Marshal(map[string]any{
		"method": method, "path": path, "headers": headers, "body": string(body),
	})
	replayID, err := s.repo.SaveReplay(ctx, rc.RequestID, rc.ProjectID, modified, resp.StatusCode, respBody, durationMs)
	if err != nil {
		return nil, err
	}

	return &ReplayResult{
		ReplayID:   replayID,
		Status:     resp.StatusCode,
		Body:       string(respBody),
		DurationMs: durationMs,
	}, nil
}

// hostFromPublicURL strips the scheme from a tunnel public URL to get its Host.
func hostFromPublicURL(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return u
}
