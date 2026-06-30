package billing

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

// Handler serves billing endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a billing Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts the authenticated billing endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	b := rg.Group("/billing")
	{
		b.GET("", h.current)
		b.POST("/plan", h.changePlan)
	}
}

// RegisterPublic mounts the unauthenticated Stripe webhook (verified by signature).
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.POST("/billing/webhook", h.webhook)
}

func (h *Handler) webhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}
	if err := h.service.ApplyWebhook(payload, c.GetHeader("Stripe-Signature")); err != nil {
		// Ignore events we don't handle / stub processor; reject bad signatures.
		if errors.Is(err, ErrNoWebhook) || errors.Is(err, ErrUnhandledEvent) {
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) current(c *gin.Context) {
	info, err := h.service.Current(c, currentUserID(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h *Handler) changePlan(c *gin.Context) {
	var body struct {
		Plan string `json:"plan" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	info, err := h.service.ChangePlan(c, currentUserID(c), body.Plan)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func currentUserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(middleware.ContextClaims); ok {
		return v.(*security.Claims).UserID
	}
	return uuid.Nil
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidPlan):
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown plan"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
