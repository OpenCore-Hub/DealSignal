package roomacl

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeQ struct {
	wsRole     string
	wsErr      error
	room       db.RoomMember
	roomErr    error
}

func (f *fakeQ) GetWorkspaceMember(context.Context, db.GetWorkspaceMemberParams) (db.WorkspaceMember, error) {
	if f.wsErr != nil {
		return db.WorkspaceMember{}, f.wsErr
	}
	return db.WorkspaceMember{Role: f.wsRole}, nil
}

func (f *fakeQ) GetRoomMemberByUserID(context.Context, db.GetRoomMemberByUserIDParams) (db.RoomMember, error) {
	if f.roomErr != nil {
		return db.RoomMember{}, f.roomErr
	}
	return f.room, nil
}

func TestResolve_RoomAdminHasManage(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsRole: "member",
		room:   db.RoomMember{Status: "active", Role: "admin", Email: "a@x.com"},
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Manage || !caps.Contribute || !caps.View || caps.Oversight {
		t.Fatalf("caps=%+v", caps)
	}
}

func TestResolve_RoomMemberContributes(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsRole: "guest",
		room:   db.RoomMember{Status: "active", Role: "contributor", Email: "m@x.com"},
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Contribute || caps.Manage || caps.RoomRole != RoleMember {
		t.Fatalf("caps=%+v", caps)
	}
}

func TestResolve_RoomGuestViewOnly(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsRole: "member",
		room:   db.RoomMember{Status: "active", Role: "viewer"},
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.View || caps.Contribute || caps.Manage {
		t.Fatalf("caps=%+v", caps)
	}
}

func TestResolve_WorkspaceAdminOversightOnly(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsRole:  "admin",
		roomErr: pgx.ErrNoRows,
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.View || !caps.Oversight || caps.Manage || caps.Contribute {
		t.Fatalf("caps=%+v", caps)
	}
	if _, err := Require(context.Background(), &fakeQ{wsRole: "admin", roomErr: pgx.ErrNoRows}, ws, room, uuid.NewString(), NeedManage); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("oversight must not manage: %v", err)
	}
}

func TestResolve_ActiveRoomRoleWithoutWorkspaceMembership(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsErr: pgx.ErrNoRows,
		room:  db.RoomMember{Status: "active", Role: "owner"},
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Manage || !caps.Delete || caps.Oversight {
		t.Fatalf("active room owner must manage without a workspace row: %+v", caps)
	}
}

func TestResolve_WorkspaceMemberWithoutRoomDenied(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsRole:  "member",
		roomErr: pgx.ErrNoRows,
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if caps.View {
		t.Fatalf("member without room row must not view: %+v", caps)
	}
}

func TestResolve_PendingMemberIsInvitedNotView(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsRole: "guest",
		room:   db.RoomMember{Status: "pending", Role: "member", Email: "p@x.com"},
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.InvitedPending || caps.View || caps.Contribute || caps.Manage {
		t.Fatalf("pending must enter without document view: %+v", caps)
	}
	if caps.RoomRole != RoleMember || caps.MemberEmail != "p@x.com" {
		t.Fatalf("pending should keep role/email: %+v", caps)
	}
	if _, err := Require(context.Background(), &fakeQ{
		wsRole: "guest",
		room:   db.RoomMember{Status: "pending", Role: "member", Email: "p@x.com"},
	}, ws, room, uuid.NewString(), NeedView); !errors.Is(err, ErrDenied) {
		t.Fatalf("pending must not satisfy NeedView: %v", err)
	}
}

func TestResolve_WorkspaceAdminOversightBeatsPendingRow(t *testing.T) {
	ws, room := ids()
	caps, err := Resolve(context.Background(), &fakeQ{
		wsRole: "admin",
		room:   db.RoomMember{Status: "pending", Role: "guest", Email: "a@x.com"},
	}, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.View || !caps.Oversight || caps.InvitedPending {
		t.Fatalf("workspace admin must keep oversight over pending row: %+v", caps)
	}
}

func TestGrantableRole(t *testing.T) {
	if GrantableRole("contributor") != RoleMember {
		t.Fatal("contributor")
	}
	if GrantableRole("viewer") != RoleGuest {
		t.Fatal("viewer")
	}
	if GrantableRole("owner") != "" {
		t.Fatal("owner")
	}
	if GrantableRole("admin") != RoleAdmin {
		t.Fatal("admin")
	}
}

func ids() (pgtype.UUID, pgtype.UUID) {
	return pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true},
		pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}
}

func TestPickMailboxMemberPrefersPending(t *testing.T) {
	email := "janedoe@gmail.com"
	rows := []db.RoomMember{
		{Email: "jane.doe+old@gmail.com", Status: "suspended"},
		{Email: "jane.doe+vdr@gmail.com", Status: "pending", Role: "guest"},
		{Email: "janedoe@gmail.com", Status: "active", Role: "member"},
	}
	got, ok := PickMailboxMember(rows, email)
	if !ok || got.Status != "pending" || got.Email != "jane.doe+vdr@gmail.com" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

type bindFake struct {
	fakeQ
	user     db.User
	unbound  []db.RoomMember
	bound    db.RoomMember
	boundOK  bool
	bindKeys []string
}

func (f *bindFake) GetUserByID(context.Context, pgtype.UUID) (db.User, error) {
	return f.user, nil
}

func (f *bindFake) BindRoomMembersUserByEmail(_ context.Context, arg db.BindRoomMembersUserByEmailParams) (int64, error) {
	f.bindKeys = append(f.bindKeys, arg.Email)
	for i := range f.unbound {
		m := &f.unbound[i]
		if m.Email == arg.Email && !m.UserID.Valid && (m.Status == "pending" || m.Status == "active") {
			m.UserID = arg.UserID
			f.bound = *m
			f.boundOK = true
			return 1, nil
		}
	}
	return 0, nil
}

func (f *bindFake) ListRoomMembers(context.Context, pgtype.UUID) ([]db.RoomMember, error) {
	return f.unbound, nil
}

func (f *bindFake) GetRoomMemberByUserID(_ context.Context, arg db.GetRoomMemberByUserIDParams) (db.RoomMember, error) {
	if f.boundOK && f.bound.UserID == arg.UserID {
		return f.bound, nil
	}
	return db.RoomMember{}, pgx.ErrNoRows
}

func TestResolve_GuestHealsGmailAliasInvite(t *testing.T) {
	ws, room := ids()
	uid := uuid.New()
	f := &bindFake{
		fakeQ: fakeQ{wsRole: "guest", roomErr: pgx.ErrNoRows},
		user:  db.User{Email: "janedoe@gmail.com"},
		unbound: []db.RoomMember{{
			Email:  "jane.doe+vdr@gmail.com",
			Status: "pending",
			Role:   "guest",
		}},
	}
	caps, err := Resolve(context.Background(), f, ws, room, uid.String())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.InvitedPending || caps.View {
		t.Fatalf("guest alias invite must be pending wall: %+v", caps)
	}
	if f.bound.Email != "jane.doe+vdr@gmail.com" || !f.bound.UserID.Valid {
		t.Fatalf("must bind stored invite email, got %+v keys=%v", f.bound, f.bindKeys)
	}
}

func TestResolve_GuestDoesNotStealBoundGmailAliasInvite(t *testing.T) {
	ws, room := ids()
	other := pgtype.UUID{Bytes: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Valid: true}
	f := &bindFake{
		fakeQ: fakeQ{wsRole: "guest", roomErr: pgx.ErrNoRows},
		user:  db.User{Email: "janedoe@gmail.com"},
		unbound: []db.RoomMember{{
			Email:  "jane.doe+vdr@gmail.com",
			Status: "pending",
			Role:   "guest",
			UserID: other,
		}},
	}
	caps, err := Resolve(context.Background(), f, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if caps.InvitedPending || caps.View {
		t.Fatalf("must not inherit a bound invite: %+v", caps)
	}
	if f.boundOK {
		t.Fatalf("must not rebind, keys=%v", f.bindKeys)
	}
}

func TestResolve_OversightDoesNotConsumeGmailAliasInvite(t *testing.T) {
	ws, room := ids()
	f := &bindFake{
		fakeQ: fakeQ{wsRole: "admin", roomErr: pgx.ErrNoRows},
		user:  db.User{Email: "janedoe@gmail.com"},
		unbound: []db.RoomMember{{
			Email:  "jane.doe+lp@gmail.com",
			Status: "pending",
			Role:   "guest",
		}},
	}
	caps, err := Resolve(context.Background(), f, ws, room, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.View || !caps.Oversight || caps.InvitedPending {
		t.Fatalf("admin must keep oversight: %+v", caps)
	}
	if f.boundOK {
		t.Fatalf("oversight must not consume alias invite, keys=%v", f.bindKeys)
	}
}
