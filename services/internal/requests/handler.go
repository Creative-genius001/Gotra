package requests

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

// Handler exposes request inspection and replay endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a requests Handler.
func NewHandler(cfg *config.Config, db *database.DB) *Handler {
	return &Handler{service: NewService(cfg, NewRepository(db.Pool))}
}

// RegisterRoutes mounts the request inspection & replay endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	r := rg.Group("/requests")
	{
		r.GET("", h.list)
		r.GET("/:id", h.get)
		r.POST("/:id/replay", h.replay)
	}
}

func (h *Handler) list(c *gin.Context) {
	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.service.List(c.Request.Context(), currentUserID(c), projectID, limit)
	if err != nil {
		writeError(c, err)
		return
	}
	if items == nil {
		items = []Summary{}
	}
	c.JSON(http.StatusOK, gin.H{"requests": items})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}
	d, err := h.service.Get(c.Request.Context(), currentUserID(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *Handler) replay(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}
	var in ReplayInput
	_ = c.ShouldBindJSON(&in) // body optional; empty replays the original verbatim

	res, err := h.service.Replay(c.Request.Context(), currentUserID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
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
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	}
}
