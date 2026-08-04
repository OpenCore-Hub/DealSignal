package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type stubRoomAccess struct {
	err error
}

func (s stubRoomAccess) GetRoom(context.Context, string, string) (db.DealRoom, error) {
	return db.DealRoom{}, nil
}

func (s stubRoomAccess) RequireActiveRoomMember(context.Context, string, string, string) error {
	return s.err
}

func TestRecordDeskEventCiteOpen(t *testing.T) {
	svc := &Service{access: stubRoomAccess{}}
	before := testutil.ToFloat64(knowledgeQACiteOpensTotal.WithLabelValues("grounded"))
	if err := svc.RecordDeskEvent(context.Background(), "r", "w", "u", DeskEventRequest{
		Type:        "cite_open",
		TurnOutcome: "grounded",
	}); err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(knowledgeQACiteOpensTotal.WithLabelValues("grounded")) < before+1 {
		t.Fatal("cite open counter")
	}
}

func TestRecordDeskEventForbidden(t *testing.T) {
	svc := &Service{access: stubRoomAccess{err: ErrForbidden}}
	err := svc.RecordDeskEvent(context.Background(), "r", "w", "u", DeskEventRequest{Type: "cite_open"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestRecordDeskEventInvalid(t *testing.T) {
	svc := &Service{access: stubRoomAccess{}}
	if err := svc.RecordDeskEvent(context.Background(), "r", "w", "u", DeskEventRequest{Type: "nope"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestRecordDeskEventFollowUpsUpgradeFailed(t *testing.T) {
	svc := &Service{access: stubRoomAccess{}}
	before := testutil.ToFloat64(knowledgeQAFollowUpsUpgradeFailedTotal)
	if err := svc.RecordDeskEvent(context.Background(), "r", "w", "u", DeskEventRequest{
		Type: "followups_upgrade_failed",
	}); err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(knowledgeQAFollowUpsUpgradeFailedTotal) < before+1 {
		t.Fatal("followups_upgrade_failed counter")
	}
}
