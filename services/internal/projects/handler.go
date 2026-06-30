// Package projects implements project/workspace management per the Backend Bible.
package projects

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

// Handler holds dependencies for project endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a projects Handler.
func NewHandler(db *database.DB) *Handler {
	return &Handler{service: NewService(NewRepository(db.Pool))}
}

// RegisterRoutes mounts authenticated project endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	p := rg.Group("/projects")
	{
		p.GET("", h.list)
		p.POST("", h.create)
		p.GET("/:id", h.get)
		p.DELETE("/:id", h.delete)
	}
}

func (h *Handler) list(c *gin.Context) {
	userID := currentUserID(c)
	items, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	if items == nil {
		items = []Project{}
	}
	c.JSON(http.StatusOK, gin.H{"projects": items})
}

func (h *Handler) create(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)

	p, err := h.service.Create(c.Request.Context(), currentUserID(c), req.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	p, err := h.service.Get(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), currentUserID(c), id); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// currentUserID extracts the authenticated user id from JWT claims.
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
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
