package upload

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLiveTitleLookupExcludesArchived(t *testing.T) {
	sql := readQueriesSQL(t)

	live := queryNamed(sql, "GetDocumentByTitleInWorkspaceCategory")
	if !strings.Contains(live, "status IS DISTINCT FROM 'archived'") {
		t.Fatal("live title lookup must ignore archived overwrite snapshots")
	}
	if !strings.Contains(live, "category = $3") {
		t.Fatal("live library lookup must be category-scoped")
	}

	roomLive := queryNamed(sql, "GetLiveDealRoomDocumentByTitle")
	if roomLive == "" {
		t.Fatal("room upload must look up titles inside one live room")
	}
	if !strings.Contains(roomLive, "deal_room_documents") {
		t.Fatal("room title lookup must join membership")
	}

	any := queryNamed(sql, "GetDocumentByTitleInWorkspaceAny")
	if any == "" {
		t.Fatal("snapshot/restore title minting needs Any lookup including archived")
	}
	if strings.Contains(any, "status IS DISTINCT FROM 'archived'") {
		t.Fatal("Any lookup must see archived rows so snapshot titles stay unique")
	}

	recent := queryNamed(sql, "ListRecentDocumentsByWorkspace")
	if !strings.Contains(recent, "status IS DISTINCT FROM 'archived'") {
		t.Fatal("dashboard recent documents must not surface overwrite snapshots")
	}

	unarchive := queryNamed(sql, "UnarchiveDocument")
	if !strings.Contains(unarchive, "title = $4") {
		t.Fatal("unarchive must set title in the same write to rename on live collision")
	}
}

func TestLiveTitleUniqueIndexExcludesArchived(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "migrations", "174_documents_title_unique_live_partition.up.sql"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	if !strings.Contains(sql, "DROP INDEX IF EXISTS idx_documents_workspace_title_live") {
		t.Fatal("must drop the workspace-wide live title unique")
	}
	if !strings.Contains(sql, "idx_documents_workspace_title_live_general") {
		t.Fatal("must create the general live-only unique index")
	}
	if !strings.Contains(sql, "idx_documents_workspace_title_live_agreement") {
		t.Fatal("must create the agreement live-only unique index")
	}
	if !strings.Contains(sql, "status IS DISTINCT FROM 'archived'") {
		t.Fatal("live unique index must allow archived rows to reuse a filename")
	}
	if strings.Contains(sql, "category = 'deal_room'") {
		t.Fatal("deal_room must not have a workspace-wide live title unique")
	}
}

func readQueriesSQL(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "queries.sql"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read queries.sql: %v", err)
	}
	return string(raw)
}
