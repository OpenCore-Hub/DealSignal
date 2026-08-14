package knowledge

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type roomAccessFakeDB struct {
	t                *testing.T
	room             db.DealRoom
	roomMembers      []db.RoomMember
	workspaceMembers []db.WorkspaceMember
}

func newRoomAccessFakeDB(t *testing.T) *roomAccessFakeDB {
	return &roomAccessFakeDB{t: t}
}

func (f *roomAccessFakeDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *roomAccessFakeDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &raFakeRows{rows: nil}, nil
}

func (f *roomAccessFakeDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	sqlLower := strings.ToLower(strings.Join(strings.Fields(sql), " "))
	switch {
	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where id = $1"):
		id := args[0].(pgtype.UUID)
		wsID := args[1].(pgtype.UUID)
		if f.room.ID == id && f.room.WorkspaceID == wsID {
			return raFakeRow{values: roomRow(f.room)}
		}
		return raFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sqlLower, "from workspace_members") && strings.Contains(sqlLower, "where workspace_id = $1 and user_id = $2"):
		wsID := args[0].(pgtype.UUID)
		userID := args[1].(pgtype.UUID)
		for _, m := range f.workspaceMembers {
			if m.WorkspaceID == wsID && m.UserID == userID {
				return raFakeRow{values: []any{m.WorkspaceID, m.UserID, m.Role, m.JoinedAt}}
			}
		}
		return raFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sqlLower, "from room_members") && strings.Contains(sqlLower, "where room_id = $1 and user_id = $2"):
		roomID := args[0].(pgtype.UUID)
		userID := args[1].(pgtype.UUID)
		for _, m := range f.roomMembers {
			if m.RoomID == roomID && m.UserID == userID {
				return raFakeRow{values: memberRow(m)}
			}
		}
		return raFakeRow{err: pgx.ErrNoRows}
	default:
		f.t.Fatalf("unexpected query: %s", sqlLower)
		return raFakeRow{err: pgx.ErrNoRows}
	}
}

func TestRoomAccessAdapterRequireActiveRoomMember_AllowsWorkspaceManager(t *testing.T) {
	fake := newRoomAccessFakeDB(t)
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	ownerID := pgUUID(uuid.NewString())
	fake.room = db.DealRoom{ID: roomID, WorkspaceID: wsID, TenantID: pgUUID(uuid.NewString())}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsID, UserID: ownerID, Role: "owner"},
	}

	adapter := roomAccessAdapter{queries: db.New(fake)}
	err := adapter.RequireActiveRoomMember(t.Context(), uuid.UUID(roomID.Bytes).String(), uuid.UUID(wsID.Bytes).String(), uuid.UUID(ownerID.Bytes).String())
	if err != nil {
		t.Fatalf("expected owner to pass room access, got %v", err)
	}
}

func TestRoomAccessAdapterRequireActiveRoomMember_AllowsWorkspaceAdmin(t *testing.T) {
	fake := newRoomAccessFakeDB(t)
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	adminID := pgUUID(uuid.NewString())
	fake.room = db.DealRoom{ID: roomID, WorkspaceID: wsID, TenantID: pgUUID(uuid.NewString())}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsID, UserID: adminID, Role: "admin"},
	}

	adapter := roomAccessAdapter{queries: db.New(fake)}
	err := adapter.RequireActiveRoomMember(t.Context(), uuid.UUID(roomID.Bytes).String(), uuid.UUID(wsID.Bytes).String(), uuid.UUID(adminID.Bytes).String())
	if err != nil {
		t.Fatalf("expected admin to pass room access, got %v", err)
	}
}

func TestRoomAccessAdapterRequireActiveRoomMember_RejectsNonManagerOutsider(t *testing.T) {
	fake := newRoomAccessFakeDB(t)
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	outsiderID := pgUUID(uuid.NewString())
	fake.room = db.DealRoom{ID: roomID, WorkspaceID: wsID, TenantID: pgUUID(uuid.NewString())}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsID, UserID: outsiderID, Role: "member"},
	}

	adapter := roomAccessAdapter{queries: db.New(fake)}
	err := adapter.RequireActiveRoomMember(t.Context(), uuid.UUID(roomID.Bytes).String(), uuid.UUID(wsID.Bytes).String(), uuid.UUID(outsiderID.Bytes).String())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestRoomAccessAdapterRequireActiveRoomMember_RejectsWrongWorkspaceEvenForManager(t *testing.T) {
	fake := newRoomAccessFakeDB(t)
	wsID := pgUUID(uuid.NewString())
	otherWS := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	ownerID := pgUUID(uuid.NewString())
	fake.room = db.DealRoom{ID: roomID, WorkspaceID: wsID, TenantID: pgUUID(uuid.NewString())}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: otherWS, UserID: ownerID, Role: "owner"},
	}

	adapter := roomAccessAdapter{queries: db.New(fake)}
	err := adapter.RequireActiveRoomMember(
		t.Context(),
		uuid.UUID(roomID.Bytes).String(),
		uuid.UUID(otherWS.Bytes).String(),
		uuid.UUID(ownerID.Bytes).String(),
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when room is not in caller workspace, got %v", err)
	}
}

func TestRoomAccessAdapterRequireActiveRoomMember_AllowsActiveRoomMember(t *testing.T) {
	fake := newRoomAccessFakeDB(t)
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	userID := pgUUID(uuid.NewString())
	fake.room = db.DealRoom{ID: roomID, WorkspaceID: wsID, TenantID: pgUUID(uuid.NewString())}
	fake.roomMembers = []db.RoomMember{
		{RoomID: roomID, WorkspaceID: wsID, UserID: userID, Status: "active"},
	}

	adapter := roomAccessAdapter{queries: db.New(fake)}
	err := adapter.RequireActiveRoomMember(t.Context(), uuid.UUID(roomID.Bytes).String(), uuid.UUID(wsID.Bytes).String(), uuid.UUID(userID.Bytes).String())
	if err != nil {
		t.Fatalf("expected active room member to pass, got %v", err)
	}
}

func roomRow(r db.DealRoom) []any {
	return []any{
		r.ID, r.TenantID, r.WorkspaceID, r.Slug, r.Name, r.Description,
		r.TemplateType, r.Settings, r.RequiresNda, r.RequiresApproval, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.ExpiresAt,
		r.NdaTemplateID, r.NdaDocumentID,
	}
}

func memberRow(m db.RoomMember) []any {
	return []any{
		m.ID, m.TenantID, m.WorkspaceID, m.RoomID, m.Email, m.UserID,
		m.Role, m.NdaStatus, m.NdaSignedAt, m.Status, m.CreatedAt, m.UpdatedAt,
	}
}

type raFakeRow struct {
	values []any
	err    error
}

func (r raFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(r.values))
	}
	for i, v := range r.values {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("destination is not a pointer")
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("cannot assign %s to %s", sv.Type(), dv.Elem().Type())
		}
		dv.Elem().Set(sv)
	}
	return nil
}

type raFakeRows struct {
	rows [][]any
}

func (r *raFakeRows) Close()                                       {}
func (r *raFakeRows) Err() error                                   { return nil }
func (r *raFakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *raFakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *raFakeRows) Next() bool                                   { return false }
func (r *raFakeRows) Scan(...any) error                            { return pgx.ErrNoRows }
func (r *raFakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *raFakeRows) RawValues() [][]byte                          { return nil }
func (r *raFakeRows) Conn() *pgx.Conn                              { return nil }
