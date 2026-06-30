package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/pkg/security"
)

// Service-level sentinel errors, mapped to HTTP status codes by the handler.
var (
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrWeakPassword       = errors.New("auth: password must be at least 8 characters")
	ErrInvalidRefresh     = errors.New("auth: invalid or expired refresh token")
	ErrInvalidToken       = errors.New("auth: invalid or expired token")
)

// Token lifetimes for email verification and password reset.
const (
	emailVerificationTTL = 24 * time.Hour
	passwordResetTTL     = 1 * time.Hour
)

// Service implements the authentication and provisioning use cases.
type Service struct {
	cfg    *config.Config
	repo   *Repository
	tokens *security.TokenManager
	mailer Mailer
}

// NewService constructs an auth Service.
func NewService(cfg *config.Config, repo *Repository, tm *security.TokenManager, mailer Mailer) *Service {
	return &Service{cfg: cfg, repo: repo, tokens: tm, mailer: mailer}
}

// Result is the outcome of a successful authentication.
type Result struct {
	AccessToken  string
	RefreshToken string
	User         *User
	WorkspaceID  uuid.UUID
	Role         string
}

// ClientInfo carries request metadata recorded on the session.
type ClientInfo struct {
	IP        string
	UserAgent string
}

// Register creates a new password-based identity, provisions a workspace, and
// issues tokens. (Password Signup Flow — Auth Bible.)
func (s *Service) Register(ctx context.Context, email, name, password string, client ClientInfo) (*Result, error) {
	email = normalizeEmail(email)
	if len(password) < 8 {
		return nil, ErrWeakPassword
	}

	if _, err := s.repo.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	user, err := s.repo.CreateUser(ctx, tx, email, name, false)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreatePasswordProvider(ctx, tx, user.ID, email, hash); err != nil {
		return nil, err
	}
	workspaceID, err := s.provision(ctx, tx, user)
	if err != nil {
		return nil, err
	}

	// Issue an email-verification token within the same transaction.
	verifyToken, err := security.GenerateOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateEmailVerification(ctx, tx, user.ID, security.HashToken(verifyToken), time.Now().Add(emailVerificationTTL)); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Best-effort send after commit; failure must not block registration.
	if err := s.mailer.SendVerificationEmail(ctx, user.Email, verifyToken); err != nil {
		_ = err
	}

	return s.issueTokens(ctx, user, workspaceID, string(security.RoleOwner), client)
}

// Login authenticates an email/password identity. (Auth Bible — login flow.)
func (s *Service) Login(ctx context.Context, email, password string, client ClientInfo) (*Result, error) {
	email = normalizeEmail(email)

	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrInvalidCredentials
	} else if err != nil {
		return nil, err
	}

	hash, err := s.repo.GetPasswordHash(ctx, user.ID)
	if errors.Is(err, ErrNotFound) {
		// User exists but has no password provider (OAuth-only account).
		return nil, ErrInvalidCredentials
	} else if err != nil {
		return nil, err
	}

	ok, err := security.VerifyPassword(password, hash)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}

	_ = s.repo.TouchLastLogin(ctx, user.ID)

	pw, err := s.repo.GetPrimaryWorkspace(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user, pw.WorkspaceID, pw.Role, client)
}

// Refresh rotates a refresh token: the presented token is revoked and a fresh
// access+refresh pair is issued (Refresh Token Rotation — Auth Bible).
func (s *Service) Refresh(ctx context.Context, refreshToken string, client ClientInfo) (*Result, error) {
	hash := security.HashToken(refreshToken)

	session, err := s.repo.GetActiveSessionByHash(ctx, hash)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrInvalidRefresh
	} else if err != nil {
		return nil, err
	}

	if err := s.repo.RevokeSessionByHash(ctx, hash); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	pw, err := s.repo.GetPrimaryWorkspace(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user, pw.WorkspaceID, pw.Role, client)
}

// Logout revokes the session backing a refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.repo.RevokeSessionByHash(ctx, security.HashToken(refreshToken))
}

// CurrentUser returns the user for an id (used by /auth/me).
func (s *Service) CurrentUser(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// VerifyEmail consumes a verification token and marks the user's email verified.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	userID, err := s.repo.ConsumeEmailVerification(ctx, tx, security.HashToken(token))
	if errors.Is(err, ErrNotFound) {
		return ErrInvalidToken
	} else if err != nil {
		return err
	}
	if err := s.repo.SetUserEmailVerified(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RequestPasswordReset issues a reset token for an email if it exists. It never
// reveals whether the email is registered (enumeration-safe); the returned
// token is non-empty only for callers that surface it in development.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	email = normalizeEmail(email)
	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return "", nil // silently succeed
	} else if err != nil {
		return "", err
	}

	token, err := security.GenerateOpaqueToken(32)
	if err != nil {
		return "", err
	}
	if err := s.repo.CreatePasswordReset(ctx, s.repo.Pool(), user.ID, security.HashToken(token), time.Now().Add(passwordResetTTL)); err != nil {
		return "", err
	}
	if err := s.mailer.SendPasswordResetEmail(ctx, user.Email, token); err != nil {
		_ = err // non-fatal
	}
	return token, nil
}

// ResetPassword consumes a reset token, sets a new password and revokes all of
// the user's existing sessions.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}

	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	userID, err := s.repo.ConsumePasswordReset(ctx, tx, security.HashToken(token))
	if errors.Is(err, ErrNotFound) {
		return ErrInvalidToken
	} else if err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.repo.UpsertPasswordHash(ctx, tx, userID, user.Email, hash); err != nil {
		return err
	}
	if err := s.repo.RevokeAllUserSessions(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AuthenticateOAuth resolves an external profile to a Gotra identity following
// the Auth Bible provider-linking rules:
//   - existing provider identity      → log in
//   - existing user with matching email → link the new provider, then log in
//   - otherwise                        → create a verified account and provision
func (s *Service) AuthenticateOAuth(ctx context.Context, provider Provider, profile *OAuthProfile, client ClientInfo) (*Result, error) {
	// 1. Known provider identity → straight login.
	if user, err := s.repo.GetUserByProvider(ctx, provider, profile.ProviderUserID); err == nil {
		_ = s.repo.TouchLastLogin(ctx, user.ID)
		pw, perr := s.repo.GetPrimaryWorkspace(ctx, user.ID)
		if perr != nil {
			return nil, perr
		}
		return s.issueTokens(ctx, user, pw.WorkspaceID, pw.Role, client)
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	metadata, _ := json.Marshal(map[string]string{"name": profile.Name, "avatar_url": profile.AvatarURL})
	email := normalizeEmail(profile.Email)

	// 2. Existing user with the same email → link provider (emails match rule).
	if email != "" {
		if user, err := s.repo.GetUserByEmail(ctx, email); err == nil {
			if lerr := s.repo.CreateOAuthProvider(ctx, s.repo.Pool(), user.ID, provider, profile.ProviderUserID, email, metadata); lerr != nil {
				return nil, lerr
			}
			_ = s.repo.TouchLastLogin(ctx, user.ID)
			pw, perr := s.repo.GetPrimaryWorkspace(ctx, user.ID)
			if perr != nil {
				return nil, perr
			}
			return s.issueTokens(ctx, user, pw.WorkspaceID, pw.Role, client)
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	// 3. New account — OAuth email is provider-verified, so mark verified.
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	user, err := s.repo.CreateUser(ctx, tx, email, profile.Name, true)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateOAuthProvider(ctx, tx, user.ID, provider, profile.ProviderUserID, email, metadata); err != nil {
		return nil, err
	}
	workspaceID, err := s.provision(ctx, tx, user)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, user, workspaceID, string(security.RoleOwner), client)
}

// provision performs first-time provisioning inside a transaction: personal
// workspace, default project, owner membership and a default API key.
func (s *Service) provision(ctx context.Context, q Querier, user *User) (uuid.UUID, error) {
	wsName := displayName(user) + "'s Workspace"
	workspaceID, err := s.repo.CreateWorkspace(ctx, q, user.ID, wsName, genSlug("ws"), true)
	if err != nil {
		return uuid.Nil, err
	}

	projectID, err := s.repo.CreateProject(ctx, q, workspaceID, user.ID, "Default Project", genSlug("proj"))
	if err != nil {
		return uuid.Nil, err
	}

	if err := s.repo.CreateProjectMember(ctx, q, projectID, user.ID, string(security.RoleOwner)); err != nil {
		return uuid.Nil, err
	}

	apiKey, err := security.GenerateOpaqueToken(24)
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.repo.CreateAPIKey(ctx, q, projectID, "default", security.HashToken("gtra_"+apiKey)); err != nil {
		return uuid.Nil, err
	}

	return workspaceID, nil
}

// issueTokens mints an access token and a rotating refresh token, persisting a
// session row for the refresh token.
func (s *Service) issueTokens(ctx context.Context, user *User, workspaceID uuid.UUID, role string, client ClientInfo) (*Result, error) {
	access, err := s.tokens.IssueAccessToken(user.ID, workspaceID, []security.Role{security.Role(role)})
	if err != nil {
		return nil, err
	}

	refresh, err := security.GenerateOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(s.cfg.RefreshTokenTTL)
	if _, err := s.repo.CreateSession(ctx, s.repo.Pool(), user.ID, security.HashToken(refresh), client.IP, client.UserAgent, expiresAt); err != nil {
		return nil, err
	}

	return &Result{
		AccessToken:  access,
		RefreshToken: refresh,
		User:         user,
		WorkspaceID:  workspaceID,
		Role:         role,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func displayName(u *User) string {
	if u.Name != "" {
		return u.Name
	}
	if i := strings.IndexByte(u.Email, '@'); i > 0 {
		return u.Email[:i]
	}
	return "Personal"
}

// genSlug returns a collision-resistant slug with the given prefix.
func genSlug(prefix string) string {
	suffix, err := security.GenerateOpaqueToken(6)
	if err != nil {
		return prefix + "-" + uuid.NewString()
	}
	return prefix + "-" + strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(suffix))
}
