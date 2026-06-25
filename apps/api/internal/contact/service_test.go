package contact

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockContactQuerier struct {
	unsyncedEmails   []pgtype.Text
	upserted         []db.UpsertContactByEmailParams
	contacts         []db.Contact
	contactByID      db.Contact
	contactByIDErr   error
	aggregate        db.GetContactAggregateByEmailRow
	aggregates       []db.GetContactAggregatesByWorkspaceRow
	activities       []db.ListContactActivitiesByEmailRow
	viewedDocs       []string
}

func (m *mockContactQuerier) FindUnsyncedContactEmails(_ context.Context, _ pgtype.UUID) ([]pgtype.Text, error) {
	return m.unsyncedEmails, nil
}

func (m *mockContactQuerier) UpsertContactByEmail(_ context.Context, arg db.UpsertContactByEmailParams) (db.Contact, error) {
	m.upserted = append(m.upserted, arg)
	return db.Contact{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID: arg.WorkspaceID,
		Email:       arg.Email,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil
}

func (m *mockContactQuerier) GetContactByID(_ context.Context, _ db.GetContactByIDParams) (db.Contact, error) {
	return m.contactByID, m.contactByIDErr
}

func (m *mockContactQuerier) ListContactsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.Contact, error) {
	return m.contacts, nil
}

func (m *mockContactQuerier) GetContactAggregateByEmail(_ context.Context, _ db.GetContactAggregateByEmailParams) (db.GetContactAggregateByEmailRow, error) {
	return m.aggregate, nil
}

func (m *mockContactQuerier) GetContactAggregatesByWorkspace(_ context.Context, _ db.GetContactAggregatesByWorkspaceParams) ([]db.GetContactAggregatesByWorkspaceRow, error) {
	return m.aggregates, nil
}

func (m *mockContactQuerier) ListContactActivitiesByEmail(_ context.Context, _ db.ListContactActivitiesByEmailParams) ([]db.ListContactActivitiesByEmailRow, error) {
	return m.activities, nil
}

func (m *mockContactQuerier) ListContactViewedDocumentIDs(_ context.Context, _ db.ListContactViewedDocumentIDsParams) ([]string, error) {
	return m.viewedDocs, nil
}

func TestDisplayNameFallsBackToEmailLocalPart(t *testing.T) {
	c := db.Contact{Name: pgtype.Text{Valid: false}}
	got := displayName(c, "sarah.chen@horizon.vc")
	if got != "Sarah Chen" {
		t.Fatalf("expected 'Sarah Chen', got %q", got)
	}
}

func TestDisplayNameUsesStoredName(t *testing.T) {
	c := db.Contact{Name: pgtype.Text{String: "Sarah Chen", Valid: true}}
	got := displayName(c, "other@example.com")
	if got != "Sarah Chen" {
		t.Fatalf("expected 'Sarah Chen', got %q", got)
	}
}

func TestBuildContactComputesHeatScore(t *testing.T) {
	q := &mockContactQuerier{}
	svc := NewService(q)
	c := db.Contact{
		ID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Email: pgtype.Text{String: "a@example.com", Valid: true},
		Name:  pgtype.Text{String: "A User", Valid: true},
	}
	agg := db.GetContactAggregatesByWorkspaceRow{
		Opens:                5,
		UniqueVisitors:       3,
		TotalPageViews:       4,
		TotalDurationSeconds: 240,
		Downloads:            1,
		LastSeenAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	contact := svc.buildContact(c, agg, []string{"doc-1", "doc-2"})

	if contact.Email != "a@example.com" {
		t.Fatalf("expected email a@example.com, got %s", contact.Email)
	}
	if contact.TotalVisits != 5 {
		t.Fatalf("expected 5 visits, got %d", contact.TotalVisits)
	}
	if contact.TotalDurationSeconds != 240 {
		t.Fatalf("expected 240s duration, got %d", contact.TotalDurationSeconds)
	}
	if contact.Score < 0 || contact.Score > 100 {
		t.Fatalf("score out of range: %d", contact.Score)
	}
	if contact.HeatLevel != "hot" && contact.HeatLevel != "warm" && contact.HeatLevel != "cold" {
		t.Fatalf("unexpected heat level: %s", contact.HeatLevel)
	}
	if len(contact.ViewedDocuments) != 2 {
		t.Fatalf("expected 2 viewed documents, got %d", len(contact.ViewedDocuments))
	}
}

func TestSyncContactsUpsertsUnsyncedEmails(t *testing.T) {
	q := &mockContactQuerier{
		unsyncedEmails: []pgtype.Text{
			{String: "a@example.com", Valid: true},
			{String: "b@example.com", Valid: true},
		},
	}
	svc := NewService(q)
	if err := svc.SyncContacts(context.Background(), uuid.New().String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.upserted) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(q.upserted))
	}
}

func TestListContactsSortsByScore(t *testing.T) {
	ws := uuid.New()
	contactA := db.Contact{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Email:       pgtype.Text{String: "a@example.com", Valid: true},
	}
	contactB := db.Contact{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Email:       pgtype.Text{String: "b@example.com", Valid: true},
	}

	q := &mockContactQuerier{
		contacts: []db.Contact{contactA, contactB},
		aggregates: []db.GetContactAggregatesByWorkspaceRow{
			{Email: "a@example.com", Opens: 1, UniqueVisitors: 1, TotalPageViews: 1, TotalDurationSeconds: 60},
			{Email: "b@example.com", Opens: 5, UniqueVisitors: 3, TotalPageViews: 4, TotalDurationSeconds: 240},
		},
		viewedDocs: []string{},
	}
	svc := NewService(q)
	contacts, err := svc.ListContacts(context.Background(), ws.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(contacts))
	}
	if contacts[0].Email != "b@example.com" {
		t.Fatalf("expected highest-score contact first, got %s", contacts[0].Email)
	}
}

func TestGetContactNotFound(t *testing.T) {
	q := &mockContactQuerier{contactByIDErr: pgx.ErrNoRows}
	svc := NewService(q)
	_, err := svc.GetContact(context.Background(), uuid.New().String(), uuid.New().String())
	if !errors.Is(err, ErrContactNotFound) {
		t.Fatalf("expected ErrContactNotFound, got %v", err)
	}
}

func TestListActivitiesMapsEventTypes(t *testing.T) {
	ws := uuid.New()
	cid := uuid.New()
	linkID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	c := db.Contact{
		ID:          pgtype.UUID{Bytes: cid, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Email:       pgtype.Text{String: "a@example.com", Valid: true},
	}

	q := &mockContactQuerier{
		contactByID: c,
		activities: []db.ListContactActivitiesByEmailRow{
			{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, LinkID: linkID, EventType: "link_opened", DocumentTitle: "Pitch Deck", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, LinkID: linkID, EventType: "page_viewed", PageNumber: 3, DurationSeconds: 45, DocumentTitle: "Pitch Deck", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, LinkID: linkID, EventType: "download_attempted", DocumentTitle: "Pitch Deck", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	svc := NewService(q)
	acts, err := svc.ListActivities(context.Background(), ws.String(), cid.String(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(acts) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(acts))
	}
	if acts[0].EventType != "open" {
		t.Fatalf("expected open, got %s", acts[0].EventType)
	}
	if acts[1].EventType != "page_view" || acts[1].PageNumber != 3 {
		t.Fatalf("expected page_view page 3, got %s %d", acts[1].EventType, acts[1].PageNumber)
	}
	if acts[2].EventType != "download" {
		t.Fatalf("expected download, got %s", acts[2].EventType)
	}
}
