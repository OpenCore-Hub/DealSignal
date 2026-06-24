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
	r.POST("/search", h.SearchPost)
}

// Search handles GET /search?q=...&limit=...
func (h *Handler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "query parameter q is required"})
		return
	}

	limit := parseLimit(c.Query("limit"))
	workspaceID := middleware.WorkspaceIDFrom(c)

	evidence, err := h.service.Search(c.Request.Context(), pgUUID(workspaceID), q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "search_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"query": q, "evidence": evidence})
}

// SearchPost handles POST /search with JSON body supporting document_id filtering.
func (h *Handler) SearchPost(c *gin.Context) {
	var req struct {
		Query      string `json:"query"`
		DocumentID string `json:"document_id,omitempty"`
		Mode       string `json:"mode,omitempty"`
		TopK       int    `json:"top_k,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "query is required"})
		return
	}

	limit := req.TopK
	if limit <= 0 {
		limit = defaultTopK
	}
	if limit > maxTopK {
		limit = maxTopK
	}

	workspaceID := middleware.WorkspaceIDFrom(c)

	evidence, err := h.service.Search(c.Request.Context(), pgUUID(workspaceID), req.Query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "search_error", "message": err.Error()})
		return
	}

	// Filter by document_id if specified
	if req.DocumentID != "" {
		filtered := make([]Evidence, 0, len(evidence))
		for _, e := range evidence {
			if e.DocumentID == req.DocumentID {
				filtered = append(filtered, e)
			}
		}
		evidence = filtered
	}

	// Return both "evidence" and "results" for backward compatibility
	c.JSON(http.StatusOK, gin.H{
		"query":       req.Query,
		"document_id": req.DocumentID,
		"evidence":    evidence,
		"results":     evidence,
	})
}

func parseLimit(s string) int {
	limit := defaultTopK
	if s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxTopK {
		limit = maxTopK
	}
	return limit
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}
