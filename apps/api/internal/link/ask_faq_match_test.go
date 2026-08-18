package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func faqTurn(question, host, alias string, sort int32, pinnedAt time.Time) db.LinkAskTurn {
	id := uuid.New()
	turn := db.LinkAskTurn{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Question:      question,
		Lane:          askLaneHost,
		Status:        askStatusHostAnswered,
		HostAnswer:    pgtype.Text{String: host, Valid: true},
		PinnedFaqAt:   pgtype.Timestamptz{Time: pinnedAt, Valid: true},
		PinnedFaqSort: pgtype.Int4{Int32: sort, Valid: true},
	}
	if alias != "" {
		turn.PinnedFaqAliases = []string{alias}
	}
	return turn
}

func TestMatchPinnedFAQFromTurns(t *testing.T) {
	now := time.Now().UTC()
	gmv := faqTurn("GMV?", "1亿", "", 0, now)
	growth := faqTurn("GMV增长率", "20%", "", 1, now)
	aliased := faqTurn("年度经常性收入是多少", "ARR is $12M", "What is ARR?", 2, now)

	t.Run("punctuation and case match", func(t *testing.T) {
		got, ok := matchPinnedFAQFromTurns([]db.LinkAskTurn{gmv}, "  gmv?? ")
		if !ok || uuid.UUID(got.ID.Bytes) != uuid.UUID(gmv.ID.Bytes) {
			t.Fatalf("ok=%v got=%s", ok, uuid.UUID(got.ID.Bytes))
		}
	})

	t.Run("near synonym without alias does not match", func(t *testing.T) {
		_, ok := matchPinnedFAQFromTurns([]db.LinkAskTurn{gmv, growth}, "GMV增长率")
		if !ok {
			t.Fatal("growth question should match its own pin")
		}
		_, ok = matchPinnedFAQFromTurns([]db.LinkAskTurn{gmv}, "GMV增长率")
		if ok {
			t.Fatal("GMV pin must not match GMV增长率")
		}
	})

	t.Run("alias hits", func(t *testing.T) {
		got, ok := matchPinnedFAQFromTurns([]db.LinkAskTurn{aliased}, "what is arr")
		if !ok || uuid.UUID(got.ID.Bytes) != uuid.UUID(aliased.ID.Bytes) {
			t.Fatalf("alias miss ok=%v", ok)
		}
	})

	t.Run("sort then pinned_at wins collisions", func(t *testing.T) {
		later := faqTurn("GMV", "later", "", 5, now.Add(time.Hour))
		earlier := faqTurn("GMV", "earlier", "", 1, now)
		got, ok := matchPinnedFAQFromTurns([]db.LinkAskTurn{later, earlier}, "gmv")
		if !ok || uuid.UUID(got.ID.Bytes) != uuid.UUID(earlier.ID.Bytes) {
			t.Fatalf("expected lower sort, got %s", uuid.UUID(got.ID.Bytes))
		}
	})
}

func TestNormalizeFAQAliasList(t *testing.T) {
	got, err := normalizeFAQAliasList([]string{" What is ARR? ", "what is arr", "  "})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "What is ARR?" {
		t.Fatalf("got %#v", got)
	}

	tooLong := make([]byte, maxPinnedFAQAliasChars+1)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	if _, err := normalizeFAQAliasList([]string{string(tooLong)}); err != ErrAskFAQAliasesInvalid {
		t.Fatalf("err=%v", err)
	}

	many := make([]string, maxPinnedFAQAliases+1)
	for i := range many {
		many[i] = "alias " + string(rune('a'+i))
	}
	if _, err := normalizeFAQAliasList(many); err != ErrAskFAQAliasesInvalid {
		t.Fatalf("err=%v", err)
	}
}

func TestShouldInterceptPinnedFAQ(t *testing.T) {
	if shouldInterceptPinnedFAQ(routeReasonUserEscalate) || shouldInterceptPinnedFAQ(routeReasonPolicyFormal) {
		t.Fatal("formal and escalate must skip intercept")
	}
	if !shouldInterceptPinnedFAQ(routeReasonAILanePending) || !shouldInterceptPinnedFAQ(routeReasonAINotEnabled) {
		t.Fatal("other routes should intercept")
	}
}

func TestFaqReplaySkipsAIRefusePayload(t *testing.T) {
	source := db.LinkAskTurn{
		HostAnswer: pgtype.Text{String: "暂不公开", Valid: true},
		AiPayload:  []byte(`{"answer":"","refused":true,"resultStatus":"refused","hits":[]}`),
	}
	if pinnedFAQAnswer(source) != "暂不公开" {
		t.Fatal("host answer must win")
	}

	refusedOnly := db.LinkAskTurn{
		AiPayload: []byte(`{"answer":"cannot share","refused":true,"resultStatus":"refused","hits":[]}`),
	}
	if pinnedFAQAnswer(refusedOnly) != "" {
		t.Fatal("refused AI payload must not become an FAQ answer")
	}
}

func TestMatchesOwnerAskInboxFilterExcludesFAQReplay(t *testing.T) {
	replay := OwnerAskTurn{
		PublicAskTurn: PublicAskTurn{
			Lane:        askLaneAI,
			Status:      askStatusAIAnswered,
			RouteReason: routeReasonPinnedFAQ,
		},
	}
	if matchesOwnerAskInboxFilter(replay, askLaneAI, askStatusAIAnswered) {
		t.Fatal("ai_handled must hide FAQ replay")
	}
	if matchesOwnerAskInboxFilter(replay, askLaneHost, askStatusHostPending) {
		t.Fatal("needs_host must hide FAQ replay")
	}
	if !matchesOwnerAskInboxFilter(replay, "", "") {
		t.Fatal("all view should include FAQ replay")
	}
}
