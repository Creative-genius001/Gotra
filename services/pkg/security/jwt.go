package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Role represents an RBAC role. Per the bibles: Owner, Admin, Developer, Viewer.
type Role string

const (
	RoleOwner     Role = "owner"
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleViewer    Role = "viewer"
)

// Claims is the JWT access-token payload. It carries user identity, the active
// workspace context and roles, per the JWT Architecture section of the Auth Bible.
type Claims struct {
	UserID      uuid.UUID `json:"uid"`
	WorkspaceID uuid.UUID `json:"wid,omitempty"`
	Roles       []Role    `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies signed JWT access tokens.
type TokenManager struct {
	secret         []byte
	accessTokenTTL time.Duration
}

// NewTokenManager constructs a TokenManager.
func NewTokenManager(secret string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTokenTTL: accessTTL}
}

// ErrInvalidToken is returned when a token fails verification.
var ErrInvalidToken = errors.New("security: invalid token")

// IssueAccessToken creates a signed access token for the given identity.
func (tm *TokenManager) IssueAccessToken(userID, workspaceID uuid.UUID, roles []Role) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Roles:       roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.accessTokenTTL)),
			Issuer:    "gotra",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

// ParseAccessToken verifies a token string and returns its claims.
func (tm *TokenManager) ParseAccessToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return tm.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
