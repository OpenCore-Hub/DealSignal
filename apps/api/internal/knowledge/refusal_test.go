package knowledge

import "testing"

func TestIsUngroundedAnswer(t *testing.T) {
	t.Parallel()
	if !isUngroundedAnswer("The provided context does not contain an answer to the question.") {
		t.Fatal("expected ungrounded")
	}
	if !isUngroundedAnswer("The provided context does not contain the answer.") {
		t.Fatal("expected ungrounded alt")
	}
	// Screenshot regression: soft Chinese refusal mixed with NDA prose.
	zh := "根据您提供的上下文，文档属于单向保密协议，未提及员工期权池。因此，无法根据现有上下文回答该问题。"
	if !isUngroundedAnswer(zh) {
		t.Fatal("expected chinese soft refusal")
	}
	if !looksLikeNonRoomFactMeta("因此，无法根据现有上下文回答该问题。") {
		t.Fatal("meta sentence must be rejected")
	}
	if isUngroundedAnswer("The cap is $10M [1]") {
		t.Fatal("grounded answer")
	}
	if !isUngroundedAnswer("I cannot determine from these documents whether the pool exists.") {
		t.Fatal("expected novel english soft refusal")
	}
	if !looksLikeNonRoomFactMeta("现有材料不足以确定该问题。") {
		t.Fatal("novel chinese meta must be rejected")
	}
}

func TestLooksLikeOutOfRoomGeneralKnowledge(t *testing.T) {
	t.Parallel()
	bad := []string{
		"The EBITDA multiple for this sector is typically 12x.",
		"What does market-standard indemnity look like?",
		"同行一般怎么谈对赌条款？",
	}
	for _, b := range bad {
		if !looksLikeOutOfRoomGeneralKnowledge(b) {
			t.Fatalf("expected out-of-room: %q", b)
		}
	}
	good := "该模式仅约束接收方保守披露方的信息，对接收方明显不利。"
	if looksLikeOutOfRoomGeneralKnowledge(good) {
		t.Fatalf("room-local claim false positive: %q", good)
	}
}

func TestClassifyTurnResultTypedRefusal(t *testing.T) {
	t.Parallel()
	refused, status, info := classifyTurnResult("does not contain an answer", 2)
	if !refused || status != "refused" || info == nil || info.Kind != RefusalKindUngrounded {
		t.Fatalf("got refused=%v status=%q info=%#v", refused, status, info)
	}
	if !info.HadHits || info.HitCount != 2 {
		t.Fatalf("hadHits audit %#v", info)
	}

	refused, status, info = classifyTurnResult("", 0)
	if refused || status != "no_hits" || info == nil || info.Kind != RefusalKindNoHits {
		t.Fatalf("got refused=%v status=%q info=%#v", refused, status, info)
	}

	refused, status, info = classifyTurnResult("", 3)
	if refused || status != "no_hits" || info == nil || !info.HadHits || info.HitCount != 3 {
		t.Fatalf("empty answer with hits: refused=%v status=%q info=%#v", refused, status, info)
	}

	refused, status, info = classifyTurnResult("ok [1]", 1)
	if refused || status != "answered" || info != nil {
		t.Fatalf("got refused=%v status=%q info=%#v", refused, status, info)
	}
}

func TestHasGroundedClaim(t *testing.T) {
	t.Parallel()
	if hasGroundedClaim(BoundAnswer{Claims: []AnswerClaim{{Confidence: claimConfidenceWeak, HitIDs: []string{"c1"}}}}) {
		t.Fatal("weak is not grounded")
	}
	if !hasGroundedClaim(BoundAnswer{Claims: []AnswerClaim{{Confidence: claimConfidenceGrounded, HitIDs: []string{"c1"}}}}) {
		t.Fatal("expected grounded")
	}
}
