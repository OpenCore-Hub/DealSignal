//go:build integration

package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func setLinkAskMode(t *testing.T, f *testFixture, mode string) {
	t.Helper()
	if _, err := f.tx.Exec(f.ctx, `UPDATE links SET ask_mode = $1 WHERE id = $2`, mode, f.link.ID); err != nil {
		t.Fatalf("set ask_mode: %v", err)
	}
	f.link.AskMode = mode
}

func TestSubmitPublicAsk_FormalMode_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	setLinkAskMode(t, f, AskModeFormal)

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, visitorID, "visitor@example.com", "Formal disclosure question?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if turn.RouteReason != routeReasonPolicyFormal {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
	if turn.FormalStatus != formalStatusPendingReview {
		t.Fatalf("formal_status = %q", turn.FormalStatus)
	}
	if turn.HostAnswer != "" {
		t.Fatal("expected masked host answer for visitor")
	}

	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}
	published, err := f.svc.PublishFormalAskTurn(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		FormalPublishInput{Answer: "Approved public answer."},
	)
	if err != nil {
		t.Fatalf("PublishFormalAskTurn: %v", err)
	}
	if published.FormalStatus != formalStatusPublished {
		t.Fatalf("formal_status after publish = %q", published.FormalStatus)
	}
	if published.HostAnswer != "Approved public answer." {
		t.Fatalf("host_answer = %q", published.HostAnswer)
	}

	formalBoard, err := f.svc.ListPublicFormalAsk(f.ctx, f.link)
	if err != nil {
		t.Fatalf("ListPublicFormalAsk: %v", err)
	}
	if len(formalBoard) != 1 {
		t.Fatalf("formal board len = %d", len(formalBoard))
	}
	if formalBoard[0].Answer != "Approved public answer." {
		t.Fatalf("formal answer = %q", formalBoard[0].Answer)
	}

	inbox, err := f.svc.ListLinkAskInbox(f.ctx, f.link, uuid.UUID(f.user.ID.Bytes).String(), "", ownerAskInboxFormalQueue)
	if err != nil {
		t.Fatalf("ListLinkAskInbox formal_queue: %v", err)
	}
	if len(inbox) != 0 {
		t.Fatalf("expected empty formal queue after publish, got %d", len(inbox))
	}
}

func TestPublishFormalAskTurn_ScheduledThenDue_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	setLinkAskMode(t, f, AskModeFormal)

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, visitorID, "visitor@example.com", "Delayed disclosure?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}

	publishAt := time.Now().UTC().Add(-time.Minute)
	scheduled, err := f.svc.PublishFormalAskTurn(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		FormalPublishInput{
			Answer:    "Scheduled answer.",
			PublishAt: &publishAt,
		},
	)
	if err != nil {
		t.Fatalf("PublishFormalAskTurn schedule: %v", err)
	}
	if scheduled.FormalStatus != formalStatusScheduled {
		t.Fatalf("formal_status = %q", scheduled.FormalStatus)
	}

	formalBoard, err := f.svc.ListPublicFormalAsk(f.ctx, f.link)
	if err != nil {
		t.Fatalf("ListPublicFormalAsk: %v", err)
	}
	if len(formalBoard) != 1 {
		t.Fatalf("expected 1 published after due sweep, got %d", len(formalBoard))
	}
	if formalBoard[0].Answer != "Scheduled answer." {
		t.Fatalf("answer = %q", formalBoard[0].Answer)
	}

	legacy, err := f.q.GetLinkAskTurnByID(f.ctx, db.GetLinkAskTurnByIDParams{
		ID:          pgtype.UUID{Bytes: turnUUID, Valid: true},
		WorkspaceID: f.link.WorkspaceID,
		LinkID:      f.link.ID,
	})
	if err != nil {
		t.Fatalf("GetLinkAskTurnByID: %v", err)
	}
	if legacy.Status != askStatusHostAnswered {
		t.Fatalf("status after due publish = %q", legacy.Status)
	}
	if !legacy.HostAnswer.Valid || legacy.HostAnswer.String != "Scheduled answer." {
		t.Fatalf("host answer = %+v", legacy.HostAnswer)
	}
}
