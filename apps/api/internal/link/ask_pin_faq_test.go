package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMatchesOwnerAskInboxFilter(t *testing.T) {
	now := time.Now().UTC()
	base := OwnerAskTurn{
		PublicAskTurn: PublicAskTurn{CreatedAt: now, UpdatedAt: now},
		LinkID:        "link-1",
	}

	cases := []struct {
		name   string
		turn   OwnerAskTurn
		lane   string
		status string
		want   bool
	}{
		{
			name: "needs_host includes hybrid pending",
			turn: OwnerAskTurn{
				PublicAskTurn: PublicAskTurn{
					Lane:   askLaneHybrid,
					Status: askStatusHostPending,
				},
			},
			lane:   askLaneHost,
			status: askStatusHostPending,
			want:   true,
		},
		{
			name: "needs_host excludes pure ai answered",
			turn: OwnerAskTurn{
				PublicAskTurn: PublicAskTurn{
					Lane:   askLaneAI,
					Status: askStatusAIAnswered,
				},
			},
			lane:   askLaneHost,
			status: askStatusHostPending,
			want:   false,
		},
		{
			name: "ai_handled includes pure ai answered",
			turn: OwnerAskTurn{
				PublicAskTurn: PublicAskTurn{
					Lane:   askLaneAI,
					Status: askStatusAIAnswered,
				},
			},
			lane:   askLaneAI,
			status: askStatusAIAnswered,
			want:   true,
		},
		{
			name: "ai_handled excludes hybrid pending",
			turn: OwnerAskTurn{
				PublicAskTurn: PublicAskTurn{
					Lane:   askLaneHybrid,
					Status: askStatusHostPending,
				},
			},
			lane:   askLaneAI,
			status: askStatusAIAnswered,
			want:   false,
		},
		{
			name: "exact host pending",
			turn: OwnerAskTurn{
				PublicAskTurn: PublicAskTurn{
					Lane:   askLaneHost,
					Status: askStatusHostPending,
				},
			},
			lane:   askLaneHost,
			status: askStatusHostPending,
			want:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turn := base
			turn.PublicAskTurn = tc.turn.PublicAskTurn
			if got := matchesOwnerAskInboxFilter(turn, tc.lane, tc.status); got != tc.want {
				t.Fatalf("matchesOwnerAskInboxFilter() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAskTurnPinFAQEligible(t *testing.T) {
	aiPayload := []byte(`{"answer":"yes","refused":false,"resultStatus":"answered","hits":[]}`)

	cases := []struct {
		name   string
		status string
		answer string
		ai     []byte
		want   bool
	}{
		{name: "ai answered with payload", status: askStatusAIAnswered, ai: aiPayload, want: true},
		{name: "host answered with text", status: askStatusHostAnswered, answer: "host reply", want: true},
		{name: "host pending", status: askStatusHostPending, want: false},
		{name: "ai refused", status: askStatusAIRefused, ai: aiPayload, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turn := db.LinkAskTurn{
				ID:     pgtype.UUID{Bytes: uuid.New(), Valid: true},
				Status: tc.status,
				AiPayload: tc.ai,
			}
			if tc.answer != "" {
				turn.HostAnswer = pgtype.Text{String: tc.answer, Valid: true}
			}
			if got := askTurnPinFAQEligible(turn); got != tc.want {
				t.Fatalf("askTurnPinFAQEligible() = %v, want %v", got, tc.want)
			}
		})
	}
}
