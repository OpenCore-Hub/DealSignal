//go:build integration

package link

import (
	"errors"
	"fmt"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (drf *dealRoomFixture) addWorkspaceUser(t *testing.T, wsRole string) (db.User, string) {
	t.Helper()
	user, err := drf.f.q.CreateUser(drf.ctx(), db.CreateUserParams{
		Email:         fmt.Sprintf("%s-%s@example.com", wsRole, uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := drf.f.q.AddWorkspaceMember(drf.ctx(), db.AddWorkspaceMemberParams{
		WorkspaceID: drf.f.workspace.ID,
		UserID:      user.ID,
		Role:        wsRole,
	}); err != nil {
		t.Fatalf("add workspace %s: %v", wsRole, err)
	}
	return user, uuid.UUID(user.ID.Bytes).String()
}

func TestListDealRoomLinks_NeedView_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	created, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "ACL View Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}

	links, err := drf.f.svc.ListDealRoomLinks(drf.ctx(), drf.userID, drf.wsID, drf.roomID)
	if err != nil {
		t.Fatalf("room owner should list links: %v", err)
	}
	if len(links) != 1 || links[0].ID.Bytes != created.ID.Bytes {
		t.Fatalf("expected the created room link, got %d", len(links))
	}

	_, outsiderID := drf.addWorkspaceUser(t, "member")
	_, err = drf.f.svc.ListDealRoomLinks(drf.ctx(), outsiderID, drf.wsID, drf.roomID)
	if !errors.Is(err, dealroom.ErrApprovalRequired) {
		t.Fatalf("workspace member without room view must be forbidden, got %v", err)
	}

	_, adminID := drf.addWorkspaceUser(t, "admin")
	oversightLinks, err := drf.f.svc.ListDealRoomLinks(drf.ctx(), adminID, drf.wsID, drf.roomID)
	if err != nil {
		t.Fatalf("oversight should list links: %v", err)
	}
	if len(oversightLinks) != 1 {
		t.Fatalf("oversight expected 1 link, got %d", len(oversightLinks))
	}

	guest, guestID := drf.addWorkspaceUser(t, "guest")
	if _, err := drf.drSvc.AddMember(drf.ctx(), drf.roomID, drf.wsID, drf.userID, guest.Email, "guest"); err != nil {
		t.Fatalf("invite room guest: %v", err)
	}
	guestLinks, err := drf.f.svc.ListDealRoomLinks(drf.ctx(), guestID, drf.wsID, drf.roomID)
	if err != nil {
		t.Fatalf("room guest should list links: %v", err)
	}
	if len(guestLinks) != 1 {
		t.Fatalf("room guest expected 1 link, got %d", len(guestLinks))
	}

	_, err = drf.f.svc.ListDealRoomLinks(drf.ctx(), drf.userID, drf.wsID, uuid.NewString())
	if !errors.Is(err, ErrDealRoomNotFound) {
		t.Fatalf("unknown room uuid must 404, got %v", err)
	}
}

func TestGetRoomAccessPolicy_NeedView_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	if _, err := drf.f.svc.GetRoomAccessPolicy(drf.ctx(), drf.userID, drf.wsID, drf.roomID); err != nil {
		t.Fatalf("room owner should get access policy: %v", err)
	}

	_, outsiderID := drf.addWorkspaceUser(t, "member")
	_, err := drf.f.svc.GetRoomAccessPolicy(drf.ctx(), outsiderID, drf.wsID, drf.roomID)
	if !errors.Is(err, dealroom.ErrApprovalRequired) {
		t.Fatalf("workspace member without room view must be forbidden, got %v", err)
	}

	_, adminID := drf.addWorkspaceUser(t, "admin")
	if _, err := drf.f.svc.GetRoomAccessPolicy(drf.ctx(), adminID, drf.wsID, drf.roomID); err != nil {
		t.Fatalf("oversight should get access policy: %v", err)
	}
}

func TestCreateDealRoomLink_OversightForbidden_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	_, adminID := drf.addWorkspaceUser(t, "admin")
	_, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), adminID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Oversight Link",
	})
	if !errors.Is(err, dealroom.ErrNotRoomAdmin) {
		t.Fatalf("oversight must not create room links, got %v", err)
	}
}

func TestAuthorizeLinkMutate_OversightForbidden_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	_, adminID := drf.addWorkspaceUser(t, "admin")
	err := authorizeLinkMutate(
		drf.ctx(),
		drf.f.q,
		drf.f.workspace.ID,
		pgtype.UUID{Bytes: drf.room.ID.Bytes, Valid: true},
		adminID,
		"admin",
	)
	if !errors.Is(err, ErrLinkMutateForbidden) {
		t.Fatalf("oversight must not mutate room links, got %v", err)
	}
}

func TestCreateDealRoomLink_WorkspaceGuestRoomAdminAllowed_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	guest, guestID := drf.addWorkspaceUser(t, "guest")
	if _, err := drf.drSvc.AddMember(drf.ctx(), drf.roomID, drf.wsID, drf.userID, guest.Email, "admin"); err != nil {
		t.Fatalf("invite room admin: %v", err)
	}
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), guestID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Guest Admin Link",
	})
	if err != nil {
		t.Fatalf("workspace guest with room admin must create room links: %v", err)
	}
	if !link.ID.Valid {
		t.Fatal("expected created link id")
	}
}

func TestCreateDealRoomLink_WorkspaceMemberRoomGuestForbidden_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	member, memberID := drf.addWorkspaceUser(t, "member")
	if _, err := drf.drSvc.AddMember(drf.ctx(), drf.roomID, drf.wsID, drf.userID, member.Email, "guest"); err != nil {
		t.Fatalf("invite room guest: %v", err)
	}
	_, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), memberID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Member Guest Link",
	})
	if !errors.Is(err, dealroom.ErrNotRoomAdmin) {
		t.Fatalf("workspace member with room guest must not create room links, got %v", err)
	}
}
