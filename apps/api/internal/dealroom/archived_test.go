package dealroom

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsArchivedDocumentStatus(t *testing.T) {
	if !IsArchivedDocumentStatus("archived") || !IsArchivedDocumentStatus("Archived") {
		t.Fatal("archived status must match")
	}
	if IsArchivedDocumentStatus("ready") || IsArchivedDocumentStatus("") {
		t.Fatal("live/empty status must not match")
	}
}

func TestRoomAggregateDocumentCountSQLExcludesArchived(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "queries.sql"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read queries.sql: %v", err)
	}
	sql := string(raw)
	for _, name := range []string{"GetDealRoomAggregatesForRooms", "GetDealRoomAggregatesByWorkspace"} {
		block := queryNamedSQL(sql, name)
		if block == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(block, "JOIN documents d ON d.id = drd.document_id") {
			t.Fatalf("%s document_count must join documents", name)
		}
		if !strings.Contains(block, "d.status IS DISTINCT FROM 'archived'") {
			t.Fatalf("%s document_count must exclude archived library rows", name)
		}
		if !strings.Contains(block, "d.deleted_at IS NULL") {
			t.Fatalf("%s document_count must exclude soft-deleted library rows", name)
		}
	}
}

func queryNamedSQL(sql, name string) string {
	marker := "-- name: " + name + " "
	start := strings.Index(sql, marker)
	if start < 0 {
		return ""
	}
	rest := sql[start:]
	next := strings.Index(rest[len(marker):], "\n-- name: ")
	if next < 0 {
		return rest
	}
	return rest[:len(marker)+next]
}
