package link

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
)

type stubPlanChecker struct {
	plan.Unrestricted
	linkErr          error
	ndaErr           error
	askAIErr         error
	watermarkErr     error
	accessControlErr error
	brandingErr      error
	formalAskErr     error
}

func (s stubPlanChecker) AssertCanCreateLink(context.Context, string) error {
	return s.linkErr
}
func (s stubPlanChecker) AssertCanUseWatermark(context.Context, string) error {
	return s.watermarkErr
}
func (s stubPlanChecker) AssertCanUseNDA(context.Context, string) error { return s.ndaErr }
func (s stubPlanChecker) AssertCanUseVisitorAskAI(context.Context, string) error {
	return s.askAIErr
}
func (s stubPlanChecker) AssertCanUseBranding(context.Context, string) error {
	return s.brandingErr
}
func (s stubPlanChecker) AssertCanUseAccessControls(context.Context, string) error {
	return s.accessControlErr
}
func (s stubPlanChecker) AssertCanUseFormalAsk(context.Context, string) error {
	return s.formalAskErr
}
func (s stubPlanChecker) WithCreateLinkQuota(ctx context.Context, workspaceID string, fn func(context.Context) error) error {
	if err := s.AssertCanCreateLink(ctx, workspaceID); err != nil {
		return err
	}
	return fn(ctx)
}

func TestAssertCanEnableNDA(t *testing.T) {
	svc := &Service{}
	if err := svc.assertCanEnableNDA(context.Background(), "ws"); err != nil {
		t.Fatalf("nil checker must no-op: %v", err)
	}

	svc = &Service{planChecker: stubPlanChecker{ndaErr: plan.ErrFeatureNDA}}
	if err := svc.assertCanEnableNDA(context.Background(), "ws"); !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("expected ErrFeatureNDA, got %v", err)
	}

	svc = &Service{planChecker: stubPlanChecker{}}
	if err := svc.assertCanEnableNDA(context.Background(), "ws"); err != nil {
		t.Fatalf("allowed: %v", err)
	}
}

func TestAssertCanEnableVisitorAskAI(t *testing.T) {
	svc := &Service{}
	if err := svc.assertCanEnableVisitorAskAI(context.Background(), "ws"); err != nil {
		t.Fatalf("nil checker must no-op: %v", err)
	}

	svc = &Service{planChecker: stubPlanChecker{askAIErr: plan.ErrFeatureVisitorAskAI}}
	if err := svc.assertCanEnableVisitorAskAI(context.Background(), "ws"); !errors.Is(err, plan.ErrFeatureVisitorAskAI) {
		t.Fatalf("expected ErrFeatureVisitorAskAI, got %v", err)
	}
}

func TestAssertCanEnableWatermark(t *testing.T) {
	svc := &Service{}
	if err := svc.assertCanEnableWatermark(context.Background(), "ws"); err != nil {
		t.Fatalf("nil checker must no-op: %v", err)
	}

	svc = &Service{planChecker: stubPlanChecker{watermarkErr: plan.ErrFeatureWatermark}}
	if err := svc.assertCanEnableWatermark(context.Background(), "ws"); !errors.Is(err, plan.ErrFeatureWatermark) {
		t.Fatalf("expected ErrFeatureWatermark, got %v", err)
	}
}

func TestAssertCanEnableAccessControls(t *testing.T) {
	svc := &Service{}
	if err := svc.assertCanEnableAccessControls(context.Background(), "ws"); err != nil {
		t.Fatalf("nil checker must no-op: %v", err)
	}

	svc = &Service{planChecker: stubPlanChecker{accessControlErr: plan.ErrFeatureAccessControl}}
	if err := svc.assertCanEnableAccessControls(context.Background(), "ws"); !errors.Is(err, plan.ErrFeatureAccessControl) {
		t.Fatalf("expected ErrFeatureAccessControl, got %v", err)
	}
}

func TestAssertCanShareMultipleDocuments(t *testing.T) {
	svc := &Service{planChecker: stubPlanChecker{brandingErr: plan.ErrFeatureBranding}}
	if err := svc.assertCanShareMultipleDocuments(context.Background(), "ws", 1); err != nil {
		t.Fatalf("single document must no-op: %v", err)
	}
	if err := svc.assertCanShareMultipleDocuments(context.Background(), "ws", 2); !errors.Is(err, plan.ErrFeatureBranding) {
		t.Fatalf("expected ErrFeatureBranding, got %v", err)
	}
}
