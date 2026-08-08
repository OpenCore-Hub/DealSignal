package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAccessAuditWindow(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	start, end := accessAuditWindow(7, now)
	if start.Format(time.RFC3339) != "2026-08-02T00:00:00Z" {
		t.Fatalf("start=%s", start)
	}
	if end.Format(time.RFC3339) != "2026-08-09T00:00:00Z" {
		t.Fatalf("end=%s", end)
	}
}

func TestClampAccessAuditLimit(t *testing.T) {
	if got := clampAccessAuditLimit(0); got != accessAuditDefaultLimit {
		t.Fatalf("got %d", got)
	}
	if got := clampAccessAuditLimit(500); got != accessAuditMaxLimit {
		t.Fatalf("got %d", got)
	}
	if got := clampAccessAuditLimit(40); got != 40 {
		t.Fatalf("got %d", got)
	}
}

func TestServiceAccessAuditCustomRange(t *testing.T) {
	ws := uuid.New()
	svc := NewService(&mockAnalyticsQuerier{}, nil, testCfg())
	out, err := svc.AccessAudit(context.Background(), ws.String(), AccessAuditQuery{
		From:  "2026-07-01",
		To:    "2026-07-14",
		Limit: 25,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !out.RangeCustom || out.RangeDays != 14 || out.RangeFrom != "2026-07-01" || out.RangeTo != "2026-07-14" {
		t.Fatalf("unexpected custom range: %+v", out)
	}
}

func TestServiceAccessAuditRejectsBadCustomRange(t *testing.T) {
	ws := uuid.New()
	svc := NewService(&mockAnalyticsQuerier{}, nil, testCfg())
	_, err := svc.AccessAudit(context.Background(), ws.String(), AccessAuditQuery{From: "2026-07-01"})
	if !errors.Is(err, errInsightsRangeInvalid) {
		t.Fatalf("got %v want errInsightsRangeInvalid", err)
	}
}

func TestServiceAccessAudit(t *testing.T) {
	ws := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	evID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	roomID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	memberID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	q := &mockAnalyticsQuerier{
		accessAuditByType: []db.CountWorkspaceAccessAuditByTypeRow{
			{EventType: "invalid_password", Count: 2},
		},
		accessAuditByRoom: []db.CountWorkspaceAccessAuditByDealRoomRow{
			{DealRoomID: pgtype.UUID{Bytes: roomID, Valid: true}, DealRoomName: "Series A", Count: 2},
		},
		accessAuditByMember: []db.CountWorkspaceAccessAuditByMemberRow{
			{MemberID: pgtype.UUID{Bytes: memberID, Valid: true}, MemberEmail: "owner@example.com", Count: 2},
		},
		accessAuditByFolder: []db.CountWorkspaceAccessAuditByFolderRow{
			{
				FolderPath:   "Finance",
				DealRoomID:   pgtype.UUID{Bytes: roomID, Valid: true},
				DealRoomName: "Series A",
				Count:        2,
			},
		},
		accessAuditEvents: []db.ListWorkspaceAccessAuditEventsRow{
			{
				ID:            pgtype.UUID{Bytes: evID, Valid: true},
				LinkID:        pgtype.UUID{Bytes: linkID, Valid: true},
				EventType:     "invalid_password",
				Email:         pgtype.Text{String: "a@example.com", Valid: true},
				CreatedAt:     pgtype.Timestamptz{Time: time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC), Valid: true},
				DocumentTitle: "Deck",
				DealRoomID:    pgtype.UUID{Bytes: roomID, Valid: true},
				DealRoomName:  "Series A",
				MemberID:      pgtype.UUID{Bytes: memberID, Valid: true},
				MemberEmail:   "owner@example.com",
				FolderPath:    "Finance",
			},
		},
	}
	svc := NewService(q, nil, testCfg())
	out, err := svc.AccessAudit(context.Background(), ws.String(), AccessAuditQuery{Days: 30, Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if out.TotalEvents != 2 {
		t.Fatalf("total=%d", out.TotalEvents)
	}
	if len(out.Events) != 1 || out.Events[0].Email != "a@example.com" {
		t.Fatalf("events=%+v", out.Events)
	}
	if out.ByDealRoom[0].DealRoomID != roomID.String() {
		t.Fatalf("room=%s", out.ByDealRoom[0].DealRoomID)
	}
	if len(out.ByMember) != 1 || out.ByMember[0].MemberID != memberID.String() {
		t.Fatalf("byMember=%+v", out.ByMember)
	}
	if len(out.ByFolder) != 1 || out.ByFolder[0].FolderPath != "Finance" {
		t.Fatalf("byFolder=%+v", out.ByFolder)
	}
	if out.Events[0].MemberEmail != "owner@example.com" || out.Events[0].FolderPath != "Finance" {
		t.Fatalf("event enrichment=%+v", out.Events[0])
	}
}

func TestServiceAccessAuditRejectsBadEventType(t *testing.T) {
	ws := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	svc := NewService(&mockAnalyticsQuerier{}, nil, testCfg())
	_, err := svc.AccessAudit(context.Background(), ws.String(), AccessAuditQuery{EventType: "not_a_real_type"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceAccessAuditRejectsBadMemberID(t *testing.T) {
	ws := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	svc := NewService(&mockAnalyticsQuerier{}, nil, testCfg())
	_, err := svc.AccessAudit(context.Background(), ws.String(), AccessAuditQuery{MemberID: "not-a-uuid"})
	if err == nil {
		t.Fatal("expected error")
	}
}
