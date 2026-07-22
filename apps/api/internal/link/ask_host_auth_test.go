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
	return db.RoomMember{Status: f.roomStatus, Role: "viewer"}, nil
}

func TestAuthorizeAskHostOwnerView_AllowsActiveRoomMember(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	room := pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
	err := authorizeAskHostOwnerView(context.Background(), &fakeAskHostAuth{
		wsErr:      pgx.ErrNoRows,
		roomStatus: "active",
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatalf("expected room member allowed, got %v", err)
	}
}

func TestAuthorizeAskHostOwnerView_AllowsWorkspaceAdmin(t *testing.T) {
	ws := pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true}
	err := authorizeAskHostOwnerView(context.Background(), &fakeAskHostAuth{wsRole: "admin"}, ws, pgtype.UUID{}, uuid.NewString())
	if err != nil {
		t.Fatalf("expected workspace admin allowed, got %v", err)
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
