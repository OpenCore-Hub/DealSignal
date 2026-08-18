package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMapPublicAskFAQ(t *testing.T) {
	turnID := uuid.New()
	pinnedAt := time.Now().UTC()
	aiPayload := []byte(`{"answer":"AI reply","refused":false,"resultStatus":"answered","hits":[]}`)

	t.Run("host answered pinned turn", func(t *testing.T) {
		got, ok := mapPublicAskFAQ(db.LinkAskTurn{
			ID:          pgtype.UUID{Bytes: turnID, Valid: true},
			Question:    "What is ARR?",
			Lane:        askLaneHost,
			HostAnswer:  pgtype.Text{String: "Host reply", Valid: true},
			PinnedFaqAt: pgtype.Timestamptz{Time: pinnedAt, Valid: true},
		})
		if !ok {
			t.Fatal("expected mappable FAQ")
		}
		if got.Answer != "Host reply" || got.Source != askLaneHost {
			t.Fatalf("unexpected FAQ: %+v", got)
		}
	})

	t.Run("ai answered pinned turn", func(t *testing.T) {
		got, ok := mapPublicAskFAQ(db.LinkAskTurn{
			ID:          pgtype.UUID{Bytes: turnID, Valid: true},
			Question:    "What is ARR?",
			Lane:        askLaneAI,
			AiPayload:   aiPayload,
			PinnedFaqAt: pgtype.Timestamptz{Time: pinnedAt, Valid: true},
		})
		if !ok {
			t.Fatal("expected mappable FAQ")
		}
		if got.Answer != "AI reply" || got.AIPayload == nil {
			t.Fatalf("unexpected FAQ: %+v", got)
		}
	})

	t.Run("unpinned turn skipped", func(t *testing.T) {
		_, ok := mapPublicAskFAQ(db.LinkAskTurn{
			ID:       pgtype.UUID{Bytes: turnID, Valid: true},
			Question: "orphan",
		})
		if ok {
			t.Fatal("expected skip")
		}
	})

	t.Run("hybrid prefers host answer", func(t *testing.T) {
		got, ok := mapPublicAskFAQ(db.LinkAskTurn{
			ID:          pgtype.UUID{Bytes: turnID, Valid: true},
			Question:    "Hybrid Q",
			Lane:        askLaneHybrid,
			HostAnswer:  pgtype.Text{String: "Host wins", Valid: true},
			AiPayload:   aiPayload,
			PinnedFaqAt: pgtype.Timestamptz{Time: pinnedAt, Valid: true},
		})
		if !ok || got.Answer != "Host wins" {
			t.Fatalf("unexpected FAQ: %+v ok=%v", got, ok)
		}
	})

	t.Run("hybrid refuse payload is not exposed", func(t *testing.T) {
		got, ok := mapPublicAskFAQ(db.LinkAskTurn{
			ID:          pgtype.UUID{Bytes: turnID, Valid: true},
			Question:    "Hybrid refuse",
			Lane:        askLaneHybrid,
			HostAnswer:  pgtype.Text{String: "暂不公开", Valid: true},
			AiPayload:   []byte(`{"answer":"cannot share","refused":true,"resultStatus":"refused","hits":[]}`),
			PinnedFaqAt: pgtype.Timestamptz{Time: pinnedAt, Valid: true},
		})
		if !ok || got.Answer != "暂不公开" {
			t.Fatalf("unexpected FAQ: %+v ok=%v", got, ok)
		}
		if got.AIPayload != nil {
			t.Fatalf("refused payload leaked: %+v", got.AIPayload)
		}
	})

	t.Run("refused AI-only skipped", func(t *testing.T) {
		_, ok := mapPublicAskFAQ(db.LinkAskTurn{
			ID:          pgtype.UUID{Bytes: turnID, Valid: true},
			Question:    "Refused",
			Lane:        askLaneAI,
			AiPayload:   []byte(`{"answer":"cannot share","refused":true,"resultStatus":"refused","hits":[]}`),
			PinnedFaqAt: pgtype.Timestamptz{Time: pinnedAt, Valid: true},
		})
		if ok {
			t.Fatal("expected skip")
		}
	})

	t.Run("includes link metadata", func(t *testing.T) {
		linkID := uuid.New()
		got, ok := mapPublicAskFAQWithMeta(db.LinkAskTurn{
			ID:          pgtype.UUID{Bytes: turnID, Valid: true},
			LinkID:      pgtype.UUID{Bytes: linkID, Valid: true},
			Question:    "Q",
			Lane:        askLaneAI,
			AiPayload:   aiPayload,
			PinnedFaqAt: pgtype.Timestamptz{Time: pinnedAt, Valid: true},
		}, pgtype.UUID{Bytes: linkID, Valid: true}, "Investor deck")
		if !ok || got.LinkID != linkID.String() || got.LinkName != "Investor deck" {
			t.Fatalf("unexpected FAQ: %+v", got)
		}
	})
}

func TestPinnedFAQAnswerPrefersHost(t *testing.T) {
	answer := pinnedFAQAnswer(db.LinkAskTurn{
		HostAnswer: pgtype.Text{String: "official", Valid: true},
		AiPayload:  []byte(`{"answer":"draft","refused":false,"resultStatus":"answered","hits":[]}`),
	})
	if answer != "official" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestPinnedFAQAnswerSkipsRefusedAI(t *testing.T) {
	answer := pinnedFAQAnswer(db.LinkAskTurn{
		AiPayload: []byte(`{"answer":"cannot share","refused":true,"resultStatus":"refused","hits":[]}`),
	})
	if answer != "" {
		t.Fatalf("answer = %q", answer)
	}
}
