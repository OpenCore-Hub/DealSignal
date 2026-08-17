package upload

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestArchiveShareSQLKeepsLiveBundleMembers(t *testing.T) {
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

	if strings.Contains(sql, "-- name: ArchiveActiveLinksByDocument :many") {
		t.Fatal("legacy ArchiveActiveLinksByDocument parks every primary share; use live-member park")
	}
	if strings.Contains(sql, "-- name: ArchiveOrphanScopedActiveLinksForDocument :many") {
		t.Fatal("legacy orphan park ignores archived siblings still in link_documents")
	}
	if strings.Contains(sql, "-- name: RepointActiveLinkPrimaryFromDocument :exec") {
		t.Fatal("do not rebind links.document_id; page_views is append-only and S1 follows primary")
	}
	if strings.Contains(sql, "-- name: StampNullPageViewsForPrimaryDocument :exec") {
		t.Fatal("page_views partitions are append-only; do not UPDATE historical rows")
	}
	if !strings.Contains(sql, "-- name: ArchiveActiveLinksWithNoLiveMembersForDocument :many") {
		t.Fatal("archive must park only shares with no remaining live members")
	}
	if !strings.Contains(sql, "-- name: SoftDeleteActiveLinksWithNoLiveMembersForDocument :many") {
		t.Fatal("delete must revoke only shares with no remaining live members")
	}

	park := queryNamed(sql, "ArchiveActiveLinksWithNoLiveMembersForDocument")
	if !strings.Contains(park, "l.document_id = $2") || !strings.Contains(park, "link_documents") {
		t.Fatal("park must consider primary and link_documents membership")
	}
	if !strings.Contains(park, "d.status IS DISTINCT FROM 'archived'") {
		t.Fatal("park must treat archived documents as gone")
	}
	if !strings.Contains(park, "d.deleted_at IS NULL") {
		t.Fatal("park must ignore soft-deleted documents as live members")
	}

	del := queryNamed(sql, "SoftDeleteActiveLinksWithNoLiveMembersForDocument")
	if !strings.Contains(del, "status = 'deleted'") {
		t.Fatal("library delete must set status deleted, not archived")
	}
	if !strings.Contains(del, "d.status IS DISTINCT FROM 'archived'") {
		t.Fatal("delete must treat archived siblings as gone")
	}

	impact := queryNamed(sql, "GetDocumentDeleteImpact")
	if !strings.Contains(impact, "AS revoked_link_count") {
		t.Fatal("delete-impact must count shares that will actually be revoked")
	}
}

func TestArchiveDocumentDoesNotRebindPrimary(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "service.go"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	src := string(raw)
	if strings.Contains(src, "RepointActiveLinkPrimaryFromDocument") ||
		strings.Contains(src, "StampNullPageViewsForPrimaryDocument") {
		t.Fatal("archive/delete must not rebind primary or update page_views")
	}
}

func queryNamed(sql, name string) string {
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
