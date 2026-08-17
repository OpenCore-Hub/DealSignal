//go:build integration

package dealroom

import (
	"errors"
	"fmt"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (f *deleteRoomFixture) addUser(t *testing.T, wsRole string) (db.User, string) {
	t.Helper()
	user, err := f.q.CreateUser(f.ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("%s-%s@example.com", wsRole, uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.ws.ID,
		UserID:      user.ID,
		Role:        wsRole,
	}); err != nil {
		t.Fatalf("add workspace %s: %v", wsRole, err)
	}
	return user, uuid.UUID(user.ID.Bytes).String()
}

func TestRoomACLOversightCannotMutate_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()
	_, adminID := f.addUser(t, "admin")

	if _, err := f.svc.CreateFolder(f.ctx, roomID, f.wsID, adminID, "Oversight", "/"); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("oversight create folder: %v", err)
	}
	if _, err := f.svc.AddMember(f.ctx, roomID, f.wsID, adminID, "invitee@example.com", "guest"); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("oversight invite: %v", err)
	}
	if err := f.svc.DeleteRoom(f.ctx, roomID, f.wsID, adminID); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("oversight delete: %v", err)
	}

	detail, err := f.svc.GetRoomDetail(f.ctx, roomID, f.wsID, adminID)
	if err != nil {
		t.Fatalf("oversight should view room: %v", err)
	}
	if detail.IsAdmin {
		t.Fatal("oversight must not be IsAdmin")
	}
	if len(detail.Members) == 0 {
		t.Fatal("oversight should see member list")
	}
}

func TestRoomACLRoomAdminCanMutateEvenAsWorkspaceGuest_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()
	guest, guestID := f.addUser(t, "guest")

	member, err := f.svc.AddMember(f.ctx, roomID, f.wsID, f.userID, guest.Email, "admin")
	if err != nil {
		t.Fatalf("invite room admin: %v", err)
	}
	if member.Role != "admin" {
		t.Fatalf("expected room admin, got %s", member.Role)
	}
	if !member.UserID.Valid || member.UserID.Bytes != guest.ID.Bytes {
		t.Fatal("expected invite to bind existing user_id")
	}

	if _, err := f.svc.CreateFolder(f.ctx, roomID, f.wsID, guestID, "GuestAdmin", "/"); err != nil {
		t.Fatalf("room admin create folder: %v", err)
	}
	invited, err := f.svc.AddMember(f.ctx, roomID, f.wsID, guestID, "room-guest@example.com", "member")
	if err != nil {
		t.Fatalf("room admin invite member: %v", err)
	}
	if invited.Role != "member" {
		t.Fatalf("expected grantable role member, got %s", invited.Role)
	}
}

func TestRoomACLMemberCanContributeNotManage_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()
	memberUser, memberID := f.addUser(t, "member")
	if _, err := f.svc.AddMember(f.ctx, roomID, f.wsID, f.userID, memberUser.Email, "member"); err != nil {
		t.Fatalf("invite room member: %v", err)
	}

	doc, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		TenantID:    f.tenant.ID,
		WorkspaceID: f.ws.ID,
		CreatedBy:   memberUser.ID,
		Title:       "Member Deck",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "member-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	docID := uuid.UUID(doc.ID.Bytes).String()
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, memberID, docID, "/docs", 0); err != nil {
		t.Fatalf("room member should add document: %v", err)
	}
	if err := f.svc.RemoveDocument(f.ctx, roomID, f.wsID, memberID, docID); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("room member must not remove document, got %v", err)
	}
	if _, err := f.svc.CreateFolder(f.ctx, roomID, f.wsID, memberID, "MemberFolder", "/"); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("room member must not create folder, got %v", err)
	}
}

func TestRoomACLBindExistingUserOnInvite_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()

	existing, err := f.q.CreateUser(f.ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("bind-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	member, err := f.svc.AddMember(f.ctx, roomID, f.wsID, f.userID, existing.Email, "guest")
	if err != nil {
		t.Fatalf("invite existing email: %v", err)
	}
	if !member.UserID.Valid || member.UserID.Bytes != existing.ID.Bytes {
		t.Fatal("expected user_id bound on invite")
	}
	wsMember, err := f.q.GetWorkspaceMember(f.ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: f.ws.ID,
		UserID:      existing.ID,
	})
	if err != nil {
		t.Fatalf("expected auto workspace guest: %v", err)
	}
	if wsMember.Role != "guest" {
		t.Fatalf("expected workspace guest, got %s", wsMember.Role)
	}
}

func TestRoomACLWorkspaceRemoveRejectsSoleOperator_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	creator, creatorID := f.addUser(t, "member")
	room, err := f.svc.CreateRoom(f.ctx, creatorID, f.wsID, CreateRoomRequest{
		Slug: "sole-" + uuid.NewString()[:8],
		Name: "Sole Operator Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if room.CreatedBy.Bytes != creator.ID.Bytes {
		t.Fatal("expected creator to own the room")
	}

	wsSvc := workspace.NewService(f.q)
	if err := wsSvc.RemoveMember(f.ctx, f.userID, f.wsID, "", creatorID); !errors.Is(err, workspace.ErrCannotRemoveSoleRoomOperator) {
		t.Fatalf("expected sole-operator reject, got %v", err)
	}
}

func TestRoomACLInviteDoesNotConsumeSeat_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()

	before, err := f.q.CountInternalSeatsByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("count seats before: %v", err)
	}
	if _, err := f.svc.AddMember(f.ctx, roomID, f.wsID, f.userID, "external@example.com", "guest"); err != nil {
		t.Fatalf("email-only invite: %v", err)
	}
	after, err := f.q.CountInternalSeatsByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("count seats after: %v", err)
	}
	if after != before {
		t.Fatalf("room invite must not change seats: before=%d after=%d", before, after)
	}
}

func TestRoomACLInviteDoesNotChangeGetBillingSeatsUsed_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.ws.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free billing: %v", err)
	}

	wsSvc := workspace.NewService(f.q)
	before, err := wsSvc.GetBilling(f.ctx, f.wsID)
	if err != nil {
		t.Fatalf("GetBilling before: %v", err)
	}
	if before.SeatsUsed != 1 || before.SeatsLimit != 1 {
		t.Fatalf("expected free owner seat 1/1, got used=%d limit=%d", before.SeatsUsed, before.SeatsLimit)
	}

	invitee, err := f.q.CreateUser(f.ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("room-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create invitee: %v", err)
	}
	if _, err := f.svc.AddMember(f.ctx, roomID, f.wsID, f.userID, invitee.Email, "member"); err != nil {
		t.Fatalf("invite room member: %v", err)
	}

	after, err := wsSvc.GetBilling(f.ctx, f.wsID)
	if err != nil {
		t.Fatalf("GetBilling after: %v", err)
	}
	if after.SeatsUsed != before.SeatsUsed {
		t.Fatalf("GetBilling.seatsUsed must stay %d after room member invite, got %d", before.SeatsUsed, after.SeatsUsed)
	}
}

func TestRoomACLGrantableMemberRole_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()

	member, err := f.svc.AddMember(f.ctx, roomID, f.wsID, f.userID, "contrib@example.com", "contributor")
	if err != nil {
		t.Fatalf("invite contributor alias: %v", err)
	}
	if member.Role != "member" {
		t.Fatalf("expected member, got %s", member.Role)
	}
}

func TestRoomACLListScopedToMembership_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	owned, _ := f.createRoomWithDoc(t)
	memberUser, memberID := f.addUser(t, "member")
	if _, err := f.svc.CreateRoom(f.ctx, f.userID, f.wsID, CreateRoomRequest{
		Slug: "hidden-" + uuid.NewString()[:8],
		Name: "Hidden Room",
	}); err != nil {
		t.Fatalf("create hidden room: %v", err)
	}
	if _, err := f.svc.AddMember(f.ctx, uuid.UUID(owned.ID.Bytes).String(), f.wsID, f.userID, memberUser.Email, "guest"); err != nil {
		t.Fatalf("add room guest: %v", err)
	}

	rooms, err := f.svc.ListRoomsForUser(f.ctx, f.wsID, memberID)
	if err != nil {
		t.Fatalf("list rooms: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("workspace member should see only invited room, got %d", len(rooms))
	}
	if rooms[0].Room.ID.Bytes != owned.ID.Bytes {
		t.Fatal("listed unexpected room")
	}
}
