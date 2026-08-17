// Package contact exposes workspace-scoped contact and activity APIs.
package contact

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heatkw"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ErrContactNotFound is returned when a contact does not exist in the workspace.
var ErrContactNotFound = errors.New("contact not found")

// Querier isolates the database operations required by the contact service.
type Querier interface {
	CreateContact(ctx context.Context, arg db.CreateContactParams) (db.Contact, error)
	FindUnsyncedContactEmails(ctx context.Context, workspaceID pgtype.UUID) ([]pgtype.Text, error)
	UpsertContactByEmail(ctx context.Context, arg db.UpsertContactByEmailParams) (db.Contact, error)
	GetContactByID(ctx context.Context, arg db.GetContactByIDParams) (db.Contact, error)
	ListContactsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.Contact, error)
	GetContactAggregateByEmail(ctx context.Context, arg db.GetContactAggregateByEmailParams) (db.GetContactAggregateByEmailRow, error)
	GetContactAggregatesByWorkspace(ctx context.Context, arg db.GetContactAggregatesByWorkspaceParams) ([]db.GetContactAggregatesByWorkspaceRow, error)
	GetContactKeyPageViewDetails(ctx context.Context, arg db.GetContactKeyPageViewDetailsParams) ([]db.GetContactKeyPageViewDetailsRow, error)
	ListContactActivitiesByEmail(ctx context.Context, arg db.ListContactActivitiesByEmailParams) ([]db.ListContactActivitiesByEmailRow, error)
	ListContactViewedDocumentIDs(ctx context.Context, arg db.ListContactViewedDocumentIDsParams) ([]string, error)
	ListContactViewedDocuments(ctx context.Context, arg db.ListContactViewedDocumentsParams) ([]db.ListContactViewedDocumentsRow, error)
	ListContactViewedDocumentIDsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.ListContactViewedDocumentIDsByWorkspaceRow, error)
	GetWorkspaceKeyPageSettings(ctx context.Context, workspaceID pgtype.UUID) (db.WorkspaceKeyPageSetting, error)
}

// Cache is a minimal key/value cache for contact list enrichment.
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

const contactListCacheTTL = 20 * time.Second

func contactListCacheKey(workspaceID string) string {
	// v6: viewed docs include bundle members (align ListRecentlyAccessedDocumentsByWorkspace).
	return fmt.Sprintf("contacts:list:v6:%s", workspaceID)
}

// Service aggregates visitor activity into contact records.
type Service struct {
	queries Querier
	cache   Cache
}

// ServiceOption configures a contact Service.
type ServiceOption func(*Service)

// WithCache enables Redis caching for enriched contact lists.
func WithCache(c Cache) ServiceOption {
	return func(s *Service) { s.cache = c }
}

// NewService creates a contact service.
func NewService(q Querier, opts ...ServiceOption) *Service {
	s := &Service{queries: q}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Contact is the enriched response model for a workspace contact.
type Contact struct {
	ID                   string           `json:"id"`
	Email                string           `json:"email"`
	Name                 string           `json:"name"`
	Organization         string           `json:"organization,omitempty"`
	Role                 string           `json:"role,omitempty"`
	HeatLevel            string           `json:"heatLevel"`
	Score                int              `json:"score"`
	ScoreHistory         []ScorePoint     `json:"scoreHistory"`
	TotalVisits          int64            `json:"totalVisits"`
	TotalDurationSeconds int64            `json:"totalDurationSeconds"`
	LastSeenAt           string           `json:"lastSeenAt,omitempty"`
	ViewedDocuments      []string         `json:"viewedDocuments"`
	ViewedDocumentItems  []ViewedDocument `json:"viewedDocumentItems,omitempty"`
	// KeyPages is explain-only on GET. List/create leave this nil.
	KeyPages *ContactKeyPages `json:"keyPages,omitempty"`
}

const contactKeyPageMinSeconds = 3

// ContactKeyPages is title-match evidence. Compute uses Engaged only.
type ContactKeyPages struct {
	Engaged    int64            `json:"engaged"`
	Total      int64            `json:"total"`
	MinSeconds int              `json:"minSeconds"`
	Pages      []ContactKeyPage `json:"pages"`
}

// ContactKeyPage is one title-matched page this visitor viewed.
type ContactKeyPage struct {
	PageNumber   int32  `json:"pageNumber"`
	Title        string `json:"title"`
	EngagedViews int64  `json:"engagedViews"`
	TotalViews   int64  `json:"totalViews"`
}

// ScorePoint is one day of engagement intensity for trend charts.
// Events is the raw event count that day (opens + page views + downloads).
// Score mirrors Events for backward-compatible clients; prefer Events.
type ScorePoint struct {
	Date   string `json:"date"`
	Events int    `json:"events"`
	Score  int    `json:"score"`
}

// ViewedDocument is a document the contact accessed via a share link.
type ViewedDocument struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Activity is a single contact engagement event.
type Activity struct {
	ID              string `json:"id"`
	ContactID       string `json:"contactId"`
	ContactEmail    string `json:"contactEmail"`
	LinkID          string `json:"linkId"`
	DocumentID      string `json:"documentId,omitempty"`
	DocumentTitle   string `json:"documentTitle"`
	EventType       string `json:"eventType"`
	PageNumber      int32  `json:"pageNumber,omitempty"`
	DurationSeconds int32  `json:"durationSeconds"`
	Timestamp       string `json:"timestamp"`
	Description     string `json:"description"`
}

// CreateContactRequest is the input for manually creating a contact.
type CreateContactRequest struct {
	Email string
	Name  string
}

// CreateContact creates a new contact in the workspace.
func (s *Service) CreateContact(ctx context.Context, workspaceID string, req CreateContactRequest) (Contact, error) {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return Contact{}, err
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		return Contact{}, errors.New("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return Contact{}, fmt.Errorf("invalid email: %w", err)
	}

	// Upsert: workspace email is unique; ContactSelector "create" must succeed when the
	// contact already exists (synced visitor / prior create), not 500 on 23505.
	c, err := s.queries.UpsertContactByEmail(ctx, db.UpsertContactByEmailParams{
		WorkspaceID: wsUUID,
		Email:       pgtype.Text{String: strings.ToLower(email), Valid: true},
		Name:        strings.TrimSpace(req.Name),
	})
	if err != nil {
		return Contact{}, fmt.Errorf("create contact: %w", err)
	}

	s.invalidateListCache(ctx, workspaceID)
	rs, _ := heatkw.Load(ctx, s.queries, workspaceID, nil)
	return s.buildContact(c, db.GetContactAggregatesByWorkspaceRow{}, nil, rs.Circle), nil
}

func normalizeContactCircle(circle heat.Circle) heat.Circle {
	switch circle {
	case heat.CircleFounder, heat.CircleInvestor, heat.CircleSales:
		return circle
	default:
		return heat.CircleDefault
	}
}

// SyncContacts materializes contact rows for every visitor email seen in the workspace.
func (s *Service) SyncContacts(ctx context.Context, workspaceID string) error {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return err
	}

	emails, err := s.queries.FindUnsyncedContactEmails(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("find unsynced emails: %w", err)
	}

	upserted := 0
	for _, email := range emails {
		if !email.Valid || email.String == "" {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(email.String))
		if normalized == "" {
			continue
		}
		_, err := s.queries.UpsertContactByEmail(ctx, db.UpsertContactByEmailParams{
			WorkspaceID: wsUUID,
			Email:       pgtype.Text{String: normalized, Valid: true},
			Name:        "",
		})
		if err != nil {
			return fmt.Errorf("upsert contact %s: %w", normalized, err)
		}
		upserted++
	}
	if upserted > 0 {
		s.invalidateListCache(ctx, workspaceID)
	}
	return nil
}

// ListContacts returns enriched contacts for a workspace.
func (s *Service) ListContacts(ctx context.Context, workspaceID string) ([]Contact, error) {
	cacheKey := contactListCacheKey(workspaceID)
	if s.cache != nil {
		var cached []Contact
		if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}

	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	if err := s.SyncContacts(ctx, workspaceID); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListContactsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}

	rs, _ := heatkw.Load(ctx, s.queries, workspaceID, nil)
	aggRows, err := s.queries.GetContactAggregatesByWorkspace(ctx, db.GetContactAggregatesByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       10000,
		Patterns:    rs.Patterns(),
	})
	if err != nil {
		return nil, fmt.Errorf("contact aggregates: %w", err)
	}
	aggByEmail := make(map[string]db.GetContactAggregatesByWorkspaceRow, len(aggRows))
	for _, a := range aggRows {
		aggByEmail[strings.ToLower(a.Email)] = a
	}

	viewedRows, err := s.queries.ListContactViewedDocumentIDsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, fmt.Errorf("batch viewed documents: %w", err)
	}
	viewedByEmail := make(map[string][]string, len(rows))
	for _, row := range viewedRows {
		email := strings.ToLower(row.Email)
		viewedByEmail[email] = append(viewedByEmail[email], row.DocumentID)
	}

	out := make([]Contact, 0, len(rows))
	for _, c := range rows {
		email := strings.ToLower(c.Email.String)
		agg := aggByEmail[email]
		out = append(out, s.buildContact(c, agg, viewedByEmail[email], rs.Circle))
	}

	sortContacts(out)
	if s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, out, contactListCacheTTL)
	}
	return out, nil
}

// GetContact returns a single enriched contact.
func (s *Service) GetContact(ctx context.Context, workspaceID, contactID string) (Contact, error) {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return Contact{}, err
	}
	contactUUID, err := parseUUID(contactID)
	if err != nil {
		return Contact{}, err
	}

	c, err := s.queries.GetContactByID(ctx, db.GetContactByIDParams{
		ID:          contactUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Contact{}, ErrContactNotFound
		}
		return Contact{}, fmt.Errorf("get contact: %w", err)
	}

	visitorEmail := ""
	if c.Email.Valid {
		visitorEmail = c.Email.String
	}
	rs, _ := heatkw.Load(ctx, s.queries, workspaceID, nil)
	agg, err := s.queries.GetContactAggregateByEmail(ctx, db.GetContactAggregateByEmailParams{
		WorkspaceID:  wsUUID,
		VisitorEmail: visitorEmail,
		Patterns:     rs.Patterns(),
	})
	if err != nil {
		return Contact{}, fmt.Errorf("contact aggregate: %w", err)
	}

	viewedRows, err := s.queries.ListContactViewedDocuments(ctx, db.ListContactViewedDocumentsParams{
		WorkspaceID:  wsUUID,
		VisitorEmail: visitorEmail,
	})
	if err != nil {
		return Contact{}, fmt.Errorf("viewed documents: %w", err)
	}
	viewedIDs := make([]string, 0, len(viewedRows))
	viewedItems := make([]ViewedDocument, 0, len(viewedRows))
	for _, row := range viewedRows {
		viewedIDs = append(viewedIDs, row.DocumentID)
		title := strings.TrimSpace(row.Title)
		if title == "" {
			title = row.DocumentID
		}
		viewedItems = append(viewedItems, ViewedDocument{ID: row.DocumentID, Title: title})
	}

	wsAgg := toWorkspaceAggregate(agg)
	contact := s.buildContact(c, wsAgg, viewedIDs, rs.Circle)
	contact.ViewedDocumentItems = viewedItems
	contact.KeyPages = s.contactKeyPageEvidence(ctx, wsUUID, visitorEmail, rs.Patterns(), wsAgg)

	// Detail trend: real daily engagement from recent events (not a stub empty array).
	actRows, err := s.queries.ListContactActivitiesByEmail(ctx, db.ListContactActivitiesByEmailParams{
		WorkspaceID:  wsUUID,
		VisitorEmail: visitorEmail,
		RowLimit:     500,
	})
	if err != nil {
		return Contact{}, fmt.Errorf("engagement history: %w", err)
	}
	contact.ScoreHistory = engagementHistoryFromActivities(actRows, 14)

	return contact, nil
}

// ListActivities returns engagement events for a contact.
func (s *Service) ListActivities(ctx context.Context, workspaceID, contactID string, limit int32) ([]Activity, error) {
	if limit <= 0 {
		limit = 100
	}

	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	contactUUID, err := parseUUID(contactID)
	if err != nil {
		return nil, err
	}

	c, err := s.queries.GetContactByID(ctx, db.GetContactByIDParams{
		ID:          contactUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrContactNotFound
		}
		return nil, fmt.Errorf("get contact: %w", err)
	}

	visitorEmail := ""
	if c.Email.Valid {
		visitorEmail = c.Email.String
	}
	rows, err := s.queries.ListContactActivitiesByEmail(ctx, db.ListContactActivitiesByEmailParams{
		WorkspaceID:  wsUUID,
		VisitorEmail: visitorEmail,
		RowLimit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}

	contactIDStr := uuidToString(c.ID)
	out := make([]Activity, 0, len(rows))
	for _, r := range rows {
		eventType := mapEventType(r.EventType)
		docID := ""
		if r.DocumentID.Valid {
			docID = uuidToString(r.DocumentID)
		}
		out = append(out, Activity{
			ID:              uuidToString(r.ID),
			ContactID:       contactIDStr,
			ContactEmail:    visitorEmail,
			LinkID:          uuidToString(r.LinkID),
			DocumentID:      docID,
			DocumentTitle:   r.DocumentTitle,
			EventType:       eventType,
			PageNumber:      r.PageNumber,
			DurationSeconds: r.DurationSeconds,
			Timestamp:       r.CreatedAt.Time.Format(time.RFC3339),
			Description:     activityDocumentTitle(r.DocumentTitle),
		})
	}
	return out, nil
}

func (s *Service) buildContact(c db.Contact, agg db.GetContactAggregatesByWorkspaceRow, viewed []string, circle heat.Circle) Contact {
	email := c.Email.String
	name := displayName(c, email)

	avgMin := 0.0
	if agg.TotalPageViews > 0 {
		avgMin = float64(agg.TotalDurationSeconds) / 60.0 / float64(agg.TotalPageViews)
	}
	revisits := int(agg.Opens) - int(agg.UniqueVisitors)
	if revisits < 0 {
		revisits = 0
	}

	res := heat.Compute(normalizeContactCircle(circle), heat.Input{
		Opens:              int(agg.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: avgMin,
		KeyPageViews:       int(agg.KeyPageViews),
		ForwardSignals:     int(agg.ForwardSignals),
		Downloads:          int(agg.Downloads),
		BouncePenalty:      int(agg.Bounces),
		DecayDays:          contactDecayDays(agg.LastSeenAt),
	})
	if res.Level == "" {
		res.Level = "cold"
	}

	contact := Contact{
		ID:    uuidToString(c.ID),
		Email: email,
		Name:  name,
		// Organization is only set when we have a real CRM value — never invent
		// one from the email domain (that presents as fake company data).
		HeatLevel:            res.Level,
		Score:                res.Score,
		ScoreHistory:         []ScorePoint{},
		TotalVisits:          agg.Opens,
		TotalDurationSeconds: agg.TotalDurationSeconds,
		ViewedDocuments:      viewed,
	}
	if agg.LastSeenAt.Valid {
		contact.LastSeenAt = agg.LastSeenAt.Time.Format(time.RFC3339)
	}
	return contact
}

// contactDecayDays mirrors link heat: days since last real activity.
// Missing LastSeenAt means no decay factor (typically zero-activity rows).
func contactDecayDays(lastSeen pgtype.Timestamptz) float64 {
	if !lastSeen.Valid {
		return 0
	}
	return time.Since(lastSeen.Time).Hours() / 24
}

// engagementHistoryFromActivities buckets recent events by UTC day for trend charts.
func engagementHistoryFromActivities(rows []db.ListContactActivitiesByEmailRow, days int) []ScorePoint {
	if days <= 0 {
		days = 14
	}
	now := time.Now().UTC()
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	counts := make(map[string]int, days)
	for _, r := range rows {
		if !r.CreatedAt.Valid {
			continue
		}
		ts := r.CreatedAt.Time.UTC()
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
		if day.Before(startDay) {
			continue
		}
		counts[day.Format("2006-01-02")]++
	}
	out := make([]ScorePoint, 0, days)
	total := 0
	for d := 0; d < days; d++ {
		day := startDay.AddDate(0, 0, d)
		key := day.Format("2006-01-02")
		n := counts[key]
		total += n
		out = append(out, ScorePoint{
			Date:   day.Format(time.RFC3339),
			Events: n,
			Score:  n, // deprecated alias of Events
		})
	}
	// Omit all-zero series so the UI can show a true empty state.
	if total == 0 {
		return []ScorePoint{}
	}
	return out
}

// activityDocumentTitle returns a machine-neutral document title for clients.
// UI copy (event verbs, page labels) must be localized on the frontend.
func activityDocumentTitle(documentTitle string) string {
	return strings.TrimSpace(documentTitle)
}

// contactKeyPageEvidence is GET-only. Fail-open: totals from the aggregate still return.
func (s *Service) contactKeyPageEvidence(ctx context.Context, workspaceID pgtype.UUID, visitorEmail string, patterns []string, agg db.GetContactAggregatesByWorkspaceRow) *ContactKeyPages {
	if agg.KeyPageViews <= 0 && agg.TotalKeyPageViews <= 0 {
		return nil
	}
	out := &ContactKeyPages{
		Engaged:    agg.KeyPageViews,
		Total:      agg.TotalKeyPageViews,
		MinSeconds: contactKeyPageMinSeconds,
		Pages:      []ContactKeyPage{},
	}
	if visitorEmail == "" || len(patterns) == 0 {
		return out
	}
	rows, err := s.queries.GetContactKeyPageViewDetails(ctx, db.GetContactKeyPageViewDetailsParams{
		WorkspaceID:  workspaceID,
		VisitorEmail: visitorEmail,
		Patterns:     patterns,
	})
	if err != nil {
		return out
	}
	out.Pages = make([]ContactKeyPage, 0, len(rows))
	for _, row := range rows {
		out.Pages = append(out.Pages, ContactKeyPage{
			PageNumber:   row.PageNumber,
			Title:        heat.DisplayablePageTitle(row.Title),
			EngagedViews: row.EngagedViews,
			TotalViews:   row.TotalViews,
		})
	}
	return out
}

func toWorkspaceAggregate(r db.GetContactAggregateByEmailRow) db.GetContactAggregatesByWorkspaceRow {
	return db.GetContactAggregatesByWorkspaceRow{
		Email:                "",
		Opens:                r.Opens,
		UniqueLinks:          r.UniqueLinks,
		UniqueVisitors:       r.UniqueVisitors,
		TotalDurationSeconds: r.TotalDurationSeconds,
		TotalPageViews:       r.TotalPageViews,
		KeyPageViews:         r.KeyPageViews,
		TotalKeyPageViews:    r.TotalKeyPageViews,
		ForwardSignals:       r.ForwardSignals,
		Downloads:            r.Downloads,
		Bounces:              r.Bounces,
		LastSeenAt:           r.LastSeenAt,
	}
}

func displayName(c db.Contact, email string) string {
	if c.Name.Valid && strings.TrimSpace(c.Name.String) != "" {
		return strings.TrimSpace(c.Name.String)
	}
	if email == "" {
		return "Unknown"
	}
	local := strings.Split(email, "@")[0]
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.ReplaceAll(local, "-", " ")
	return cases.Title(language.English).String(local)
}

func mapEventType(t string) string {
	switch t {
	case "link_opened":
		return "open"
	case "download_attempted":
		return "download"
	case "page_viewed":
		return "page_view"
	case "forward_signal":
		// Detected share/forward marker persisted on access_logs.
		return "share"
	case "return_visit":
		// Same visitor returning after a prior open (DetectForwardOrReturn).
		return "revisit"
	default:
		return t
	}
}

func sortContacts(contacts []Contact) {
	sort.Slice(contacts, func(i, j int) bool {
		if contacts[i].Score != contacts[j].Score {
			return contacts[i].Score > contacts[j].Score
		}
		return contacts[i].LastSeenAt > contacts[j].LastSeenAt
	})
}

func (s *Service) invalidateListCache(ctx context.Context, workspaceID string) {
	if s == nil || s.cache == nil || workspaceID == "" {
		return
	}
	_ = s.cache.Delete(ctx, contactListCacheKey(workspaceID))
}

func parseUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}
