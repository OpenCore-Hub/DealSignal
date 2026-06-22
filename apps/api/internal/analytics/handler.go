// Package analytics exposes analytics and heat-score HTTP endpoints.
package analytics

import (
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Handler exposes analytics endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates an analytics handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterWorkspaceRoutes mounts workspace analytics routes.
func (h *Handler) RegisterWorkspaceRoutes(r *gin.RouterGroup) {
	g := r.Group("/analytics")
	g.GET("/links/:linkId/score", h.GetScore)
}

// GetScore returns the heat score for a link.
func (h *Handler) GetScore(c *gin.Context) {
	linkID := c.Param("linkId")
	workspaceID := middleware.WorkspaceIDFrom(c)

	score, err := h.service.GetScore(c.Request.Context(), pgUUID(linkID), pgUUID(workspaceID), circleFromQuery(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"link_id":    linkID,
		"score":      score.Score,
		"level":      score.Level,
		"trend":      score.Trend,
		"factors":    score.Breakdown,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func circleFromQuery(c *gin.Context) heat.Circle {
	circle := heat.Circle(c.Query("circle"))
	if circle == "" {
		return heat.CircleDefault
	}
	return circle
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}
