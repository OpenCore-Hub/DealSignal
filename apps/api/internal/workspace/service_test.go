package workspace

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCreateWorkspaceInvalidSlug(t *testing.T) {
	svc := NewService(db.New(&fakeDB{t: t}))
	_, err := svc.Create(context.Background(), uuid.NewString(), "Demo", "my workspace!", "")
	if !errors.Is(err, ErrInvalidSlug) {
		t.Fatalf("expected ErrInvalidSlug, got %v", err)
	}
}

func TestCreateWorkspace(t *testing.T) {
	fake := &fakeDB{t: t}
	svc := NewService(db.New(fake))
	userID := uuid.NewString()

	ws, err := svc.Create(context.Background(), userID, "Demo Capital", "demo-capital", "#ff0000")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if ws.Name != "Demo Capital" {
		t.Fatalf("expected name Demo Capital, got %s", ws.Name)
	}
	if ws.Slug != "demo-capital" {
		t.Fatalf("expected slug demo-capital, got %s", ws.Slug)
	}
	if ws.BrandColor != "#ff0000" {
		t.Fatalf("expected brand color #ff0000, got %s", ws.BrandColor)
	}
}

func TestListWorkspaces(t *testing.T) {
	userID := uuid.NewString()
	wsID := uuid.New()
	tenantID := uuid.New()
	now := time.Now()
	fake := &fakeDB{
		t: t,
		listRows: []db.ListWorkspacesByUserRow{
			{
				ID:         pgtype.UUID{Bytes: wsID, Valid: true},
				TenantID:   pgtype.UUID{Bytes: tenantID, Valid: true},
				Name:       "Demo Capital",
				Slug:       "demo-capital",
				BrandColor: pgtype.Text{String: "#ff0000", Valid: true},
				CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
				Role:       RoleOwner,
			},
		},
	}
	svc := NewService(db.New(fake))

	workspaces, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(workspaces))
	}
	if workspaces[0].Role != RoleOwner {
		t.Fatalf("expected role owner, got %s", workspaces[0].Role)
	}
}

func TestGetWorkspaceNotMember(t *testing.T) {
	fake := &fakeDB{t: t}
	svc := NewService(db.New(fake))

	_, err := svc.Get(context.Background(), uuid.NewString(), uuid.NewString(), "")
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestAddMemberInvalidRole(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), "superuser")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if err.Error() != "invalid role" {
		t.Fatalf("expected invalid role error, got %v", err)
	}
}

func TestAddMemberSuccess(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	member, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleAdmin)
	if err != nil {
		t.Fatalf("add member: %v", err)
	}
	if member.Role != RoleAdmin {
		t.Fatalf("expected role admin, got %s", member.Role)
	}
}

// fakeDB is a minimal in-memory implementation of db.DBTX for service tests.
type fakeDB struct {
	t           *testing.T
	memberRole  string
	actorUserID string
	listRows    []db.ListWorkspacesByUserRow

	tenant      db.Tenant
	workspace   db.Workspace
	member      db.WorkspaceMember
	invitation  db.WorkspaceInvitation
}

func (f *fakeDB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	rows := make([][]interface{}, len(f.listRows))
	for i, r := range f.listRows {
		rows[i] = []interface{}{
			r.ID, r.TenantID, r.Name, r.Slug, r.BrandColor, r.CreatedAt, r.Role,
		}
	}
	return &fakeRows{rows: rows}, nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	sqlLower := strings.ToLower(sql)

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	switch {
	case strings.Contains(sqlLower, "insert into tenants"):
		f.tenant = db.Tenant{
			ID:        newPGUUID(),
			Name:      argString(args, 0),
			Slug:      pgtype.Text{String: argString(args, 1), Valid: true},
			CreatedAt: now,
		}
		return fakeRow{values: []interface{}{f.tenant.ID, f.tenant.Name, f.tenant.Slug, f.tenant.CreatedAt}}

	case strings.Contains(sqlLower, "insert into workspaces"):
		f.workspace = db.Workspace{
			ID:         newPGUUID(),
			TenantID:   argUUID(args, 0),
			Name:       argString(args, 1),
			Slug:       argString(args, 2),
			BrandColor: argText(args, 3),
			CreatedAt:  now,
		}
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt}}

	case strings.Contains(sqlLower, "insert into workspace_members"):
		f.member = db.WorkspaceMember{
			WorkspaceID: argUUID(args, 0),
			UserID:      argUUID(args, 1),
			Role:        argString(args, 2),
			JoinedAt:    now,
		}
		return fakeRow{values: []interface{}{f.member.WorkspaceID, f.member.UserID, f.member.Role, f.member.JoinedAt}}

	case strings.Contains(sqlLower, "insert into workspace_invitations"):
		f.invitation = db.WorkspaceInvitation{
			Token:       newPGUUID(),
			WorkspaceID: argUUID(args, 0),
			Email:       argString(args, 1),
			Role:        argString(args, 2),
			ExpiresAt:   argTimestamptz(args, 3),
			CreatedAt:   now,
		}
		return fakeRow{values: []interface{}{f.invitation.Token, f.invitation.WorkspaceID, f.invitation.Email, f.invitation.Role, f.invitation.ExpiresAt, f.invitation.UsedAt, f.invitation.CreatedAt}}

	case strings.Contains(sqlLower, "from workspace_invitations"):
		return fakeRow{values: []interface{}{f.invitation.Token, f.invitation.WorkspaceID, f.invitation.Email, f.invitation.Role, f.invitation.ExpiresAt, f.invitation.UsedAt, f.invitation.CreatedAt}}

	case strings.Contains(sqlLower, "from workspaces") && (strings.Contains(sqlLower, "where w.id") || strings.Contains(sqlLower, "where id = $1 and tenant_id")):
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt}}

	case strings.Contains(sqlLower, "from workspaces") && strings.Contains(sqlLower, "where slug"):
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt}}

	case strings.Contains(sqlLower, "from workspace_members") && strings.Contains(sqlLower, "where workspace_id"):
		if f.memberRole == "" || !bytesEqual(argUUID(args, 1).Bytes, pgUUIDFromString(f.actorUserID).Bytes) {
			return fakeRow{err: pgx.ErrNoRows}
		}
		f.member = db.WorkspaceMember{
			WorkspaceID: argUUID(args, 0),
			UserID:      argUUID(args, 1),
			Role:        f.memberRole,
			JoinedAt:    now,
		}
		return fakeRow{values: []interface{}{f.member.WorkspaceID, f.member.UserID, f.member.Role, f.member.JoinedAt}}
	}

	return fakeRow{err: errors.New("unexpected query")}
}

type fakeRow struct {
	values []interface{}
	err    error
}

func (r fakeRow) Scan(dest ...interface{}) error {
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

type fakeRows struct {
	rows [][]interface{}
	pos  int
}

func (r *fakeRows) Next() bool { return r.pos < len(r.rows) }
func (r *fakeRows) Err() error { return nil }
func (r *fakeRows) Close()     {}
func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) Scan(dest ...interface{}) error {
	if r.pos >= len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.pos]
	r.pos++
	if len(dest) != len(row) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(row))
	}
	for i, v := range row {
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

func newPGUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}

func argString(args []interface{}, i int) string {
	if i >= len(args) {
		return ""
	}
	if s, ok := args[i].(string); ok {
		return s
	}
	if t, ok := args[i].(pgtype.Text); ok {
		return t.String
	}
	return ""
}

func argUUID(args []interface{}, i int) pgtype.UUID {
	if i >= len(args) {
		return pgtype.UUID{}
	}
	if u, ok := args[i].(pgtype.UUID); ok {
		return u
	}
	return pgtype.UUID{}
}

func argText(args []interface{}, i int) pgtype.Text {
	if i >= len(args) {
		return pgtype.Text{}
	}
	if t, ok := args[i].(pgtype.Text); ok {
		return t
	}
	return pgtype.Text{}
}

func pgUUIDFromString(s string) pgtype.UUID {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func bytesEqual(a, b [16]byte) bool {
	return a == b
}

func TestAddMemberRequiresManager(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleMember, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleAdmin)
	if !errors.Is(err, ErrNotManager) {
		t.Fatalf("expected ErrNotManager, got %v", err)
	}
}

func TestGuestRoleValid(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	member, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleGuest)
	if err != nil {
		t.Fatalf("add guest member: %v", err)
	}
	if member.Role != RoleGuest {
		t.Fatalf("expected role guest, got %s", member.Role)
	}
}

func TestCreateInvitationRequiresManager(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleMember, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if !errors.Is(err, ErrNotManager) {
		t.Fatalf("expected ErrNotManager, got %v", err)
	}
}

func TestCreateInvitationInvalidRole(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", "superuser", 7)
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func argTimestamptz(args []interface{}, i int) pgtype.Timestamptz {
	if i >= len(args) {
		return pgtype.Timestamptz{}
	}
	if t, ok := args[i].(pgtype.Timestamptz); ok {
		return t
	}
	return pgtype.Timestamptz{}
}

func TestAcceptInvitationSuccess(t *testing.T) {
	actorID := uuid.NewString()
	userID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	member, err := svc.AcceptInvitation(context.Background(), inv.Token, userID)
	if err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	if member.Role != RoleGuest {
		t.Fatalf("expected role guest, got %s", member.Role)
	}
}

func TestAcceptInvitationExpired(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.invitation.ExpiresAt = pgtype.Timestamptz{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true}

	_, err = svc.AcceptInvitation(context.Background(), inv.Token, uuid.NewString())
	if !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("expected ErrInvitationExpired, got %v", err)
	}
}

func TestAcceptInvitationAlreadyUsed(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.invitation.UsedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}

	_, err = svc.AcceptInvitation(context.Background(), inv.Token, uuid.NewString())
	if !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("expected ErrInvitationUsed, got %v", err)
	}
}
