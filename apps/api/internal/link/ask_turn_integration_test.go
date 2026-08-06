//go:build integration

package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func enableLinkQA(t *testing.T, f *testFixture) {
	t.Helper()
	if _, err := f.tx.Exec(f.ctx, `UPDATE links SET qa_enabled = true WHERE id = $1`, f.link.ID); err != nil {
		t.Fatalf("enable qa: %v", err)
	}
	f.link.QaEnabled = true
}

func TestCreateHostAskTurn_DualWrite_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.CreateHostAskTurn(f.ctx, f.link, visitorID, "visitor@example.com", "What is the timeline?", false)
	if err != nil {
		t.Fatalf("CreateHostAskTurn: %v", err)
	}
	if turn.Question != "What is the timeline?" {
		t.Fatalf("turn question = %q", turn.Question)
	}
	if turn.HostQuestionID == "" {
		t.Fatal("expected host_question_id on turn")
	}

	legacy, err := f.q.ListVisitorQuestionsByVisitor(f.ctx, db.ListVisitorQuestionsByVisitorParams{
		LinkID:    f.link.ID,
		VisitorID: visitorID,
	})
	if err != nil {
		t.Fatalf("ListVisitorQuestionsByVisitor: %v", err)
	}
	if len(legacy) != 1 {
		t.Fatalf("expected 1 legacy question, got %d", len(legacy))
	}
	if legacy[0].Question != "What is the timeline?" {
		t.Fatalf("legacy question = %q", legacy[0].Question)
	}

	turns, err := f.q.ListLinkAskTurnsByVisitor(f.ctx, db.ListLinkAskTurnsByVisitorParams{
		LinkID:    f.link.ID,
		VisitorID: visitorID,
	})
	if err != nil {
		t.Fatalf("ListLinkAskTurnsByVisitor: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
}

func TestListMyAskTurns_LegacyDualRead_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	legacyQ, err := f.q.CreateVisitorQuestion(f.ctx, db.CreateVisitorQuestionParams{
		TenantID:     f.link.TenantID,
		WorkspaceID:  f.link.WorkspaceID,
		LinkID:       f.link.ID,
		VisitorID:    visitorID,
		VisitorEmail: pgtype.Text{String: "visitor@example.com", Valid: true},
		Question:     "Legacy-only question",
	})
	if err != nil {
		t.Fatalf("CreateVisitorQuestion direct: %v", err)
	}

	got, err := f.svc.ListMyAskTurns(f.ctx, f.link.ID, visitorID)
	if err != nil {
		t.Fatalf("ListMyAskTurns: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 merged turn, got %d", len(got))
	}
	if got[0].Question != "Legacy-only question" {
		t.Fatalf("question = %q", got[0].Question)
	}
	if got[0].HostQuestionID != uuid.UUID(legacyQ.ID.Bytes).String() {
		t.Fatalf("host_question_id = %q", got[0].HostQuestionID)
	}
}

func TestAnswerVisitorQuestion_SyncsAskTurn_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.CreateHostAskTurn(f.ctx, f.link, visitorID, "visitor@example.com", "Need clarification", false)
	if err != nil {
		t.Fatalf("CreateHostAskTurn: %v", err)
	}
	hostQID, err := uuid.Parse(turn.HostQuestionID)
	if err != nil {
		t.Fatalf("parse host question id: %v", err)
	}

	_, err = f.svc.AnswerVisitorQuestion(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: hostQID, Valid: true},
		f.user.ID,
		"We will follow up next week.",
	)
	if err != nil {
		t.Fatalf("AnswerVisitorQuestion: %v", err)
	}

	turns, err := f.svc.ListMyAskTurns(f.ctx, f.link.ID, visitorID)
	if err != nil {
		t.Fatalf("ListMyAskTurns: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Status != askStatusHostAnswered {
		t.Fatalf("status = %q", turns[0].Status)
	}
	if turns[0].HostAnswer != "We will follow up next week." {
		t.Fatalf("host_answer = %q", turns[0].HostAnswer)
	}
}

func TestListLinkAskInbox_LegacyDualRead_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	legacyQ, err := f.q.CreateVisitorQuestion(f.ctx, db.CreateVisitorQuestionParams{
		TenantID:     f.link.TenantID,
		WorkspaceID:  f.link.WorkspaceID,
		LinkID:       f.link.ID,
		VisitorID:    visitorID,
		VisitorEmail: pgtype.Text{String: "visitor@example.com", Valid: true},
		Question:     "Legacy inbox question",
	})
	if err != nil {
		t.Fatalf("CreateVisitorQuestion direct: %v", err)
	}

	userID := uuid.UUID(f.user.ID.Bytes).String()
	inbox, err := f.svc.ListLinkAskInbox(f.ctx, f.link, userID, askLaneHost, "")
	if err != nil {
		t.Fatalf("ListLinkAskInbox: %v", err)
	}
	if len(inbox) != 1 {
		t.Fatalf("expected 1 inbox item, got %d", len(inbox))
	}
	if inbox[0].Question != "Legacy inbox question" {
		t.Fatalf("question = %q", inbox[0].Question)
	}
	if inbox[0].HostQuestionID != uuid.UUID(legacyQ.ID.Bytes).String() {
		t.Fatalf("host_question_id = %q", inbox[0].HostQuestionID)
	}
}

func TestListMyVisitorQuestions_DualRead_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	legacyQ, err := f.q.CreateVisitorQuestion(f.ctx, db.CreateVisitorQuestionParams{
		TenantID:     f.link.TenantID,
		WorkspaceID:  f.link.WorkspaceID,
		LinkID:       f.link.ID,
		VisitorID:    visitorID,
		VisitorEmail: pgtype.Text{String: "visitor@example.com", Valid: true},
		Question:     "Legacy-only for questions/me",
	})
	if err != nil {
		t.Fatalf("CreateVisitorQuestion direct: %v", err)
	}

	got, err := f.svc.ListMyVisitorQuestions(f.ctx, f.link.ID, visitorID)
	if err != nil {
		t.Fatalf("ListMyVisitorQuestions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 merged question, got %d", len(got))
	}
	if got[0].Question != "Legacy-only for questions/me" {
		t.Fatalf("question = %q", got[0].Question)
	}
	if got[0].ID != uuid.UUID(legacyQ.ID.Bytes).String() {
		t.Fatalf("id = %q", got[0].ID)
	}
}

func TestCreateHostAskTurn_EscalateRouteReason_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.CreateHostAskTurn(f.ctx, f.link, visitorID, "visitor@example.com", "Please confirm", true)
	if err != nil {
		t.Fatalf("CreateHostAskTurn: %v", err)
	}
	if turn.RouteReason != "user_escalate" {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
}

func TestSubmitPublicAsk_AILanePending_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	if _, err := f.tx.Exec(f.ctx, `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, f.link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	f.link.AskAiEnabled = true

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, visitorID, "visitor@example.com", "AI route?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if turn.RouteReason != routeReasonAILanePending {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
}

func TestSubmitPublicAsk_AINotEnabled_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, visitorID, "visitor@example.com", "Host only?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if turn.RouteReason != routeReasonAINotEnabled {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
}

func TestAnswerAskTurnHostAnswer_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.CreateHostAskTurn(f.ctx, f.link, visitorID, "visitor@example.com", "Need clarification", false)
	if err != nil {
		t.Fatalf("CreateHostAskTurn: %v", err)
	}
	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}

	answered, err := f.svc.AnswerAskTurnHostAnswer(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		"Answer via turn API",
	)
	if err != nil {
		t.Fatalf("AnswerAskTurnHostAnswer: %v", err)
	}
	if answered.Status != askStatusHostAnswered {
		t.Fatalf("status = %q", answered.Status)
	}
	if answered.HostAnswer != "Answer via turn API" {
		t.Fatalf("host_answer = %q", answered.HostAnswer)
	}

	turns, err := f.svc.ListMyAskTurns(f.ctx, f.link.ID, visitorID)
	if err != nil {
		t.Fatalf("ListMyAskTurns: %v", err)
	}
	if len(turns) != 1 || turns[0].HostAnswer != "Answer via turn API" {
		t.Fatalf("visitor timeline not updated: %+v", turns)
	}
}
