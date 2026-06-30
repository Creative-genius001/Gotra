package billing

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates the workspace/subscription was not found.
var ErrNotFound = errors.New("billing: not found")

// Repository provides subscription and usage data access.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Subscription describes a workspace's current plan.
type Subscription struct {
	WorkspaceID uuid.UUID
	Plan        string
	Status      string
}

// PrimarySubscription resolves the user's personal workspace and its plan,
// defaulting to free if no subscription row exists.
func (r *Repository) PrimarySubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	s := &Subscription{}
	err := r.pool.QueryRow(ctx,
		`SELECT w.id, COALESCE(s.plan, 'free'), COALESCE(s.status, 'active')
		 FROM workspaces w
		 LEFT JOIN subscriptions s ON s.workspace_id = w.id
		 WHERE w.owner_id = $1
		 ORDER BY w.is_personal DESC, w.created_at ASC
		 LIMIT 1`, userID,
	).Scan(&s.WorkspaceID, &s.Plan, &s.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// SetPlan upserts the workspace's subscription plan.
func (r *Repository) SetPlan(ctx context.Context, workspaceID uuid.UUID, plan string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO subscriptions (workspace_id, plan, status)
		 VALUES ($1, $2, 'active')
		 ON CONFLICT (workspace_id) DO UPDATE SET plan = EXCLUDED.plan, status = 'active', updated_at = now()`,
		workspaceID, plan,
	)
	return err
}

// Usage is the workspace's current resource consumption.
type Usage struct {
	Projects      int `json:"projects"`
	ActiveTunnels int `json:"active_tunnels"`
	RequestsToday int `json:"requests_today"`
}

// WorkspaceUsage counts projects, active tunnels and today's requests.
func (r *Repository) WorkspaceUsage(ctx context.Context, workspaceID uuid.UUID) (Usage, error) {
	var u Usage
	err := r.pool.QueryRow(ctx,
		`SELECT
		   (SELECT count(*) FROM projects WHERE workspace_id = $1),
		   (SELECT count(*) FROM tunnels t JOIN projects p ON p.id = t.project_id
		      WHERE p.workspace_id = $1 AND t.status IN ('created','active')),
		   (SELECT count(*) FROM requests rq JOIN projects p ON p.id = rq.project_id
		      WHERE p.workspace_id = $1 AND rq.received_at >= date_trunc('day', now()))`,
		workspaceID,
	).Scan(&u.Projects, &u.ActiveTunnels, &u.RequestsToday)
	return u, err
}

// ActiveTunnelsForUser counts active tunnels in the user's primary workspace.
func (r *Repository) ActiveTunnelsForUser(ctx context.Context, userID uuid.UUID) (int, string, error) {
	sub, err := r.PrimarySubscription(ctx, userID)
	if err != nil {
		return 0, "", err
	}
	var n int
	err = r.pool.QueryRow(ctx,
		`SELECT count(*) FROM tunnels t JOIN projects p ON p.id = t.project_id
		 WHERE p.workspace_id = $1 AND t.status IN ('created','active')`, sub.WorkspaceID,
	).Scan(&n)
	return n, sub.Plan, err
}
