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

	live := queryNamed(sql, "GetDocumentByTitleInWorkspace")
	if !strings.Contains(live, "status IS DISTINCT FROM 'archived'") {
		t.Fatal("live title lookup must ignore archived overwrite snapshots")
	}
	if strings.Contains(live, "GetDocumentByTitleInWorkspaceAny") {
		t.Fatal("live lookup query body must not include the Any variant")
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
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "migrations", "172_documents_title_unique_live.up.sql"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	if !strings.Contains(sql, "DROP INDEX IF EXISTS idx_documents_workspace_title_alive") {
		t.Fatal("must drop the alive index that treated archived rows as occupying the title")
	}
	if !strings.Contains(sql, "idx_documents_workspace_title_live") {
		t.Fatal("must create the live-only unique index")
	}
	if !strings.Contains(sql, "status IS DISTINCT FROM 'archived'") {
		t.Fatal("live unique index must allow archived rows to reuse a filename")
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
