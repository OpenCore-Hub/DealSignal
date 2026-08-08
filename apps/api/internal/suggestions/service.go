package suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heatkw"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrLinkNotFound       = errors.New("link not found")
	ErrSuggestionNotFound = errors.New("suggestion not found")
)

// Suggestion is the public view of a generated suggestion.
type Suggestion struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	WorkspaceID  string `json:"workspace_id"`
	ContactID    string `json:"contact_id,omitempty"`
	LinkID       string `json:"link_id"`
	DocumentID   string `json:"document_id,omitempty"`
	Type         string `json:"type"`
	Subtype      string `json:"subtype,omitempty"`
	Priority     string `json:"priority"`
	Title        string `json:"title"`
	Reason       string `json:"reason"`
	Action       string `json:"action"`
	Dismissed    bool   `json:"dismissed"`
	SnoozedUntil string `json:"snoozed_until,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
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
func (s *Service) Generate(ctx context.Context, workspaceID, linkID, lang string) (out []Suggestion, err error) {
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

	// Audit every run, success or failure.
	var matchedRuleIDs []string
	var bucketSkippedRuleIDs []string
	var shadowMatchedRuleIDs []string
	var generatedIDs []pgtype.UUID
	var metrics suggestionMetrics
	var behavior BehaviorInput
	var result heat.Result
	var securityEvents []db.ListRecentSecurityEventsByLinkRow
	defer func() {
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
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		_, _ = s.queries.CreateSignalRuleRun(ctx, db.CreateSignalRuleRunParams{
			TenantID:               link.TenantID,
			WorkspaceID:            link.WorkspaceID,
			LinkID:                 pgtype.UUID{Bytes: linkUUID.Bytes, Valid: true},
			RunStartedAt:           pgtype.Timestamptz{Time: start, Valid: true},
			DurationMs:             pgtype.Int4{Int32: int32(time.Since(start).Milliseconds()), Valid: true},
			InputSnapshot:          snapshot,
			MatchedRuleIds:         matchedRuleIDs,
			GeneratedSuggestionIds: generatedIDs,
			BucketSkippedRuleIds:   bucketSkippedRuleIDs,
			ShadowMatchedRuleIds:   shadowMatchedRuleIDs,
			Error:                  pgText(errStr),
		})
	}()

	// Fetch feature snapshot once per generation so metrics() and behaviorFeatures()
	// see the same cached state.
	var snap *FeatureSnapshot
	if s.featureStore != nil {
		if fs, serr := s.featureStore.GetForLink(ctx, linkUUID); serr == nil && fs.Found {
			snap = &fs
		}
	}

	metrics, err = s.metrics(ctx, linkUUID, snap)
	if err != nil {
		recordSuggestionGenerationError("metrics")
		return nil, err
	}

	// Time-decay the heat score based on the link's last activity.
	decayDays := 0.0
	if lastAccess, aerr := s.queries.GetLinkLastAccessAt(ctx, linkUUID); aerr == nil && lastAccess.Valid {
		decayDays = time.Since(lastAccess.Time).Hours() / 24
	} else if link.CreatedAt.Valid {
		decayDays = time.Since(link.CreatedAt.Time).Hours() / 24
	}
	rs, _ := heatkw.LoadForWorkspaceUUID(ctx, s.queries, link.WorkspaceID, nil)
	result = heat.Compute(rs.Circle, metrics.heatInput(decayDays))

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
		Patterns: rs.Patterns(),
	})
	keyPageTitles := make([]string, 0, len(keyPages))
	for _, kp := range keyPages {
		keyPageTitles = append(keyPageTitles, kp.Title)
	}
	focus := s.resolveFocusPages(ctx, linkUUID, keyPages)

	totalDurationSeconds24h := 0
	if metrics.totalPageViews24h > 0 {
		totalDurationSeconds24h = int(metrics.avgDurationMinutes24h*60.0*float64(metrics.totalPageViews24h) + 0.5)
	}

	// Context reflects the same 24-hour window the rules evaluate.
	ctxSnapshot := Context{
		Opens:           metrics.opens24h,
		UniqueVisitors:  metrics.uniqueVisitors24h,
		DurationSeconds: totalDurationSeconds24h,
		KeyPageCount:    metrics.keyPageViews24h,
		KeyPageTitles:   keyPageTitles,
		ContactName:     contactName,
		ContactEmail:    contactEmail,
		DocumentTitle:   docTitle,
	}

	securityEvents, _ = s.queries.ListRecentSecurityEventsByLink(ctx, linkUUID)
	behavior, err = s.behaviorFeatures(ctx, linkUUID, snap)
	if err != nil {
		return nil, err
	}

	var candidates []candidate
	candidates, bucketSkippedRuleIDs, shadowMatchedRuleIDs, err = s.evaluateRules(link, result, metrics, behavior, ctxSnapshot, securityEvents)
	if err != nil {
		recordSuggestionGenerationError("evaluate_rules")
		return nil, fmt.Errorf("evaluate rules: %w", err)
	}

	matchedRuleIDs = make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c.RuleID != "" {
			matchedRuleIDs = append(matchedRuleIDs, c.RuleID)
		}
	}

	out = make([]Suggestion, 0, len(candidates))
	generatedIDs = make([]pgtype.UUID, 0, len(candidates))
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

		metadata := attachFocusMetadata(c.Type, c.Subtype, c.Metadata, focus)
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
			Metadata:    metadataToBytes(metadata),
			Context:     c.Context.ToJSONB(),
			RuleID:      pgText(c.RuleID),
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

	observeSuggestionGenerationDuration(workspaceID, start)
	return out, nil
}

func (s *Service) evaluateRules(link db.Link, result heat.Result, m suggestionMetrics, behavior BehaviorInput, ctxSnapshot Context, events []db.ListRecentSecurityEventsByLinkRow) ([]candidate, []string, []string, error) {
	if s.ruleEngine == nil {
		return nil, nil, nil, nil
	}

	sec := make([]SecurityEventInput, 0, len(events))
	for _, ev := range events {
		sec = append(sec, SecurityEventInput{
			EventType: ev.EventType,
			Reason:    ev.Reason.String,
		})
	}

	matches, bucketSkipped, shadowMatched, err := s.ruleEngine.Evaluate(RuleInput{
		TenantID:    uuid.UUID(link.TenantID.Bytes).String(),
		WorkspaceID: uuid.UUID(link.WorkspaceID.Bytes).String(),
		LinkID:      uuid.UUID(link.ID.Bytes).String(),
		Heat:        HeatInput{Level: result.Level, Score: result.Score, Trend: result.Trend},
		Metrics: MetricsInput{
			Opens:                 m.opens,
			Revisits:              m.revisits,
			AvgDurationMinutes:    m.avgDurationMinutes,
			Bounces:               m.bounces,
			Downloads:             m.downloads,
			TotalPageViews:        m.totalPageViews,
			KeyPageViews:          m.keyPageViews,
			UniqueVisitors:        m.uniqueVisitors,
			Opens24h:              m.opens24h,
			Revisits24h:           m.revisits24h,
			AvgDurationMinutes24h: m.avgDurationMinutes24h,
			Bounces24h:            m.bounces24h,
			Downloads24h:          m.downloads24h,
			TotalPageViews24h:     m.totalPageViews24h,
			KeyPageViews24h:       m.keyPageViews24h,
			UniqueVisitors24h:     m.uniqueVisitors24h,
		},
		Behavior:       behavior,
		Context:        ctxSnapshot,
		SecurityEvents: sec,
	})
	if err != nil {
		return nil, bucketSkipped, shadowMatched, err
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
	return candidates, bucketSkipped, shadowMatched, nil
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
	DealRoomID     string `json:"dealRoomId,omitempty"`
	HeatLevel      string `json:"heatLevel"`
	Score          int    `json:"score"`
	Reason         string `json:"reason"`
	Action         string `json:"action"`
	Kind           string `json:"kind,omitempty"` // e.g. formal_ask
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
		if r.Subtype.Valid && r.Subtype.String == SubtypeFormalAsk {
			su.Kind = SubtypeFormalAsk
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
			if link, lerr := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
				ID:          r.LinkID,
				WorkspaceID: wsUUID,
			}); lerr == nil && link.DealRoomID.Valid {
				su.DealRoomID = uuidToString(link.DealRoomID)
			}
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
	patterns, circle := s.keyPagePatternsForLink(ctx, linkID)
	keyPageViews, err := countKeyPageViews(ctx, s.queries, linkID, patterns)
	if err != nil {
		return heat.Result{Level: "cold"}
	}
	lastAccess, _ := s.queries.GetLinkLastAccessAt(ctx, linkID)
	decayDays := 0.0
	if lastAccess.Valid {
		decayDays = time.Since(lastAccess.Time).Hours() / 24
	}
	return heat.Compute(circle, heat.Input{
		Opens:              int(access.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: pageViews.AvgDurationSeconds / 60.0,
		KeyPageViews:       keyPageViews,
		ForwardSignals:     int(access.ForwardSignals),
		Downloads:          int(access.Downloads),
		BouncePenalty:      int(bounce),
		DecayDays:          decayDays,
	})
}

// Dismiss marks a suggestion as dismissed and records user feedback.
func (s *Service) Dismiss(ctx context.Context, workspaceID, suggestionID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	id, err := pgUUID(suggestionID)
	if err != nil {
		return err
	}
	suggestion, err := s.queries.GetSuggestionByID(ctx, db.GetSuggestionByIDParams{ID: id, WorkspaceID: wsUUID})
	if err != nil {
		return ErrSuggestionNotFound
	}
	if derr := s.queries.DismissSuggestion(ctx, db.DismissSuggestionParams{ID: id, WorkspaceID: wsUUID}); derr != nil {
		return derr
	}
	_, _ = s.queries.CreateSuggestionFeedback(ctx, db.CreateSuggestionFeedbackParams{
		TenantID:     suggestion.TenantID,
		WorkspaceID:  suggestion.WorkspaceID,
		SuggestionID: suggestion.ID,
		FeedbackType: "dismissed",
	})
	return nil
}

// AllowedSnoozeHours are the supported snooze durations (1d / 3d / 7d).
var AllowedSnoozeHours = map[int]struct{}{24: {}, 72: {}, 168: {}}

var ErrInvalidSnoozeDuration = errors.New("invalid snooze duration")

// Snooze hides a suggestion until now+hours and mirrors snooze onto the linked radar action when present.
func (s *Service) Snooze(ctx context.Context, workspaceID, suggestionID string, hours int) (Suggestion, error) {
	if _, ok := AllowedSnoozeHours[hours]; !ok {
		return Suggestion{}, ErrInvalidSnoozeDuration
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Suggestion{}, err
	}
	id, err := pgUUID(suggestionID)
	if err != nil {
		return Suggestion{}, err
	}
	until := time.Now().UTC().Add(time.Duration(hours) * time.Hour)
	row, err := s.queries.SnoozeSuggestion(ctx, db.SnoozeSuggestionParams{
		ID:           id,
		WorkspaceID:  wsUUID,
		SnoozedUntil: pgtype.Timestamptz{Time: until, Valid: true},
	})
	if err != nil {
		return Suggestion{}, ErrSuggestionNotFound
	}
	_, _ = s.queries.CreateSuggestionFeedback(ctx, db.CreateSuggestionFeedbackParams{
		TenantID:     row.TenantID,
		WorkspaceID:  row.WorkspaceID,
		SuggestionID: row.ID,
		FeedbackType: "snoozed",
	})
	s.mirrorSnoozeRadarActions(ctx, wsUUID, id, row.Metadata)
	return suggestionFromRow(row, ""), nil
}

// mirrorSnoozeRadarActions snoozes suggestion-linked signal actions and Formal Ask operational todos (by turn_id).
func (s *Service) mirrorSnoozeRadarActions(ctx context.Context, wsUUID, suggestionID pgtype.UUID, metadata []byte) {
	if sig, err := s.queries.GetSignalBySuggestion(ctx, db.GetSignalBySuggestionParams{
		SuggestionID: suggestionID,
		WorkspaceID:  wsUUID,
	}); err == nil {
		_ = s.queries.SnoozeActionItemsBySignal(ctx, db.SnoozeActionItemsBySignalParams{
			SignalID:    sig.ID,
			WorkspaceID: wsUUID,
		})
	}
	turnID := metadataString(metadata, "turn_id")
	if turnID == "" {
		return
	}
	for _, sourceType := range []string{"link_question", "deal_room_link_question"} {
		_ = s.queries.SnoozeActionItemBySource(ctx, db.SnoozeActionItemBySourceParams{
			WorkspaceID: wsUUID,
			SourceType:  pgtype.Text{String: sourceType, Valid: true},
			SourceID:    pgtype.Text{String: turnID, Valid: true},
		})
	}
}

// RulePerformance aggregates per-rule precision/recall signals for a workspace.
type RulePerformance struct {
	RuleID         string `json:"rule_id"`
	GeneratedCount int64  `json:"generated_count"`
	DismissedCount int64  `json:"dismissed_count"`
	ActedCount     int64  `json:"acted_count"`
	SpamCount      int64  `json:"spam_count"`
}

// ListRulePerformance returns per-rule calibration metrics.
func (s *Service) ListRulePerformance(ctx context.Context, workspaceID string) ([]RulePerformance, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.GetRulePerformanceSummary(ctx, wsUUID)
	if err != nil {
		return nil, fmt.Errorf("list rule performance: %w", err)
	}
	out := make([]RulePerformance, len(rows))
	for i, r := range rows {
		out[i] = RulePerformance{
			RuleID:         r.RuleID.String,
			GeneratedCount: r.GeneratedCount,
			DismissedCount: r.DismissedCount,
			ActedCount:     r.ActedCount,
			SpamCount:      r.SpamCount,
		}
	}
	return out, nil
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

func (s *Service) metrics(ctx context.Context, linkID pgtype.UUID, snap *FeatureSnapshot) (suggestionMetrics, error) {
	var m suggestionMetrics

	// Lifetime metrics: use a fresh feature snapshot if available.
	// Always overlay live forward_signals so heat stays truthful when
	// link_features.forward_signals is still 0 (pre-migration-150 / stale cache).
	if snap != nil && snap.Found {
		m = snap.toSuggestionMetrics()
		access, err := s.queries.GetLinkAccessMetrics(ctx, linkID)
		if err != nil {
			return m, err
		}
		m.forwardSignals = int(access.ForwardSignals)
	} else {
		access, err := s.queries.GetLinkAccessMetrics(ctx, linkID)
		if err != nil {
			return m, err
		}
		m.opens = int(access.Opens)
		m.uniqueVisitors = int(access.UniqueVisitors)
		m.forwardSignals = int(access.ForwardSignals)
		m.downloads = int(access.Downloads)

		pv, err := s.queries.GetLinkPageViewMetrics(ctx, linkID)
		if err != nil {
			return m, err
		}
		m.avgDurationMinutes = pv.AvgDurationSeconds / 60.0
		patterns, _ := s.keyPagePatternsForLink(ctx, linkID)
		keyViews, err := countKeyPageViews(ctx, s.queries, linkID, patterns)
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
	}

	// Rolling 24-hour metrics are always computed live so rules match their wording.
	access24h, err := s.queries.GetLinkAccessMetrics24h(ctx, linkID)
	if err != nil {
		return m, fmt.Errorf("access metrics 24h: %w", err)
	}
	m.opens24h = int(access24h.Opens)
	m.uniqueVisitors24h = int(access24h.UniqueVisitors)
	m.downloads24h = int(access24h.Downloads)
	m.revisits24h = m.opens24h - m.uniqueVisitors24h
	if m.revisits24h < 0 {
		m.revisits24h = 0
	}

	pv24h, err := s.queries.GetLinkPageViewMetrics24h(ctx, linkID)
	if err != nil {
		return m, fmt.Errorf("page view metrics 24h: %w", err)
	}
	m.avgDurationMinutes24h = pv24h.AvgDurationSeconds / 60.0
	m.totalPageViews24h = int(pv24h.TotalPageViews)

	patterns24h, _ := s.keyPagePatternsForLink(ctx, linkID)
	keyViews24h, err := countKeyPageViews24h(ctx, s.queries, linkID, patterns24h)
	if err != nil {
		return m, fmt.Errorf("key page view metrics 24h: %w", err)
	}
	m.keyPageViews24h = keyViews24h

	bounceCount24h, err := s.queries.GetLinkBounceCount24h(ctx, linkID)
	if err != nil {
		return m, fmt.Errorf("bounce count 24h: %w", err)
	}
	m.bounces24h = int(bounceCount24h)

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
	forwardSignals     int
	// 24h rolling window fields used by expression rules.
	opens24h              int
	uniqueVisitors24h     int
	revisits24h           int
	avgDurationMinutes24h float64
	keyPageViews24h       int
	totalPageViews24h     int
	downloads24h          int
	bounces24h            int
}

func (s *Service) behaviorFeatures(ctx context.Context, linkID pgtype.UUID, snap *FeatureSnapshot) (BehaviorInput, error) {
	if snap != nil && snap.Found {
		return snap.toBehaviorInput(), nil
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

func (m suggestionMetrics) heatInput(decayDays float64) heat.Input {
	return heat.Input{
		Opens:              m.opens,
		Revisits:           m.revisits,
		AvgDurationMinutes: m.avgDurationMinutes,
		KeyPageViews:       m.keyPageViews,
		ForwardSignals:     m.forwardSignals,
		Downloads:          m.downloads,
		BouncePenalty:      m.bounces,
		DecayDays:          decayDays,
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
	if r.SnoozedUntil.Valid {
		s.SnoozedUntil = r.SnoozedUntil.Time.UTC().Format(time.RFC3339)
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

// countKeyPageViews counts page views whose page title matches the given patterns.
func countKeyPageViews(ctx context.Context, queries *db.Queries, linkID pgtype.UUID, patterns []string) (int, error) {
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

// countKeyPageViews24h counts 24-hour key-page views for the given patterns.
func countKeyPageViews24h(ctx context.Context, queries *db.Queries, linkID pgtype.UUID, patterns []string) (int, error) {
	if len(patterns) == 0 {
		return 0, nil
	}
	metrics, err := queries.GetLinkKeyPageViewMetrics24h(ctx, db.GetLinkKeyPageViewMetrics24hParams{
		LinkID:   linkID,
		Patterns: patterns,
	})
	if err != nil {
		return 0, err
	}
	return int(metrics.TotalKeyPageViews), nil
}

func (s *Service) keyPagePatternsForLink(ctx context.Context, linkID pgtype.UUID) ([]string, heat.Circle) {
	link, err := s.queries.GetLinkByID(ctx, linkID)
	if err != nil {
		rs := heat.NewRuleSet(heat.CircleDefault, nil)
		return rs.Patterns(), rs.Circle
	}
	rs, _ := heatkw.LoadForWorkspaceUUID(ctx, s.queries, link.WorkspaceID, nil)
	return rs.Patterns(), rs.Circle
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

func metadataString(raw []byte, key string) string {
	if len(raw) == 0 || key == "" {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	return strings.TrimSpace(m[key])
}

// suggestionFocusPages holds real page anchors for deep links.
type suggestionFocusPages struct {
	hot            int
	bounce         int
	bounceExitRate float64 // 0..1; only set when bounce page comes from exit ranking
}

func (s *Service) resolveFocusPages(
	ctx context.Context,
	linkID pgtype.UUID,
	keyPages []db.GetLinkKeyPageViewDetailsRow,
) suggestionFocusPages {
	var out suggestionFocusPages
	out.hot = focusPageFromKeyPages(keyPages)
	if out.hot <= 0 {
		if topPages, err := s.queries.ListTopPagesByLink(ctx, linkID); err == nil {
			out.hot = focusPageFromTopPages(topPages)
		}
	}

	if highExit, err := s.queries.ListHighExitPagesByLink(ctx, linkID); err == nil {
		if page, rate := focusPageFromHighExit(highExit); page > 0 {
			out.bounce = page
			out.bounceExitRate = rate
		}
	}
	// Bounce visitors may leave before any page_view; fall back to engagement focus.
	if out.bounce <= 0 {
		out.bounce = out.hot
	}
	return out
}

// focusPageFromKeyPages returns the highest-ranked key page number, or 0.
func focusPageFromKeyPages(keyPages []db.GetLinkKeyPageViewDetailsRow) int {
	for _, kp := range keyPages {
		if kp.PageNumber > 0 {
			return int(kp.PageNumber)
		}
	}
	return 0
}

// focusPageFromTopPages returns the most-viewed page number, or 0.
func focusPageFromTopPages(pages []db.ListTopPagesByLinkRow) int {
	for _, p := range pages {
		if p.PageNumber > 0 {
			return int(p.PageNumber)
		}
	}
	return 0
}

// focusPageFromHighExit returns the highest exit-rate page and its rate.
func focusPageFromHighExit(pages []db.ListHighExitPagesByLinkRow) (page int, exitRate float64) {
	for _, p := range pages {
		if p.PageNumber > 0 && p.ExitRate > 0 {
			return int(p.PageNumber), p.ExitRate
		}
	}
	return 0, 0
}

// attachFocusMetadata writes typed deep-link metadata.
// hot_signal → engagement focus page; bounce risk → high-exit page (+ exit_rate).
func attachFocusMetadata(typ, subtype string, md map[string]string, focus suggestionFocusPages) map[string]string {
	page := 0
	exitRate := 0.0
	switch {
	case typ == "hot_signal":
		page = focus.hot
	case typ == "risk_alert" && subtype == SubtypeBounce:
		page = focus.bounce
		exitRate = focus.bounceExitRate
	default:
		return md
	}
	if page <= 0 {
		return md
	}
	out := make(map[string]string, len(md)+2)
	for k, v := range md {
		out[k] = v
	}
	out["page_number"] = strconv.Itoa(page)
	if exitRate > 0 {
		out["exit_rate"] = formatExitRatePercent(exitRate)
	}
	return out
}

func formatExitRatePercent(rate float64) string {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return strconv.FormatInt(int64(rate*100+0.5), 10) + "%"
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
