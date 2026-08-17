package contact

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockContactQuerier struct {
	unsyncedEmails    []pgtype.Text
	upserted          []db.UpsertContactByEmailParams
	contacts          []db.Contact
	contactByID       db.Contact
	contactByIDErr    error
	aggregate         db.GetContactAggregateByEmailRow
	aggregates        []db.GetContactAggregatesByWorkspaceRow
	activities        []db.ListContactActivitiesByEmailRow
	viewedDocs        []string
	viewedDocRows     []db.ListContactViewedDocumentsRow
	keyPageDetails    []db.GetContactKeyPageViewDetailsRow
	keyPageDetailsN   int
	keyPageDetailsErr error
}

func (m *mockContactQuerier) CreateContact(_ context.Context, arg db.CreateContactParams) (db.Contact, error) {
	var name pgtype.Text
	if v, ok := arg.Name.(pgtype.Text); ok {
		name = v
	}
	return db.Contact{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID: arg.WorkspaceID,
		Email:       arg.Email,
		Name:        name,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil
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

func (m *mockContactQuerier) GetContactByID(_ context.Context, arg db.GetContactByIDParams) (db.Contact, error) {
	if m.contactByIDErr != nil {
		return db.Contact{}, m.contactByIDErr
	}
	// Mirror SQL: WHERE id = $1 AND workspace_id = $2
	if m.contactByID.ID.Valid && m.contactByID.WorkspaceID.Valid {
		if uuid.UUID(m.contactByID.ID.Bytes) != uuid.UUID(arg.ID.Bytes) ||
			uuid.UUID(m.contactByID.WorkspaceID.Bytes) != uuid.UUID(arg.WorkspaceID.Bytes) {
			return db.Contact{}, pgx.ErrNoRows
		}
	}
	return m.contactByID, nil
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

func (m *mockContactQuerier) GetContactKeyPageViewDetails(_ context.Context, _ db.GetContactKeyPageViewDetailsParams) ([]db.GetContactKeyPageViewDetailsRow, error) {
	m.keyPageDetailsN++
	if m.keyPageDetailsErr != nil {
		return nil, m.keyPageDetailsErr
	}
	return m.keyPageDetails, nil
}

func (m *mockContactQuerier) ListContactActivitiesByEmail(_ context.Context, _ db.ListContactActivitiesByEmailParams) ([]db.ListContactActivitiesByEmailRow, error) {
	return m.activities, nil
}

func (m *mockContactQuerier) ListContactViewedDocumentIDs(_ context.Context, _ db.ListContactViewedDocumentIDsParams) ([]string, error) {
	return m.viewedDocs, nil
}

func (m *mockContactQuerier) ListContactViewedDocuments(_ context.Context, _ db.ListContactViewedDocumentsParams) ([]db.ListContactViewedDocumentsRow, error) {
	if len(m.viewedDocRows) > 0 {
		return m.viewedDocRows, nil
	}
	out := make([]db.ListContactViewedDocumentsRow, 0, len(m.viewedDocs))
	for _, id := range m.viewedDocs {
		out = append(out, db.ListContactViewedDocumentsRow{DocumentID: id, Title: id})
	}
	return out, nil
}

func (m *mockContactQuerier) ListContactViewedDocumentIDsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.ListContactViewedDocumentIDsByWorkspaceRow, error) {
	out := make([]db.ListContactViewedDocumentIDsByWorkspaceRow, 0, len(m.viewedDocs))
	for _, id := range m.viewedDocs {
		out = append(out, db.ListContactViewedDocumentIDsByWorkspaceRow{
			Email:      "a@example.com",
			DocumentID: id,
		})
	}
	return out, nil
}

func (m *mockContactQuerier) GetWorkspaceKeyPageSettings(_ context.Context, _ pgtype.UUID) (db.WorkspaceKeyPageSetting, error) {
	return db.WorkspaceKeyPageSetting{}, pgx.ErrNoRows
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

func TestEngagementHistoryFromActivities(t *testing.T) {
	now := time.Now().UTC()
	rows := []db.ListContactActivitiesByEmailRow{
		{CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}},
		{CreatedAt: pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true}},
		{CreatedAt: pgtype.Timestamptz{Time: now.AddDate(0, 0, -1), Valid: true}},
	}
	hist := engagementHistoryFromActivities(rows, 3)
	if len(hist) != 3 {
		t.Fatalf("len = %d, want 3", len(hist))
	}
	total := hist[0].Score + hist[1].Score + hist[2].Score
	if total != 3 {
		t.Fatalf("total events = %d, want 3 (hist=%v)", total, hist)
	}
	if hist[2].Score < 1 {
		t.Fatalf("today bucket should include events, got %v", hist)
	}
}

func TestBuildContactComputesHeatScore(t *testing.T) {
	q := &mockContactQuerier{}
	svc := NewService(q)
	c := db.Contact{
		ID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Email: pgtype.Text{String: "a@horizon.vc", Valid: true},
		Name:  pgtype.Text{String: "A User", Valid: true},
	}
	agg := db.GetContactAggregatesByWorkspaceRow{
		Opens:                5,
		UniqueVisitors:       3,
		TotalPageViews:       4,
		KeyPageViews:         2,
		ForwardSignals:       1,
		TotalDurationSeconds: 240,
		Downloads:            1,
		LastSeenAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	contact := svc.buildContact(c, agg, []string{"doc-1", "doc-2"}, heat.CircleDefault)

	if contact.Email != "a@horizon.vc" {
		t.Fatalf("expected email a@horizon.vc, got %s", contact.Email)
	}
	if contact.Organization != "" {
		t.Fatalf("organization must not be invented from email domain, got %q", contact.Organization)
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

	// Must not treat TotalPageViews as KeyPageViews (historical fake proxy).
	base := db.GetContactAggregatesByWorkspaceRow{
		Opens:                2,
		UniqueVisitors:       1,
		TotalPageViews:       8,
		KeyPageViews:         0,
		ForwardSignals:       0,
		Bounces:              0,
		TotalDurationSeconds: 60,
	}
	asProxy := base
	asProxy.KeyPageViews = asProxy.TotalPageViews // old fake: all page views = key pages
	honestScore := svc.buildContact(c, base, nil, heat.CircleDefault).Score
	proxyScore := svc.buildContact(c, asProxy, nil, heat.CircleDefault).Score
	if proxyScore <= honestScore {
		t.Fatalf("proxy KeyPageViews=TotalPageViews must inflate score; honest=%d proxy=%d", honestScore, proxyScore)
	}

	// Real bounce counts must lower heat vs hard-coded BouncePenalty=0.
	withBounces := base
	withBounces.Bounces = 5
	bouncedScore := svc.buildContact(c, withBounces, nil, heat.CircleDefault).Score
	if bouncedScore >= honestScore {
		t.Fatalf("bounces must reduce score; honest=%d bounced=%d", honestScore, bouncedScore)
	}

	// Stale LastSeenAt must apply DecayDays (same contract as link heat).
	fresh := agg
	fresh.LastSeenAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	stale := agg
	stale.LastSeenAt = pgtype.Timestamptz{Time: time.Now().Add(-30 * 24 * time.Hour), Valid: true}
	freshScore := svc.buildContact(c, fresh, nil, heat.CircleDefault).Score
	staleScore := svc.buildContact(c, stale, nil, heat.CircleDefault).Score
	if staleScore >= freshScore {
		t.Fatalf("stale LastSeenAt must decay score; fresh=%d stale=%d", freshScore, staleScore)
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

func TestSyncContactsNormalizesEmailCase(t *testing.T) {
	q := &mockContactQuerier{
		unsyncedEmails: []pgtype.Text{
			{String: "  Alice@Example.COM ", Valid: true},
		},
	}
	svc := NewService(q)
	if err := svc.SyncContacts(context.Background(), uuid.New().String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(q.upserted))
	}
	got := q.upserted[0].Email.String
	if got != "alice@example.com" {
		t.Fatalf("expected lowercased email, got %q", got)
	}
}

type mockContactCache struct {
	deleted []string
	store   map[string][]Contact
}

func (m *mockContactCache) Get(_ context.Context, key string, dest interface{}) error {
	if m.store == nil {
		return errors.New("miss")
	}
	v, ok := m.store[key]
	if !ok {
		return errors.New("miss")
	}
	ptr, ok := dest.(*[]Contact)
	if !ok {
		return errors.New("bad dest")
	}
	*ptr = v
	return nil
}

func (m *mockContactCache) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	if m.store == nil {
		m.store = map[string][]Contact{}
	}
	v, ok := value.([]Contact)
	if !ok {
		return errors.New("bad value")
	}
	m.store[key] = v
	return nil
}

func (m *mockContactCache) Delete(_ context.Context, key string) error {
	m.deleted = append(m.deleted, key)
	if m.store != nil {
		delete(m.store, key)
	}
	return nil
}

func TestCreateContactInvalidatesListCache(t *testing.T) {
	ws := uuid.New().String()
	cache := &mockContactCache{
		store: map[string][]Contact{
			contactListCacheKey(ws): {{ID: "stale", Email: "stale@example.com"}},
		},
	}
	q := &mockContactQuerier{}
	svc := NewService(q, WithCache(cache))
	_, err := svc.CreateContact(context.Background(), ws, CreateContactRequest{
		Email: "New@Example.com",
		Name:  "New",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.upserted) != 1 {
		t.Fatalf("expected UpsertContactByEmail, got %#v", q.upserted)
	}
	if got := q.upserted[0].Email.String; got != "new@example.com" {
		t.Fatalf("email normalized = %q, want new@example.com", got)
	}
	wantKey := contactListCacheKey(ws)
	if len(cache.deleted) != 1 || cache.deleted[0] != wantKey {
		t.Fatalf("expected Delete(%q), got %#v", wantKey, cache.deleted)
	}
	if _, ok := cache.store[wantKey]; ok {
		t.Fatal("stale list cache entry should be removed")
	}
}

func TestCreateContactIsIdempotentViaUpsert(t *testing.T) {
	ws := uuid.New().String()
	q := &mockContactQuerier{}
	svc := NewService(q)
	first, err := svc.CreateContact(context.Background(), ws, CreateContactRequest{Email: "dup@example.com"})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := svc.CreateContact(context.Background(), ws, CreateContactRequest{Email: "dup@example.com", Name: "Dup"})
	if err != nil {
		t.Fatalf("second create must not fail: %v", err)
	}
	if first.Email != "dup@example.com" || second.Email != "dup@example.com" {
		t.Fatalf("emails = %q / %q", first.Email, second.Email)
	}
	if len(q.upserted) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(q.upserted))
	}
}

func TestSyncContactsInvalidatesListCacheWhenUpserted(t *testing.T) {
	ws := uuid.New().String()
	cache := &mockContactCache{
		store: map[string][]Contact{
			contactListCacheKey(ws): {{ID: "stale", Email: "stale@example.com"}},
		},
	}
	q := &mockContactQuerier{
		unsyncedEmails: []pgtype.Text{{String: "a@example.com", Valid: true}},
	}
	svc := NewService(q, WithCache(cache))
	if err := svc.SyncContacts(context.Background(), ws); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantKey := contactListCacheKey(ws)
	if len(cache.deleted) != 1 || cache.deleted[0] != wantKey {
		t.Fatalf("expected Delete(%q), got %#v", wantKey, cache.deleted)
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
	if q.keyPageDetailsN != 0 {
		t.Fatalf("list must not load key-page explain details, calls=%d", q.keyPageDetailsN)
	}
	if contacts[0].KeyPages != nil || contacts[1].KeyPages != nil {
		t.Fatal("list must omit keyPages")
	}
}

func TestGetContactAttachesKeyPageEvidence(t *testing.T) {
	ws := uuid.New()
	contactID := uuid.New()
	q := &mockContactQuerier{
		contactByID: db.Contact{
			ID:          pgtype.UUID{Bytes: contactID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
			Email:       pgtype.Text{String: "a@example.com", Valid: true},
			Name:        pgtype.Text{String: "A User", Valid: true},
		},
		aggregate: db.GetContactAggregateByEmailRow{
			Opens:             2,
			UniqueVisitors:    1,
			KeyPageViews:      0,
			TotalKeyPageViews: 1,
		},
		keyPageDetails: []db.GetContactKeyPageViewDetailsRow{
			{PageNumber: 1, Title: "Financials", EngagedViews: 0, TotalViews: 1},
		},
	}
	svc := NewService(q)
	got, err := svc.GetContact(context.Background(), ws.String(), contactID.String())
	if err != nil {
		t.Fatalf("GetContact: %v", err)
	}
	if got.KeyPages == nil || got.KeyPages.Total != 1 || got.KeyPages.Engaged != 0 {
		t.Fatalf("keyPages=%#v", got.KeyPages)
	}
	if got.KeyPages.MinSeconds != contactKeyPageMinSeconds {
		t.Fatalf("minSeconds=%d", got.KeyPages.MinSeconds)
	}
	if len(got.KeyPages.Pages) != 1 || got.KeyPages.Pages[0].Title != "Financials" {
		t.Fatalf("pages=%#v", got.KeyPages.Pages)
	}
}

func TestGetContactKeyPageEvidenceFailOpen(t *testing.T) {
	ws := uuid.New()
	contactID := uuid.New()
	q := &mockContactQuerier{
		contactByID: db.Contact{
			ID:          pgtype.UUID{Bytes: contactID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
			Email:       pgtype.Text{String: "a@example.com", Valid: true},
		},
		aggregate: db.GetContactAggregateByEmailRow{
			KeyPageViews:      1,
			TotalKeyPageViews: 2,
		},
		keyPageDetailsErr: errors.New("details unavailable"),
	}
	svc := NewService(q)
	got, err := svc.GetContact(context.Background(), ws.String(), contactID.String())
	if err != nil {
		t.Fatalf("GetContact must fail-open: %v", err)
	}
	if got.KeyPages == nil || got.KeyPages.Engaged != 1 || got.KeyPages.Total != 2 {
		t.Fatalf("fail-open totals=%#v", got.KeyPages)
	}
	if len(got.KeyPages.Pages) != 0 {
		t.Fatalf("fail-open pages=%#v", got.KeyPages.Pages)
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

func TestGetContactRejectsOtherWorkspaceIDOR(t *testing.T) {
	homeWS := uuid.New()
	otherWS := uuid.New()
	contactID := uuid.New()
	q := &mockContactQuerier{
		contactByID: db.Contact{
			ID:          pgtype.UUID{Bytes: contactID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: homeWS, Valid: true},
			Email:       pgtype.Text{String: "secret@home-ws.test", Valid: true},
			Name:        pgtype.Text{String: "Secret", Valid: true},
		},
	}
	svc := NewService(q)
	_, err := svc.GetContact(context.Background(), otherWS.String(), contactID.String())
	if !errors.Is(err, ErrContactNotFound) {
		t.Fatalf("expected ErrContactNotFound for cross-workspace IDOR, got %v", err)
	}
}

func TestListActivitiesRejectsOtherWorkspaceIDOR(t *testing.T) {
	homeWS := uuid.New()
	otherWS := uuid.New()
	contactID := uuid.New()
	q := &mockContactQuerier{
		contactByID: db.Contact{
			ID:          pgtype.UUID{Bytes: contactID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: homeWS, Valid: true},
			Email:       pgtype.Text{String: "secret@home-ws.test", Valid: true},
		},
		activities: []db.ListContactActivitiesByEmailRow{
			{
				ID:            pgtype.UUID{Bytes: uuid.New(), Valid: true},
				LinkID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
				EventType:     "link_opened",
				DocumentTitle: "Secret Deck",
				CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
		},
	}
	svc := NewService(q)
	_, err := svc.ListActivities(context.Background(), otherWS.String(), contactID.String(), 10)
	if !errors.Is(err, ErrContactNotFound) {
		t.Fatalf("expected ErrContactNotFound for cross-workspace activities IDOR, got %v", err)
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
			{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, LinkID: linkID, EventType: "forward_signal", DocumentTitle: "Pitch Deck", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, LinkID: linkID, EventType: "return_visit", DocumentTitle: "Pitch Deck", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	svc := NewService(q)
	acts, err := svc.ListActivities(context.Background(), ws.String(), cid.String(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(acts) != 5 {
		t.Fatalf("expected 5 activities, got %d", len(acts))
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
	if acts[3].EventType != "share" {
		t.Fatalf("expected share for forward_signal, got %s", acts[3].EventType)
	}
	if acts[4].EventType != "revisit" {
		t.Fatalf("expected revisit for return_visit, got %s", acts[4].EventType)
	}
}
