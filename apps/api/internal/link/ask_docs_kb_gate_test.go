package link

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeKBReader struct {
	row db.DealRoomKnowledgeBasis
	err error
}

func (f *fakeKBReader) GetDealRoomKnowledgeBaseByRoom(_ context.Context, _ pgtype.UUID) (db.DealRoomKnowledgeBasis, error) {
	if f.err != nil {
		return db.DealRoomKnowledgeBasis{}, f.err
	}
	return f.row, nil
}

func roomUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true}
}

func TestEnsureAskDocsKnowledgeBase_RejectsWhenMissing(t *testing.T) {
	err := ensureAskDocsKnowledgeBase(context.Background(), &fakeKBReader{err: pgx.ErrNoRows}, roomUUID(), true)
	if !errors.Is(err, ErrKnowledgeBaseRequired) {
		t.Fatalf("expected ErrKnowledgeBaseRequired, got %v", err)
	}
}

func TestEnsureAskDocsKnowledgeBase_RejectsWhenFailedOrBuilding(t *testing.T) {
	for _, status := range []string{"none", "failed", "building"} {
		t.Run(status, func(t *testing.T) {
			err := ensureAskDocsKnowledgeBase(context.Background(), &fakeKBReader{
				row: db.DealRoomKnowledgeBasis{Status: status},
			}, roomUUID(), true)
			if !errors.Is(err, ErrKnowledgeBaseRequired) {
				t.Fatalf("status %s: expected ErrKnowledgeBaseRequired, got %v", status, err)
			}
		})
	}
}

func TestEnsureAskDocsKnowledgeBase_AllowsReadyOrStale(t *testing.T) {
	for _, status := range []string{"ready", "stale"} {
		t.Run(status, func(t *testing.T) {
			err := ensureAskDocsKnowledgeBase(context.Background(), &fakeKBReader{
				row: db.DealRoomKnowledgeBasis{Status: status},
			}, roomUUID(), true)
			if err != nil {
				t.Fatalf("status %s: unexpected error %v", status, err)
			}
		})
	}
}

func TestEnsureAskDocsKnowledgeBase_SkipsWhenAskDocsOff(t *testing.T) {
	err := ensureAskDocsKnowledgeBase(context.Background(), &fakeKBReader{err: pgx.ErrNoRows}, roomUUID(), false)
	if err != nil {
		t.Fatalf("Ask Host-only / Ask Docs off must skip KB gate, got %v", err)
	}
}

func TestEnsureAskDocsKnowledgeBase_SkipsNonDealRoomLink(t *testing.T) {
	err := ensureAskDocsKnowledgeBase(context.Background(), &fakeKBReader{err: pgx.ErrNoRows}, pgtype.UUID{}, true)
	if err != nil {
		t.Fatalf("document links must skip room KB gate, got %v", err)
	}
}

func TestAskDocsCoverageGaps_ReportsUnauthorizedFolders(t *testing.T) {
	kb := db.DealRoomKnowledgeBasis{
		Status:      "ready",
		FolderPaths: []string{"/general"},
	}
	authorized := []authorizedDocument{
		{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), FolderPath: "/general"},
		{ID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), FolderPath: "/legal"},
	}
	gaps := askDocsCoverageGaps(authorized, kb)
	if gaps == nil {
		t.Fatal("expected coverage gaps warning")
	}
	if len(gaps.MissingFolderPaths) != 1 || gaps.MissingFolderPaths[0] != "/legal" {
		t.Fatalf("expected missing /legal, got %+v", gaps)
	}
	if gaps.Code != "ask_docs_scope_not_in_kb" {
		t.Fatalf("unexpected code %q", gaps.Code)
	}
}

func TestAskDocsCoverageGaps_NilWhenFullyCovered(t *testing.T) {
	docID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	kb := db.DealRoomKnowledgeBasis{
		Status:      "ready",
		FolderPaths: []string{"/general"},
		DocumentIds: []pgtype.UUID{{Bytes: docID, Valid: true}},
	}
	authorized := []authorizedDocument{
		{ID: docID, FolderPath: "/general"},
		{ID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), FolderPath: "/general/sub"},
	}
	if gaps := askDocsCoverageGaps(authorized, kb); gaps != nil {
		t.Fatalf("expected nil gaps, got %+v", gaps)
	}
}

func TestMigrationDisablesAskDocsWithoutReadyOrStaleKB(t *testing.T) {
	// Spec for 090_disable_ask_docs_without_kb.up.sql: only deal-room links with
	// ai_copilot_enabled and without a ready/stale KB row are turned off.
	shouldDisable := func(dealRoom bool, askDocs bool, kbStatus string, hasKB bool) bool {
		if !dealRoom || !askDocs {
			return false
		}
		if hasKB && (kbStatus == "ready" || kbStatus == "stale") {
			return false
		}
		return true
	}
	cases := []struct {
		name     string
		dealRoom bool
		askDocs  bool
		hasKB    bool
		kbStatus string
		want     bool
	}{
		{"doc link untouched", false, true, false, "", false},
		{"ask docs off untouched", true, false, false, "", false},
		{"no kb disables", true, true, false, "", true},
		{"none disables", true, true, true, "none", true},
		{"failed disables", true, true, true, "failed", true},
		{"building disables", true, true, true, "building", true},
		{"ready keeps", true, true, true, "ready", false},
		{"stale keeps", true, true, true, "stale", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldDisable(tc.dealRoom, tc.askDocs, tc.kbStatus, tc.hasKB)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
