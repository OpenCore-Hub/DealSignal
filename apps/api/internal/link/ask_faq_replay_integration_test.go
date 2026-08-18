//go:build integration

package link

import (
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func pinHostAnsweredFAQ(t *testing.T, f *testFixture, link db.Link, question, answer string) OwnerAskTurn {
	t.Helper()
	visitorID := "visitor-" + uuid.NewString()
	created, err := f.svc.SubmitPublicAsk(f.ctx, link, visitorID, "visitor@example.com", question, false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	turnUUID, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse turn: %v", err)
	}
	if created.Status != askStatusHostPending && created.Status != askStatusHostAnswered {
		_, err = f.tx.Exec(f.ctx, `
			UPDATE link_ask_turns
			SET lane = 'host',
			    status = 'host_pending',
			    host_answer = NULL,
			    ai_payload = NULL,
			    updated_at = now()
			WHERE id = $1`, pgtype.UUID{Bytes: turnUUID, Valid: true})
		if err != nil {
			t.Fatalf("force host pending: %v", err)
		}
	}
	answered, err := f.svc.AnswerAskTurnHostAnswer(
		f.ctx,
		link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		answer,
	)
	if err != nil {
		t.Fatalf("AnswerAskTurnHostAnswer: %v", err)
	}
	pinned, err := f.svc.PinAskTurnFAQ(
		f.ctx,
		link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
	)
	if err != nil {
		t.Fatalf("PinAskTurnFAQ: %v", err)
	}
	if pinned.HostAnswer == "" {
		t.Fatalf("expected host answer on pin, got %+v answered=%+v", pinned, answered)
	}
	return pinned
}

func enableVisitorAskAI(t *testing.T, f *testFixture, link *db.Link) {
	t.Helper()
	if _, err := f.tx.Exec(f.ctx, `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	link.AskAiEnabled = true
	f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true})
}

func pinAIAnsweredFAQ(t *testing.T, f *testFixture, link db.Link, question, answer string) OwnerAskTurn {
	t.Helper()
	visitorID := "visitor-" + uuid.NewString()
	created, err := f.svc.SubmitPublicAsk(f.ctx, link, visitorID, "visitor@example.com", question, false)
	if err != nil {
		t.Fatalf("SubmitPublicAsk: %v", err)
	}
	if created.Lane != askLaneAI {
		t.Fatalf("expected AI source turn, lane=%q route=%q", created.Lane, created.RouteReason)
	}
	turnUUID, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse turn: %v", err)
	}
	_, err = f.tx.Exec(f.ctx, `
		UPDATE link_ask_turns
		SET status = 'ai_answered',
		    ai_payload = jsonb_build_object(
		      'answer', $2::text,
		      'refused', false,
		      'resultStatus', 'answered',
		      'hits', '[]'::jsonb
		    ),
		    updated_at = now()
		WHERE id = $1`, pgtype.UUID{Bytes: turnUUID, Valid: true}, answer)
	if err != nil {
		t.Fatalf("force ai answered: %v", err)
	}
	pinned, err := f.svc.PinAskTurnFAQ(
		f.ctx,
		link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
	)
	if err != nil {
		t.Fatalf("PinAskTurnFAQ: %v", err)
	}
	if pinned.AIPayload == nil || pinned.AIPayload.Answer != answer {
		t.Fatalf("expected AI answer on pin, got %+v", pinned)
	}
	return pinned
}

func TestSubmitPublicAsk_PinnedFAQInterceptsRepeat_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "FAQ Intercept Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	question := "What is GMV?"
	answer := "GMV is 1亿."
	pinHostAnsweredFAQ(t, drf.f, link, question, answer)

	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	link.AskAiEnabled = true
	drf.f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true})

	visitorID := "visitor-" + uuid.NewString()
	replay, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", "what is gmv??", false)
	if err != nil {
		t.Fatalf("replay SubmitPublicAsk: %v", err)
	}
	if replay.RouteReason != routeReasonPinnedFAQ {
		t.Fatalf("route_reason=%q", replay.RouteReason)
	}
	if replay.Lane != askLaneHost || replay.Status != askStatusHostAnswered {
		t.Fatalf("lane=%q status=%q", replay.Lane, replay.Status)
	}
	if replay.HostAnswer != answer {
		t.Fatalf("host_answer=%q", replay.HostAnswer)
	}
	if replay.AIPayload != nil {
		t.Fatal("hybrid/host replay must not copy AI payload")
	}
	if replay.FaqSourceTurnID == "" {
		t.Fatal("expected faq_source_turn_id")
	}

	used, err := drf.f.q.CountLinkAskAITurnsThisMonth(drf.ctx(), link.ID)
	if err != nil {
		t.Fatalf("quota count: %v", err)
	}
	if used != 0 {
		t.Fatalf("pinned_faq must not count toward AI quota, used=%d", used)
	}

	miss, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", "GMV增长率", false)
	if err != nil {
		t.Fatalf("near-synonym SubmitPublicAsk: %v", err)
	}
	if miss.RouteReason == routeReasonPinnedFAQ {
		t.Fatal("GMV增长率 must not hit GMV pin")
	}
	if miss.Lane != askLaneAI {
		t.Fatalf("unpinned near-synonym lane=%q", miss.Lane)
	}
}

func TestSubmitPublicAsk_PinnedFAQAliasAndUnpin_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "FAQ Alias Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	pinned := pinHostAnsweredFAQ(t, drf.f, link, "年度经常性收入是多少", "ARR is $12M")
	turnUUID, err := uuid.Parse(pinned.ID)
	if err != nil {
		t.Fatalf("parse pin id: %v", err)
	}
	updated, err := drf.f.svc.SetAskTurnFAQAliases(
		drf.ctx(),
		link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		drf.userID,
		[]string{"What is ARR?"},
	)
	if err != nil {
		t.Fatalf("SetAskTurnFAQAliases: %v", err)
	}
	if len(updated.Aliases) != 1 {
		t.Fatalf("aliases=%v", updated.Aliases)
	}

	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_ai_enabled = true WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("enable ask ai: %v", err)
	}
	link.AskAiEnabled = true
	drf.f.svc.WithVisitorAskKnowledge(stubVisitorAskKnowledge{enabled: true})

	visitorID := "visitor-" + uuid.NewString()
	hit, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "v@example.com", "what is arr", false)
	if err != nil {
		t.Fatalf("alias ask: %v", err)
	}
	if hit.RouteReason != routeReasonPinnedFAQ {
		t.Fatalf("alias route_reason=%q", hit.RouteReason)
	}

	if _, err := drf.f.svc.UnpinAskTurnFAQ(drf.ctx(), link, pgtype.UUID{Bytes: turnUUID, Valid: true}, drf.userID); err != nil {
		t.Fatalf("UnpinAskTurnFAQ: %v", err)
	}
	after, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "v@example.com", "what is arr", false)
	if err != nil {
		t.Fatalf("after unpin: %v", err)
	}
	if after.RouteReason == routeReasonPinnedFAQ {
		t.Fatal("unpin must stop intercept")
	}
}

func TestSubmitPublicAsk_PinnedFAQSkipsFormalAndEscalate_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "FAQ Skip Formal",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	question := "Need a signed NDA copy"
	pinHostAnsweredFAQ(t, drf.f, link, question, "See legal folder.")

	visitorID := "visitor-" + uuid.NewString()
	escalated, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "v@example.com", question, true)
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if escalated.RouteReason != routeReasonUserEscalate {
		t.Fatalf("escalate route_reason=%q", escalated.RouteReason)
	}

	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_mode = 'formal' WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("set formal: %v", err)
	}
	link.AskMode = AskModeFormal
	formal, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "v@example.com", question, false)
	if err != nil {
		t.Fatalf("formal: %v", err)
	}
	if formal.RouteReason != routeReasonPolicyFormal {
		t.Fatalf("formal route_reason=%q", formal.RouteReason)
	}
}

func TestSubmitPublicAsk_PinnedAIFAQReplayExcludesQuota_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "FAQ AI Quota Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	enableVisitorAskAI(t, drf.f, &link)

	question := "What is monthly burn?"
	answer := "Monthly burn is $420K."
	pinned := pinAIAnsweredFAQ(t, drf.f, link, question, answer)

	used, err := drf.f.q.CountLinkAskAITurnsThisMonth(drf.ctx(), link.ID)
	if err != nil {
		t.Fatalf("quota count: %v", err)
	}
	if used != 1 {
		t.Fatalf("source AI pin should count once, used=%d", used)
	}

	if _, err := drf.f.tx.Exec(drf.ctx(), `UPDATE links SET ask_ai_monthly_quota = 1 WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("set link quota: %v", err)
	}
	link.AskAiMonthlyQuota = pgtype.Int4{Int32: 1, Valid: true}

	visitorID := "visitor-" + uuid.NewString()
	replay, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "visitor@example.com", question, false)
	if err != nil {
		t.Fatalf("replay SubmitPublicAsk: %v", err)
	}
	if replay.RouteReason != routeReasonPinnedFAQ {
		t.Fatalf("route_reason=%q", replay.RouteReason)
	}
	if replay.Lane != askLaneAI || replay.Status != askStatusAIAnswered {
		t.Fatalf("lane=%q status=%q", replay.Lane, replay.Status)
	}
	if replay.AIPayload == nil || replay.AIPayload.Answer != answer {
		t.Fatalf("ai payload=%+v", replay.AIPayload)
	}
	if replay.AIPayload.Refused {
		t.Fatal("official AI pin replay must not mark refused")
	}
	if replay.FaqSourceTurnID != pinned.ID {
		t.Fatalf("faq_source_turn_id=%q want %q", replay.FaqSourceTurnID, pinned.ID)
	}

	used, err = drf.f.q.CountLinkAskAITurnsThisMonth(drf.ctx(), link.ID)
	if err != nil {
		t.Fatalf("quota count after replay: %v", err)
	}
	if used != 1 {
		t.Fatalf("pinned_faq AI replay must not increment quota, used=%d", used)
	}

	wsUsed, err := drf.f.q.CountWorkspaceAskAITurnsThisMonth(drf.ctx(), link.WorkspaceID)
	if err != nil {
		t.Fatalf("workspace quota count: %v", err)
	}
	if wsUsed != 1 {
		t.Fatalf("workspace pinned_faq replay must not increment quota, used=%d", wsUsed)
	}
}

func TestSubmitPublicAsk_HybridPinDoesNotCopyRefusePayload_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "FAQ Hybrid Refuse Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	question := "What is the cap table?"
	answer := "暂不公开"
	pinned := pinHostAnsweredFAQ(t, drf.f, link, question, answer)
	turnUUID, err := uuid.Parse(pinned.ID)
	if err != nil {
		t.Fatalf("parse pin id: %v", err)
	}
	_, err = drf.f.tx.Exec(drf.ctx(), `
		UPDATE link_ask_turns
		SET lane = 'hybrid',
		    ai_payload = jsonb_build_object(
		      'answer', '',
		      'refused', true,
		      'resultStatus', 'refused',
		      'hits', '[]'::jsonb
		    ),
		    updated_at = now()
		WHERE id = $1`, pgtype.UUID{Bytes: turnUUID, Valid: true})
	if err != nil {
		t.Fatalf("inject refuse payload: %v", err)
	}

	enableVisitorAskAI(t, drf.f, &link)
	visitorID := "visitor-" + uuid.NewString()
	replay, err := drf.f.svc.SubmitPublicAsk(drf.ctx(), link, visitorID, "v@example.com", question, false)
	if err != nil {
		t.Fatalf("replay SubmitPublicAsk: %v", err)
	}
	if replay.RouteReason != routeReasonPinnedFAQ {
		t.Fatalf("route_reason=%q", replay.RouteReason)
	}
	if replay.HostAnswer != answer {
		t.Fatalf("host_answer=%q", replay.HostAnswer)
	}
	if replay.AIPayload != nil {
		t.Fatalf("hybrid replay must not copy refuse ai_payload: %+v", replay.AIPayload)
	}
	if replay.Lane != askLaneHost || replay.Status != askStatusHostAnswered {
		t.Fatalf("lane=%q status=%q", replay.Lane, replay.Status)
	}
}

func TestSetAskTurnFAQAliases_Conflict_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "FAQ Alias Conflict Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	pinHostAnsweredFAQ(t, drf.f, link, "What is GMV?", "GMV is 1亿.")
	arr := pinHostAnsweredFAQ(t, drf.f, link, "What is ARR?", "ARR is $12M")
	arrID, err := uuid.Parse(arr.ID)
	if err != nil {
		t.Fatalf("parse arr id: %v", err)
	}

	_, err = drf.f.svc.SetAskTurnFAQAliases(
		drf.ctx(),
		link,
		pgtype.UUID{Bytes: arrID, Valid: true},
		drf.userID,
		[]string{"What is GMV?"},
	)
	if !errors.Is(err, ErrAskFAQAliasConflict) {
		t.Fatalf("expected alias conflict, err=%v", err)
	}
}

func TestPinAskTurnFAQ_QuestionConflict_Integration(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.f.cleanup()
	enableLinkQA(t, drf.f)
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name: "FAQ Pin Conflict Link",
	})
	if err != nil {
		t.Fatalf("CreateDealRoomLink: %v", err)
	}
	pinHostAnsweredFAQ(t, drf.f, link, "What is GMV?", "GMV is 1亿.")
	second := pinHostAnsweredFAQ(t, drf.f, link, "What is ARR?", "ARR is $12M")
	turnUUID, err := uuid.Parse(second.ID)
	if err != nil {
		t.Fatalf("parse second id: %v", err)
	}
	if _, err := drf.f.svc.UnpinAskTurnFAQ(drf.ctx(), link, pgtype.UUID{Bytes: turnUUID, Valid: true}, drf.userID); err != nil {
		t.Fatalf("UnpinAskTurnFAQ: %v", err)
	}
	if _, err := drf.f.tx.Exec(drf.ctx(), `
		UPDATE link_ask_turns
		SET question = 'what is gmv??',
		    updated_at = now()
		WHERE id = $1`, pgtype.UUID{Bytes: turnUUID, Valid: true}); err != nil {
		t.Fatalf("overlap question: %v", err)
	}
	_, err = drf.f.svc.PinAskTurnFAQ(
		drf.ctx(),
		link,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		drf.f.user.ID,
	)
	if !errors.Is(err, ErrAskFAQAliasConflict) {
		t.Fatalf("expected pin conflict, err=%v", err)
	}
}
