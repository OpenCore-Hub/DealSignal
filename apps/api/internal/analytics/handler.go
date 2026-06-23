// Package analytics exposes analytics and heat-score HTTP endpoints.
package analytics

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Handler exposes analytics endpoints.
type Handler struct {
	service *Service
	cfg     *config.Config
}

// NewHandler creates an analytics handler.
func NewHandler(s *Service, cfg *config.Config) *Handler {
	return &Handler{service: s, cfg: cfg}
}

// RegisterWorkspaceRoutes mounts workspace analytics routes.
func (h *Handler) RegisterWorkspaceRoutes(r *gin.RouterGroup) {
	g := r.Group("/analytics")
	g.GET("/links/:linkId/score", h.GetScore)

	r.GET("/dashboard/stats", h.GetDashboardStats)
	r.GET("/insights/overview", h.GetInsightsOverview)
	r.GET("/insights/pages/:documentId", h.GetPageAnalytics)
	r.POST("/events", h.RecordViewerEvent)
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
		"linkId":    linkID,
		"score":     score.Score,
		"level":     score.Level,
		"trend":     score.Trend,
		"breakdown": score.Breakdown,
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetDashboardStats returns workspace-level dashboard data.
func (h *Handler) GetDashboardStats(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	stats, err := h.service.DashboardStats(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hotCount":        stats.HotCount,
		"warmCount":       stats.WarmCount,
		"coldCount":       stats.ColdCount,
		"recentDocuments": documentList(stats.RecentDocuments),
		"recentLinks":     linkOverviewList(c, h.cfg, stats.RecentLinks),
		"heatAlerts":      heatAlertList(stats.Signals),
		"riskAlerts":      riskAlertList(stats.Signals),
		"signals":         signalFeedList(stats.Signals),
		"actionItems":     actionItemList(stats.Actions),
	})
}

// GetInsightsOverview returns discovery analytics.
func (h *Handler) GetInsightsOverview(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	overview, err := h.service.InsightsOverview(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tierCounts":   overview.TierCounts,
		"topDocuments": documentScoreList(overview.TopDocuments),
		"topLinks":     linkScoreList(c, h.cfg, overview.TopLinks),
		"topContacts":  contactScoreList(overview.TopContacts),
	})
}

// GetPageAnalytics returns per-page metrics for a document.
func (h *Handler) GetPageAnalytics(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rows, err := h.service.PageAnalytics(c.Request.Context(), c.Param("documentId"), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	out := make([]gin.H, len(rows))
	for i, r := range rows {
		exitRate := 0.0
		if r.ViewCount > 0 && r.PageNumber > 1 {
			exitRate = 0.1
		}
		out[i] = gin.H{
			"pageNumber":         r.PageNumber,
			"viewCount":          r.ViewCount,
			"avgDurationSeconds": r.AvgDurationSeconds,
			"exitRate":           exitRate,
			"title":              nil,
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

type viewerEventRequest struct {
	DocumentID      string  `json:"documentId" binding:"required,uuid"`
	EventType       string  `json:"eventType" binding:"required,oneof=page_viewed download_attempted"`
	PageNumber      int32   `json:"pageNumber"`
	DurationSeconds int32   `json:"durationSeconds"`
	ScrollDepth     float64 `json:"scrollDepth"`
}

// RecordViewerEvent records an authenticated viewer event (page view / download).
func (h *Handler) RecordViewerEvent(c *gin.Context) {
	var req viewerEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	visitorID := middleware.UserIDFrom(c)
	ip := c.ClientIP()
	ua := c.Request.UserAgent()

	err := h.service.RecordAuthenticatedEvent(c.Request.Context(), workspaceID, req.DocumentID, visitorID, "", ip, ua, req.EventType, req.PageNumber, req.DurationSeconds, req.ScrollDepth)
	if err != nil {
		if errors.Is(err, ErrNoLinkForDocument) {
			c.JSON(http.StatusNotFound, gin.H{"code": "no_link_for_document", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
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

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func documentList(docs []db.Document) []gin.H {
	out := make([]gin.H, len(docs))
	for i, d := range docs {
		out[i] = documentItem(d)
	}
	return out
}

func documentItem(d db.Document) gin.H {
	status := d.Status
	progress := 50
	if status == "ready" {
		progress = 100
	} else if status == "failed" {
		progress = 0
	}
	item := gin.H{
		"id":        uuidToString(d.ID),
		"title":     d.Title,
		"fileName":  d.Title,
		"fileType":  strings.ToLower(d.SourceType),
		"fileSize":  0,
		"status":    status,
		"progress":  progress,
		"createdAt": d.CreatedAt.Time.Format(time.RFC3339),
		"updatedAt": d.UpdatedAt.Time.Format(time.RFC3339),
	}
	if d.PageCount.Valid {
		item["pageCount"] = d.PageCount.Int32
	}
	return item
}

func linkOverviewList(c *gin.Context, cfg *config.Config, links []LinkOverview) []gin.H {
	out := make([]gin.H, len(links))
	for i, l := range links {
		out[i] = linkOverviewItem(c, cfg, l)
	}
	return out
}

func linkOverviewItem(c *gin.Context, cfg *config.Config, l LinkOverview) gin.H {
	now := time.Now()
	isActive := l.Link.Status == "active" && (!l.Link.ExpiresAt.Valid || l.Link.ExpiresAt.Time.After(now))
	item := gin.H{
		"id":                 uuidToString(l.Link.ID),
		"documentId":         uuidToString(l.Link.DocumentID),
		"documentTitle":      l.DocumentTitle,
		"shortUrl":           publicURL(c, cfg, l.Link.PublicToken),
		"accessCount":        l.Link.AccessCount,
		"heatLevel":          l.Level,
		"status":             l.Link.Status,
		"createdAt":          l.Link.CreatedAt.Time.Format(time.RFC3339),
		"isActive":           isActive,
		"permissionType":     mapPermissionType(l.Link.PermissionType),
		"avgDurationSeconds": int(l.AvgDurationSeconds),
	}
	if l.Link.ExpiresAt.Valid {
		item["expiresAt"] = l.Link.ExpiresAt.Time.Format(time.RFC3339)
	}
	if l.LastViewedAt.Valid {
		item["lastViewedAt"] = l.LastViewedAt.Time.Format(time.RFC3339)
	}
	return item
}

func linkScoreList(c *gin.Context, cfg *config.Config, links []LinkScore) []gin.H {
	out := make([]gin.H, len(links))
	for i, l := range links {
		out[i] = gin.H{
			"id":        uuidToString(l.Link.ID),
			"shortUrl":  publicURL(c, cfg, l.Link.PublicToken),
			"views":     l.Link.AccessCount,
			"heatLevel": l.Level,
		}
	}
	return out
}

func documentScoreList(docs []DocumentScore) []gin.H {
	out := make([]gin.H, len(docs))
	for i, d := range docs {
		out[i] = gin.H{
			"id":        uuidToString(d.ID),
			"title":     d.Title,
			"views":     d.Views,
			"heatLevel": d.Level,
		}
	}
	return out
}

func contactScoreList(contacts []ContactScore) []gin.H {
	out := make([]gin.H, len(contacts))
	for i, c := range contacts {
		item := gin.H{
			"id":    c.Email,
			"email": c.Email,
			"score": c.Score,
			"heatLevel": c.Level,
		}
		if c.LastSeenAt.Valid {
			item["lastSeenAt"] = c.LastSeenAt.Time.Format(time.RFC3339)
		}
		out[i] = item
	}
	return out
}

func signalFeedList(signals []db.Signal) []gin.H {
	out := make([]gin.H, 0, len(signals))
	for _, s := range signals {
		item := gin.H{
			"id":          uuidToString(s.ID),
			"type":        s.Type,
			"title":       s.Title,
			"description": s.Description,
			"explanation": s.Explanation,
			"suggestion":  s.Suggestion,
			"priority":    s.Priority,
			"createdAt":   s.CreatedAt.Time.Format(time.RFC3339),
		}
		if s.DocumentID.Valid {
			item["documentId"] = uuidToString(s.DocumentID)
		}
		if s.ContactID.Valid {
			item["contactId"] = uuidToString(s.ContactID)
		}
		if s.LinkID.Valid {
			item["linkId"] = uuidToString(s.LinkID)
		}
		out = append(out, item)
	}
	return out
}

func actionItemList(actions []db.ActionItem) []gin.H {
	out := make([]gin.H, len(actions))
	for i, a := range actions {
		out[i] = gin.H{
			"id":         uuidToString(a.ID),
			"signalId":   uuidToString(a.SignalID),
			"title":      a.Title,
			"impact":     a.Impact,
			"dueAt":      a.DueAt.Time.Format(time.RFC3339),
			"status":     a.Status,
			"actionType": a.ActionType,
			"createdAt":  a.CreatedAt.Time.Format(time.RFC3339),
			"updatedAt":  a.UpdatedAt.Time.Format(time.RFC3339),
		}
	}
	return out
}

func heatAlertList(signals []db.Signal) []gin.H {
	out := make([]gin.H, 0)
	for _, s := range signals {
		if s.Type != "hot" && s.Type != "warm" {
			continue
		}
		item := gin.H{
			"id":          uuidToString(s.ID),
			"heatLevel":   s.Type,
			"score":       0,
			"suggestion":  s.Suggestion,
			"lastSeenAt":  s.CreatedAt.Time.Format(time.RFC3339),
			"documentTitle": "",
			"visitorEmail":  "",
		}
		if s.LinkID.Valid {
			item["linkId"] = uuidToString(s.LinkID)
		}
		out = append(out, item)
	}
	return out
}

func riskAlertList(signals []db.Signal) []gin.H {
	out := make([]gin.H, 0)
	for _, s := range signals {
		if s.Type != "risk" {
			continue
		}
		alertType := "forward"
		if strings.Contains(strings.ToLower(s.Title), "download") {
			alertType = "download"
		} else if strings.Contains(strings.ToLower(s.Title), "expir") {
			alertType = "expired"
		}
		item := gin.H{
			"id":          uuidToString(s.ID),
			"type":        alertType,
			"title":       s.Title,
			"description": s.Description,
			"createdAt":   s.CreatedAt.Time.Format(time.RFC3339),
		}
		if s.LinkID.Valid {
			item["linkId"] = uuidToString(s.LinkID)
		}
		if s.DocumentID.Valid {
			item["documentId"] = uuidToString(s.DocumentID)
		}
		out = append(out, item)
	}
	return out
}

func publicURL(c *gin.Context, cfg *config.Config, token string) string {
	base := cfg.ViewerBaseURL
	if base == "" {
		base = c.Request.Header.Get("Origin")
	}
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := c.Request.Host
		if host == "" {
			host = "localhost"
		}
		base = scheme + "://" + host
	}
	return strings.TrimSuffix(base, "/") + "/l/" + token
}

func mapPermissionType(t string) string {
	switch strings.ToLower(t) {
	case "email_required":
		return "email"
	default:
		return t
	}
}
