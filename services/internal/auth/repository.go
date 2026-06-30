package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx, so repository methods
// can run either standalone or inside a transaction (used for provisioning).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository provides data access for auth, identity and provisioning.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("auth: not found")

// Pool exposes the underlying pool for transaction management.
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// --- Users ------------------------------------------------------------------

// CreateUser inserts a user and returns the populated record.
func (r *Repository) CreateUser(ctx context.Context, q Querier, email, name string, emailVerified bool) (*User, error) {
	u := &User{}
	err := q.QueryRow(ctx,
		`INSERT INTO users (email, name, email_verified)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, name, COALESCE(avatar_url, ''), email_verified, last_login_at, created_at, updated_at`,
		email, name, emailVerified,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.EmailVerified, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserByEmail looks up a user by (case-insensitive) email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, name, COALESCE(avatar_url, ''), email_verified, last_login_at, created_at, updated_at
		 FROM users WHERE lower(email) = lower($1)`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.EmailVerified, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByID looks up a user by id.
func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, name, COALESCE(avatar_url, ''), email_verified, last_login_at, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.EmailVerified, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// TouchLastLogin updates last_login_at to now.
func (r *Repository) TouchLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = now(), updated_at = now() WHERE id = $1`, userID)
	return err
}

// --- Auth providers ---------------------------------------------------------

// CreatePasswordProvider links a password login method to a user.
func (r *Repository) CreatePasswordProvider(ctx context.Context, q Querier, userID uuid.UUID, email, passwordHash string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO user_auth_providers (user_id, provider, provider_user_id, provider_email, password_hash)
		 VALUES ($1, 'password', $2, $3, $4)`,
		userID, userID.String(), email, passwordHash,
	)
	return err
}

// CreateOAuthProvider links an OAuth login method to a user.
func (r *Repository) CreateOAuthProvider(ctx context.Context, q Querier, userID uuid.UUID, provider Provider, providerUserID, email string, metadata []byte) error {
	_, err := q.Exec(ctx,
		`INSERT INTO user_auth_providers (user_id, provider, provider_user_id, provider_email, provider_metadata)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, string(provider), providerUserID, email, metadata,
	)
	return err
}

// GetPasswordHash returns the stored Argon2id hash for a user's password provider.
func (r *Repository) GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(password_hash, '') FROM user_auth_providers
		 WHERE user_id = $1 AND provider = 'password'`, userID,
	).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

// GetUserByProvider finds the user linked to an external provider identity.
func (r *Repository) GetUserByProvider(ctx context.Context, provider Provider, providerUserID string) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx,
		`SELECT u.id, u.email, u.name, COALESCE(u.avatar_url, ''), u.email_verified, u.last_login_at, u.created_at, u.updated_at
		 FROM users u
		 JOIN user_auth_providers p ON p.user_id = u.id
		 WHERE p.provider = $1 AND p.provider_user_id = $2`,
		string(provider), providerUserID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.EmailVerified, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// --- Sessions ---------------------------------------------------------------

// CreateSession stores a refresh-token-backed session.
func (r *Repository) CreateSession(ctx context.Context, q Querier, userID uuid.UUID, refreshHash, ip, ua string, expiresAt time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx,
		`INSERT INTO sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID, refreshHash, ip, ua, expiresAt,
	).Scan(&id)
	return id, err
}

// GetActiveSessionByHash returns a non-revoked, non-expired session for a hash.
func (r *Repository) GetActiveSessionByHash(ctx context.Context, refreshHash string) (*Session, error) {
	s := &Session{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, refresh_token_hash, COALESCE(ip_address,''), COALESCE(user_agent,''), expires_at, created_at, revoked_at
		 FROM sessions
		 WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`,
		refreshHash,
	).Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.IPAddress, &s.UserAgent, &s.ExpiresAt, &s.CreatedAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// RevokeSession marks a session revoked.
func (r *Repository) RevokeSession(ctx context.Context, q Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1`, id)
	return err
}

// RevokeSessionByHash marks a session revoked by its refresh token hash.
func (r *Repository) RevokeSessionByHash(ctx context.Context, refreshHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE refresh_token_hash = $1`, refreshHash)
	return err
}

// --- Provisioning -----------------------------------------------------------

// PrimaryWorkspace is the workspace context attached to a user's tokens.
type PrimaryWorkspace struct {
	WorkspaceID uuid.UUID
	Role        string
}

// CreateWorkspace inserts a workspace.
func (r *Repository) CreateWorkspace(ctx context.Context, q Querier, ownerID uuid.UUID, name, slug string, personal bool) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx,
		`INSERT INTO workspaces (owner_id, name, slug, is_personal) VALUES ($1, $2, $3, $4) RETURNING id`,
		ownerID, name, slug, personal,
	).Scan(&id)
	return id, err
}

// CreateProject inserts a project.
func (r *Repository) CreateProject(ctx context.Context, q Querier, workspaceID, ownerID uuid.UUID, name, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, owner_id, name, slug) VALUES ($1, $2, $3, $4) RETURNING id`,
		workspaceID, ownerID, name, slug,
	).Scan(&id)
	return id, err
}

// CreateProjectMember adds a member with a role to a project.
func (r *Repository) CreateProjectMember(ctx context.Context, q Querier, projectID, userID uuid.UUID, role string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`,
		projectID, userID, role,
	)
	return err
}

// CreateAPIKey stores a hashed API key for a project.
func (r *Repository) CreateAPIKey(ctx context.Context, q Querier, projectID uuid.UUID, name, keyHash string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO api_keys (project_id, name, key_hash) VALUES ($1, $2, $3)`,
		projectID, name, keyHash,
	)
	return err
}

// --- Email verification & password reset ------------------------------------

// CreateEmailVerification stores a hashed email-verification token.
func (r *Repository) CreateEmailVerification(ctx context.Context, q Querier, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := q.Exec(ctx,
		`INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

// ConsumeEmailVerification marks a valid token used and returns its user id.
// Returns ErrNotFound if the token is unknown, already used, or expired.
func (r *Repository) ConsumeEmailVerification(ctx context.Context, q Querier, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := q.QueryRow(ctx,
		`UPDATE email_verifications SET used_at = now()
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		 RETURNING user_id`, tokenHash,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return userID, err
}

// SetUserEmailVerified flags a user's email as verified.
func (r *Repository) SetUserEmailVerified(ctx context.Context, q Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE users SET email_verified = TRUE, updated_at = now() WHERE id = $1`, userID)
	return err
}

// CreatePasswordReset stores a hashed password-reset token.
func (r *Repository) CreatePasswordReset(ctx context.Context, q Querier, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := q.Exec(ctx,
		`INSERT INTO password_resets (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

// ConsumePasswordReset marks a valid token used and returns its user id.
func (r *Repository) ConsumePasswordReset(ctx context.Context, q Querier, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := q.QueryRow(ctx,
		`UPDATE password_resets SET used_at = now()
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		 RETURNING user_id`, tokenHash,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return userID, err
}

// UpsertPasswordHash sets the password hash for a user's password provider,
// creating the provider row if the user previously had only OAuth logins.
func (r *Repository) UpsertPasswordHash(ctx context.Context, q Querier, userID uuid.UUID, email, hash string) error {
	tag, err := q.Exec(ctx,
		`UPDATE user_auth_providers SET password_hash = $2, updated_at = now()
		 WHERE user_id = $1 AND provider = 'password'`, userID, hash,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return r.CreatePasswordProvider(ctx, q, userID, email, hash)
	}
	return nil
}

// RevokeAllUserSessions revokes every active session for a user (used after a
// password reset).
func (r *Repository) RevokeAllUserSessions(ctx context.Context, q Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// GetPrimaryWorkspace returns the user's personal workspace and their role in it.
func (r *Repository) GetPrimaryWorkspace(ctx context.Context, userID uuid.UUID) (*PrimaryWorkspace, error) {
	pw := &PrimaryWorkspace{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, CASE WHEN owner_id = $1 THEN 'owner' ELSE 'developer' END
		 FROM workspaces
		 WHERE owner_id = $1
		 ORDER BY is_personal DESC, created_at ASC
		 LIMIT 1`, userID,
	).Scan(&pw.WorkspaceID, &pw.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return pw, err
}
