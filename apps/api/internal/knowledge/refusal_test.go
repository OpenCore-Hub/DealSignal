package knowledge

import "testing"

func TestIsUngroundedAnswer(t *testing.T) {
	if !isUngroundedAnswer("The provided context does not contain an answer to the question.") {
		t.Fatal("expected refusal")
	}
	if isUngroundedAnswer("The cap is $10M [1]") {
		t.Fatal("expected grounded answer")
	}
}

func TestClassifyTurnResult(t *testing.T) {
	refused, status := classifyTurnResult("does not contain an answer", 2)
	if !refused || status != "refused" {
		t.Fatalf("got refused=%v status=%q", refused, status)
	}
	refused, status = classifyTurnResult("", 0)
	if refused || status != "no_hits" {
		t.Fatalf("got refused=%v status=%q", refused, status)
	}
	refused, status = classifyTurnResult("ok [1]", 1)
	if refused || status != "answered" {
		t.Fatalf("got refused=%v status=%q", refused, status)
	}
}

func TestTruncateRunes(t *testing.T) {
	got := truncateRunes("一二三四五六七八九十", 5)
	if got != "一二三四五" {
		t.Fatalf("got %q", got)
	}
}
