package assistant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeAuditAuth struct {
	wsRole     string
	wsErr      error
	roomStatus string
	roomErr    error
}

func (f *fakeAuditAuth) GetWorkspaceMember(_ context.Context, _ db.GetWorkspaceMemberParams) (db.WorkspaceMember, error) {
	if f.wsErr != nil {
		return db.WorkspaceMember{}, f.wsErr
	}
	return db.WorkspaceMember{Role: f.wsRole}, nil
}

func (f *fakeAuditAuth) GetRoomMemberByUserID(_ context.Context, _ db.GetRoomMemberByUserIDParams) (db.RoomMember, error) {
	if f.roomErr != nil {
		return db.RoomMember{}, f.roomErr
	}
	return db.RoomMember{Status: f.roomStatus, Role: "viewer"}, nil
}

func TestAuthorizeAskDocsAudit_AllowsActiveRoomMember(t *testing.T) {
	link := db.Link{
		WorkspaceID: pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true},
		DealRoomID:  pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true},
	}
	err := authorizeAskDocsAudit(context.Background(), &fakeAuditAuth{
		wsErr:      pgx.ErrNoRows,
		roomStatus: "active",
	}, link, uuid.NewString())
	if err != nil {
		t.Fatalf("expected room member allowed, got %v", err)
	}
}

func TestAuthorizeAskDocsAudit_AllowsWorkspaceAdmin(t *testing.T) {
	link := db.Link{
		WorkspaceID: pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true},
	}
	err := authorizeAskDocsAudit(context.Background(), &fakeAuditAuth{wsRole: "admin"}, link, uuid.NewString())
	if err != nil {
		t.Fatalf("expected workspace admin allowed, got %v", err)
	}
}

func TestAuthorizeAskDocsAudit_RejectsNonMember(t *testing.T) {
	link := db.Link{
		WorkspaceID: pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true},
		DealRoomID:  pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true},
	}
	err := authorizeAskDocsAudit(context.Background(), &fakeAuditAuth{
		wsRole:  "member",
		roomErr: pgx.ErrNoRows,
	}, link, uuid.NewString())
	if !errors.Is(err, ErrAskDocsAuditForbidden) {
		t.Fatalf("expected ErrAskDocsAuditForbidden, got %v", err)
	}
}

func TestFilterAskDocsAuditHotWindow(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	hot := now.AddDate(0, 0, -30)
	archived := now.AddDate(0, 0, -91)
	entries := []AskDocsAuditEntry{
		{SessionID: "hot", CreatedAt: hot},
		{SessionID: "old", CreatedAt: archived},
	}
	got := filterAskDocsAuditEntries(entries, now, false)
	if len(got) != 1 || got[0].SessionID != "hot" {
		t.Fatalf("default list must keep 90-day hot only, got %+v", got)
	}
	gotAll := filterAskDocsAuditEntries(entries, now, true)
	if len(gotAll) != 2 {
		t.Fatalf("include archived must return both, got %d", len(gotAll))
	}
	if !gotAll[1].Archived {
		t.Fatal("expected old entry marked archived")
	}
}
