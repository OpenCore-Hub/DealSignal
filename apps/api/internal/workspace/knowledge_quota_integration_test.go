//go:build integration

package workspace_test

import (
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBillingKnowledgeAnswersQuota_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	got, included, err := f.wsSvc.KnowledgeAnswersQuota(f.ctx, wsID)
	if err != nil || got != 0 || included {
		t.Fatalf("free knowledge cap=%d included=%v err=%v want 0 off", got, included, err)
	}

	kn := knowledge.NewService(f.q, config.DoclingRAGConfig{}, nil, nil, "test-secret").
		WithAnswersPlanLimiter(f.wsSvc)
	if !errors.Is(kn.CheckAnswersQuota(f.ctx, wsID), knowledge.ErrQueryQuotaExceeded) {
		t.Fatal("free desk must be denied at 0")
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "knowledge-quota-" + uuid.NewString()[:8],
		Name: "Knowledge Quota Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	session, err := f.q.CreateKnowledgeQASession(f.ctx, db.CreateKnowledgeQASessionParams{
		WorkspaceID: f.workspace.ID,
		RoomID:      room.ID,
		CreatedBy:   f.user.ID,
		Title:       pgtype.Text{String: "quota", Valid: true},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := f.q.CreateKnowledgeQATurn(f.ctx, db.CreateKnowledgeQATurnParams{
		SessionID:      session.ID,
		RoomID:         room.ID,
		WorkspaceID:    f.workspace.ID,
		Sequence:       1,
		Question:       "q",
		Refused:        false,
		ResultStatus:   "answered",
		Hits:           []byte("[]"),
		CreatedBy:      f.user.ID,
		RewriteApplied: false,
		DurationMs:     1,
	}); err != nil {
		t.Fatalf("seed turn: %v", err)
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	got, included, err = f.wsSvc.KnowledgeAnswersQuota(f.ctx, wsID)
	if err != nil || got != 100 || !included {
		t.Fatalf("pro knowledge cap=%d included=%v err=%v want 100", got, included, err)
	}
	if err := kn.CheckAnswersQuota(f.ctx, wsID); err != nil {
		t.Fatalf("pro cap 100 must allow 1 turn: %v", err)
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(14 * 24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("upsert trial: %v", err)
	}
	got, included, err = f.wsSvc.KnowledgeAnswersQuota(f.ctx, wsID)
	if err != nil || got != 200 || !included {
		t.Fatalf("trial knowledge cap=%d included=%v err=%v want 200", got, included, err)
	}
	if err := kn.CheckAnswersQuota(f.ctx, wsID); err != nil {
		t.Fatalf("trial cap 200 must allow 1 turn: %v", err)
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeEnterprise,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert enterprise: %v", err)
	}
	got, included, err = f.wsSvc.KnowledgeAnswersQuota(f.ctx, wsID)
	if err != nil || got != 0 || !included {
		t.Fatalf("enterprise knowledge cap=%d included=%v err=%v want 0 unlimited", got, included, err)
	}
	if err := kn.CheckAnswersQuota(f.ctx, wsID); err != nil {
		t.Fatalf("enterprise unlimited: %v", err)
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("upsert expired trial: %v", err)
	}
	got, included, err = f.wsSvc.KnowledgeAnswersQuota(f.ctx, wsID)
	if err != nil || got != 0 || included {
		t.Fatalf("expired trial knowledge cap=%d included=%v err=%v want free off", got, included, err)
	}
	if !errors.Is(kn.CheckAnswersQuota(f.ctx, wsID), knowledge.ErrQueryQuotaExceeded) {
		t.Fatal("expired trial must use free (desk off)")
	}
}
