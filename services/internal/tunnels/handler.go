// Package tunnels implements tunnel registry, request capture and replay
// endpoints per the Backend Bible. Tunnel CRUD is implemented; request
// inspection and replay are stubbed until the capture engine lands.
package tunnels

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/billing"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

// Handler holds dependencies for tunnel, request and replay endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a tunnels Handler. quota may be nil to disable limits.
func NewHandler(db *database.DB, quota QuotaChecker) *Handler {
	return &Handler{service: NewService(NewRepository(db.Pool), quota)}
}

// RegisterRoutes mounts tunnel, request-inspection and replay endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	t := rg.Group("/tunnels")
	{
		t.GET("", h.listTunnels)
		t.POST("", h.createTunnel)
		t.GET("/:id", h.getTunnel)
		t.DELETE("/:id", h.deleteTunnel)
	}
	// Request inspection & replay are served by the requests module.
}

func (h *Handler) listTunnels(c *gin.Context) {
	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return
	}
	items, err := h.service.List(c.Request.Context(), currentUserID(c), projectID)
	if err != nil {
		writeError(c, err)
		return
	}
	if items == nil {
		items = []Tunnel{}
	}
	c.JSON(http.StatusOK, gin.H{"tunnels": items})
}

func (h *Handler) createTunnel(c *gin.Context) {
	var req struct {
		ProjectID string `json:"project_id" binding:"required"`
		LocalPort int    `json:"local_port" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}
	t, err := h.service.Create(c.Request.Context(), currentUserID(c), projectID, req.LocalPort)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *Handler) getTunnel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tunnel id"})
		return
	}
	t, err := h.service.Get(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *Handler) deleteTunnel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tunnel id"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), currentUserID(c), id); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func currentUserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(middleware.ContextClaims); ok {
		return v.(*security.Claims).UserID
	}
	return uuid.Nil
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, billing.ErrQuotaExceeded):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "tunnel limit reached for your plan — upgrade in Billing"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
