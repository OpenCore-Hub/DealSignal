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
	"github.com/google/uuid"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	ListContactActivitiesByEmail(ctx context.Context, arg db.ListContactActivitiesByEmailParams) ([]db.ListContactActivitiesByEmailRow, error)
	ListContactViewedDocumentIDs(ctx context.Context, arg db.ListContactViewedDocumentIDsParams) ([]string, error)
}

// Service aggregates visitor activity into contact records.
type Service struct {
	queries Querier
}

// NewService creates a contact service.
func NewService(q Querier) *Service {
	return &Service{queries: q}
}

// Contact is the enriched response model for a workspace contact.
type Contact struct {
	ID                   string       `json:"id"`
	Email                string       `json:"email"`
	Name                 string       `json:"name"`
	Organization         string       `json:"organization,omitempty"`
	Role                 string       `json:"role,omitempty"`
	HeatLevel            string       `json:"heatLevel"`
	Score                int          `json:"score"`
	ScoreHistory         []ScorePoint `json:"scoreHistory"`
	TotalVisits          int64        `json:"totalVisits"`
	TotalDurationSeconds int64        `json:"totalDurationSeconds"`
	LastSeenAt           string       `json:"lastSeenAt,omitempty"`
	ViewedDocuments      []string     `json:"viewedDocuments"`
}

// ScorePoint is a single score snapshot.
type ScorePoint struct {
	Date  string `json:"date"`
	Score int    `json:"score"`
}

// Activity is a single contact engagement event.
type Activity struct {
	ID              string `json:"id"`
	ContactID       string `json:"contactId"`
	ContactEmail    string `json:"contactEmail"`
	LinkID          string `json:"linkId"`
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

	c, err := s.queries.CreateContact(ctx, db.CreateContactParams{
		WorkspaceID: wsUUID,
		Email:       pgtype.Text{String: strings.ToLower(email), Valid: true},
		Name:        pgtype.Text{String: strings.TrimSpace(req.Name), Valid: req.Name != ""},
	})
	if err != nil {
		return Contact{}, fmt.Errorf("create contact: %w", err)
	}

	return s.buildContact(c, db.GetContactAggregatesByWorkspaceRow{}, nil), nil
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

	for _, email := range emails {
		if !email.Valid || email.String == "" {
			continue
		}
		_, err := s.queries.UpsertContactByEmail(ctx, db.UpsertContactByEmailParams{
			WorkspaceID: wsUUID,
			Email:       email,
			Name:        "",
		})
		if err != nil {
			return fmt.Errorf("upsert contact %s: %w", email.String, err)
		}
	}
	return nil
}

// ListContacts returns enriched contacts for a workspace.
func (s *Service) ListContacts(ctx context.Context, workspaceID string) ([]Contact, error) {
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

	aggRows, err := s.queries.GetContactAggregatesByWorkspace(ctx, db.GetContactAggregatesByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       10000,
	})
	if err != nil {
		return nil, fmt.Errorf("contact aggregates: %w", err)
	}
	aggByEmail := make(map[string]db.GetContactAggregatesByWorkspaceRow, len(aggRows))
	for _, a := range aggRows {
		aggByEmail[strings.ToLower(a.Email)] = a
	}

	out := make([]Contact, 0, len(rows))
	for _, c := range rows {
		email := c.Email.String
		agg := aggByEmail[strings.ToLower(email)]
		viewed, err := s.queries.ListContactViewedDocumentIDs(ctx, db.ListContactViewedDocumentIDsParams{
			WorkspaceID:  wsUUID,
			VisitorEmail: c.Email,
		})
		if err != nil {
			return nil, fmt.Errorf("viewed documents for %s: %w", email, err)
		}
		contact := s.buildContact(c, agg, viewed)
		out = append(out, contact)
	}

	sortContacts(out)
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

	agg, err := s.queries.GetContactAggregateByEmail(ctx, db.GetContactAggregateByEmailParams{
		WorkspaceID:  wsUUID,
		VisitorEmail: c.Email,
	})
	if err != nil {
		return Contact{}, fmt.Errorf("contact aggregate: %w", err)
	}

	viewed, err := s.queries.ListContactViewedDocumentIDs(ctx, db.ListContactViewedDocumentIDsParams{
		WorkspaceID:  wsUUID,
		VisitorEmail: c.Email,
	})
	if err != nil {
		return Contact{}, fmt.Errorf("viewed documents: %w", err)
	}

	return s.buildContact(c, toWorkspaceAggregate(agg), viewed), nil
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

	rows, err := s.queries.ListContactActivitiesByEmail(ctx, db.ListContactActivitiesByEmailParams{
		WorkspaceID:  wsUUID,
		VisitorEmail: c.Email,
		Limit:        limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}

	contactIDStr := uuidToString(c.ID)
	email := c.Email.String
	out := make([]Activity, 0, len(rows))
	for _, r := range rows {
		out = append(out, Activity{
			ID:              uuidToString(r.ID),
			ContactID:       contactIDStr,
			ContactEmail:    email,
			LinkID:          uuidToString(r.LinkID),
			DocumentTitle:   r.DocumentTitle,
			EventType:       mapEventType(r.EventType),
			PageNumber:      r.PageNumber,
			DurationSeconds: r.DurationSeconds,
			Timestamp:       r.CreatedAt.Time.Format(time.RFC3339),
			Description:     "",
		})
	}
	return out, nil
}

func (s *Service) buildContact(c db.Contact, agg db.GetContactAggregatesByWorkspaceRow, viewed []string) Contact {
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

	res := heat.Compute(heat.CircleDefault, heat.Input{
		Opens:              int(agg.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: avgMin,
		KeyPageViews:       int(agg.TotalPageViews),
		ForwardSignals:     int(agg.UniqueVisitors),
		Downloads:          int(agg.Downloads),
		BouncePenalty:      0,
	})
	if res.Level == "" {
		res.Level = "cold"
	}

	contact := Contact{
		ID:                   uuidToString(c.ID),
		Email:                email,
		Name:                 name,
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

func toWorkspaceAggregate(r db.GetContactAggregateByEmailRow) db.GetContactAggregatesByWorkspaceRow {
	return db.GetContactAggregatesByWorkspaceRow{
		Email:                "",
		Opens:                r.Opens,
		UniqueLinks:          r.UniqueLinks,
		UniqueVisitors:       r.UniqueVisitors,
		TotalDurationSeconds: r.TotalDurationSeconds,
		TotalPageViews:       r.TotalPageViews,
		Downloads:            r.Downloads,
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
