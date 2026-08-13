package link

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAskAIQuotaExceededView(t *testing.T) {
	if askAIQuotaExceededView(AskAIQuotaView{Used: 0, Limit: 500, Included: true}) {
		t.Fatal("expected not exceeded at 0/500")
	}
	if !askAIQuotaExceededView(AskAIQuotaView{Used: 500, Limit: 500, Included: true}) {
		t.Fatal("expected exceeded at 500/500")
	}
	if askAIQuotaExceededView(AskAIQuotaView{Used: 10, Limit: 0, Included: true}) {
		t.Fatal("zero limit should not mark exceeded when included")
	}
	if !askAIQuotaExceededView(AskAIQuotaView{Used: 0, Limit: 0, Included: false}) {
		t.Fatal("feature-off must exceed even when limit is 0")
	}
	if !askAIQuotaExceededView(AskAIQuotaView{}) {
		t.Fatal("zero-value view must fail-closed (not included)")
	}
}

func TestAskAIQuotaExceededWhenVisitorAskNotIncluded(t *testing.T) {
	s := &Service{planChecker: stubPlanChecker{askAIErr: plan.ErrFeatureVisitorAskAI}}
	id := uuid.New()
	link := db.Link{WorkspaceID: pgtype.UUID{Bytes: id, Valid: true}}
	if !s.askAIQuotaExceeded(context.Background(), link) {
		t.Fatal("expected AI lane exhausted when plan does not include visitor ask")
	}
}
