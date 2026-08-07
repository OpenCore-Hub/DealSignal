package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMatchesOwnerAskInboxFilter_FormalQueue(t *testing.T) {
	formalPending := OwnerAskTurn{
		PublicAskTurn: PublicAskTurn{
			Lane:         askLaneHost,
			Status:       askStatusHostPending,
			RouteReason:  routeReasonPolicyFormal,
			FormalStatus: formalStatusPendingReview,
		},
	}

	if !matchesOwnerAskInboxFilter(formalPending, "", ownerAskInboxFormalQueue) {
		t.Fatal("expected formal pending in formal_queue tab")
	}
	if matchesOwnerAskInboxFilter(formalPending, askLaneHost, askStatusHostPending) {
		t.Fatal("formal pending must not appear in needs_host tab")
	}
}

func TestMapPublicFormalAsk(t *testing.T) {
	now := time.Now().UTC()
	turn := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	row := dbLinkAskTurnFormalPublished(turn, now, "What is revenue?", " $12M ")
	entry, ok := mapPublicFormalAsk(row, pgtype.UUID{}, "")
	if !ok {
		t.Fatal("expected published formal ask")
	}
	if entry.Answer != "$12M" {
		t.Fatalf("answer = %q", entry.Answer)
	}
	if entry.Question != "What is revenue?" {
		t.Fatalf("question = %q", entry.Question)
	}
}

func TestApplyFormalVisitorMask(t *testing.T) {
	out := PublicAskTurn{HostAnswer: "secret draft"}
	tTurn := db.LinkAskTurn{
		FormalStatus: pgtype.Text{String: formalStatusPendingReview, Valid: true},
		HostAnswer:   pgtype.Text{String: "secret draft", Valid: true},
	}
	applyFormalVisitorMask(&out, tTurn)
	if out.HostAnswer != "" {
		t.Fatalf("expected masked answer, got %q", out.HostAnswer)
	}
	if out.FormalStatus != formalStatusPendingReview {
		t.Fatalf("formal_status = %q", out.FormalStatus)
	}
}

func dbLinkAskTurnFormalPublished(id pgtype.UUID, publishedAt time.Time, question, answer string) db.LinkAskTurn {
	return db.LinkAskTurn{
		ID:                id,
		Question:          question,
		HostAnswer:        pgtype.Text{String: answer, Valid: true},
		FormalStatus:      pgtype.Text{String: formalStatusPublished, Valid: true},
		FormalPublishedAt: pgtype.Timestamptz{Time: publishedAt, Valid: true},
		UpdatedAt:         pgtype.Timestamptz{Time: publishedAt, Valid: true},
	}
}
