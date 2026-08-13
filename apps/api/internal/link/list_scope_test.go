package link

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func queriesSQL(t *testing.T) string {
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

func extractNamedQuery(sql, name string) string {
	marker := "-- name: " + name + " "
	idx := strings.Index(sql, marker)
	if idx < 0 {
		return ""
	}
	rest := sql[idx:]
	next := strings.Index(rest[len(marker):], "\n-- name: ")
	if next < 0 {
		return rest
	}
	return rest[:len(marker)+next]
}

func TestDocumentAndDealRoomLinkListSQLScopes(t *testing.T) {
	sql := queriesSQL(t)

	doc := extractNamedQuery(sql, "ListDocumentLinksByWorkspace")
	if doc == "" {
		t.Fatal("missing ListDocumentLinksByWorkspace")
	}
	if !strings.Contains(doc, "deal_room_id IS NULL") {
		t.Fatal("ListDocumentLinksByWorkspace must exclude deal-room shares")
	}
	if !strings.Contains(doc, "document_id IS NOT NULL") {
		t.Fatal("ListDocumentLinksByWorkspace must require a document_id")
	}

	byDoc := extractNamedQuery(sql, "ListLinksByDocument")
	if byDoc == "" {
		t.Fatal("missing ListLinksByDocument")
	}
	if !strings.Contains(byDoc, "deal_room_id IS NULL") {
		t.Fatal("ListLinksByDocument must exclude deal-room shares")
	}

	room := extractNamedQuery(sql, "ListLinksByDealRoom")
	if room == "" {
		t.Fatal("missing ListLinksByDealRoom")
	}
	if !strings.Contains(room, "deal_room_id = $2") && !strings.Contains(room, "deal_room_id = sqlc.arg(deal_room_id)") {
		t.Fatal("ListLinksByDealRoom must scope by deal_room_id")
	}

	// Workspace-wide list used by analytics may include both kinds.
	all := extractNamedQuery(sql, "ListLinksByWorkspace")
	if all == "" {
		t.Fatal("missing ListLinksByWorkspace")
	}
	if strings.Contains(all, "deal_room_id IS NULL") {
		t.Fatal("ListLinksByWorkspace must remain unscoped for analytics")
	}
	count := extractNamedQuery(sql, "CountLinksByWorkspace")
	if count == "" {
		t.Fatal("missing CountLinksByWorkspace")
	}
	if strings.Contains(count, "deal_room_id IS NULL") {
		t.Fatal("CountLinksByWorkspace must remain unscoped for billing")
	}
}

func TestPendingAccessRequestInboxSQLScopes(t *testing.T) {
	sql := queriesSQL(t)

	doc := extractNamedQuery(sql, "ListPendingDocumentLinkAccessRequestsDetailedByWorkspace")
	if doc == "" {
		t.Fatal("missing ListPendingDocumentLinkAccessRequestsDetailedByWorkspace")
	}
	if !strings.Contains(doc, "deal_room_id IS NULL") || !strings.Contains(doc, "document_id IS NOT NULL") {
		t.Fatal("document pending inbox must exclude deal-room shares")
	}
	if !strings.Contains(doc, "created_by") {
		t.Fatal("document pending inbox must remain creator-scoped")
	}
	if !strings.Contains(doc, "is_workspace_member") {
		t.Fatal("document pending inbox must expose is_workspace_member for radar honesty labels")
	}

	room := extractNamedQuery(sql, "ListPendingDealRoomLinkAccessRequestsDetailedByWorkspace")
	if room == "" {
		t.Fatal("missing ListPendingDealRoomLinkAccessRequestsDetailedByWorkspace")
	}
	if !strings.Contains(room, "deal_room_id = $3") {
		t.Fatal("deal-room pending inbox must filter by deal_room_id")
	}
	if !strings.Contains(room, "created_by") {
		t.Fatal("deal-room pending inbox must remain creator-scoped")
	}
	if !strings.Contains(room, "is_workspace_member") {
		t.Fatal("deal-room pending inbox must expose is_workspace_member for radar honesty labels")
	}

	// Unscoped detailed inbox must not exist — that would mix applicant PII.
	if extractNamedQuery(sql, "ListPendingLinkAccessRequestsDetailedByWorkspace") != "" {
		t.Fatal("unscoped ListPendingLinkAccessRequestsDetailedByWorkspace must be removed")
	}

	// Dashboard sync queries must stay surface-split (same boundary as inboxes).
	syncDoc := extractNamedQuery(sql, "ListPendingDocumentLinkAccessRequestsByWorkspace")
	if syncDoc == "" {
		t.Fatal("missing ListPendingDocumentLinkAccessRequestsByWorkspace")
	}
	if !strings.Contains(syncDoc, "deal_room_id IS NULL") || !strings.Contains(syncDoc, "document_id IS NOT NULL") {
		t.Fatal("dashboard document sync must exclude deal-room shares")
	}
	syncRoom := extractNamedQuery(sql, "ListPendingDealRoomLinkAccessRequestsByWorkspace")
	if syncRoom == "" {
		t.Fatal("missing ListPendingDealRoomLinkAccessRequestsByWorkspace")
	}
	if !strings.Contains(syncRoom, "deal_room_id IS NOT NULL") {
		t.Fatal("dashboard deal-room sync must require deal_room_id")
	}
	if extractNamedQuery(sql, "ListPendingLinkAccessRequestsByWorkspace") != "" {
		t.Fatal("unscoped ListPendingLinkAccessRequestsByWorkspace must be removed")
	}

	// Diligence Evidence enrichment must exclude workspace members and support
	// applicant-email attribution (not "any latest pending on the link/room").
	for _, name := range []string{
		"GetLatestPendingLinkAccessRequestByLink",
		"GetLatestPendingRoomAccessRequestByRoom",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "workspace_members") || !strings.Contains(q, "NOT EXISTS") {
			t.Fatalf("%s must exclude workspace-member applicants", name)
		}
		if !strings.Contains(q, "applicant_email") {
			t.Fatalf("%s must accept applicant_email for action attribution", name)
		}
	}

	feed := extractNamedQuery(sql, "ListActionItemsByWorkspaceForUser")
	if feed == "" {
		t.Fatal("missing ListActionItemsByWorkspaceForUser")
	}
	if !strings.Contains(feed, "deal_room_link_access_request") {
		t.Fatal("action feed must creator-scope deal_room_link_access_request todos")
	}
	if !strings.Contains(feed, "link_access_request") {
		t.Fatal("action feed must creator-scope link_access_request todos")
	}
}
