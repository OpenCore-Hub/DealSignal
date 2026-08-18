package radar

import "testing"

func TestNavigatePathDealRoomAccess(t *testing.T) {
	got := navigatePath("acme", "deal_room_link_access_request", "link-1", "room-1", false)
	want := "/acme/deal-rooms/room-1?tab=access&linkId=link-1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNavigatePathRoomNDAUsesTargetRoom(t *testing.T) {
	got := navigatePath("acme", "room_nda", "member-1", "room-9", false)
	want := "/acme/deal-rooms/room-9?tab=access"
	if got != want {
		t.Fatalf("member-keyed got %q want %q", got, want)
	}
	legacy := navigatePath("acme", "room_nda", "room-9", "", false)
	if legacy != want {
		t.Fatalf("legacy room-keyed got %q want %q", legacy, want)
	}
}

func TestNavigatePathFormalAsk(t *testing.T) {
	got := navigatePath("acme", "deal_room_link_question", "turn-1", "room-1/link-9", true)
	if got != "/acme/deal-rooms/room-1?askInbox=formal_queue&linkId=link-9" {
		t.Fatalf("got %q", got)
	}
}

func TestParseDealRoomAskTarget(t *testing.T) {
	room, link := parseDealRoomAskTarget("r1/l2")
	if room != "r1" || link != "l2" {
		t.Fatalf("room=%s link=%s", room, link)
	}
}

func TestDiligenceRemediationPath(t *testing.T) {
	room := diligenceRemediationPath("acme", "room-1", "link-9")
	if room != "/acme/deal-rooms/room-1?tab=access&linkId=link-9" {
		t.Fatalf("room+link=%q", room)
	}
	roomOnly := diligenceRemediationPath("acme", "room-1", "")
	if roomOnly != "/acme/deal-rooms/room-1?tab=access" {
		t.Fatalf("room-only=%q", roomOnly)
	}
	doc := diligenceRemediationPath("acme", "", "link-doc")
	if doc != "/acme/documents?tab=shared&linkId=link-doc" {
		t.Fatalf("document link=%q", doc)
	}
	if diligenceRemediationPath("acme", "", "") != "" {
		t.Fatal("empty ids must not invent a path")
	}
}
