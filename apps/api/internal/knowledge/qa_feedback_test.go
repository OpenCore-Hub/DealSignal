package knowledge

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidFeedbackKind(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		FeedbackKindHelpful,
		FeedbackKindWrongCitation,
		FeedbackKindNotAnswering,
	} {
		if !validFeedbackKind(kind) {
			t.Fatalf("expected %q valid", kind)
		}
	}
	if validFeedbackKind("thumbs_up") {
		t.Fatal("unexpected kind accepted")
	}
}

func TestNormalizeFeedbackRequest(t *testing.T) {
	t.Parallel()

	kind, note, err := normalizeFeedbackRequest(FeedbackRequest{
		Kind: "  helpful ",
		Note: "  ok  ",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if kind != FeedbackKindHelpful || note != "ok" {
		t.Fatalf("got kind=%q note=%q", kind, note)
	}

	_, _, err = normalizeFeedbackRequest(FeedbackRequest{Kind: "thumbs_up"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("kind err=%v, want ErrInvalidInput", err)
	}

	long := strings.Repeat("测", knowledgeQAFeedbackNoteMaxRunes+1)
	if utf8.RuneCountInString(long) <= knowledgeQAFeedbackNoteMaxRunes {
		t.Fatal("fixture too short")
	}
	_, _, err = normalizeFeedbackRequest(FeedbackRequest{
		Kind: FeedbackKindWrongCitation,
		Note: long,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("note err=%v, want ErrInvalidInput", err)
	}

	exact := strings.Repeat("a", knowledgeQAFeedbackNoteMaxRunes)
	_, note, err = normalizeFeedbackRequest(FeedbackRequest{
		Kind: FeedbackKindWrongCitation,
		Note: exact,
	})
	if err != nil {
		t.Fatalf("exact-length note rejected: %v", err)
	}
	if note != exact {
		t.Fatalf("note mutated")
	}
}
