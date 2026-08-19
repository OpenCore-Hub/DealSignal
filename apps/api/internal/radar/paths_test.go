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
	if got != "/acme/deal-rooms/room-1?askInbox=formal_queue&linkId=link-9&tab=qa" {
		t.Fatalf("got %q", got)
	}
}

func TestNavigatePathHostAsk(t *testing.T) {
	got := navigatePath("acme", "deal_room_link_question", "turn-1", "room-1/link-9", false)
	if got != "/acme/deal-rooms/room-1?askInbox=needs_host&linkId=link-9&tab=qa" {
		t.Fatalf("got %q", got)
	}
}

func TestNavigatePathLibraryAsk(t *testing.T) {
	got := navigatePath("acme", "link_question", "turn-1", "link-lib", false)
	if got != "/acme/links/link-lib?askInbox=needs_host" {
		t.Fatalf("got %q", got)
	}
}

func TestParseDealRoomAskTarget(t *testing.T) {
	room, link := parseDealRoomAskTarget("r1/l2")
	if room != "r1" || link != "l2" {
		t.Fatalf("room=%s link=%s", room, link)
	}
}

func TestExpiringLinkPath(t *testing.T) {
	library := expiringLinkPath("acme", "link-doc", "")
	if library != "/acme/links/link-doc/edit?focus=expiry" {
		t.Fatalf("library=%q", library)
	}
	room := expiringLinkPath("acme", "link-room", "room-1")
	if room != "/acme/links/link-room" {
		t.Fatalf("deal-room share=%q", room)
	}
	if expiringLinkPath("acme", "", "") != "" {
		t.Fatal("empty link must not invent a path")
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
	if doc != "/acme/links/link-doc" {
		t.Fatalf("document link=%q", doc)
	}
	if diligenceRemediationPath("acme", "", "") != "" {
		t.Fatal("empty ids must not invent a path")
	}
}
