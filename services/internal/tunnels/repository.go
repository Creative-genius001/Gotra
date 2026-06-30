package tunnels

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates the tunnel/project does not exist or is not accessible.
var ErrNotFound = errors.New("tunnels: not found")

// Tunnel is a tunnel record.
type Tunnel struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	PublicURL string    `json:"public_url"`
	LocalPort int       `json:"local_port"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Repository provides data access for tunnels with project-scoped authorization.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ProjectRole returns the user's role on a project, or ErrNotFound if the user
// is not a member.
func (r *Repository) ProjectRole(ctx context.Context, userID, projectID uuid.UUID) (string, error) {
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

// ListForProject returns tunnels for a project.
func (r *Repository) ListForProject(ctx context.Context, projectID uuid.UUID) ([]Tunnel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, public_url, local_port, status, created_at, updated_at
		 FROM tunnels WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Tunnel
	for rows.Next() {
		var t Tunnel
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.PublicURL, &t.LocalPort, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Create inserts a tunnel.
func (r *Repository) Create(ctx context.Context, projectID uuid.UUID, publicURL string, localPort int) (*Tunnel, error) {
	t := &Tunnel{}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO tunnels (project_id, public_url, local_port, status)
		 VALUES ($1, $2, $3, 'created')
		 RETURNING id, project_id, public_url, local_port, status, created_at, updated_at`,
		projectID, publicURL, localPort,
	).Scan(&t.ID, &t.ProjectID, &t.PublicURL, &t.LocalPort, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// GetWithAccess returns a tunnel only if the user is a member of its project,
// along with the user's role on that project.
func (r *Repository) GetWithAccess(ctx context.Context, userID, tunnelID uuid.UUID) (*Tunnel, string, error) {
	t := &Tunnel{}
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT t.id, t.project_id, t.public_url, t.local_port, t.status, t.created_at, t.updated_at, pm.role
		 FROM tunnels t
		 JOIN project_members pm ON pm.project_id = t.project_id
		 WHERE t.id = $1 AND pm.user_id = $2`, tunnelID, userID,
	).Scan(&t.ID, &t.ProjectID, &t.PublicURL, &t.LocalPort, &t.Status, &t.CreatedAt, &t.UpdatedAt, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return t, role, err
}

// SetStatus updates a tunnel's lifecycle status.
func (r *Repository) SetStatus(ctx context.Context, tunnelID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tunnels SET status = $2, updated_at = now() WHERE id = $1`, tunnelID, status)
	return err
}

// Delete removes a tunnel by id.
func (r *Repository) Delete(ctx context.Context, tunnelID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tunnels WHERE id = $1`, tunnelID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
