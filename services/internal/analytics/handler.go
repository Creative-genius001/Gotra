package analytics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

// Handler serves analytics queries.
type Handler struct {
	store *Store
	db    *database.DB
}

// NewHandler constructs an analytics Handler.
func NewHandler(store *Store, db *database.DB) *Handler {
	return &Handler{store: store, db: db}
}

// RegisterRoutes mounts the analytics endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/analytics", h.summary)
}

func (h *Handler) summary(c *gin.Context) {
	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return
	}

	userID := uuid.Nil
	if v, ok := c.Get(middleware.ContextClaims); ok {
		userID = v.(*security.Claims).UserID
	}
	if !h.isMember(c, userID, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	hours := 24
	if v, err := strconv.Atoi(c.Query("hours")); err == nil && v > 0 && v <= 24*30 {
		hours = v
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	summary, err := h.store.QuerySummary(c, projectID, since)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) isMember(c *gin.Context, userID, projectID uuid.UUID) bool {
	var exists bool
	err := h.db.Pool.QueryRow(c,
		`SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`,
		projectID, userID,
	).Scan(&exists)
	return err == nil && exists
}
