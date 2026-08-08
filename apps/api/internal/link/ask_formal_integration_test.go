//go:build integration

package link

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type allowAllFormalAskEntitlement struct{}

func (allowAllFormalAskEntitlement) IsFormalAskEntitled(context.Context, string) (bool, error) {
	return true, nil
}

type denyFormalAskEntitlement struct{}

func (denyFormalAskEntitlement) IsFormalAskEntitled(context.Context, string) (bool, error) {
	return false, nil
}

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

func TestPublishDueFormalAskTurnsGlobal_WorkerPath_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	setLinkAskMode(t, f, AskModeFormal)

	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, "visitor-"+uuid.NewString(), "worker@example.com", "Worker due publish?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}

	future := time.Now().UTC().Add(time.Hour)
	scheduled, err := f.svc.PublishFormalAskTurn(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		FormalPublishInput{
			Answer:    "Worker published answer.",
			PublishAt: &future,
		},
	)
	if err != nil {
		t.Fatalf("PublishFormalAskTurn: %v", err)
	}
	if scheduled.FormalStatus != formalStatusScheduled {
		t.Fatalf("formal_status = %q", scheduled.FormalStatus)
	}

	// Force due without visitor lazy-on-read: move publish_at into the past.
	if _, err := f.tx.Exec(f.ctx, `
		UPDATE link_ask_turns
		SET formal_publish_at = now() - interval '1 minute'
		WHERE id = $1
	`, pgtype.UUID{Bytes: turnUUID, Valid: true}); err != nil {
		t.Fatalf("backdate formal_publish_at: %v", err)
	}

	published, err := f.svc.PublishDueFormalAskTurnsGlobal(f.ctx, 10)
	if err != nil {
		t.Fatalf("PublishDueFormalAskTurnsGlobal: %v", err)
	}
	if published != 1 {
		t.Fatalf("published count = %d", published)
	}

	legacy, err := f.q.GetLinkAskTurnByID(f.ctx, db.GetLinkAskTurnByIDParams{
		ID:          pgtype.UUID{Bytes: turnUUID, Valid: true},
		WorkspaceID: f.link.WorkspaceID,
		LinkID:      f.link.ID,
	})
	if err != nil {
		t.Fatalf("GetLinkAskTurnByID: %v", err)
	}
	if !legacy.FormalStatus.Valid || legacy.FormalStatus.String != formalStatusPublished {
		t.Fatalf("formal_status after worker = %q", legacy.FormalStatus.String)
	}
	if legacy.Status != askStatusHostAnswered {
		t.Fatalf("status after worker = %q", legacy.Status)
	}

	// Second sweep is a no-op.
	again, err := f.svc.PublishDueFormalAskTurnsGlobal(f.ctx, 10)
	if err != nil {
		t.Fatalf("second PublishDueFormalAskTurnsGlobal: %v", err)
	}
	if again != 0 {
		t.Fatalf("expected 0 on second sweep, got %d", again)
	}
}

func TestListPublicFormalAsk_VisitorEmailWhenNotAnonymized_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	setLinkAskMode(t, f, AskModeFormal)

	visitorEmail := "compliance-board@example.com"
	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, "visitor-"+uuid.NewString(), visitorEmail, "Who asked about EBITDA?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}

	anonymize := false
	_, err = f.svc.PublishFormalAskTurn(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		FormalPublishInput{
			Answer:    "Attribution allowed on the public board.",
			Anonymize: &anonymize,
		},
	)
	if err != nil {
		t.Fatalf("PublishFormalAskTurn: %v", err)
	}

	formalBoard, err := f.svc.ListPublicFormalAsk(f.ctx, f.link)
	if err != nil {
		t.Fatalf("ListPublicFormalAsk: %v", err)
	}
	if len(formalBoard) != 1 {
		t.Fatalf("formal board len = %d", len(formalBoard))
	}
	if formalBoard[0].VisitorEmail != visitorEmail {
		t.Fatalf("visitor_email = %q, want %q", formalBoard[0].VisitorEmail, visitorEmail)
	}
}

func TestListPublicFormalAsk_AnonymizedOmitsVisitorEmail_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	setLinkAskMode(t, f, AskModeFormal)

	visitorEmail := "hidden-board@example.com"
	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, "visitor-"+uuid.NewString(), visitorEmail, "Hidden asker?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}

	anonymize := true
	_, err = f.svc.PublishFormalAskTurn(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		FormalPublishInput{
			Answer:    "Public guidance without attribution.",
			Anonymize: &anonymize,
		},
	)
	if err != nil {
		t.Fatalf("PublishFormalAskTurn: %v", err)
	}

	formalBoard, err := f.svc.ListPublicFormalAsk(f.ctx, f.link)
	if err != nil {
		t.Fatalf("ListPublicFormalAsk: %v", err)
	}
	if len(formalBoard) != 1 {
		t.Fatalf("formal board len = %d", len(formalBoard))
	}
	if formalBoard[0].VisitorEmail != "" {
		t.Fatalf("visitor_email = %q, want empty", formalBoard[0].VisitorEmail)
	}
}

func TestSubmitPublicAsk_FormalNotEntitledRejects_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	setLinkAskMode(t, f, AskModeFormal)
	f.svc.formalAskEntitlement = denyFormalAskEntitlement{}

	_, err := f.svc.SubmitPublicAsk(
		f.ctx,
		f.link,
		"visitor-"+uuid.NewString(),
		"denied-formal@example.com",
		"Formal should not queue without entitlement?",
		false,
	)
	if err == nil || !errors.Is(err, ErrAskFormalNotEntitled) {
		t.Fatalf("expected ErrAskFormalNotEntitled, got %v", err)
	}
}

func TestPublishFormalAskTurn_DeniedEntitlement_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	setLinkAskMode(t, f, AskModeFormal)

	turn, err := f.svc.SubmitPublicAsk(
		f.ctx,
		f.link,
		"visitor-"+uuid.NewString(),
		"publish-denied@example.com",
		"Queue before entitlement downgrade?",
		false,
	)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}

	f.svc.formalAskEntitlement = denyFormalAskEntitlement{}
	_, err = f.svc.PublishFormalAskTurn(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		FormalPublishInput{Answer: "Must not publish without entitlement."},
	)
	if err == nil || !errors.Is(err, ErrAskFormalNotEntitled) {
		t.Fatalf("expected ErrAskFormalNotEntitled, got %v", err)
	}
}
