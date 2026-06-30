// Package ai implements the AI Debugging Service per the AI Debugging Service
// Architecture Bible: explain errors/logs, analyze requests/replays, generate
// incidents, and serve analyses/incidents/reports/history.
//
// The orchestrator uses Gemini as the primary provider and Claude as the
// secondary/fallback (with a local heuristic stub when no keys are set), and
// every model output is structured JSON with a 0–100 confidence score.
package ai

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/internal/requests"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

// Handler holds dependencies for AI debugging endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs an ai Handler.
func NewHandler(cfg *config.Config, db *database.DB, log *slog.Logger) *Handler {
	return &Handler{service: NewService(cfg, db, log)}
}

// RegisterRoutes mounts the AI debugging endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	a := rg.Group("/ai")
	{
		a.POST("/explain-error", h.explainError)
		a.POST("/explain-logs", h.explainLogs)
		a.POST("/analyze-request", h.analyzeRequest)
		a.POST("/analyze-replay", h.analyzeReplay)
		a.POST("/generate-incident", h.generateIncident)

		a.GET("/analyses", h.listAnalyses)
		a.GET("/incidents", h.listIncidents)
		a.GET("/reports", h.listReports)
		a.GET("/history", h.history)
	}
}

type requestIDBody struct {
	RequestID string `json:"request_id" binding:"required"`
}

func (h *Handler) explainError(c *gin.Context)   { h.runRequestOp(c, h.service.ExplainError) }
func (h *Handler) analyzeRequest(c *gin.Context) { h.runRequestOp(c, h.service.AnalyzeRequest) }
func (h *Handler) analyzeReplay(c *gin.Context)  { h.runRequestOp(c, h.service.AnalyzeReplay) }

// runRequestOp handles the request-id-based analysis endpoints.
func (h *Handler) runRequestOp(c *gin.Context, op func(ctx context.Context, userID, requestID uuid.UUID) (*StoredAnalysis, error)) {
	var body requestIDBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	requestID, err := uuid.Parse(body.RequestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request_id"})
		return
	}
	res, err := op(c, currentUserID(c), requestID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) explainLogs(c *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id" binding:"required"`
		Logs      string `json:"logs" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	projectID, err := uuid.Parse(body.ProjectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}
	res, err := h.service.ExplainLogs(c, currentUserID(c), projectID, body.Logs)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) generateIncident(c *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	projectID, err := uuid.Parse(body.ProjectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}
	res, err := h.service.GenerateIncident(c, currentUserID(c), projectID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) listAnalyses(c *gin.Context) {
	projectID, ok := projectIDQuery(c)
	if !ok {
		return
	}
	items, err := h.service.ListAnalyses(c, currentUserID(c), projectID, 0)
	if err != nil {
		writeError(c, err)
		return
	}
	if items == nil {
		items = []AnalysisRow{}
	}
	c.JSON(http.StatusOK, gin.H{"analyses": items})
}

func (h *Handler) listIncidents(c *gin.Context) {
	projectID, ok := projectIDQuery(c)
	if !ok {
		return
	}
	items, err := h.service.ListIncidents(c, currentUserID(c), projectID, 0)
	if err != nil {
		writeError(c, err)
		return
	}
	if items == nil {
		items = []IncidentRow{}
	}
	c.JSON(http.StatusOK, gin.H{"incidents": items})
}

// listReports returns AI reports. Periodic report generation is future work
// (AI Bible — Reports); for now this returns incidents as the report feed.
func (h *Handler) listReports(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"reports": []any{}})
}

// history aliases the analyses feed for a project.
func (h *Handler) history(c *gin.Context) { h.listAnalyses(c) }

func projectIDQuery(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return uuid.Nil, false
	}
	return id, true
}

func currentUserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(middleware.ContextClaims); ok {
		return v.(*security.Claims).UserID
	}
	return uuid.Nil
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, requests.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, ErrNoData):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no recent failed requests to analyze"})
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	}
}
