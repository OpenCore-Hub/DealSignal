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
	if !strings.Contains(byDoc, "link_documents") {
		t.Fatal("ListLinksByDocument must include bundle members via link_documents")
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

func TestDocumentScopedAnalyticsSQLIncludesBundleMembers(t *testing.T) {
	sql := queriesSQL(t)
	for _, name := range []string{
		"GetDocumentViewMetrics",
		"ListPopularDocumentsByWorkspace",
		"ListUnsharedDocumentsByWorkspace",
		"ListSharedDocumentsByWorkspace",
		"ListRecentlyAccessedDocumentsByWorkspace",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "link_documents") {
			t.Fatalf("%s must include bundle members via link_documents", name)
		}
	}

	for _, name := range []string{
		"GetVisitorSummariesByDocument",
		"GetVisitorSummariesByDocumentInRange",
		"GetPageAnalyticsByDocument",
		"GetPageAnalyticsByDocumentInRange",
		"GetPageExitCountsByDocument",
		"GetPageExitCountsByDocumentInRange",
		"GetDocumentVisitorReach",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "COALESCE(pv.document_id, l.document_id)") {
			t.Fatalf("%s must attribute page_views via COALESCE (library + room + bundle)", name)
		}
		if strings.Contains(q, "deal_room_id IS NULL") {
			t.Fatalf("%s must not drop deal-room shares", name)
		}
	}

	for _, name := range []string{
		"GetDocumentReadingSessionReach",
		"GetDocumentReadingSessionReachInRange",
		"ListDocumentReadingSessions",
		"ListDocumentReadingSessionsInRange",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if strings.Contains(q, "deal_room_id IS NULL") {
			t.Fatalf("%s must not drop deal-room shares", name)
		}
		if strings.Contains(q, "link_documents") {
			t.Fatalf("%s must not require library/bundle membership; room sessions use document_id", name)
		}
	}

	topPages := extractNamedQuery(sql, "ListTopPagesByLink")
	if topPages == "" {
		t.Fatal("missing ListTopPagesByLink")
	}
	if !strings.Contains(topPages, "COALESCE(pv.document_id, l.document_id)") {
		t.Fatal("ListTopPagesByLink must attribute legacy NULL page_views to the primary document")
	}
	if !strings.Contains(topPages, "page_number") || !strings.Contains(topPages, "GROUP BY") {
		t.Fatal("ListTopPagesByLink must group by document and page")
	}
	if !strings.Contains(topPages, "LIMIT 10") {
		t.Fatal("ListTopPagesByLink must stay top-N for radar evidence")
	}

	pageDurations := extractNamedQuery(sql, "ListPageDurationsByLink")
	if pageDurations == "" {
		t.Fatal("missing ListPageDurationsByLink")
	}
	if !strings.Contains(pageDurations, "COALESCE(pv.document_id, l.document_id)") {
		t.Fatal("ListPageDurationsByLink must attribute legacy NULL page_views to the primary document")
	}
	if strings.Contains(pageDurations, "LIMIT 10") {
		t.Fatal("ListPageDurationsByLink must not be top-N; the share chart needs every page")
	}

	keyPages := extractNamedQuery(sql, "GetLinkKeyPageViewDetails")
	if keyPages == "" {
		t.Fatal("missing GetLinkKeyPageViewDetails")
	}
	if !strings.Contains(keyPages, "COALESCE(pv.document_id, l.document_id)") {
		t.Fatal("GetLinkKeyPageViewDetails must attribute legacy NULL page_views to the primary document")
	}
	if !strings.Contains(keyPages, "GROUP BY COALESCE(pv.document_id, l.document_id)") ||
		!strings.Contains(keyPages, "pv.page_number") {
		t.Fatal("GetLinkKeyPageViewDetails must group by document and page")
	}

	highExit := extractNamedQuery(sql, "ListHighExitPagesByLink")
	if highExit == "" {
		t.Fatal("missing ListHighExitPagesByLink")
	}
	if !strings.Contains(highExit, "COALESCE(pv.document_id, l.document_id)") {
		t.Fatal("ListHighExitPagesByLink must attribute legacy NULL page_views to the primary document")
	}

	for _, name := range []string{
		"GetVisitorSummariesByDocument",
		"GetVisitorSummariesByDocumentInRange",
		"GetPageAnalyticsByDocument",
		"GetPageAnalyticsByDocumentInRange",
		"GetPageExitCountsByDocument",
		"GetPageExitCountsByDocumentInRange",
		"GetDocumentVisitorReach",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "COALESCE(pv.document_id, l.document_id)") {
			t.Fatalf("%s must attribute legacy NULL page_views to the primary document", name)
		}
	}

	logs := extractNamedQuery(sql, "ListAccessLogsByLink")
	if logs == "" {
		t.Fatal("missing ListAccessLogsByLink")
	}
	if !strings.Contains(logs, "e.document_id") {
		t.Fatal("ListAccessLogsByLink must expose page-view document_id")
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
	if !strings.Contains(room, "deal_room_id = $2") {
		t.Fatal("deal-room pending inbox must filter by deal_room_id")
	}
	if strings.Contains(room, "l.created_by") {
		t.Fatal("deal-room pending inbox must not be creator-scoped; NeedManage is enforced in service")
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

func TestInsightsQueueAndBundleSQLContracts(t *testing.T) {
	sql := queriesSQL(t)

	docs := extractNamedQuery(sql, "ListLinkDocumentIDsByWorkspace")
	if docs == "" {
		t.Fatal("missing ListLinkDocumentIDsByWorkspace")
	}
	if !strings.Contains(docs, "link_documents") {
		t.Fatal("ListLinkDocumentIDsByWorkspace must read bundle members")
	}

	pending := extractNamedQuery(sql, "CountPendingActionItemsByWorkspace")
	if pending == "" {
		t.Fatal("missing CountPendingActionItemsByWorkspace")
	}
	if !strings.Contains(pending, "status = 'pending'") {
		t.Fatal("open-signal count must be pending actions, not all signals")
	}

	links := extractNamedQuery(sql, "ListPendingActionLinkIDsByWorkspace")
	if links == "" {
		t.Fatal("missing ListPendingActionLinkIDsByWorkspace")
	}
	if !strings.Contains(links, "signals") || !strings.Contains(links, "source_id") {
		t.Fatal("pending action links must resolve signal.link_id or source_id")
	}
}

func TestInsightsUniqueVisitorsMatchRadarIdentity(t *testing.T) {
	sql := queriesSQL(t)
	for _, name := range []string{
		"GetWorkspaceDailyLinkOpens",
		"GetWorkspaceDailyLinkOpensInRange",
		"CountWorkspaceLinkOpenVisitorsInRange",
		"GetDealRoomAnalytics",
		"GetDealRoomAggregatesByWorkspace",
		"GetDealRoomAggregatesForRooms",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "COUNT(DISTINCT") || !strings.Contains(q, "visitor_id") {
			t.Fatalf("%s must count DISTINCT visitor_id (align GetLinkAccessMetrics)", name)
		}
		if strings.Contains(q, "COUNT(DISTINCT al.visitor_email)") {
			t.Fatalf("%s must not fall back to email for unique visitors", name)
		}
		if !strings.Contains(q, "visitor_id <> ''") {
			t.Fatalf("%s display unique visitors must exclude empty visitor_id (align GetLinkAnalytics / ListRecentVisitors)", name)
		}
	}

	heat := extractNamedQuery(sql, "GetLinkAccessMetrics")
	if heat == "" {
		t.Fatal("missing GetLinkAccessMetrics")
	}
	if strings.Contains(heat, "visitor_id <> ''") {
		t.Fatal("GetLinkAccessMetrics must not exclude empty visitor_id (heat / MV identity)")
	}

	for _, name := range []string{
		"GetDealRoomAnalytics",
		"GetDealRoomAggregatesByWorkspace",
		"GetDealRoomAggregatesForRooms",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "AS active_link_count") || !strings.Contains(q, "status = 'active'") {
			t.Fatalf("%s must count status=active links", name)
		}
		if !strings.Contains(q, "expires_at IS NULL") || !strings.Contains(q, "expires_at > now()") {
			t.Fatalf("%s active_link_count must skip past-due shares (align GetDocumentDeleteImpact / isLinkActive)", name)
		}
	}

	for _, name := range []string{
		"GetDealRoomAggregatesByWorkspace",
		"GetDealRoomAggregatesForRooms",
	} {
		q := extractNamedQuery(sql, name)
		if !strings.Contains(q, "AS open_count") {
			t.Fatalf("%s must expose open_count (list viewCount)", name)
		}
		if !strings.Contains(q, "AS active_link_count") || !strings.Contains(q, "status = 'active'") {
			t.Fatalf("%s must count status=active links (align GetDealRoomAnalytics)", name)
		}
		if !strings.Contains(q, "event_type = 'link_opened'") {
			t.Fatalf("%s visitor/open counts must use link_opened", name)
		}
		if strings.Contains(q, "visitor_count") && strings.Contains(q, "* 5") {
			t.Fatalf("%s must not use the visitors*5+events*2 heat heuristic", name)
		}
		if strings.Contains(q, "AS heat_score") {
			t.Fatalf("%s must not compute heat_score; overlayRoomHeatScores uses heat.Compute", name)
		}
	}

	roomHeat := extractNamedQuery(sql, "ListRoomLinkHeatScoresByWorkspace")
	if roomHeat == "" {
		t.Fatal("missing ListRoomLinkHeatScoresByWorkspace")
	}
	if !strings.Contains(roomHeat, "link_heat_scores") {
		t.Fatal("room heat must read link_heat_scores (same MV as Insights)")
	}
	if !strings.Contains(roomHeat, "deal_room_id") {
		t.Fatal("room heat must map shares to deal_room_id")
	}
	if !strings.Contains(roomHeat, "NOT IN ('deleted', 'disabled')") {
		t.Fatal("room heat must skip deleted/disabled shares (align room_links)")
	}

	weekly := extractNamedQuery(sql, "CountWeeklyVisitorsByWorkspace")
	if weekly == "" {
		t.Fatal("missing CountWeeklyVisitorsByWorkspace")
	}
	if !strings.Contains(weekly, "COALESCE(visitor_id, visitor_email)") {
		t.Fatal("weekly visitors leftover API must stay unchanged")
	}
}

func TestInsightsEngagementExcludesWorkspaceMembers(t *testing.T) {
	sql := queriesSQL(t)
	accessAligned := []string{
		"GetWorkspaceDailyLinkOpens",
		"GetWorkspaceDailyLinkOpensInRange",
		"CountWorkspaceLinkOpenVisitorsInRange",
		"CountWorkspaceForwardSignalsByLinkInRange",
		"GetLinkAnalytics",
		"GetLastAccessLogByLink",
		"GetLastAccessLogsByLinks",
		"GetDealRoomAnalytics",
		"GetDealRoomAggregatesByWorkspace",
		"GetDealRoomAggregatesForRooms",
		"ListRecentVisitorsByDealRoom",
		"ListRecentlyAccessedDocumentsByWorkspace",
	}
	for _, name := range accessAligned {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "workspace_members") || !strings.Contains(q, "LOWER(u.email) = LOWER(al.visitor_email)") {
			t.Fatalf("%s must exclude workspace members via access_logs email", name)
		}
	}

	pageAligned := []string{
		"GetWorkspacePageViewEngagementInRange",
		"GetWorkspaceReadingSessionStatsInRange",
		"GetPageAnalyticsByDocument",
		"GetPageAnalyticsByDocumentInRange",
		"GetPageExitCountsByDocument",
		"GetPageExitCountsByDocumentInRange",
		"GetVisitorSummariesByDocument",
		"GetVisitorSummariesByDocumentInRange",
		"GetDocumentVisitorReach",
		"GetDocumentReadingSessionReach",
		"GetDocumentReadingSessionReachInRange",
		"ListDocumentReadingSessions",
		"ListDocumentReadingSessionsInRange",
		"GetAverageDurationByLink",
		"ListTopPagesByLink",
		"ListPageDurationsByLink",
		"ListHighExitPagesByLink",
	}
	contactAligned := []string{
		"GetContactAggregatesByWorkspace",
		"GetContactAggregateByEmail",
		"GetContactKeyPageViewDetails",
		"ListContactViewedDocumentIDsByWorkspace",
		"FindUnsyncedContactEmails",
		"ListContactActivitiesByEmail",
		"ListContactViewedDocumentIDs",
		"ListContactViewedDocuments",
	}
	for _, name := range pageAligned {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "workspace_members") {
			t.Fatalf("%s must exclude workspace members (align GetLinkPageViewMetrics)", name)
		}
	}
	for _, name := range contactAligned {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "workspace_members") {
			t.Fatalf("%s must exclude workspace members (align GetLinkAccessMetrics)", name)
		}
	}

	digest := extractNamedQuery(sql, "CountWorkspaceLinkOpensInRange")
	if digest == "" {
		t.Fatal("missing CountWorkspaceLinkOpensInRange")
	}
	if strings.Contains(digest, "workspace_members") {
		t.Fatal("daily digest open count must keep including members")
	}

	compliance := extractNamedQuery(sql, "GetWorkspaceKeyPageComplianceSummary")
	if compliance == "" {
		t.Fatal("missing GetWorkspaceKeyPageComplianceSummary")
	}
	if strings.Contains(compliance, "workspace_members") {
		t.Fatal("key-page compliance must stay an inclusive audit")
	}

	timeline := extractNamedQuery(sql, "ListAccessLogsByLink")
	if timeline == "" {
		t.Fatal("missing ListAccessLogsByLink")
	}
	if strings.Contains(timeline, "workspace_members") {
		t.Fatal("share-detail visit timeline must stay complete, including members")
	}

	activity := extractNamedQuery(sql, "ListRecentActivitiesByWorkspace")
	if activity == "" {
		t.Fatal("missing ListRecentActivitiesByWorkspace")
	}
	if strings.Contains(activity, "workspace_members") {
		t.Fatal("workspace activity feed must stay complete, including members")
	}

	decay := extractNamedQuery(sql, "GetLinkLastAccessAt")
	if decay == "" {
		t.Fatal("missing GetLinkLastAccessAt")
	}
	if strings.Contains(decay, "workspace_members") {
		t.Fatal("heat decay last_access_at must stay unfiltered")
	}
}

func TestContactViewedDocumentsIncludeBundleMembers(t *testing.T) {
	sql := queriesSQL(t)
	for _, name := range []string{
		"ListContactViewedDocumentIDs",
		"ListContactViewedDocuments",
		"ListContactViewedDocumentIDsByWorkspace",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(q, "link_documents") {
			t.Fatalf("%s must attribute opens to bundle members", name)
		}
		if !strings.Contains(q, "l.document_id <> ld.document_id") {
			t.Fatalf("%s must exclude primary from the link_documents branch", name)
		}
		if !strings.Contains(q, "COALESCE(pv.document_id, l.document_id)") {
			t.Fatalf("%s page views must keep S1 COALESCE(pv.document_id, l.document_id)", name)
		}
	}

	activities := extractNamedQuery(sql, "ListContactActivitiesByEmail")
	if activities == "" {
		t.Fatal("missing ListContactActivitiesByEmail")
	}
	if strings.Contains(activities, "link_documents") {
		t.Fatal("contact activity timeline must stay one row per event, not explode bundle members")
	}
}

func TestPageAnalyticsLastViewedRequiresAView(t *testing.T) {
	sql := queriesSQL(t)
	for _, name := range []string{
		"GetPageAnalyticsByDocument",
		"GetPageAnalyticsByDocumentInRange",
	} {
		q := extractNamedQuery(sql, name)
		if q == "" {
			t.Fatalf("missing %s", name)
		}
		if strings.Contains(q, "COALESCE(MAX(pv.created_at), p.created_at)") {
			t.Fatalf("%s must not treat page upload time as last viewed", name)
		}
		if !strings.Contains(q, "MAX(a.created_at)::timestamptz") {
			t.Fatalf("%s must use typed MAX(a.created_at)::timestamptz for last_viewed_at", name)
		}
	}
}
