package suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrLinkNotFound       = errors.New("link not found")
	ErrSuggestionNotFound = errors.New("suggestion not found")
)

// Suggestion is the public view of a generated suggestion.
type Suggestion struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
	ContactID   string `json:"contact_id,omitempty"`
	LinkID      string `json:"link_id"`
	DocumentID  string `json:"document_id,omitempty"`
	Type        string `json:"type"`
	Subtype     string `json:"subtype,omitempty"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Reason      string `json:"reason"`
	Action      string `json:"action"`
	Dismissed   bool   `json:"dismissed"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Notifier enqueues notifications for high-intent signals.
type Notifier interface {
	Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string) error
}

// Enricher optionally rewrites a candidate's reason and action via an LLM.
type Enricher interface {
	Enrich(ctx context.Context, input EnrichInput) (reason, action string, ok bool)
}

// Service generates follow-up suggestions from link analytics.
type Service struct {
	queries      *db.Queries
	notifier     Notifier
	enricher     Enricher
	ruleEngine   *RuleEngine
	featureStore *FeatureStore
}

// ServiceOption configures a suggestion service.
type ServiceOption func(*Service)

// WithFeatureStore enables the service to read pre-aggregated link features.
func WithFeatureStore(fs *FeatureStore) ServiceOption {
	return func(s *Service) { s.featureStore = fs }
}

// WithEnricher sets the optional LLM enricher.
func WithEnricher(enricher Enricher) ServiceOption {
	return func(s *Service) { s.enricher = enricher }
}

// NewService creates a suggestion service.
func NewService(q *db.Queries, n Notifier, ruleEngine *RuleEngine, opts ...ServiceOption) *Service {
	s := &Service{queries: q, notifier: n, ruleEngine: ruleEngine}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ScheduleGenerate writes a suggestion generation request to the outbox table
// so the HTTP handler can return immediately. A background worker will pick it up.
func (s *Service) ScheduleGenerate(ctx context.Context, link db.Link, lang string) error {
	_, err := s.queries.InsertSuggestionOutbox(ctx, db.InsertSuggestionOutboxParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
		Lang:        lang,
	})
	return err
}

// Generate creates suggestions for a link based on recent access events.
func (s *Service) Generate(ctx context.Context, workspaceID, linkID, lang string) ([]Suggestion, error) {
	start := time.Now()
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		recordSuggestionGenerationError("parse_workspace")
		return nil, err
	}
	linkUUID, err := pgUUID(linkID)
	if err != nil {
		recordSuggestionGenerationError("parse_link")
		return nil, err
	}

	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          linkUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		recordSuggestionGenerationError("get_link")
		return nil, ErrLinkNotFound
	}

	metrics, err := s.metrics(ctx, linkUUID)
	if err != nil {
		recordSuggestionGenerationError("metrics")
		return nil, err
	}

	result := heat.Compute(heat.CircleDefault, metrics.heatInput())

	contactIDs, err := s.queries.ListLinkContactsByLinkID(ctx, linkUUID)
	if err != nil {
		return nil, fmt.Errorf("list link contacts: %w", err)
	}
	var contactID pgtype.UUID
	var contactName, contactEmail string
	if len(contactIDs) > 0 {
		contactID = contactIDs[0]
		contact, cerr := s.queries.GetContactByID(ctx, db.GetContactByIDParams{ID: contactID, WorkspaceID: link.WorkspaceID})
		if cerr == nil {
			contactName = contact.Name.String
			contactEmail = contact.Email.String
		}
	}

	docTitle := ""
	if link.DocumentID.Valid {
		doc, derr := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{ID: link.DocumentID, WorkspaceID: link.WorkspaceID})
		if derr == nil {
			docTitle = doc.Title
		}
	}

	keyPages, _ := s.queries.GetLinkKeyPageViewDetails(ctx, db.GetLinkKeyPageViewDetailsParams{
		LinkID:   linkUUID,
		Patterns: heat.KeyPagePatterns(heat.CircleDefault),
	})
	keyPageTitles := make([]string, 0, len(keyPages))
	for _, kp := range keyPages {
		keyPageTitles = append(keyPageTitles, kp.Title)
	}

	totalDurationSeconds := 0
	if metrics.totalPageViews > 0 {
		totalDurationSeconds = int(metrics.avgDurationMinutes*60.0*float64(metrics.totalPageViews) + 0.5)
	}

	ctxSnapshot := Context{
		Opens:           metrics.opens,
		UniqueVisitors:  metrics.uniqueVisitors,
		DurationSeconds: totalDurationSeconds,
		KeyPageCount:    metrics.keyPageViews,
		KeyPageTitles:   keyPageTitles,
		ContactName:     contactName,
		ContactEmail:    contactEmail,
		DocumentTitle:   docTitle,
	}

	securityEvents, _ := s.queries.ListRecentSecurityEventsByLink(ctx, linkUUID)
	behavior, err := s.behaviorFeatures(ctx, linkUUID)
	if err != nil {
		return nil, err
	}

	candidates, err := s.evaluateRules(link, result, metrics, behavior, ctxSnapshot, securityEvents)
	if err != nil {
		recordSuggestionGenerationError("evaluate_rules")
		return nil, fmt.Errorf("evaluate rules: %w", err)
	}

	matchedRuleIDs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c.RuleID != "" {
			matchedRuleIDs = append(matchedRuleIDs, c.RuleID)
		}
	}

	out := make([]Suggestion, 0, len(candidates))
	generatedIDs := make([]pgtype.UUID, 0, len(candidates))
	for _, c := range candidates {
		exists, err := s.recentExists(ctx, link.WorkspaceID, linkUUID, c.Type, c.Subtype)
		if err != nil {
			recordSuggestionGenerationError("recent_exists")
			return nil, fmt.Errorf("check recent suggestion: %w", err)
		}
		if exists {
			continue
		}

		reason, action := c.Reason, c.Action
		if s.enricher != nil && shouldEnrich(c.Type, c.Subtype) {
			if er, ea, ok := s.enricher.Enrich(ctx, EnrichInput{
				Lang:           lang,
				Type:           c.Type,
				Subtype:        c.Subtype,
				DocumentTitle:  docTitle,
				Context:        c.Context,
				HeatResult:     result,
				OriginalReason: c.Reason,
				OriginalAction: c.Action,
			}); ok {
				reason, action = er, ea
			}
		}

		row, err := s.queries.CreateSuggestion(ctx, db.CreateSuggestionParams{
			TenantID:    link.TenantID,
			WorkspaceID: link.WorkspaceID,
			ContactID:   contactID,
			LinkID:      pgtype.UUID{Bytes: linkUUID.Bytes, Valid: true},
			DocumentID:  link.DocumentID,
			Type:        c.Type,
			Subtype:     pgText(c.Subtype),
			Reason:      reason,
			Action:      action,
			Metadata:    metadataToBytes(c.Metadata),
			Context:     c.Context.ToJSONB(),
		})
		if err != nil {
			recordSuggestionGenerationError("create_suggestion")
			return nil, fmt.Errorf("create suggestion: %w", err)
		}
		out = append(out, suggestionFromRow(row, lang))
		generatedIDs = append(generatedIDs, row.ID)
		recordSuggestionGenerated(c.Type, c.Subtype)

		if c.Type == "hot_signal" && s.notifier != nil {
			userID := ""
			if link.CreatedBy.Valid {
				userID = uuid.UUID(link.CreatedBy.Bytes).String()
			}
			_ = s.notifier.Enqueue(ctx, workspaceID, userID, "email", titleForSubtype(c.Subtype, c.Type, lang), reason+"\n"+action)
		}
	}

	snapshot, _ := json.Marshal(map[string]any{
		"heat": map[string]any{
			"level": result.Level,
			"score": result.Score,
			"trend": result.Trend,
		},
		"metrics":         metrics,
		"behavior":        behavior,
		"security_events": securityEvents,
	})

	_, _ = s.queries.CreateSignalRuleRun(ctx, db.CreateSignalRuleRunParams{
		TenantID:               link.TenantID,
		WorkspaceID:            link.WorkspaceID,
		LinkID:                 pgtype.UUID{Bytes: linkUUID.Bytes, Valid: true},
		RunStartedAt:           pgtype.Timestamptz{Time: start, Valid: true},
		DurationMs:             pgtype.Int4{Int32: int32(time.Since(start).Milliseconds()), Valid: true},
		InputSnapshot:          snapshot,
		MatchedRuleIds:         matchedRuleIDs,
		GeneratedSuggestionIds: generatedIDs,
	})

	observeSuggestionGenerationDuration(workspaceID, start)
	return out, nil
}

func (s *Service) evaluateRules(link db.Link, result heat.Result, m suggestionMetrics, behavior BehaviorInput, ctxSnapshot Context, events []db.ListRecentSecurityEventsByLinkRow) ([]candidate, error) {
	if s.ruleEngine == nil {
		return nil, nil
	}

	sec := make([]SecurityEventInput, 0, len(events))
	for _, ev := range events {
		sec = append(sec, SecurityEventInput{
			EventType: ev.EventType,
			Reason:    ev.Reason.String,
		})
	}

	matches, err := s.ruleEngine.Evaluate(RuleInput{
		TenantID:    uuid.UUID(link.TenantID.Bytes).String(),
		WorkspaceID: uuid.UUID(link.WorkspaceID.Bytes).String(),
		LinkID:      uuid.UUID(link.ID.Bytes).String(),
		Heat:        HeatInput{Level: result.Level, Score: result.Score, Trend: result.Trend},
		Metrics: MetricsInput{
			Opens:              m.opens,
			Revisits:           m.revisits,
			AvgDurationMinutes: m.avgDurationMinutes,
			Bounces:            m.bounces,
			Downloads:          m.downloads,
			TotalPageViews:     m.totalPageViews,
			KeyPageViews:       m.keyPageViews,
			UniqueVisitors:     m.uniqueVisitors,
		},
		Behavior:       behavior,
		Context:        ctxSnapshot,
		SecurityEvents: sec,
	})
	if err != nil {
		return nil, err
	}

	candidates := make([]candidate, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, candidate{
			RuleID:   match.ID,
			Type:     match.Type,
			Subtype:  match.Subtype,
			Reason:   match.Reason,
			Action:   match.Action,
			Metadata: match.Metadata,
			Context:  ctxSnapshot,
		})
	}
	return candidates, nil
}

// List returns active suggestions for a link.
func (s *Service) List(ctx context.Context, workspaceID, linkID, lang string) ([]Suggestion, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	linkUUID, err := pgUUID(linkID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListSuggestionsByLink(ctx, db.ListSuggestionsByLinkParams{
		LinkID:      linkUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Suggestion, len(rows))
	for i, r := range rows {
		out[i] = suggestionFromRow(r, lang)
	}
	return out, nil
}

// WorkspaceSuggestion is the camelCase view used by the workspace insights list.
type WorkspaceSuggestion struct {
	ID             string `json:"id"`
	ContactID      string `json:"contactId"`
	ContactEmail   string `json:"contactEmail"`
	DocumentTitle  string `json:"documentTitle"`
	LinkID         string `json:"linkId"`
	HeatLevel      string `json:"heatLevel"`
	Score          int    `json:"score"`
	Reason         string `json:"reason"`
	Action         string `json:"action"`
	LastActivityAt string `json:"lastActivityAt"`
}

// ListWorkspace returns active suggestions across the workspace enriched for display.
func (s *Service) ListWorkspace(ctx context.Context, workspaceID, lang string) ([]WorkspaceSuggestion, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListSuggestionsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, err
	}

	contacts, err := s.queries.ListContactsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, err
	}
	contactEmailByID := make(map[string]string, len(contacts))
	for _, c := range contacts {
		contactEmailByID[uuidToString(c.ID)] = c.Email.String
	}

	out := make([]WorkspaceSuggestion, 0, len(rows))
	for _, r := range rows {
		su := WorkspaceSuggestion{
			ID:             uuidToString(r.ID),
			ContactID:      uuidToString(r.ContactID),
			LinkID:         uuidToString(r.LinkID),
			Reason:         r.Reason,
			Action:         r.Action,
			LastActivityAt: r.UpdatedAt.Time.Format(time.RFC3339),
		}
		if su.ContactID != "" {
			su.ContactEmail = contactEmailByID[su.ContactID]
		}

		if r.DocumentID.Valid {
			doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
				ID:          r.DocumentID,
				WorkspaceID: wsUUID,
			})
			if err == nil {
				su.DocumentTitle = doc.Title
			}
		}

		if r.LinkID.Valid {
			res := s.linkHeatResult(ctx, r.LinkID)
			su.Score = res.Score
			su.HeatLevel = res.Level
			if su.HeatLevel == "" {
				su.HeatLevel = "cold"
			}
		}

		out = append(out, su)
	}
	return out, nil
}

func (s *Service) linkHeatResult(ctx context.Context, linkID pgtype.UUID) heat.Result {
	access, err := s.queries.GetLinkAccessMetrics(ctx, linkID)
	if err != nil {
		return heat.Result{Level: "cold"}
	}
	pageViews, err := s.queries.GetLinkPageViewMetrics(ctx, linkID)
	if err != nil {
		return heat.Result{Level: "cold"}
	}
	bounce, err := s.queries.GetLinkBounceCount(ctx, linkID)
	if err != nil {
		bounce = 0
	}
	revisits := int(access.Opens) - int(access.UniqueVisitors)
	if revisits < 0 {
		revisits = 0
	}
	keyPageViews, err := countKeyPageViews(ctx, s.queries, linkID, heat.CircleDefault)
	if err != nil {
		return heat.Result{Level: "cold"}
	}
	return heat.Compute(heat.CircleDefault, heat.Input{
		Opens:              int(access.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: pageViews.AvgDurationSeconds / 60.0,
		KeyPageViews:       keyPageViews,
		ForwardSignals:     int(access.UniqueVisitors),
		Downloads:          int(access.Downloads),
		BouncePenalty:      int(bounce),
	})
}

// Dismiss marks a suggestion as dismissed.
func (s *Service) Dismiss(ctx context.Context, workspaceID, suggestionID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	id, err := pgUUID(suggestionID)
	if err != nil {
		return err
	}
	_, err = s.queries.GetSuggestionByID(ctx, db.GetSuggestionByIDParams{ID: id, WorkspaceID: wsUUID})
	if err != nil {
		return ErrSuggestionNotFound
	}
	return s.queries.DismissSuggestion(ctx, db.DismissSuggestionParams{ID: id, WorkspaceID: wsUUID})
}

func (s *Service) recentExists(ctx context.Context, workspaceID, linkID pgtype.UUID, typ, subtype string) (bool, error) {
	count, err := s.queries.CountRecentSuggestionsByLinkTypeSubtype(ctx, db.CountRecentSuggestionsByLinkTypeSubtypeParams{
		LinkID:      linkID,
		WorkspaceID: workspaceID,
		Type:        typ,
		Subtype:     pgText(subtype),
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Service) metrics(ctx context.Context, linkID pgtype.UUID) (suggestionMetrics, error) {
	if s.featureStore != nil {
		if snap, err := s.featureStore.GetForLink(ctx, linkID); err == nil && snap.Found {
			return snap.toSuggestionMetrics(), nil
		}
	}

	var m suggestionMetrics
	access, err := s.queries.GetLinkAccessMetrics(ctx, linkID)
	if err != nil {
		return m, err
	}
	m.opens = int(access.Opens)
	m.uniqueVisitors = int(access.UniqueVisitors)
	m.downloads = int(access.Downloads)

	pv, err := s.queries.GetLinkPageViewMetrics(ctx, linkID)
	if err != nil {
		return m, err
	}
	m.avgDurationMinutes = pv.AvgDurationSeconds / 60.0
	keyViews, err := countKeyPageViews(ctx, s.queries, linkID, heat.CircleDefault)
	if err != nil {
		return m, fmt.Errorf("key page view metrics: %w", err)
	}
	m.keyPageViews = keyViews
	m.totalPageViews = int(pv.TotalPageViews)

	bounceCount, err := s.queries.GetLinkBounceCount(ctx, linkID)
	if err != nil {
		return m, err
	}
	m.bounces = int(bounceCount)
	m.revisits = m.opens - m.uniqueVisitors
	if m.revisits < 0 {
		m.revisits = 0
	}
	return m, nil
}

type suggestionMetrics struct {
	opens              int
	uniqueVisitors     int
	revisits           int
	avgDurationMinutes float64
	keyPageViews       int
	totalPageViews     int
	downloads          int
	bounces            int
}

func (s *Service) behaviorFeatures(ctx context.Context, linkID pgtype.UUID) (BehaviorInput, error) {
	if s.featureStore != nil {
		if snap, err := s.featureStore.GetForLink(ctx, linkID); err == nil && snap.Found {
			return snap.toBehaviorInput(), nil
		}
	}

	var out BehaviorInput

	distinctIPs, err := s.queries.CountRecentDistinctIPsByLink(ctx, linkID)
	if err != nil {
		return out, fmt.Errorf("count distinct IPs: %w", err)
	}
	out.DistinctIPs1h = distinctIPs

	downloads, err := s.queries.CountRecentDownloadAttemptsByLink(ctx, linkID)
	if err != nil {
		return out, fmt.Errorf("count downloads: %w", err)
	}
	out.Downloads24h = downloads.TotalDownloads
	out.DistinctEmails24h = downloads.DistinctEmails
	out.UnknownEmails24h = downloads.DistinctUnknownEmails

	return out, nil
}

func (m suggestionMetrics) heatInput() heat.Input {
	return heat.Input{
		Opens:              m.opens,
		Revisits:           m.revisits,
		AvgDurationMinutes: m.avgDurationMinutes,
		KeyPageViews:       m.keyPageViews,
		ForwardSignals:     m.uniqueVisitors,
		Downloads:          m.downloads,
		BouncePenalty:      m.bounces,
	}
}

type candidate struct {
	RuleID   string
	Type     string
	Subtype  string
	Reason   string
	Action   string
	Metadata map[string]string
	Context  Context
}

func suggestionFromRow(r db.Suggestion, lang string) Suggestion {
	s := Suggestion{
		ID:          uuidToString(r.ID),
		TenantID:    uuidToString(r.TenantID),
		WorkspaceID: uuidToString(r.WorkspaceID),
		LinkID:      uuidToString(r.LinkID),
		DocumentID:  uuidToString(r.DocumentID),
		Type:        r.Type,
		Subtype:     r.Subtype.String,
		Reason:      r.Reason,
		Action:      r.Action,
		Dismissed:   r.Dismissed,
		CreatedAt:   r.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:   r.UpdatedAt.Time.Format(time.RFC3339),
	}
	if r.ContactID.Valid {
		s.ContactID = uuidToString(r.ContactID)
	}
	s.Priority = priorityForType(r.Type)
	s.Title = titleForSubtype(r.Subtype.String, r.Type, lang)
	return s
}

func priorityForType(typ string) string {
	switch typ {
	case "hot_signal":
		return "high"
	case "risk_alert":
		return "medium"
	default:
		return "low"
	}
}

func titleForType(typ, lang string) string {
	ls := newLocalizedStrings(lang)
	switch typ {
	case "hot_signal":
		return ls.hotSignalTitle
	case "risk_alert":
		return ls.riskAlertTitle
	default:
		return ls.followUpTitle
	}
}

// countKeyPageViews counts page views whose page title matches the circle's key-page keywords.
func countKeyPageViews(ctx context.Context, queries *db.Queries, linkID pgtype.UUID, circle heat.Circle) (int, error) {
	patterns := heat.KeyPagePatterns(circle)
	if len(patterns) == 0 {
		return 0, nil
	}
	metrics, err := queries.GetLinkKeyPageViewMetrics(ctx, db.GetLinkKeyPageViewMetricsParams{
		LinkID:   linkID,
		Patterns: patterns,
	})
	if err != nil {
		return 0, err
	}
	return int(metrics.TotalKeyPageViews), nil
}

// TitleForType returns the localized title for a suggestion/signal type.
func TitleForType(typ, lang string) string {
	return titleForType(typ, lang)
}


func shouldEnrich(typ, subtype string) bool {
	if typ == "hot_signal" {
		return true
	}
	return subtype == SubtypeQuestion
}

func metadataToBytes(m map[string]string) []byte {
	if len(m) == 0 {
		return []byte("{}")
	}
	b, _ := json.Marshal(m)
	return b
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
