package server

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
)

func TestDoclingFormalAskEntitlementStubWhenClientDisabled(t *testing.T) {
	e := doclingFormalAskEntitlement{
		client:    docling.NewClient("", "", 0),
		planCodes: formalPlanCodeSet([]string{"enterprise", "trial"}),
		stubPlan:  "trial",
		appEnv:    "development",
	}
	ok, err := e.IsFormalAskEntitled(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatal("expected stub trial to be entitled when docling disabled")
	}
}

func TestDoclingFormalAskEntitlementStubRejectedInProduction(t *testing.T) {
	e := doclingFormalAskEntitlement{
		client:    docling.NewClient("", "", 0),
		planCodes: formalPlanCodeSet([]string{"trial"}),
		stubPlan:  "trial",
		appEnv:    "production",
	}
	ok, err := e.IsFormalAskEntitled(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Fatal("stub must not entitle in production")
	}
}
