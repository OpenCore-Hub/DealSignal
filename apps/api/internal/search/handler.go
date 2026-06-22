// Package search exposes the document search HTTP endpoint.
package search

import (
	"net/http"
	"strconv"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Handler exposes search endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a search handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts the search route under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/search", h.Search)
}

// Search returns matching chunks for a query.
func (h *Handler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "query parameter q is required"})
		return
	}

	limit := defaultTopK
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	evidence, err := h.service.Search(c.Request.Context(), pgUUID(workspaceID), q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "search_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"query": q, "evidence": evidence})
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}
