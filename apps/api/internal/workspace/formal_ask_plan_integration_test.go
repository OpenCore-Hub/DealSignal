//go:build integration

package workspace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type allowAllFormalAskEntitlement struct{}

func (allowAllFormalAskEntitlement) IsFormalAskEntitled(context.Context, string) (bool, error) {
	return true, nil
}

func (f *billingFixture) upsertPlan(t *testing.T, code string, trialEnds pgtype.Timestamptz) {
	t.Helper()
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    code,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: trialEnds,
	}); err != nil {
		t.Fatalf("upsert %s: %v", code, err)
	}
}

func TestBillingFormalAskAssert_Integration(t *testing.T) {
	f := newBillingFixture(t)
	_, wsID, _ := f.ids()

	if err := f.wsSvc.AssertCanUseFormalAsk(f.ctx, wsID); err != nil {
		t.Fatalf("seeded trial must allow Formal Ask: %v", err)
	}

	f.upsertPlan(t, plan.CodeFree, pgtype.Timestamptz{})
	if err := f.wsSvc.AssertCanUseFormalAsk(f.ctx, wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("free: %v", err)
	}

	f.upsertPlan(t, plan.CodePro, pgtype.Timestamptz{})
	if err := f.wsSvc.AssertCanUseFormalAsk(f.ctx, wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("pro: %v", err)
	}

	f.upsertPlan(t, plan.CodeBusiness, pgtype.Timestamptz{})
	if err := f.wsSvc.AssertCanUseFormalAsk(f.ctx, wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("business: %v", err)
	}

	f.upsertPlan(t, plan.CodeEnterprise, pgtype.Timestamptz{})
	if err := f.wsSvc.AssertCanUseFormalAsk(f.ctx, wsID); err != nil {
		t.Fatalf("enterprise: %v", err)
	}

	f.upsertPlan(t, plan.CodeTrial, pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true})
	if err := f.wsSvc.AssertCanUseFormalAsk(f.ctx, wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("expired trial: %v", err)
	}

	f.upsertPlan(t, plan.CodeTrial, pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true})
	if err := f.wsSvc.AssertCanUseFormalAsk(f.ctx, wsID); err != nil {
		t.Fatalf("reactivated trial: %v", err)
	}
}

func TestFormalAskWorkspacePlanGate_SubmitPublish_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	f.linkSvc.SetFormalAskEntitlement(allowAllFormalAskEntitlement{})

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "formal-plan-gate",
		PermissionType: "public",
		RequireEmail:   true,
	})
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE links SET qa_enabled = true, ask_mode = $1 WHERE id = $2`, link.AskModeFormal, created.ID); err != nil {
		t.Fatalf("enable formal qa: %v", err)
	}
	created.QaEnabled = true
	created.AskMode = link.AskModeFormal

	submit := func(t *testing.T) error {
		t.Helper()
		_, err := f.linkSvc.SubmitPublicAsk(
			f.ctx,
			created,
			"visitor-"+uuid.NewString(),
			"formal-plan@example.com",
			"Formal disclosure question?",
			false,
		)
		return err
	}

	if err := submit(t); err != nil {
		t.Fatalf("trial submit with Docling allow-all: %v", err)
	}

	turn, err := f.linkSvc.SubmitPublicAsk(
		f.ctx,
		created,
		"visitor-"+uuid.NewString(),
		"formal-publish@example.com",
		"Queue before free downgrade?",
		false,
	)
	if err != nil {
		t.Fatalf("queue turn on trial: %v", err)
	}
	turnUUID, err := uuid.Parse(turn.ID)
	if err != nil {
		t.Fatalf("parse turn id: %v", err)
	}

	f.upsertPlan(t, plan.CodeFree, pgtype.Timestamptz{})
	if err := submit(t); !errors.Is(err, link.ErrAskFormalNotEntitled) {
		t.Fatalf("free submit: %v", err)
	}
	_, err = f.linkSvc.PublishFormalAskTurn(
		f.ctx,
		created,
		pgtype.UUID{Bytes: turnUUID, Valid: true},
		f.user.ID,
		link.FormalPublishInput{Answer: "Must not publish on free."},
	)
	if !errors.Is(err, link.ErrAskFormalNotEntitled) {
		t.Fatalf("free publish: %v", err)
	}

	f.upsertPlan(t, plan.CodePro, pgtype.Timestamptz{})
	if err := submit(t); !errors.Is(err, link.ErrAskFormalNotEntitled) {
		t.Fatalf("pro submit: %v", err)
	}

	f.upsertPlan(t, plan.CodeBusiness, pgtype.Timestamptz{})
	if err := submit(t); !errors.Is(err, link.ErrAskFormalNotEntitled) {
		t.Fatalf("business submit: %v", err)
	}

	f.upsertPlan(t, plan.CodeEnterprise, pgtype.Timestamptz{})
	if err := submit(t); err != nil {
		t.Fatalf("enterprise submit: %v", err)
	}

	f.upsertPlan(t, plan.CodeTrial, pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true})
	if err := submit(t); !errors.Is(err, link.ErrAskFormalNotEntitled) {
		t.Fatalf("expired trial submit: %v", err)
	}
}
