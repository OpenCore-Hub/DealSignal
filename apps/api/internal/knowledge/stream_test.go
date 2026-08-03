package knowledge

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAnswerTokenChunks(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := answerTokenChunks("", 36); got != nil {
			t.Fatalf("got %#v want nil", got)
		}
	})
	t.Run("short single chunk", func(t *testing.T) {
		t.Parallel()
		got := answerTokenChunks("short", 36)
		if len(got) != 1 || got[0] != "short" {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("preserves full text across chunks", func(t *testing.T) {
		t.Parallel()
		in := "Grounded answer for: What is the valuation cap? See clause [1] on page 3."
		got := answerTokenChunks(in, 20)
		if len(got) < 2 {
			t.Fatalf("expected multiple chunks, got %#v", got)
		}
		if joined := strings.Join(got, ""); joined != in {
			t.Fatalf("join mismatch:\n got %q\nwant %q", joined, in)
		}
		for _, c := range got {
			if c == "" {
				t.Fatal("empty chunk")
			}
			if utf8.RuneCountInString(c) > 20 {
				t.Fatalf("chunk too long: %q (%d)", c, utf8.RuneCountInString(c))
			}
		}
	})
	t.Run("unicode runes not bytes", func(t *testing.T) {
		t.Parallel()
		in := strings.Repeat("估值", 30) // 60 runes
		got := answerTokenChunks(in, 16)
		if joined := strings.Join(got, ""); joined != in {
			t.Fatalf("unicode join mismatch")
		}
	})
}

func TestShouldEmitAnswerTokens(t *testing.T) {
	t.Parallel()
	if shouldEmitAnswerTokens("", QATurn{ResultStatus: "answered"}) {
		t.Fatal("empty answer")
	}
	if shouldEmitAnswerTokens("x", QATurn{ResultStatus: "error"}) {
		t.Fatal("error turn")
	}
	if !shouldEmitAnswerTokens("refused text", QATurn{Refused: true, ResultStatus: "refused"}) {
		t.Fatal("refusal text should still stream")
	}
}

func TestShouldEmitGroundedSources(t *testing.T) {
	t.Parallel()
	hit := QueryHit{ChunkID: "c1", Text: "clause"}
	cases := []struct {
		name string
		turn QATurn
		want bool
	}{
		{
			name: "answered with hits",
			turn: QATurn{ResultStatus: "answered", Hits: []QueryHit{hit}},
			want: true,
		},
		{
			name: "refused flag clears hits",
			turn: QATurn{Refused: true, ResultStatus: "refused", Hits: []QueryHit{hit}},
			want: false,
		},
		{
			name: "refused status",
			turn: QATurn{ResultStatus: "refused", Hits: []QueryHit{hit}},
			want: false,
		},
		{
			name: "error turn",
			turn: QATurn{ResultStatus: "error", Hits: []QueryHit{hit}},
			want: false,
		},
		{
			name: "no hits",
			turn: QATurn{ResultStatus: "no_hits", Hits: nil},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldEmitGroundedSources(tc.turn); got != tc.want {
				t.Fatalf("shouldEmitGroundedSources=%v want %v", got, tc.want)
			}
		})
	}
}
