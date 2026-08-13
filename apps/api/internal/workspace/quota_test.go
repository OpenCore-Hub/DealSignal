package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/billing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAssertCanCreateRoomFreeCap(t *testing.T) {
	wsID := uuid.NewString()
	svc := NewService(db.New(&fakeDB{
		t:          t,
		roomsCount: 1,
		billing:    db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanCreateRoom(context.Background(), wsID); !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("expected ErrLimitRooms, got %v", err)
	}

	svc = NewService(db.New(&fakeDB{
		t:          t,
		roomsCount: 0,
		billing:    db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanCreateRoom(context.Background(), wsID); err != nil {
		t.Fatalf("first free room: %v", err)
	}
}

func TestAssertCanCreateLinkFreeCap(t *testing.T) {
	wsID := uuid.NewString()
	svc := NewService(db.New(&fakeDB{
		t:          t,
		linksCount: 20,
		billing:    db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanCreateLink(context.Background(), wsID); !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("expected ErrLimitLinks, got %v", err)
	}

	svc = NewService(db.New(&fakeDB{
		t:          t,
		linksCount: 19,
		billing:    db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanCreateLink(context.Background(), wsID); err != nil {
		t.Fatalf("20th free link: %v", err)
	}
}

func TestAssertCanAddStorageFreeCap(t *testing.T) {
	wsID := uuid.NewString()
	svc := NewService(db.New(&fakeDB{
		t:            t,
		storageUsage: 2 << 30,
		billing:      db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanAddStorage(context.Background(), wsID, 1); !errors.Is(err, plan.ErrLimitStorage) {
		t.Fatalf("expected ErrLimitStorage, got %v", err)
	}
	if err := svc.AssertCanAddStorage(context.Background(), wsID, 0); err != nil {
		t.Fatalf("zero additional must grandfather: %v", err)
	}

	svc = NewService(db.New(&fakeDB{
		t:            t,
		storageUsage: (2 << 30) - 10,
		billing:      db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanAddStorage(context.Background(), wsID, 10); err != nil {
		t.Fatalf("exact fill: %v", err)
	}
}

func TestAssertQuotaMissingRowIsFree(t *testing.T) {
	wsID := uuid.NewString()
	// Missing billing fail-closes as free (1 room / 20 links / 2GiB).
	svc := NewService(db.New(&fakeDB{t: t, roomsCount: 5, linksCount: 50, storageUsage: 1 << 30}))
	if err := svc.AssertCanCreateRoom(context.Background(), wsID); !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("missing row must use free room cap, got %v", err)
	}
	if err := svc.AssertCanCreateLink(context.Background(), wsID); !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("missing row must use free link cap, got %v", err)
	}
	under := NewService(db.New(&fakeDB{t: t, roomsCount: 0, linksCount: 0, storageUsage: 0}))
	if err := under.AssertCanCreateRoom(context.Background(), wsID); err != nil {
		t.Fatalf("first free room on missing row: %v", err)
	}
}

func TestAssertQuotaFailClosedOnLookupError(t *testing.T) {
	wsID := uuid.NewString()
	lookupErr := errors.New("db unavailable")
	svc := NewService(db.New(&fakeDB{t: t, billingLookupErr: lookupErr}))
	if err := svc.AssertCanCreateRoom(context.Background(), wsID); !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}
}

func TestAssertCanAddInternalSeatFreeCap(t *testing.T) {
	wsID := uuid.NewString()
	svc := NewService(db.New(&fakeDB{
		t:          t,
		seatsCount: 1,
		billing:    db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanAddInternalSeat(context.Background(), wsID); !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats, got %v", err)
	}

	svc = NewService(db.New(&fakeDB{
		t:          t,
		seatsCount: 0,
		billing:    db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanAddInternalSeat(context.Background(), wsID); err != nil {
		t.Fatalf("empty free seats: %v", err)
	}
}

func TestAssertReservedInternalSeatAcceptable(t *testing.T) {
	wsID := uuid.NewString()
	atCapQ := db.New(&fakeDB{
		t:          t,
		seatsCount: 3,
		billing:    db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	})
	atCap := NewService(atCapQ)
	// At cap: pending reservation already counted → accept allowed (net-zero).
	if err := atCap.assertReservedInternalSeatAcceptable(context.Background(), atCapQ, wsID, RoleMember); err != nil {
		t.Fatalf("at-cap reserved accept: %v", err)
	}
	if err := atCap.assertReservedInternalSeatAcceptable(context.Background(), atCapQ, wsID, RoleGuest); err != nil {
		t.Fatalf("guest accept: %v", err)
	}

	overQ := db.New(&fakeDB{
		t:          t,
		seatsCount: 2,
		billing:    db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	})
	over := NewService(overQ)
	if err := over.assertReservedInternalSeatAcceptable(context.Background(), overQ, wsID, RoleMember); !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("oversubscribed accept: got %v", err)
	}
}

func TestCreateInvitationRespectsSeatCap(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:           t,
		memberRole:  RoleOwner,
		actorUserID: actorID,
		seatsCount:  1,
		billing:     db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	_, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "member@example.test", RoleMember, 7)
	if !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats for member invite, got %v", err)
	}
	_, err = svc.CreateInvitation(context.Background(), actorID, wsID, "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("guest invite must not consume seats: %v", err)
	}
}

func TestAddMemberRespectsSeatCap(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:           t,
		memberRole:  RoleOwner,
		actorUserID: actorID,
		seatsCount:  1,
		billing:     db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	_, err := svc.AddMember(context.Background(), actorID, wsID, "", uuid.NewString(), RoleMember)
	if !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats, got %v", err)
	}
	_, err = svc.AddMember(context.Background(), actorID, wsID, "", uuid.NewString(), RoleGuest)
	if err != nil {
		t.Fatalf("guest add must be allowed: %v", err)
	}
}

func TestUpdateMemberRoleRespectsSeatCap(t *testing.T) {
	actorID := uuid.NewString()
	targetID := uuid.NewString()
	fake := &fakeDB{
		t:            t,
		memberRole:   RoleOwner,
		actorUserID:  actorID,
		targetUserID: targetID,
		targetRole:   RoleGuest,
		seatsCount:   1,
		billing:      db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}
	svc := NewService(db.New(fake))
	_, err := svc.UpdateMemberRole(context.Background(), actorID, uuid.NewString(), "", targetID, RoleMember)
	if !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats promoting guest, got %v", err)
	}
}

func TestUpdateInvitationRoleRespectsSeatCap(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:           t,
		memberRole:  RoleOwner,
		actorUserID: actorID,
		seatsCount:  1,
		billing:     db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	inv, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("guest invite: %v", err)
	}
	_, err = svc.UpdateInvitationRole(context.Background(), actorID, wsID, "", inv.Token, RoleMember)
	if !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats promoting guest invite, got %v", err)
	}
	// Demote-style no-op (already guest) and guest→guest must not consume seats.
	updated, err := svc.UpdateInvitationRole(context.Background(), actorID, wsID, "", inv.Token, RoleGuest)
	if err != nil {
		t.Fatalf("guest→guest invite role update: %v", err)
	}
	if updated.Role != RoleGuest {
		t.Fatalf("role=%q want guest", updated.Role)
	}
}

func TestAssertCanUseCustomDomain(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := free.AssertCanUseCustomDomain(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("expected ErrFeatureCustomDomain, got %v", err)
	}

	trial := NewService(db.New(&fakeDB{t: t, billing: activeTrialBilling()}))
	if err := trial.AssertCanUseCustomDomain(context.Background(), wsID); err != nil {
		t.Fatalf("trial custom domain: %v", err)
	}

	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	if err := pro.AssertCanUseCustomDomain(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("pro custom domain must be Business+, got %v", err)
	}

	business := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "business", Period: "monthly"},
	}))
	if err := business.AssertCanUseCustomDomain(context.Background(), wsID); err != nil {
		t.Fatalf("business custom domain: %v", err)
	}
}

func TestAssertCanUseNDA(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := free.AssertCanUseNDA(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("expected ErrFeatureNDA, got %v", err)
	}

	trial := NewService(db.New(&fakeDB{t: t, billing: activeTrialBilling()}))
	if err := trial.AssertCanUseNDA(context.Background(), wsID); err != nil {
		t.Fatalf("trial NDA: %v", err)
	}

	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	if err := pro.AssertCanUseNDA(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("pro NDA must be Business+, got %v", err)
	}

	business := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "business", Period: "monthly"},
	}))
	if err := business.AssertCanUseNDA(context.Background(), wsID); err != nil {
		t.Fatalf("business NDA: %v", err)
	}
}

func TestAssertCanUseVisitorAskAI(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := free.AssertCanUseVisitorAskAI(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureVisitorAskAI) {
		t.Fatalf("expected ErrFeatureVisitorAskAI, got %v", err)
	}

	trial := NewService(db.New(&fakeDB{t: t, billing: activeTrialBilling()}))
	if err := trial.AssertCanUseVisitorAskAI(context.Background(), wsID); err != nil {
		t.Fatalf("trial visitor ask AI: %v", err)
	}
}

func TestResolveBillingExpiredTrialUsesFreeCaps(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	active := resolveBilling(db.WorkspaceBilling{
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true},
	}, now)
	trialLimits := plan.Lookup(plan.CodeTrial)
	if active.trialExpired || active.limits.Rooms != trialLimits.Rooms ||
		active.limits.StorageBytes != trialLimits.StorageBytes || !active.limits.CustomDomain {
		t.Fatalf("active trial unexpected %+v limits=%+v", active, active.limits)
	}

	expired := resolveBilling(db.WorkspaceBilling{
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
	}, now)
	if !expired.trialExpired || expired.storedPlan != plan.CodeTrial {
		t.Fatalf("expired trial metadata %+v", expired)
	}
	free := plan.Lookup(plan.CodeFree)
	if expired.limits.Rooms != free.Rooms || expired.limits.Links != free.Links || expired.limits.CustomDomain ||
		expired.limits.FormalAsk {
		t.Fatalf("expired must use free caps, got %+v", expired.limits)
	}
	if !active.limits.FormalAsk {
		t.Fatal("active trial must include Formal Ask")
	}

	// Exact boundary: now == ends_at → expired.
	atEnd := resolveBilling(db.WorkspaceBilling{
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: now, Valid: true},
	}, now)
	if !atEnd.trialExpired {
		t.Fatal("trial_ends_at == now must be expired")
	}

	// Missing ends_at on trial → expired (free caps).
	noEnd := resolveBilling(db.WorkspaceBilling{
		PlanCode: plan.CodeTrial,
		Period:   plan.PeriodMonthly,
	}, now)
	if !noEnd.trialExpired || noEnd.limits.Rooms != free.Rooms {
		t.Fatalf("trial without ends_at must expire to free caps: %+v", noEnd)
	}
}

func TestResolveBillingPastDueGrace(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pro := plan.Lookup(plan.CodePro)
	free := plan.Lookup(plan.CodeFree)

	within := resolveBilling(db.WorkspaceBilling{
		PlanCode:             plan.CodePro,
		Period:               plan.PeriodMonthly,
		StripeSubscriptionID: pgtype.Text{String: "sub_1", Valid: true},
		BillingStatus:        pgtype.Text{String: billing.StatusPastDue, Valid: true},
		PastDueAt:            pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true},
	}, now)
	if within.limits.Rooms != pro.Rooms || !within.hasStripeSubscription {
		t.Fatalf("past_due within grace must keep pro: %+v", within)
	}

	lapsed := resolveBilling(db.WorkspaceBilling{
		PlanCode:             plan.CodePro,
		Period:               plan.PeriodMonthly,
		StripeSubscriptionID: pgtype.Text{String: "sub_1", Valid: true},
		BillingStatus:        pgtype.Text{String: billing.StatusPastDue, Valid: true},
		PastDueAt:            pgtype.Timestamptz{Time: now.Add(-73 * time.Hour), Valid: true},
	}, now)
	if lapsed.storedPlan != plan.CodePro || lapsed.limits.Rooms != free.Rooms {
		t.Fatalf("lapsed past_due must keep stored pro and free caps: %+v", lapsed)
	}

	canceled := resolveBilling(db.WorkspaceBilling{
		PlanCode:      plan.CodePro,
		Period:        plan.PeriodMonthly,
		BillingStatus: pgtype.Text{String: billing.StatusCanceled, Valid: true},
	}, now)
	if canceled.limits.Rooms != free.Rooms {
		t.Fatalf("canceled must use free caps: %+v", canceled)
	}
}

func TestAssertExpiredTrialEnforcesFreeRoomCap(t *testing.T) {
	wsID := uuid.NewString()
	svc := NewService(db.New(&fakeDB{
		t:          t,
		roomsCount: 1,
		billing: db.WorkspaceBilling{
			PlanCode:    plan.CodeTrial,
			Period:      plan.PeriodMonthly,
			TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
		},
	}))
	if err := svc.AssertCanCreateRoom(context.Background(), wsID); !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("expected ErrLimitRooms after trial expiry, got %v", err)
	}
	if err := svc.AssertCanUseCustomDomain(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("expected ErrFeatureCustomDomain after trial expiry, got %v", err)
	}
}

func TestPutViewerDomainRespectsPlan(t *testing.T) {
	wsID := uuid.NewString()
	fake := &fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}
	svc := NewService(db.New(fake), WithViewerDomain("cname.dealsignal.test"))
	_, err := svc.PutViewerDomain(context.Background(), wsID, "brand.example.com")
	if !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("expected ErrFeatureCustomDomain, got %v", err)
	}

	// Grandfather: same hostname already registered returns without re-checking feature.
	fake.viewerDomain = db.WorkspaceViewerDomain{
		WorkspaceID: pgUUIDFromString(wsID),
		Hostname:    "brand.example.com",
		Status:      "pending",
		CnameTarget: "cname.dealsignal.test",
	}
	got, err := svc.PutViewerDomain(context.Background(), wsID, "brand.example.com")
	if err != nil {
		t.Fatalf("idempotent put: %v", err)
	}
	if got.Hostname != "brand.example.com" {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestUpdateSecurityWatermarkPlanGate(t *testing.T) {
	wsID := uuid.NewString()
	fake := &fakeDB{
		t: t,
		workspace: db.Workspace{
			ID:                 pgUUIDFromString(wsID),
			Name:               "WS",
			Slug:               "ws",
			WatermarkDownloads: false,
		},
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}
	svc := NewService(db.New(fake))
	_, err := svc.UpdateSecurity(context.Background(), wsID, SecuritySettings{WatermarkDownloads: true})
	if !errors.Is(err, plan.ErrFeatureWatermark) {
		t.Fatalf("expected ErrFeatureWatermark, got %v", err)
	}

	// Grandfather: already enabled can stay on / be re-saved on free.
	fake.workspace.WatermarkDownloads = true
	got, err := svc.UpdateSecurity(context.Background(), wsID, SecuritySettings{WatermarkDownloads: true})
	if err != nil {
		t.Fatalf("grandfather keep-on: %v", err)
	}
	if !got.WatermarkDownloads {
		t.Fatal("expected watermark to remain on")
	}
	got, err = svc.UpdateSecurity(context.Background(), wsID, SecuritySettings{WatermarkDownloads: false})
	if err != nil {
		t.Fatalf("disable always allowed: %v", err)
	}
	if got.WatermarkDownloads {
		t.Fatal("expected watermark off")
	}

	trial := NewService(db.New(&fakeDB{
		t: t,
		workspace: db.Workspace{
			ID:                 pgUUIDFromString(wsID),
			Name:               "WS",
			Slug:               "ws",
			WatermarkDownloads: false,
		},
		billing: activeTrialBilling(),
	}))
	got, err = trial.UpdateSecurity(context.Background(), wsID, SecuritySettings{WatermarkDownloads: true})
	if err != nil {
		t.Fatalf("trial enable watermark: %v", err)
	}
	if !got.WatermarkDownloads {
		t.Fatal("expected watermark on for trial")
	}
}

func TestAssertCanCreateDocumentFreeCap(t *testing.T) {
	wsID := uuid.NewString()
	svc := NewService(db.New(&fakeDB{
		t:         t,
		docsCount: 50,
		billing:   db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanCreateDocument(context.Background(), wsID); !errors.Is(err, plan.ErrLimitDocuments) {
		t.Fatalf("expected ErrLimitDocuments, got %v", err)
	}

	svc = NewService(db.New(&fakeDB{
		t:         t,
		docsCount: 49,
		billing:   db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanCreateDocument(context.Background(), wsID); err != nil {
		t.Fatalf("50th free document: %v", err)
	}
}

func TestAssertCanUploadFileFreeCap(t *testing.T) {
	wsID := uuid.NewString()
	svc := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := svc.AssertCanUploadFile(context.Background(), wsID, 25<<20+1); !errors.Is(err, plan.ErrLimitUpload) {
		t.Fatalf("expected ErrLimitUpload, got %v", err)
	}
	if err := svc.AssertCanUploadFile(context.Background(), wsID, 25<<20); err != nil {
		t.Fatalf("exact free upload: %v", err)
	}
}

func TestAssertCanUseBranding(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	if err := free.AssertCanUseBranding(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureBranding) {
		t.Fatalf("expected ErrFeatureBranding, got %v", err)
	}

	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	if err := pro.AssertCanUseBranding(context.Background(), wsID); err != nil {
		t.Fatalf("pro branding: %v", err)
	}
}

func TestAssertCanUseAccessControls(t *testing.T) {
	wsID := uuid.NewString()
	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	if err := pro.AssertCanUseAccessControls(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureAccessControl) {
		t.Fatalf("pro access controls must be Business+, got %v", err)
	}

	business := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "business", Period: "monthly"},
	}))
	if err := business.AssertCanUseAccessControls(context.Background(), wsID); err != nil {
		t.Fatalf("business access controls: %v", err)
	}
}

func TestAssertCanUseIntegrationFeatures(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	business := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "business", Period: "monthly"},
	}))

	if err := free.AssertCanUseWebhooks(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureWebhooks) {
		t.Fatalf("free webhooks: %v", err)
	}
	if err := free.AssertCanUseDailyDigest(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureDailyDigest) {
		t.Fatalf("free digest: %v", err)
	}
	if err := free.AssertCanUseSlackAlerts(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureSlackAlerts) {
		t.Fatalf("free slack alerts: %v", err)
	}
	if err := pro.AssertCanUseHubSpot(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureHubSpot) {
		t.Fatalf("pro hubspot: %v", err)
	}
	if err := pro.AssertCanUseSlackAlerts(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureSlackAlerts) {
		t.Fatalf("pro slack alerts: %v", err)
	}
	if err := pro.AssertCanUseRoomAnalytics(context.Background(), wsID); err != nil {
		t.Fatalf("pro room analytics: %v", err)
	}
	if err := pro.AssertCanUseRoomInsights(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureRoomInsights) {
		t.Fatalf("pro insights: %v", err)
	}
	if err := business.AssertCanUseWebhooks(context.Background(), wsID); err != nil {
		t.Fatalf("business webhooks: %v", err)
	}
	if err := business.AssertCanUseHubSpot(context.Background(), wsID); err != nil {
		t.Fatalf("business hubspot: %v", err)
	}
	if err := business.AssertCanUseDailyDigest(context.Background(), wsID); err != nil {
		t.Fatalf("business digest: %v", err)
	}
	if err := business.AssertCanUseSlackAlerts(context.Background(), wsID); err != nil {
		t.Fatalf("business slack alerts: %v", err)
	}
	if err := business.AssertCanUseRoomInsights(context.Background(), wsID); err != nil {
		t.Fatalf("business insights: %v", err)
	}
}

func TestAssertCanUseFormalAsk(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	business := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "business", Period: "monthly"},
	}))
	enterprise := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "enterprise", Period: "monthly"},
	}))
	trial := NewService(db.New(&fakeDB{t: t, billing: activeTrialBilling()}))
	expired := NewService(db.New(&fakeDB{
		t: t,
		billing: db.WorkspaceBilling{
			PlanCode:    plan.CodeTrial,
			Period:      plan.PeriodMonthly,
			TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
		},
	}))

	if err := free.AssertCanUseFormalAsk(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("free formal: %v", err)
	}
	if err := pro.AssertCanUseFormalAsk(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("pro formal: %v", err)
	}
	if err := business.AssertCanUseFormalAsk(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("business formal: %v", err)
	}
	if err := trial.AssertCanUseFormalAsk(context.Background(), wsID); err != nil {
		t.Fatalf("trial formal: %v", err)
	}
	if err := enterprise.AssertCanUseFormalAsk(context.Background(), wsID); err != nil {
		t.Fatalf("enterprise formal: %v", err)
	}
	if err := expired.AssertCanUseFormalAsk(context.Background(), wsID); !errors.Is(err, plan.ErrFeatureFormalAsk) {
		t.Fatalf("expired trial formal: %v", err)
	}
}

func TestAskAIMonthlyLimit(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	got, err := free.AskAIMonthlyLimit(context.Background(), wsID)
	if err != nil || got != 0 {
		t.Fatalf("free ask limit=%d err=%v", got, err)
	}

	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	got, err = pro.AskAIMonthlyLimit(context.Background(), wsID)
	if err != nil || got != 200 {
		t.Fatalf("pro ask limit=%d err=%v want 200", got, err)
	}

	business := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "business", Period: "monthly"},
	}))
	got, err = business.AskAIMonthlyLimit(context.Background(), wsID)
	if err != nil || got != 1000 {
		t.Fatalf("business ask limit=%d err=%v want 1000", got, err)
	}
}

func TestKnowledgeAnswersMonthly(t *testing.T) {
	wsID := uuid.NewString()
	free := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "free", Period: "monthly"},
	}))
	got, included, err := free.KnowledgeAnswersQuota(context.Background(), wsID)
	if err != nil || got != 0 || included {
		t.Fatalf("free knowledge limit=%d included=%v err=%v want 0 off", got, included, err)
	}

	pro := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}))
	got, included, err = pro.KnowledgeAnswersQuota(context.Background(), wsID)
	if err != nil || got != 100 || !included {
		t.Fatalf("pro knowledge limit=%d included=%v err=%v want 100", got, included, err)
	}

	expired := NewService(db.New(&fakeDB{
		t: t,
		billing: db.WorkspaceBilling{
			PlanCode:    "trial",
			Period:      "monthly",
			TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
		},
	}))
	got, included, err = expired.KnowledgeAnswersQuota(context.Background(), wsID)
	if err != nil || got != 0 || included {
		t.Fatalf("expired trial knowledge limit=%d included=%v err=%v want free off", got, included, err)
	}

	ent := NewService(db.New(&fakeDB{
		t:       t,
		billing: db.WorkspaceBilling{PlanCode: "enterprise", Period: "monthly"},
	}))
	got, included, err = ent.KnowledgeAnswersQuota(context.Background(), wsID)
	if err != nil || got != 0 || !included {
		t.Fatalf("enterprise knowledge limit=%d included=%v err=%v want 0 unlimited", got, included, err)
	}
}

func TestOwnedWorkspaceLimit(t *testing.T) {
	now := time.Now().UTC()
	if got := ownedWorkspaceLimit(nil, now); got != 1 {
		t.Fatalf("missing billing cap=%d want 1", got)
	}
	if got := ownedWorkspaceLimit([]db.WorkspaceBilling{{
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
	}}, now); got != 1 {
		t.Fatalf("active trial cap=%d want 1", got)
	}
	if got := ownedWorkspaceLimit([]db.WorkspaceBilling{{
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
	}}, now); got != 1 {
		t.Fatalf("expired trial cap=%d want free 1", got)
	}
	if got := ownedWorkspaceLimit([]db.WorkspaceBilling{
		{PlanCode: plan.CodeFree, Period: plan.PeriodMonthly},
		{PlanCode: plan.CodePro, Period: plan.PeriodMonthly},
	}, now); got != 3 {
		t.Fatalf("best owned cap=%d want pro 3", got)
	}
	if got := ownedWorkspaceLimit([]db.WorkspaceBilling{{
		PlanCode: plan.CodeBusiness, Period: plan.PeriodMonthly,
	}}, now); got != 10 {
		t.Fatalf("business cap=%d want 10", got)
	}
	if got := ownedWorkspaceLimit([]db.WorkspaceBilling{{
		PlanCode: plan.CodeEnterprise, Period: plan.PeriodMonthly,
	}}, now); got != 0 {
		t.Fatalf("enterprise cap=%d want unlimited 0", got)
	}
}
