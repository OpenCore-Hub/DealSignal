// Package analytics exposes analytics and heat-score HTTP endpoints.
package analytics

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/signal"
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
	g.GET("/documents/scores", h.ListDocumentScores)
	g.GET("/documents/:documentId/score", h.GetDocumentScore)

	r.GET("/dashboard/stats", h.GetDashboardStats)
	r.GET("/insights/overview", h.GetInsightsOverview)
	r.GET("/insights/overview/export", h.ExportInsightsOverview)
	r.GET("/insights/access-audit", h.GetAccessAudit)
	r.GET("/insights/key-pages", h.GetKeyPageCompliance)
	r.GET("/insights/key-page-settings", h.GetKeyPageSettings)
	r.PUT("/insights/key-page-settings", h.PutKeyPageSettings)
	r.GET("/insights/pages/:documentId", h.GetPageAnalytics)
	r.GET("/insights/documents/:documentId/visitors", h.GetDocumentVisitors)
	r.GET("/insights/documents/:documentId/funnel", h.GetDocumentReadingFunnel)
	r.GET("/insights/documents/:documentId/sessions", h.GetDocumentReadingSessions)
	r.POST("/events", h.RecordViewerEvent)
}

// GetScore returns the heat score for a link.
func (h *Handler) GetScore(c *gin.Context) {
	linkID := c.Param("linkId")
	workspaceID := middleware.WorkspaceIDFrom(c)

	override := circleOverrideFromQuery(c)
	score, err := h.service.GetScore(c.Request.Context(), pgUUID(linkID), pgUUID(workspaceID), override)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": httpx.SafeMessage("link_not_found", err)})
		return
	}

	circle := heat.CircleDefault
	if rs, rsErr := h.service.loadWorkspaceRuleSet(c.Request.Context(), workspaceID, override); rsErr == nil {
		circle = rs.Circle
	}

	c.JSON(http.StatusOK, gin.H{
		"linkId":    linkID,
		"score":     score.Score,
		"level":     score.Level,
		"trend":     score.Trend,
		"circle":    string(circle),
		"breakdown": score.Breakdown,
		"keyPages":  documentKeyPagesJSON(h.service.LinkKeyPageEvidence(c.Request.Context(), pgUUID(linkID), pgUUID(workspaceID), override)),
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

// ListDocumentScores returns workspace document-native heat for library overlay.
func (h *Handler) ListDocumentScores(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	docs, err := h.service.ListDocumentHeatScores(c.Request.Context(), pgUUID(workspaceID), circleOverrideFromQuery(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"documents": documentScoreList(docs)})
}

// GetDocumentScore returns document-native heat for Insights explain.
func (h *Handler) GetDocumentScore(c *gin.Context) {
	documentID := c.Param("documentId")
	workspaceID := middleware.WorkspaceIDFrom(c)

	score, err := h.service.GetDocumentHeatScore(c.Request.Context(), pgUUID(documentID), pgUUID(workspaceID), circleOverrideFromQuery(c))
	if err != nil {
		if errors.Is(err, ErrDocumentHeatNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "document_not_found", "message": httpx.SafeMessage("document_not_found", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	links := make([]gin.H, 0, len(score.ContributingLinks))
	for _, l := range score.ContributingLinks {
		item := gin.H{
			"id":        l.ID,
			"name":      l.Name,
			"pageViews": l.PageViews,
			"shareKind": l.ShareKind,
		}
		if l.DealRoomID != "" {
			item["dealRoomId"] = l.DealRoomID
		}
		if l.HasDocumentScope {
			item["hasDocumentScope"] = true
		}
		links = append(links, item)
	}
	c.JSON(http.StatusOK, gin.H{
		"documentId": score.DocumentID,
		"title":      score.Title,
		"score":      score.Score,
		"level":      score.Level,
		"trend":      score.Trend,
		"circle":     score.Circle,
		"breakdown":  score.Breakdown,
		"overlay": gin.H{
			"readingDepth": score.Overlay.ReadingDepth,
			"qaCitations":  score.Overlay.QACitations,
			"crossDomain":  score.Overlay.CrossDomain,
		},
		"views":             score.Views,
		"contributingLinks": links,
		"keyPages":          documentKeyPagesJSON(score.KeyPages),
		"updatedAt":         time.Now().UTC().Format(time.RFC3339),
	})
}

func documentKeyPagesJSON(kp DocumentHeatKeyPages) gin.H {
	pages := make([]gin.H, 0, len(kp.Pages))
	for _, p := range kp.Pages {
		pages = append(pages, gin.H{
			"pageNumber":   p.PageNumber,
			"title":        heat.DisplayablePageTitle(p.Title),
			"engagedViews": p.EngagedViews,
			"totalViews":   p.TotalViews,
		})
	}
	return gin.H{
		"engaged":    kp.Engaged,
		"total":      kp.Total,
		"minSeconds": kp.MinSeconds,
		"pages":      pages,
	}
}

// GetDashboardStats returns workspace-level dashboard data.
func (h *Handler) GetDashboardStats(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	userID := middleware.UserIDFrom(c)
	stats, err := h.service.DashboardStats(c.Request.Context(), workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hotCount":         stats.HotCount,
		"warmCount":        stats.WarmCount,
		"coldCount":        stats.ColdCount,
		"weeklyVisitors":   stats.WeeklyVisitors,
		"pendingQuestions": stats.PendingQuestions,
		"recentDocuments":  documentList(stats.RecentDocuments),
		"recentLinks":      linkOverviewList(c, h.cfg, stats.RecentLinks),
		"heatAlerts":       heatAlertList(stats.Signals),
		"riskAlerts":       riskAlertList(stats.Signals),
		"signals":          signalFeedList(stats.Signals),
		"actionItems":      actionItemList(stats.Actions),
		"recentActivities": activityItemList(stats.RecentActivities),
	})
}

// GetInsightsOverview returns discovery analytics.
func (h *Handler) GetInsightsOverview(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	overview, err := h.service.InsightsOverviewQuery(c.Request.Context(), workspaceID, insightsRangeFromQuery(c))
	if err != nil {
		if httpx.WriteIfPlanLimit(c, err) {
			return
		}
		if errors.Is(err, errInsightsRangeInvalid) || errors.Is(err, errInsightsRangeTooLong) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		// tierCounts are link-level heat.Compute buckets (founder circle).
		"tierEntity":                          "link",
		"tierCounts":                          overview.TierCounts,
		"activeLinkCount":                     overview.ActiveLinkCount,
		"rangeDays":                           overview.RangeDays,
		"rangeFrom":                           overview.RangeFrom,
		"rangeTo":                             overview.RangeTo,
		"rangeCustom":                         overview.RangeCustom,
		"generatedAt":                         overview.GeneratedAt.UTC().Format(time.RFC3339),
		"eventRetentionDays":                  overview.EventRetentionDays,
		"pageViewRetentionDays":               overview.PageViewRetentionDays,
		"periodOpens":                         overview.PeriodOpens,
		"previousPeriodOpens":                 overview.PreviousPeriodOpens,
		"periodUniqueVisitors":                overview.PeriodUniqueVisitors,
		"previousPeriodUniqueVisitors":        overview.PreviousPeriodUniqueVisitors,
		"periodMedianDurationSeconds":         overview.PeriodMedianDurationSeconds,
		"previousPeriodMedianDurationSeconds": overview.PreviousPeriodMedianDurationSeconds,
		"periodAvgDurationSeconds":            overview.PeriodAvgDurationSeconds,
		"periodPageViewCount":                 overview.PeriodPageViewCount,
		"periodSessionCount":                  overview.PeriodSessionCount,
		"periodMeasurableSessions":            overview.PeriodMeasurableSessions,
		"periodCompletedSessions":             overview.PeriodCompletedSessions,
		"periodCompletionRate":                overview.PeriodCompletionRate,
		"previousPeriodSessionCount":          overview.PreviousPeriodSessionCount,
		"previousPeriodCompletedSessions":     overview.PreviousPeriodCompletedSessions,
		"previousPeriodCompletionRate":        overview.PreviousPeriodCompletionRate,
		"openSignalCount":                     overview.OpenSignalCount,
		"dailyVisits":                         dailyVisitList(overview.DailyVisits),
		// topLinks: lifetime link heat. topDocuments: attributed page_views.
		// topContacts feeds Deal Radar recent-visitors; Insights Overview CTA uses openSignalCount.
		"topDocuments": documentScoreList(overview.TopDocuments),
		"topLinks":     linkScoreList(c, h.cfg, overview.TopLinks),
		"topContacts":  contactScoreList(overview.TopContacts),
		"scenarioPack": overview.ScenarioPack,
	})
}

// ExportInsightsOverview downloads the selected-range daily series as CSV.
func (h *Handler) ExportInsightsOverview(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	overview, err := h.service.InsightsOverviewQuery(c.Request.Context(), workspaceID, insightsRangeFromQuery(c))
	if err != nil {
		if httpx.WriteIfPlanLimit(c, err) {
			return
		}
		if errors.Is(err, errInsightsRangeInvalid) || errors.Is(err, errInsightsRangeTooLong) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	filename := fmt.Sprintf("insights-daily-%s_%s.csv", overview.RangeFrom, overview.RangeTo)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Status(http.StatusOK)
	_, _ = c.Writer.WriteString("date,opens,unique_visitors\n")
	for _, d := range overview.DailyVisits {
		day := d.Date
		if t, err := time.Parse(time.RFC3339, d.Date); err == nil {
			day = t.UTC().Format("2006-01-02")
		}
		_, _ = fmt.Fprintf(c.Writer, "%s,%d,%d\n", day, d.Opens, d.UniqueVisitors)
	}
}

func insightsDaysFromQuery(c *gin.Context) int {
	raw := c.Query("days")
	if raw == "" {
		return insightsTrendDaysDefault
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return insightsTrendDaysDefault
	}
	return normalizeInsightsDays(n)
}

func insightsRangeFromQuery(c *gin.Context) InsightsRangeQuery {
	return InsightsRangeQuery{
		Days: insightsDaysFromQuery(c),
		From: strings.TrimSpace(c.Query("from")),
		To:   strings.TrimSpace(c.Query("to")),
	}
}

// insightsOptionalRangeFromQuery returns a range query only when the client
// explicitly sent days and/or from/to. Empty params mean lifetime aggregates
// (used by document viewer / legacy callers).
func insightsOptionalRangeFromQuery(c *gin.Context) (InsightsRangeQuery, bool) {
	from := strings.TrimSpace(c.Query("from"))
	to := strings.TrimSpace(c.Query("to"))
	daysRaw := strings.TrimSpace(c.Query("days"))
	if from == "" && to == "" && daysRaw == "" {
		return InsightsRangeQuery{}, false
	}
	return insightsRangeFromQuery(c), true
}

func resolveOptionalInsightsRange(c *gin.Context) (*InsightsRange, error) {
	q, ok := insightsOptionalRangeFromQuery(c)
	if !ok {
		return nil, nil
	}
	rng, err := resolveInsightsRange(q, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return &rng, nil
}

// GetKeyPageCompliance returns who viewed heat key (sensitive) pages.
func (h *Handler) GetKeyPageCompliance(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rq := insightsRangeFromQuery(c)
	q := KeyPageComplianceQuery{
		Days:   rq.Days,
		From:   rq.From,
		To:     rq.To,
		Circle: circleFromQuery(c),
	}
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			q.Limit = n
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			q.Offset = n
		}
	}
	report, err := h.service.KeyPageCompliance(c.Request.Context(), workspaceID, q)
	if err != nil {
		if errors.Is(err, errInsightsRangeInvalid) || errors.Is(err, errInsightsRangeTooLong) || strings.Contains(err.Error(), "invalid") {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	matchRules := make([]gin.H, 0, len(report.MatchRules))
	for _, rule := range report.MatchRules {
		kws := rule.Keywords
		if kws == nil {
			kws = []string{}
		}
		matchRules = append(matchRules, gin.H{"category": rule.Category, "keywords": kws})
	}
	byCategory := make([]gin.H, 0, len(report.ByCategory))
	for _, cat := range report.ByCategory {
		byCategory = append(byCategory, gin.H{"category": cat.Category, "count": cat.Count})
	}
	pages := make([]gin.H, 0, len(report.Pages))
	for _, p := range report.Pages {
		item := gin.H{
			"documentId":         p.DocumentID,
			"documentTitle":      p.DocumentTitle,
			"pageNumber":         p.PageNumber,
			"pageTitle":          p.PageTitle,
			"category":           p.Category,
			"views":              p.Views,
			"engagedViews":       p.EngagedViews,
			"uniqueVisitors":     p.UniqueVisitors,
			"avgDurationSeconds": p.AvgDurationSeconds,
		}
		if !p.LastViewedAt.IsZero() {
			item["lastViewedAt"] = p.LastViewedAt.UTC().Format(time.RFC3339)
		}
		pages = append(pages, item)
	}
	events := make([]gin.H, 0, len(report.Events))
	for _, e := range report.Events {
		item := gin.H{
			"id":              e.ID,
			"pageNumber":      e.PageNumber,
			"pageTitle":       e.PageTitle,
			"category":        e.Category,
			"documentTitle":   e.DocumentTitle,
			"durationSeconds": e.DurationSeconds,
			"createdAt":       e.CreatedAt.UTC().Format(time.RFC3339),
			"dealRoomName":    e.DealRoomName,
		}
		if e.LinkID != "" {
			item["linkId"] = e.LinkID
		}
		if e.DocumentID != "" {
			item["documentId"] = e.DocumentID
		}
		if e.VisitorID != "" {
			item["visitorId"] = e.VisitorID
		}
		if e.VisitorEmail != "" {
			item["visitorEmail"] = e.VisitorEmail
		}
		if e.DealRoomID != "" {
			item["dealRoomId"] = e.DealRoomID
		}
		events = append(events, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"rangeDays":      report.RangeDays,
		"rangeFrom":      report.RangeFrom,
		"rangeTo":        report.RangeTo,
		"rangeCustom":    report.RangeCustom,
		"circle":         report.Circle,
		"generatedAt":    report.GeneratedAt.UTC().Format(time.RFC3339),
		"totalViews":     report.TotalViews,
		"engagedViews":   report.EngagedViews,
		"uniqueVisitors": report.UniqueVisitors,
		"distinctPages":  report.DistinctPages,
		"matchRules":     matchRules,
		"byCategory":     byCategory,
		"pages":          pages,
		"events":         events,
		"hasMore":        report.HasMore,
		"limit":          report.Limit,
		"offset":         report.Offset,
	})
}

// GetKeyPageSettings returns workspace default circle + additive keywords + effective rules.
func (h *Handler) GetKeyPageSettings(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	userID := middleware.UserIDFrom(c)
	settings, err := h.service.GetKeyPageSettings(c.Request.Context(), workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, keyPageSettingsJSON(settings))
}

func keyPageSettingsJSON(settings KeyPageSettings) gin.H {
	matchRules := make([]gin.H, 0, len(settings.MatchRules))
	for _, rule := range settings.MatchRules {
		kws := rule.Keywords
		if kws == nil {
			kws = []string{}
		}
		matchRules = append(matchRules, gin.H{"category": rule.Category, "keywords": kws})
	}
	builtinRules := make([]gin.H, 0, len(settings.BuiltinRules))
	for _, rule := range settings.BuiltinRules {
		kws := rule.Keywords
		if kws == nil {
			kws = []string{}
		}
		builtinRules = append(builtinRules, gin.H{"category": rule.Category, "keywords": kws})
	}
	extras := settings.ExtraKeywords
	if extras == nil {
		extras = map[string][]string{}
	}
	out := gin.H{
		"defaultCircle": settings.DefaultCircle,
		"extraKeywords": extras,
		"builtinRules":  builtinRules,
		"matchRules":    matchRules,
		"canEdit":       settings.CanEdit,
	}
	if !settings.UpdatedAt.IsZero() {
		out["updatedAt"] = settings.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return out
}

type putKeyPageSettingsRequest struct {
	DefaultCircle string              `json:"defaultCircle"`
	ExtraKeywords map[string][]string `json:"extraKeywords"`
}

// PutKeyPageSettings upserts workspace key-page settings (owner/admin).
func (h *Handler) PutKeyPageSettings(c *gin.Context) {
	var req putKeyPageSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	workspaceID := middleware.WorkspaceIDFrom(c)
	userID := middleware.UserIDFrom(c)
	settings, err := h.service.SaveKeyPageSettings(c.Request.Context(), workspaceID, userID, KeyPageSettingsUpdate(req))
	if err != nil {
		if errors.Is(err, errKeyPageSettingsForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "only owner or admin can update key page settings"})
			return
		}
		if errors.Is(err, errKeyPageSettingsInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, keyPageSettingsJSON(settings))
}

// GetAccessAudit returns workspace permission/gate failure analytics.
func (h *Handler) GetAccessAudit(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rq := insightsRangeFromQuery(c)
	q := AccessAuditQuery{
		Days:       rq.Days,
		From:       rq.From,
		To:         rq.To,
		EventType:  c.Query("eventType"),
		DealRoomID: c.Query("dealRoomId"),
		MemberID:   c.Query("memberId"),
		FolderPath: c.Query("folderPath"),
	}
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			q.Limit = n
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			q.Offset = n
		}
	}
	audit, err := h.service.AccessAudit(c.Request.Context(), workspaceID, q)
	if err != nil {
		if errors.Is(err, errInsightsRangeInvalid) || errors.Is(err, errInsightsRangeTooLong) || strings.Contains(err.Error(), "invalid") {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	byType := make([]gin.H, 0, len(audit.ByType))
	for _, t := range audit.ByType {
		byType = append(byType, gin.H{"eventType": t.EventType, "count": t.Count})
	}
	byRoom := make([]gin.H, 0, len(audit.ByDealRoom))
	for _, r := range audit.ByDealRoom {
		item := gin.H{"count": r.Count, "dealRoomName": r.DealRoomName}
		if r.DealRoomID != "" {
			item["dealRoomId"] = r.DealRoomID
		} else {
			item["dealRoomId"] = nil
			item["dealRoomName"] = ""
			item["scope"] = "library"
		}
		byRoom = append(byRoom, item)
	}
	byMember := make([]gin.H, 0, len(audit.ByMember))
	for _, m := range audit.ByMember {
		item := gin.H{"count": m.Count, "memberEmail": m.MemberEmail}
		if m.MemberID != "" {
			item["memberId"] = m.MemberID
		} else {
			item["memberId"] = nil
			item["scope"] = "unknown"
		}
		byMember = append(byMember, item)
	}
	byFolder := make([]gin.H, 0, len(audit.ByFolder))
	for _, f := range audit.ByFolder {
		item := gin.H{
			"folderPath":   f.FolderPath,
			"dealRoomName": f.DealRoomName,
			"count":        f.Count,
		}
		if f.DealRoomID != "" {
			item["dealRoomId"] = f.DealRoomID
		} else {
			item["dealRoomId"] = nil
		}
		if f.FolderPath == "" {
			item["scope"] = "root"
		}
		byFolder = append(byFolder, item)
	}
	events := make([]gin.H, 0, len(audit.Events))
	for _, e := range audit.Events {
		item := gin.H{
			"id":            e.ID,
			"eventType":     e.EventType,
			"createdAt":     e.CreatedAt.UTC().Format(time.RFC3339),
			"documentTitle": e.DocumentTitle,
			"dealRoomName":  e.DealRoomName,
			"folderPath":    e.FolderPath,
			"memberEmail":   e.MemberEmail,
		}
		if e.LinkID != "" {
			item["linkId"] = e.LinkID
		}
		if e.Email != "" {
			item["email"] = e.Email
		}
		if e.VisitorID != "" {
			item["visitorId"] = e.VisitorID
		}
		if e.Reason != "" {
			item["reason"] = e.Reason
		}
		if e.DealRoomID != "" {
			item["dealRoomId"] = e.DealRoomID
		}
		if e.MemberID != "" {
			item["memberId"] = e.MemberID
		}
		events = append(events, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"rangeDays":   audit.RangeDays,
		"rangeFrom":   audit.RangeFrom,
		"rangeTo":     audit.RangeTo,
		"rangeCustom": audit.RangeCustom,
		"generatedAt": audit.GeneratedAt.UTC().Format(time.RFC3339),
		"totalEvents": audit.TotalEvents,
		"byType":      byType,
		"byDealRoom":  byRoom,
		"byMember":    byMember,
		"byFolder":    byFolder,
		"events":      events,
		"hasMore":     audit.HasMore,
		"limit":       audit.Limit,
		"offset":      audit.Offset,
	})
}

// GetDocumentReadingFunnel returns session completion and page reach drop-off.
// Optional days/from/to filters by reading_sessions.last_activity_at; omit for lifetime.
func (h *Handler) GetDocumentReadingFunnel(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rng, err := resolveOptionalInsightsRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	funnel, err := h.service.DocumentReadingFunnelRange(c.Request.Context(), c.Param("documentId"), workspaceID, rng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, funnel)
}

// GetDocumentReadingSessions returns the idle-gap reading session timeline for a document.
// Optional days/from/to filters by last_activity_at; omit for lifetime.
func (h *Handler) GetDocumentReadingSessions(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rng, err := resolveOptionalInsightsRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	payload, err := h.service.DocumentReadingSessionsRange(c.Request.Context(), c.Param("documentId"), workspaceID, limit, rng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, payload)
}

// GetDocumentVisitors returns per-visitor engagement for a document.
// Optional days/from/to filters page_views by created_at; omit for lifetime.
func (h *Handler) GetDocumentVisitors(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rng, err := resolveOptionalInsightsRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	visitors, err := h.service.DocumentVisitorsRange(c.Request.Context(), c.Param("documentId"), workspaceID, rng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	out := make([]gin.H, len(visitors))
	for i, v := range visitors {
		out[i] = gin.H{
			"visitorId":          v.VisitorID,
			"visitorEmail":       v.VisitorEmail,
			"pageViewCount":      v.PageViewCount,
			"avgDurationSeconds": v.AvgDurationSeconds,
			"lastSeenAt":         v.LastSeenAt.Format(time.RFC3339),
		}
	}
	resp := gin.H{"data": out}
	if rng == nil {
		resp["lifetime"] = true
	} else {
		resp["rangeDays"] = rng.Days
		resp["rangeFrom"] = rng.From
		resp["rangeTo"] = rng.To
		if rng.Custom {
			resp["rangeCustom"] = true
		}
	}
	c.JSON(http.StatusOK, resp)
}

// GetPageAnalytics returns per-page metrics for a document.
// Optional days/from/to filters page_views by created_at; omit for lifetime.
func (h *Handler) GetPageAnalytics(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rng, err := resolveOptionalInsightsRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	rows, err := h.service.PageAnalyticsRange(c.Request.Context(), c.Param("documentId"), workspaceID, rng)
	if err != nil {
		slog.Error("GetPageAnalytics failed", "document_id", c.Param("documentId"), "workspace_id", workspaceID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	out := make([]gin.H, len(rows))
	for i, r := range rows {
		out[i] = gin.H{
			"pageNumber":         r.PageNumber,
			"viewCount":          r.ViewCount,
			"avgDurationSeconds": r.AvgDurationSeconds,
			"exitRate":           r.ExitRate,
			"title":              r.Title,
		}
	}
	resp := gin.H{"data": out}
	if rng == nil {
		resp["lifetime"] = true
	} else {
		resp["rangeDays"] = rng.Days
		resp["rangeFrom"] = rng.From
		resp["rangeTo"] = rng.To
		if rng.Custom {
			resp["rangeCustom"] = true
		}
	}
	c.JSON(http.StatusOK, resp)
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	visitorID := middleware.UserIDFrom(c)
	ip := c.ClientIP()
	ua := c.Request.UserAgent()

	err := h.service.RecordAuthenticatedEvent(c.Request.Context(), workspaceID, req.DocumentID, visitorID, "", ip, ua, req.EventType, req.PageNumber, req.DurationSeconds, req.ScrollDepth)
	if err != nil {
		if errors.Is(err, ErrNoLinkForDocument) {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
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

// circleOverrideFromQuery returns nil when circle is omitted so the workspace
// default_circle applies; otherwise forces the requested circle (+ shared extras).
func circleOverrideFromQuery(c *gin.Context) *heat.Circle {
	raw := c.Query("circle")
	if raw == "" {
		return nil
	}
	circle := heat.Circle(raw)
	switch circle {
	case heat.CircleFounder, heat.CircleInvestor, heat.CircleSales:
		return &circle
	default:
		return nil
	}
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

func documentList(docs []db.ListRecentDocumentsByWorkspaceRow) []gin.H {
	out := make([]gin.H, len(docs))
	for i, d := range docs {
		out[i] = documentItem(d)
	}
	return out
}

func documentItem(d db.ListRecentDocumentsByWorkspaceRow) gin.H {
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
		"fileSize":  d.FileSize.Int64,
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
		name := ""
		if l.Link.Name.Valid {
			name = strings.TrimSpace(l.Link.Name.String)
		}
		docTitle := strings.TrimSpace(l.DocumentTitle)
		item := gin.H{
			"id":            uuidToString(l.Link.ID),
			"name":          name,
			"title":         name,
			"documentTitle": docTitle,
			"shortUrl":      publicURL(c, cfg, l.Link.PublicToken),
			"views":         l.Link.AccessCount,
			"score":         l.Score,
			"heatLevel":     l.Level,
		}
		if l.Link.DocumentID.Valid {
			item["documentId"] = uuidToString(l.Link.DocumentID)
		}
		if l.Link.DealRoomID.Valid {
			item["dealRoomId"] = uuidToString(l.Link.DealRoomID)
		}
		if l.Link.HasDocumentScope {
			item["hasDocumentScope"] = true
		}
		if l.Link.LinkType != "" {
			item["linkType"] = l.Link.LinkType
		}
		item["shareKind"] = documentHeatShareKind(l.Link.DealRoomID.Valid, l.Link.HasDocumentScope)
		out[i] = item
	}
	return out
}

func documentScoreList(docs []DocumentScore) []gin.H {
	out := make([]gin.H, len(docs))
	for i, d := range docs {
		item := gin.H{
			"id":        uuidToString(d.ID),
			"title":     d.Title,
			"views":     d.Views,
			"score":     d.Score,
			"heatLevel": d.Level,
		}
		if d.PrimaryLinkID.Valid {
			item["primaryLinkId"] = uuidToString(d.PrimaryLinkID)
		}
		out[i] = item
	}
	return out
}

func dailyVisitList(points []DailyVisitPoint) []gin.H {
	out := make([]gin.H, len(points))
	for i, p := range points {
		out[i] = gin.H{
			"date":           p.Date,
			"opens":          p.Opens,
			"uniqueVisitors": p.UniqueVisitors,
		}
	}
	return out
}

func contactScoreList(contacts []ContactScore) []gin.H {
	out := make([]gin.H, len(contacts))
	for i, c := range contacts {
		item := gin.H{
			"id":        c.ID,
			"email":     c.Email,
			"score":     c.Score,
			"heatLevel": c.Level,
		}
		if c.LastSeenAt.Valid {
			item["lastSeenAt"] = c.LastSeenAt.Time.Format(time.RFC3339)
		}
		out[i] = item
	}
	return out
}

func unmarshalJSONB[T any](b []byte) (T, bool) {
	var v T
	if len(b) == 0 {
		return v, false
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, false
	}
	return v, true
}

func signalFeedList(signals []db.Signal) []gin.H {
	out := make([]gin.H, 0, len(signals))
	for _, s := range signals {
		out = append(out, signal.SignalItem(s))
	}
	return out
}

func actionItemList(actions []db.ActionItem) []gin.H {
	out := make([]gin.H, len(actions))
	for i, a := range actions {
		out[i] = signal.ActionItem(a)
	}
	return out
}

func activityItemList(activities []ActivityItem) []gin.H {
	out := make([]gin.H, len(activities))
	for i, a := range activities {
		out[i] = gin.H{
			"id":         a.ID,
			"eventType":  a.EventType,
			"actor":      a.Actor,
			"objectType": a.ObjectType,
			"objectName": a.ObjectName,
			"objectId":   a.ObjectID,
			"createdAt":  a.CreatedAt.Format(time.RFC3339),
		}
	}
	return out
}

func heatAlertList(signals []db.Signal) []gin.H {
	out := make([]gin.H, 0)
	for _, s := range signals {
		if s.Type != "hot_signal" {
			continue
		}
		item := gin.H{
			"id":            uuidToString(s.ID),
			"heatLevel":     s.Type,
			"score":         0,
			"suggestion":    s.Suggestion,
			"lastSeenAt":    s.CreatedAt.Time.Format(time.RFC3339),
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
		if s.Type != "risk_alert" {
			continue
		}
		alertType := s.Subtype.String
		if alertType == "" {
			alertType = "forward"
		}
		priority := s.Priority
		if priority == "" {
			priority = "medium"
		}
		item := gin.H{
			"id":          uuidToString(s.ID),
			"type":        alertType,
			"title":       s.Title,
			"description": s.Description,
			"priority":    priority,
			"createdAt":   s.CreatedAt.Time.Format(time.RFC3339),
		}
		if s.LinkID.Valid {
			item["linkId"] = uuidToString(s.LinkID)
		}
		if s.DocumentID.Valid {
			item["documentId"] = uuidToString(s.DocumentID)
		}
		if md, ok := unmarshalJSONB[map[string]string](s.Metadata); ok && len(md) > 0 {
			item["metadata"] = md
		}
		out = append(out, item)
	}
	return out
}

func publicURL(c *gin.Context, cfg *config.Config, token string) string {
	path := "/l/" + token
	base := strings.TrimSpace(cfg.ViewerBaseURL)
	if base == "" {
		base = strings.TrimSpace(c.Request.Header.Get("Origin"))
	}
	if base == "" {
		// Prefer relative viewer paths over inventing localhost hosts.
		return path
	}
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return path
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return path
	}
	return strings.TrimSuffix(base, "/") + path
}

func mapPermissionType(t string) string {
	switch strings.ToLower(t) {
	case "email_required":
		return "email"
	default:
		return t
	}
}
