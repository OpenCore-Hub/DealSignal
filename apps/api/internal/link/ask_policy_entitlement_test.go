package link

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDealRoomAskAIReady(t *testing.T) {
	s := &Service{}
	link := db.Link{
		DealRoomID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}
	if s.dealRoomAskAIReady(t.Context(), link) {
		t.Fatal("expected false without knowledge service")
	}
	if s.dealRoomAskAIReady(t.Context(), db.Link{}) {
		t.Fatal("expected false for invalid room id")
	}
}

func TestCheckAskAIEntitlement_NoKnowledge(t *testing.T) {
	s := &Service{}
	link := db.Link{
		DealRoomID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}
	if err := s.checkAskAIEntitlement(t.Context(), link); err == nil {
		t.Fatal("expected entitlement error without knowledge service")
	}
}

func TestFormalAskEntitlementFailsClosedWithoutChecker(t *testing.T) {
	s := &Service{}
	if s.isFormalAskEntitled(t.Context(), db.Link{}) {
		t.Fatal("expected formal entitlement to fail closed without a checker")
	}
}

type stubFormalAskEntitlement struct {
	ok  bool
	err error
}

func (s stubFormalAskEntitlement) IsFormalAskEntitled(context.Context, string) (bool, error) {
	return s.ok, s.err
}

func TestFormalAskEntitlementFailsClosedWithoutQueries(t *testing.T) {
	s := &Service{formalAskEntitlement: stubFormalAskEntitlement{ok: true}}
	if s.isFormalAskEntitled(t.Context(), db.Link{}) {
		t.Fatal("expected fail-closed when queries are unset")
	}
}

func TestFormalAskEntitlementDeniedByWorkspacePlan(t *testing.T) {
	ws := uuid.New()
	link := db.Link{WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true}}
	s := &Service{
		formalAskEntitlement: stubFormalAskEntitlement{ok: true},
		planChecker:          stubPlanChecker{formalAskErr: plan.ErrFeatureFormalAsk},
	}
	if s.isFormalAskEntitled(t.Context(), link) {
		t.Fatal("workspace plan deny must win even when Docling stub allows")
	}
}

func TestFormalAskEntitlementNilPlanCheckerStillRequiresQueries(t *testing.T) {
	ws := uuid.New()
	s := &Service{formalAskEntitlement: stubFormalAskEntitlement{ok: true}}
	if s.isFormalAskEntitled(t.Context(), db.Link{WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true}}) {
		t.Fatal("nil plan checker must still fail closed without queries")
	}
}

func TestFormalAskEntitlementUnrestrictedPlanStillRequiresQueries(t *testing.T) {
	ws := uuid.New()
	s := &Service{
		formalAskEntitlement: stubFormalAskEntitlement{ok: true},
		planChecker:          plan.Unrestricted{},
	}
	if s.isFormalAskEntitled(t.Context(), db.Link{WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true}}) {
		t.Fatal("allowed workspace plan must still fail closed without queries")
	}
}

func TestFormalAskEntitlementInvalidWorkspaceIDFailsClosed(t *testing.T) {
	s := &Service{
		formalAskEntitlement: stubFormalAskEntitlement{ok: true},
		planChecker:          plan.Unrestricted{},
	}
	if s.isFormalAskEntitled(t.Context(), db.Link{}) {
		t.Fatal("invalid workspace id with plan checker must fail closed")
	}
}

func TestSyncDealRoomAskPolicyRequiresFormalEntitlement(t *testing.T) {
	s := &Service{}
	link := db.Link{
		DealRoomID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		AskMode:    AskModeSupervised,
	}
	err := s.syncDealRoomAskPolicy(t.Context(), nil, link, false, AskModeFormal)
	if err == nil || !errors.Is(err, ErrAskFormalNotEntitled) {
		t.Fatalf("expected ErrAskFormalNotEntitled, got %v", err)
	}
}
