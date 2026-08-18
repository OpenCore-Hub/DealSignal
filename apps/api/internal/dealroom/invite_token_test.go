package dealroom

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMintParseRoomInviteTokenRoundTrip(t *testing.T) {
	roomID := uuid.New()
	token, err := mintRoomInviteToken("test-invite-token-hash-key", roomID.String(), "Jane.Doe+vdr@Gmail.COM")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if !strings.HasPrefix(token, "dsr1.") {
		t.Fatalf("prefix: %s", token)
	}
	if strings.Contains(token, "Jane") || strings.Contains(strings.ToLower(token), "gmail") {
		t.Fatalf("token must not embed email: %s", token)
	}
	parsed, mac, err := parseRoomInviteToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed != roomID {
		t.Fatalf("room id: got %s want %s", parsed, roomID)
	}
	rows := []db.RoomMember{
		{Email: "other@example.com", Status: "pending"},
		{Email: "janedoe@gmail.com", Status: "pending"},
	}
	got, ok := matchRoomInviteMember("test-invite-token-hash-key", parsed, mac, rows)
	if !ok || got.Email != "janedoe@gmail.com" {
		t.Fatalf("match: ok=%v email=%q", ok, got.Email)
	}
}

func TestParseRoomInviteTokenRejectsTamperAndEmptySecret(t *testing.T) {
	roomID := uuid.New()
	if _, err := mintRoomInviteToken("", roomID.String(), "a@example.com"); err == nil {
		t.Fatal("empty secret must fail")
	}
	token, err := mintRoomInviteToken("secret-a", roomID.String(), "a@example.com")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if _, _, err := parseRoomInviteToken("not-a-token"); err == nil {
		t.Fatal("garbage must fail")
	}
	tampered := token[:len(token)-2] + "aa"
	rid, mac, err := parseRoomInviteToken(tampered)
	if err != nil {
		// truncated/invalid base64 is also a reject
		return
	}
	if _, ok := matchRoomInviteMember("secret-a", rid, mac, []db.RoomMember{
		{Email: "a@example.com", Status: "pending", UserID: pgtype.UUID{}},
	}); ok {
		t.Fatal("tampered mac must not match")
	}
	if _, ok := matchRoomInviteMember("secret-b", roomID, roomInviteMAC("secret-a", roomID, "a@example.com"), []db.RoomMember{
		{Email: "a@example.com", Status: "pending"},
	}); ok {
		t.Fatal("wrong secret must not match")
	}
}

func TestMatchRoomInviteMemberIgnoresRemovedAndPrefersPending(t *testing.T) {
	roomID := uuid.New()
	secret := "k"
	email := "guest@example.com"
	mac := roomInviteMAC(secret, roomID, email)
	if _, ok := matchRoomInviteMember(secret, roomID, mac, []db.RoomMember{
		{Email: email, Status: "removed"},
	}); ok {
		t.Fatal("removed must not match")
	}
	pending := db.RoomMember{Email: email, Status: "pending", Role: "guest"}
	active := db.RoomMember{Email: email, Status: "active", Role: "member"}
	got, ok := matchRoomInviteMember(secret, roomID, mac, []db.RoomMember{active, pending})
	if !ok || got.Role != "guest" {
		t.Fatalf("prefer pending: %+v ok=%v", got, ok)
	}
}
