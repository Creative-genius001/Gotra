package auth

import (
	"time"

	"github.com/google/uuid"
)

// User is the core identity record (users table). Identity is kept separate
// from authentication providers per the Auth Bible's identity model.
type User struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	AvatarURL     string     `json:"avatar_url,omitempty"`
	EmailVerified bool       `json:"email_verified"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Provider identifies an authentication method.
type Provider string

const (
	ProviderPassword Provider = "password"
	ProviderGoogle   Provider = "google"
	ProviderGitHub   Provider = "github"
	ProviderOIDC     Provider = "oidc" // generic enterprise SSO (OpenID Connect)
)

// AuthProvider is a login method linked to a user (user_auth_providers table).
// A single user may link multiple providers to one identity.
type AuthProvider struct {
	ID             uuid.UUID      `json:"id"`
	UserID         uuid.UUID      `json:"user_id"`
	Provider       Provider       `json:"provider"`
	ProviderUserID string         `json:"provider_user_id"`
	ProviderEmail  string         `json:"provider_email,omitempty"`
	Metadata       map[string]any `json:"provider_metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// Session represents a refresh-token-backed device session (sessions table).
type Session struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	RefreshTokenHash string     `json:"-"`
	IPAddress        string     `json:"ip_address,omitempty"`
	UserAgent        string     `json:"user_agent,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
}
