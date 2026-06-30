package projects

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates the project does not exist or is not accessible.
var ErrNotFound = errors.New("projects: not found")

// Project is a project record with the requesting user's role attached.
type Project struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Role        string    `json:"role,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repository provides data access for projects.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ListForUser returns projects the user is a member of, with their role.
func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.workspace_id, p.owner_id, p.name, p.slug, pm.role, p.created_at, p.updated_at
		 FROM projects p
		 JOIN project_members pm ON pm.project_id = p.id
		 WHERE pm.user_id = $1
		 ORDER BY p.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.OwnerID, &p.Name, &p.Slug, &p.Role, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetForUser returns a single project if the user is a member.
func (r *Repository) GetForUser(ctx context.Context, userID, projectID uuid.UUID) (*Project, error) {
	p := &Project{}
	err := r.pool.QueryRow(ctx,
		`SELECT p.id, p.workspace_id, p.owner_id, p.name, p.slug, pm.role, p.created_at, p.updated_at
		 FROM projects p
		 JOIN project_members pm ON pm.project_id = p.id
		 WHERE p.id = $1 AND pm.user_id = $2`, projectID, userID,
	).Scan(&p.ID, &p.WorkspaceID, &p.OwnerID, &p.Name, &p.Slug, &p.Role, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// MemberRole returns the user's role on a project, or ErrNotFound.
func (r *Repository) MemberRole(ctx context.Context, userID, projectID uuid.UUID) (string, error) {
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

// PrimaryWorkspaceID returns the user's personal workspace id.
func (r *Repository) PrimaryWorkspaceID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM workspaces WHERE owner_id = $1 ORDER BY is_personal DESC, created_at ASC LIMIT 1`, userID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// Create inserts a project and an owner membership for the creator atomically.
func (r *Repository) Create(ctx context.Context, ownerID, workspaceID uuid.UUID, name, slug string) (*Project, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	p := &Project{}
	err = tx.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_id, name, slug)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, workspace_id, owner_id, name, slug, created_at, updated_at`,
		workspaceID, ownerID, name, slug,
	).Scan(&p.ID, &p.WorkspaceID, &p.OwnerID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'owner')`,
		p.ID, ownerID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	p.Role = "owner"
	return p, nil
}

// Delete removes a project (cascades to members/tunnels via FKs).
func (r *Repository) Delete(ctx context.Context, projectID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, projectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
