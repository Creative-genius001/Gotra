// Package middleware provides shared Gin middleware (request IDs, CORS, recovery,
// and JWT authentication / RBAC).
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gotra/gotra/pkg/security"
)

// Context keys for values set by middleware.
const (
	ContextRequestID = "request_id"
	ContextClaims    = "claims"
)

// RequestID attaches a unique request ID to every request and response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(ContextRequestID, id)
		c.Writer.Header().Set("X-Request-ID", id)
		c.Next()
	}
}

// Auth verifies the Bearer access token and stores its claims in the context.
func Auth(tm *security.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := tm.ParseAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(ContextClaims, claims)
		c.Next()
	}
}

// RequireRole enforces RBAC: the authenticated user must hold at least one of
// the allowed roles. Must run after Auth.
func RequireRole(allowed ...security.Role) gin.HandlerFunc {
	allowedSet := make(map[security.Role]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		value, ok := c.Get(ContextClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}
		claims := value.(*security.Claims)

		for _, role := range claims.Roles {
			if _, ok := allowedSet[role]; ok {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}
