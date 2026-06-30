// Package auth implements identity and authentication per the Authentication &
// Identity Architecture Bible: multi-provider login (password, Google, GitHub),
// account linking, sessions, JWT issuance and first-time workspace provisioning.
package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/pkg/cache"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

const refreshCookieName = "gotra_refresh"

// Handler holds the dependencies for auth HTTP endpoints.
type Handler struct {
	cfg     *config.Config
	cache   *cache.Cache
	service *Service
	oauth   *OAuthManager
}

// NewHandler constructs an auth Handler with its service and OAuth manager.
func NewHandler(cfg *config.Config, db *database.DB, c *cache.Cache, tm *security.TokenManager, log *slog.Logger) *Handler {
	repo := NewRepository(db.Pool)
	mailer := NewLogMailer(cfg, log)
	return &Handler{
		cfg:     cfg,
		cache:   c,
		service: NewService(cfg, repo, tm, mailer),
		oauth:   NewOAuthManager(cfg),
	}
}

// RegisterRoutes mounts the public auth endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	a := rg.Group("/auth")
	{
		a.POST("/register", h.register)
		a.POST("/login", h.login)
		a.POST("/refresh", h.refresh)
		a.POST("/logout", h.logout)
		a.POST("/verify-email", h.verifyEmail)
		a.POST("/password/forgot", h.forgotPassword)
		a.POST("/password/reset", h.resetPassword)

		a.GET("/google", h.oauthStart(ProviderGoogle))
		a.GET("/google/callback", h.oauthCallback(ProviderGoogle))
		a.GET("/github", h.oauthStart(ProviderGitHub))
		a.GET("/github/callback", h.oauthCallback(ProviderGitHub))

		// Enterprise SSO (generic OIDC).
		a.GET("/oidc", h.oauthStart(ProviderOIDC))
		a.GET("/oidc/callback", h.oauthCallback(ProviderOIDC))
	}
}

// RegisterProtected mounts authenticated auth endpoints (run behind Auth mw).
func (h *Handler) RegisterProtected(rg *gin.RouterGroup) {
	rg.GET("/auth/me", h.me)
}

// --- DTOs -------------------------------------------------------------------

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name"`
	Password string `json:"password" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type authResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	User        *User  `json:"user"`
	WorkspaceID string `json:"workspace_id"`
	Role        string `json:"role"`
}

// --- Handlers ---------------------------------------------------------------

func (h *Handler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	res, err := h.service.Register(c.Request.Context(), req.Email, req.Name, req.Password, clientInfo(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.respondAuth(c, res, http.StatusCreated)
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	res, err := h.service.Login(c.Request.Context(), req.Email, req.Password, clientInfo(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.respondAuth(c, res, http.StatusOK)
}

func (h *Handler) refresh(c *gin.Context) {
	token, _ := c.Cookie(refreshCookieName)
	if token == "" {
		// Allow refresh token in body as a fallback (non-browser clients).
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.ShouldBindJSON(&body)
		token = body.RefreshToken
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}

	res, err := h.service.Refresh(c.Request.Context(), token, clientInfo(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.respondAuth(c, res, http.StatusOK)
}

func (h *Handler) logout(c *gin.Context) {
	token, _ := c.Cookie(refreshCookieName)
	_ = h.service.Logout(c.Request.Context(), token)
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

func (h *Handler) verifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.service.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "verified"})
}

func (h *Handler) forgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	token, err := h.service.RequestPasswordReset(c.Request.Context(), req.Email)
	if err != nil {
		h.writeError(c, err)
		return
	}
	// Always 202 to avoid revealing whether the email exists. In non-production
	// the token is surfaced so the flow is testable without a mail provider.
	resp := gin.H{"status": "if the email exists, a reset link was sent"}
	if !h.cfg.IsProduction() && token != "" {
		resp["dev_token"] = token
	}
	c.JSON(http.StatusAccepted, resp)
}

func (h *Handler) resetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		h.writeError(c, err)
		return
	}
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"status": "password_reset"})
}

func (h *Handler) me(c *gin.Context) {
	value, ok := c.Get(middleware.ContextClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	claims := value.(*security.Claims)
	user, err := h.service.CurrentUser(c.Request.Context(), claims.UserID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user, "workspace_id": claims.WorkspaceID, "roles": claims.Roles})
}

// --- OAuth ------------------------------------------------------------------

func (h *Handler) oauthStart(provider Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.oauth.Configured(provider) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": string(provider) + " oauth is not configured"})
			return
		}
		state, err := security.GenerateOpaqueToken(24)
		if err != nil {
			h.writeError(c, err)
			return
		}
		// Persist state for CSRF protection (10 minute TTL).
		if err := h.cache.Client.Set(c.Request.Context(), oauthStateKey(state), string(provider), 10*time.Minute).Err(); err != nil {
			h.writeError(c, err)
			return
		}
		authURL, err := h.oauth.AuthCodeURL(c.Request.Context(), provider, state)
		if err != nil {
			h.writeError(c, err)
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, authURL)
	}
}

func (h *Handler) oauthCallback(provider Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		state := c.Query("state")
		code := c.Query("code")
		if state == "" || code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code or state"})
			return
		}

		// Validate and consume state (single-use).
		stored, err := h.cache.Client.GetDel(ctx, oauthStateKey(state)).Result()
		if err != nil || stored != string(provider) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth state"})
			return
		}

		accessToken, err := h.oauth.Exchange(ctx, provider, code)
		if err != nil {
			h.writeError(c, err)
			return
		}
		profile, err := h.oauth.FetchProfile(ctx, provider, accessToken)
		if err != nil {
			h.writeError(c, err)
			return
		}

		res, err := h.service.AuthenticateOAuth(ctx, provider, profile, clientInfo(c))
		if err != nil {
			h.writeError(c, err)
			return
		}

		// Set the refresh cookie and hand off to the frontend, which exchanges
		// it for an access token via /auth/refresh.
		h.setRefreshCookie(c, res.RefreshToken)
		c.Redirect(http.StatusTemporaryRedirect, h.cfg.AppBaseURL+"/dashboard")
	}
}

// --- Helpers ----------------------------------------------------------------

func (h *Handler) respondAuth(c *gin.Context, res *Result, status int) {
	h.setRefreshCookie(c, res.RefreshToken)
	c.JSON(status, authResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   int(h.cfg.AccessTokenTTL.Seconds()),
		User:        res.User,
		WorkspaceID: res.WorkspaceID.String(),
		Role:        res.Role,
	})
}

func (h *Handler) setRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		refreshCookieName,
		token,
		int(h.cfg.RefreshTokenTTL.Seconds()),
		"/api/v1/auth",
		"",
		h.cfg.IsProduction(), // Secure only over HTTPS in production.
		true,                 // HttpOnly.
	)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, "/api/v1/auth", "", h.cfg.IsProduction(), true)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrWeakPassword):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrInvalidRefresh), errors.Is(err, ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUnsupportedProvider):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func clientInfo(c *gin.Context) ClientInfo {
	return ClientInfo{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

func oauthStateKey(state string) string { return "oauth:state:" + state }
