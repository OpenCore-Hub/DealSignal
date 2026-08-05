package link

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestNormalizeEmailList(t *testing.T) {
	got := normalizeEmailList([]string{" Alice@Example.com ", "", "bob@x.com", "alice@example.com", "  "})
	if len(got) != 2 || got[0] != "alice@example.com" || got[1] != "bob@x.com" {
		t.Fatalf("unexpected normalize result: %#v", got)
	}
}

func TestDocumentAllowEmailsFromCodes(t *testing.T) {
	got := documentAllowEmailsFromCodes([]emailCode{
		{email: "Alice@Example.com"},
		{email: "bob@x.com"},
		{email: "alice@example.com"},
	})
	if len(got) != 2 || got[0] != "alice@example.com" || got[1] != "bob@x.com" {
		t.Fatalf("unexpected emails: %#v", got)
	}
}

type fakeAccessRuleStore struct {
	rules []db.LinkAccessRule
}

func (f *fakeAccessRuleStore) ListLinkAccessRulesByLink(context.Context, pgtype.UUID) ([]db.LinkAccessRule, error) {
	out := make([]db.LinkAccessRule, len(f.rules))
	copy(out, f.rules)
	return out, nil
}

func (f *fakeAccessRuleStore) DeleteLinkAccessRulesByLink(context.Context, pgtype.UUID) error {
	f.rules = nil
	return nil
}

func (f *fakeAccessRuleStore) CreateLinkAccessRule(_ context.Context, arg db.CreateLinkAccessRuleParams) error {
	f.rules = append(f.rules, db.LinkAccessRule{
		TenantID:    arg.TenantID,
		WorkspaceID: arg.WorkspaceID,
		LinkID:      arg.LinkID,
		RuleType:    arg.RuleType,
		Value:       arg.Value,
		Action:      arg.Action,
		SortOrder:   arg.SortOrder,
	})
	return nil
}

func TestReplaceAllowEmailRulesPreservesBlocks(t *testing.T) {
	linkID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	wsID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	tenantID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	store := &fakeAccessRuleStore{
		rules: []db.LinkAccessRule{
			{LinkID: linkID, RuleType: "email", Value: "old@x.com", Action: "allow"},
			{LinkID: linkID, RuleType: "email", Value: "bad@x.com", Action: "block"},
		},
	}
	link := db.Link{ID: linkID, TenantID: tenantID, WorkspaceID: wsID}

	if err := replaceAllowEmailRules(context.Background(), store, link, wsID, []string{"new@x.com", "NEW@x.com"}); err != nil {
		t.Fatalf("replaceAllowEmailRules: %v", err)
	}
	if len(store.rules) != 2 {
		t.Fatalf("expected 2 rules, got %#v", store.rules)
	}
	byAction := map[string]string{}
	for _, r := range store.rules {
		byAction[r.Action] = r.Value
	}
	if byAction["allow"] != "new@x.com" {
		t.Fatalf("allow rule = %q, want new@x.com", byAction["allow"])
	}
	if byAction["block"] != "bad@x.com" {
		t.Fatalf("block rule = %q, want bad@x.com", byAction["block"])
	}
}

type fakeContactStore struct {
	rows []db.GetLinkContactsByPublicTokenRow
}

func (f *fakeContactStore) GetLinkContactsByPublicToken(context.Context, string) ([]db.GetLinkContactsByPublicTokenRow, error) {
	return f.rows, nil
}

func TestEnsureDocumentLinkAllowlistFromContactsBackfills(t *testing.T) {
	linkID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	wsID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	tenantID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	rules := &fakeAccessRuleStore{}
	contacts := &fakeContactStore{
		rows: []db.GetLinkContactsByPublicTokenRow{
			{ContactEmail: pgtype.Text{String: "alice@example.com", Valid: true}},
		},
	}
	link := db.Link{
		ID:                       linkID,
		TenantID:                 tenantID,
		WorkspaceID:              wsID,
		PublicToken:              "tok",
		RequireEmailVerification: true,
	}

	if err := ensureDocumentLinkAllowlistFromContacts(context.Background(), rules, contacts, link); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if len(rules.rules) != 1 || rules.rules[0].Action != "allow" || rules.rules[0].Value != "alice@example.com" {
		t.Fatalf("unexpected rules after backfill: %#v", rules.rules)
	}

	// Additive: keep existing allows and append missing contact emails.
	rules.rules = []db.LinkAccessRule{
		{LinkID: linkID, RuleType: "email", Value: "extra@example.com", Action: "allow", SortOrder: 0},
	}
	contacts.rows = []db.GetLinkContactsByPublicTokenRow{
		{ContactEmail: pgtype.Text{String: "alice@example.com", Valid: true}},
		{ContactEmail: pgtype.Text{String: "bob@example.com", Valid: true}},
	}
	if err := ensureDocumentLinkAllowlistFromContacts(context.Background(), rules, contacts, link); err != nil {
		t.Fatalf("ensure additive: %v", err)
	}
	got := map[string]bool{}
	for _, r := range rules.rules {
		if r.Action == "allow" {
			got[r.Value] = true
		}
	}
	if !got["extra@example.com"] || !got["alice@example.com"] || !got["bob@example.com"] {
		t.Fatalf("expected additive allowset, got %#v", rules.rules)
	}
}

func TestEnsureDocumentLinkAllowlistSkipsDealRoom(t *testing.T) {
	rules := &fakeAccessRuleStore{}
	contacts := &fakeContactStore{
		rows: []db.GetLinkContactsByPublicTokenRow{
			{ContactEmail: pgtype.Text{String: "alice@example.com", Valid: true}},
		},
	}
	link := db.Link{
		DealRoomID:               pgtype.UUID{Bytes: uuid.New(), Valid: true},
		RequireEmailVerification: true,
	}
	if err := ensureDocumentLinkAllowlistFromContacts(context.Background(), rules, contacts, link); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if len(rules.rules) != 0 {
		t.Fatalf("deal-room links must not be backfilled from contacts: %#v", rules.rules)
	}
}
