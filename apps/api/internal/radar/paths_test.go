package radar

import "testing"

func TestNavigatePathDealRoomAccess(t *testing.T) {
	got := navigatePath("acme", "deal_room_link_access_request", "link-1", "room-1", false)
	want := "/acme/deal-rooms/room-1?tab=access&linkId=link-1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
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
