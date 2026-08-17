package link

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeAskHostAuth struct {
	wsRole     string
	wsErr      error
	roomStatus string
	roomRole   string
	roomErr    error
}

func (f *fakeAskHostAuth) GetWorkspaceMember(_ context.Context, _ db.GetWorkspaceMemberParams) (db.WorkspaceMember, error) {
	if f.wsErr != nil {
		return db.WorkspaceMember{}, f.wsErr
	}
	return db.WorkspaceMember{Role: f.wsRole}, nil
}

func (f *fakeAskHostAuth) GetRoomMemberByUserID(_ context.Context, _ db.GetRoomMemberByUserIDParams) (db.RoomMember, error) {
	if f.roomErr != nil {
		return db.RoomMember{}, f.roomErr
	}
	role := f.roomRole
	if role == "" {
		role = "admin"
	}
	return db.RoomMember{Status: f.roomStatus, Role: role}, nil
}

func TestAuthorizeAskHostOwnerView_AllowsRoomAdmin(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	err := authorizeAskHostOwnerView(context.Background(), &fakeAskHostAuth{
		wsRole:     "guest",
		roomStatus: "active",
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatalf("expected room admin allowed, got %v", err)
	}
}

func TestAuthorizeAskHostOwnerView_RejectsWorkspaceAdminWithoutRoom(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	err := authorizeAskHostOwnerView(context.Background(), &fakeAskHostAuth{
		wsRole:  "admin",
		roomErr: pgx.ErrNoRows,
	}, ws, room, uuid.NewString())
	if !errors.Is(err, ErrAskHostForbidden) {
		t.Fatalf("expected ErrAskHostForbidden, got %v", err)
	}
}

func TestAuthorizeAskHostOwnerView_DocumentLinkAllowsWorkspaceOwner(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	err := authorizeAskHostOwnerView(context.Background(), &fakeAskHostAuth{
		wsRole: "owner",
	}, ws, pgtype.UUID{}, uuid.NewString())
	if err != nil {
		t.Fatalf("document share link must allow workspace owner, got %v", err)
	}
}

func TestAuthorizeAskHostOwnerView_DocumentLinkRejectsMember(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	err := authorizeAskHostOwnerView(context.Background(), &fakeAskHostAuth{
		wsRole: "member",
	}, ws, pgtype.UUID{}, uuid.NewString())
	if !errors.Is(err, ErrAskHostForbidden) {
		t.Fatalf("expected ErrAskHostForbidden, got %v", err)
	}
}

func TestAuthorizeAskHostOwnerView_RejectsWorkspaceMemberWithoutRoom(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	err := authorizeAskHostOwnerView(context.Background(), &fakeAskHostAuth{
		wsRole:  "member",
		roomErr: pgx.ErrNoRows,
	}, ws, room, uuid.NewString())
	if !errors.Is(err, ErrAskHostForbidden) {
		t.Fatalf("expected ErrAskHostForbidden, got %v", err)
	}
}

func TestAuthorizeLinkMutate_RoomAdminAllowedOversightDenied(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	if err := authorizeLinkMutate(context.Background(), &fakeAskHostAuth{
		wsRole:     "guest",
		roomStatus: "active",
	}, ws, room, uuid.NewString(), "guest"); err != nil {
		t.Fatalf("room admin should mutate deal-room link, got %v", err)
	}
	err := authorizeLinkMutate(context.Background(), &fakeAskHostAuth{
		wsRole:  "admin",
		roomErr: pgx.ErrNoRows,
	}, ws, room, uuid.NewString(), "admin")
	if !errors.Is(err, ErrLinkMutateForbidden) {
		t.Fatalf("oversight must not mutate deal-room link, got %v", err)
	}
}

func TestAuthorizeLinkMutate_DocumentLinkRejectsWorkspaceGuest(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	if err := authorizeLinkMutate(context.Background(), &fakeAskHostAuth{
		wsRole: "member",
	}, ws, pgtype.UUID{}, uuid.NewString(), "member"); err != nil {
		t.Fatalf("workspace member should mutate document link, got %v", err)
	}
	err := authorizeLinkMutate(context.Background(), &fakeAskHostAuth{
		wsRole: "guest",
	}, ws, pgtype.UUID{}, uuid.NewString(), "guest")
	if !errors.Is(err, ErrLinkMutateForbidden) {
		t.Fatalf("workspace guest must not mutate document link, got %v", err)
	}
}

func TestAuthorizeLinkView_RoomGuestAndOversightAllowed(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	if err := authorizeLinkView(context.Background(), &fakeAskHostAuth{
		wsRole:     "guest",
		roomStatus: "active",
		roomRole:   "guest",
	}, ws, room, uuid.NewString()); err != nil {
		t.Fatalf("room guest should view deal-room link, got %v", err)
	}
	if err := authorizeLinkView(context.Background(), &fakeAskHostAuth{
		wsRole:     "guest",
		roomStatus: "active",
	}, ws, room, uuid.NewString()); err != nil {
		t.Fatalf("room admin should view deal-room link, got %v", err)
	}
	if err := authorizeLinkView(context.Background(), &fakeAskHostAuth{
		wsRole:  "admin",
		roomErr: pgx.ErrNoRows,
	}, ws, room, uuid.NewString()); err != nil {
		t.Fatalf("oversight should view deal-room link, got %v", err)
	}
}

func TestAuthorizeLinkView_WorkspaceMemberWithoutRoomForbidden(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	err := authorizeLinkView(context.Background(), &fakeAskHostAuth{
		wsRole:  "member",
		roomErr: pgx.ErrNoRows,
	}, ws, room, uuid.NewString())
	if !errors.Is(err, ErrLinkViewForbidden) {
		t.Fatalf("expected ErrLinkViewForbidden, got %v", err)
	}
}

func TestAuthorizeLinkView_DocumentLinkAllowed(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	if err := authorizeLinkView(context.Background(), &fakeAskHostAuth{
		wsRole: "member",
	}, ws, pgtype.UUID{}, uuid.NewString()); err != nil {
		t.Fatalf("document share link must stay workspace-scoped, got %v", err)
	}
}

func TestCanReviewLinkRequests_DealRoomManageVsDocumentCreator(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	reviewer := uuid.New()
	roomLink := db.Link{WorkspaceID: ws, DealRoomID: room, CreatedBy: pgtype.UUID{Bytes: uuid.New(), Valid: true}}
	if !canReviewLinkRequests(context.Background(), &fakeAskHostAuth{
		wsRole:     "guest",
		roomStatus: "active",
	}, roomLink, reviewer.String()) {
		t.Fatal("room admin should review deal-room access requests")
	}
	if canReviewLinkRequests(context.Background(), &fakeAskHostAuth{
		wsRole:  "admin",
		roomErr: pgx.ErrNoRows,
	}, roomLink, reviewer.String()) {
		t.Fatal("oversight must not review deal-room access requests")
	}
	docLink := db.Link{WorkspaceID: ws, CreatedBy: pgtype.UUID{Bytes: reviewer, Valid: true}}
	if !canReviewLinkRequests(context.Background(), &fakeAskHostAuth{wsRole: "member"}, docLink, reviewer.String()) {
		t.Fatal("document link creator should review")
	}
	if canReviewLinkRequests(context.Background(), &fakeAskHostAuth{wsRole: "member"}, docLink, uuid.NewString()) {
		t.Fatal("non-creator must not review document link requests")
	}
}
