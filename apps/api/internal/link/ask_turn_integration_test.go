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

func TestCreateHostAskTurn_TurnsOnly_Integration(t *testing.T) {
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

func TestAnswerAskTurnHostAnswer_FromCreateHostAskTurn_Integration(t *testing.T) {
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

	_, err = f.svc.AnswerAskTurnHostAnswer(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		"We will follow up next week.",
	)
	if err != nil {
		t.Fatalf("AnswerAskTurnHostAnswer: %v", err)
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
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "AI Ask Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	link.AskAiEnabled = true
	drf.f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true})

	visitorID := "visitor-" + uuid.NewString()
	turn, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", "AI route?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if turn.RouteReason != routeReasonAILanePending {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
	if turn.Lane != askLaneAI || turn.Status != askStatusAIStreaming {
		t.Fatalf("lane=%q status=%q", turn.Lane, turn.Status)
	}
}

func TestSubmitPublicAsk_CorpusNotReady_FallsBackHost_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Corpus Not Ready Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	link.AskAiEnabled = true
	corpusNotReady := false
	drf.f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true, corpusReady: &corpusNotReady})

	visitorID := "visitor-" + uuid.NewString()
	turn, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", "Corpus gate?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if turn.RouteReason != routeReasonAIUnavailable {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
	if turn.Lane != askLaneHost {
		t.Fatalf("lane=%q", turn.Lane)
	}
}

func TestSubmitPublicAsk_AINoKnowledge_FallsBackHost_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)
	if _, err := f.tx.Exec(f.ctx, `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, f.link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	f.link.AskAiEnabled = true

	visitorID := "visitor-" + uuid.NewString()
	turn, err := f.svc.SubmitPublicAsk(f.ctx, f.link, visitorID, "visitor@example.com", "Host fallback?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if turn.RouteReason != routeReasonAIUnavailable {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
	if turn.Lane != askLaneHost {
		t.Fatalf("lane=%q", turn.Lane)
	}
}

func TestSubmitPublicAsk_AIQuotaExceeded_FallsBackHost_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Quota Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_ai_enabled = true, ask_ai_monthly_quota = 0 WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("set ai quota: %v", err)
	}
	link.AskAiEnabled = true
	drf.f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true})

	visitorID := "visitor-" + uuid.NewString()
	turn, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", "Quota exceeded?", false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if turn.RouteReason != routeReasonAIQuotaExceeded {
		t.Fatalf("route_reason = %q", turn.RouteReason)
	}
	if turn.Lane != askLaneHost || turn.Status != askStatusHostPending {
		t.Fatalf("lane=%q status=%q", turn.Lane, turn.Status)
	}
}

func TestSubmitPublicAsk_RepeatQuestionAfterEnableAI_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Repeat Ask Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	link.AskAiEnabled = false
	drf.f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true})

	visitorID := "visitor-" + uuid.NewString()
	qRepeat := "2025年净利润多少"
	qOther := "预测2027年净利润多少"

	first, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", qRepeat, false)
	if err != nil {
		t.Fatalf("first SubmitPublicAsk: %v", err)
	}
	if first.Lane != askLaneHost {
		t.Fatalf("first lane = %q want host", first.Lane)
	}

	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	link.AskAiEnabled = true

	second, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", qOther, false)
	if err != nil {
		t.Fatalf("second SubmitPublicAsk: %v", err)
	}
	if second.Lane != askLaneAI {
		t.Fatalf("second lane = %q want ai", second.Lane)
	}

	third, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", qRepeat, false)
	if err != nil {
		t.Fatalf("third SubmitPublicAsk: %v", err)
	}
	if third.Lane != askLaneAI {
		t.Fatalf("third lane = %q want ai (unpinned repeat still routes AI)", third.Lane)
	}
	if third.ID == first.ID {
		t.Fatal("repeat submit must create a new turn")
	}

	turns, err := drf.f.svc.ListMyAskTurns(drf.ctx(), link.ID, visitorID)
	if err != nil {
		t.Fatalf("ListMyAskTurns: %v", err)
	}
	var repeatCount int
	for _, row := range turns {
		if row.Question == qRepeat {
			repeatCount++
		}
	}
	if repeatCount != 2 {
		t.Fatalf("expected 2 turns for %q, got %d", qRepeat, repeatCount)
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

func TestGetLinkAnalytics_AskSummary_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	enableLinkQA(t, f)

	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	linkID := uuid.UUID(f.link.ID.Bytes).String()
	visitorID := "visitor-" + uuid.NewString()

	turn, err := f.svc.CreateHostAskTurn(f.ctx, f.link, visitorID, "visitor@example.com", "Analytics summary?", false)
	if err != nil {
		t.Fatalf("CreateHostAskTurn: %v", err)
	}

	pendingAnalytics, err := f.svc.GetLinkAnalytics(f.ctx, linkID, wsID)
	if err != nil {
		t.Fatalf("GetLinkAnalytics pending: %v", err)
	}
	if pendingAnalytics.AskSummary == nil {
		t.Fatal("expected ask_summary")
	}
	if pendingAnalytics.AskSummary.HostPending != 1 || pendingAnalytics.AskSummary.HostAnswered != 0 {
		t.Fatalf("pending summary = %+v", pendingAnalytics.AskSummary)
	}
	if pendingAnalytics.AskSummary.DeflectionRate != nil {
		t.Fatalf("expected nil deflection with no AI answers, got %v", *pendingAnalytics.AskSummary.DeflectionRate)
	}

	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}
	if _, err := f.svc.AnswerAskTurnHostAnswer(
		f.ctx,
		f.link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		"Summary answer",
	); err != nil {
		t.Fatalf("AnswerAskTurnHostAnswer: %v", err)
	}

	answeredAnalytics, err := f.svc.GetLinkAnalytics(f.ctx, linkID, wsID)
	if err != nil {
		t.Fatalf("GetLinkAnalytics answered: %v", err)
	}
	if answeredAnalytics.AskSummary == nil {
		t.Fatal("expected ask_summary after answer")
	}
	if answeredAnalytics.AskSummary.HostAnswered != 1 || answeredAnalytics.AskSummary.HostPending != 0 {
		t.Fatalf("answered summary = %+v", answeredAnalytics.AskSummary)
	}
}

func TestCreateDealRoomLink_DefaultAskAIEnabled_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name:         "Default AI Link",
		AskAiEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	if !link.QaEnabled {
		t.Fatal("expected qa_enabled true for new deal-room link")
	}
	if !link.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled true for new deal-room link")
	}
	if link.AskMode != AskModeSupervised {
		t.Fatalf("ask_mode = %q", link.AskMode)
	}
}

func TestCreateDealRoomLink_AskAiEnabledFalse_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name:         "No AI Link",
		AskAiEnabled: false,
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	if !link.QaEnabled {
		t.Fatal("expected qa_enabled true (Ask Host baseline)")
	}
	if link.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled false when AI assistant is disabled")
	}
}

func TestUpdateLink_AskAiEnabledFalse_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Toggle AI Off",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	if !link.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled true on create")
	}

	linkID := uuid.UUID(link.ID.Bytes).String()
	askOff := false
	updated, err := drf.f.svc.UpdateLink(drf.ctx(), linkID, drf.wsID, UpdateLinkRequest{
		Name:         link.Name.String,
		AskAIEnabled: &askOff,
	})
	if err != nil {
		t.Fatalf("UpdateLink: %v", err)
	}
	if !updated.QaEnabled {
		t.Fatal("expected qa_enabled true after update (Ask Host baseline)")
	}
	if updated.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled false when AI assistant is disabled")
	}
}

func TestUpdateLink_AskAiEnabledTrue_WithoutCorpus_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name:         "Toggle AI On",
		AskAiEnabled: false,
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	if link.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled false on create")
	}

	linkID := uuid.UUID(link.ID.Bytes).String()
	askOn := true
	updated, err := drf.f.svc.UpdateLink(drf.ctx(), linkID, drf.wsID, UpdateLinkRequest{
		Name:         link.Name.String,
		AskAIEnabled: &askOn,
	})
	if err != nil {
		t.Fatalf("UpdateLink enable AI without corpus: %v", err)
	}
	if !updated.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled true after update")
	}
}

func TestUpdateLinkAskPolicy_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "Policy Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	wsID := uuid.UUID(drf.f.workspace.ID.Bytes).String()
	linkID := uuid.UUID(link.ID.Bytes).String()
	drf.f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true})

	enabled := true
	updated, err := drf.f.svc.UpdateLinkAskPolicy(drf.ctx(), linkID, wsID, UpdateLinkAskPolicyRequest{
		AskAIEnabled: &enabled,
	})
	if err != nil {
		t.Fatalf("UpdateLinkAskPolicy enable: %v", err)
	}
	if !updated.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled true")
	}

	docLinkID := uuid.UUID(drf.f.link.ID.Bytes).String()
	_, err = drf.f.svc.UpdateLinkAskPolicy(drf.ctx(), docLinkID, wsID, UpdateLinkAskPolicyRequest{
		AskAIEnabled: &enabled,
	})
	if err == nil {
		t.Fatal("expected error enabling AI on document-only link")
	}
}
