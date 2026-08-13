package plan

import "testing"

func TestLookupUnknownIsFree(t *testing.T) {
	t.Parallel()
	got := Lookup("not-a-plan")
	if got.Code != CodeFree {
		t.Fatalf("Lookup unknown: got %+v", got)
	}
	if Lookup("").Code != CodeFree {
		t.Fatal("Lookup empty must fail-closed to free")
	}
}

func TestTrialMirrorsBusinessCapacity(t *testing.T) {
	t.Parallel()
	trial := Lookup(CodeTrial)
	biz := Lookup(CodeBusiness)
	if trial.StorageBytes != biz.StorageBytes {
		t.Fatalf("trial storage=%d want business %d", trial.StorageBytes, biz.StorageBytes)
	}
	if trial.Rooms != biz.Rooms || trial.Documents != biz.Documents {
		t.Fatalf("trial rooms/docs=%d/%d want business %d/%d", trial.Rooms, trial.Documents, biz.Rooms, biz.Documents)
	}
	if trial.Links != 0 {
		t.Fatalf("trial links must stay unlimited (0), got %d", trial.Links)
	}
	if trial.InternalSeats != 10 {
		t.Fatalf("trial seats=%d want 10", trial.InternalSeats)
	}
	if trial.VisitorAskAIMonthly != 1000 {
		t.Fatalf("trial ask monthly=%d want 1000", trial.VisitorAskAIMonthly)
	}
	if trial.KnowledgeAnswersMonthly != 200 {
		t.Fatalf("trial knowledge monthly=%d want 200", trial.KnowledgeAnswersMonthly)
	}
	if !trial.KnowledgeDesk {
		t.Fatal("trial must include knowledge desk")
	}
	if !trial.CustomDomain || !trial.NDA || !trial.AccessControls ||
		!trial.Webhooks || !trial.HubSpot || !trial.DailyDigest ||
		!trial.SlackAlerts || !trial.RoomInsights || !trial.RoomAnalytics {
		t.Fatal("trial must include business diligence features")
	}
	if !trial.FormalAsk {
		t.Fatal("trial must include Formal Ask for the evaluation window")
	}
	if biz.FormalAsk {
		t.Fatal("business must not include Formal Ask (enterprise + trial only)")
	}
	if OverLimit(1_000, 1, trial.Rooms) {
		t.Fatal("trial rooms must remain unlimited")
	}
	if OverLimit(1_000, 1, trial.Links) {
		t.Fatal("trial links must remain unlimited")
	}
	if trial.OwnedWorkspaces != 1 {
		t.Fatalf("trial owned workspaces=%d want 1 (farm cap, not business 10)", trial.OwnedWorkspaces)
	}
	if biz.OwnedWorkspaces != 10 {
		t.Fatalf("business owned workspaces=%d want 10", biz.OwnedWorkspaces)
	}
}

func TestFreeCaps(t *testing.T) {
	t.Parallel()
	free := Lookup(CodeFree)
	if free.Rooms != 1 || free.Links != 20 || free.StorageBytes != 2<<30 || free.Documents != 50 {
		t.Fatalf("unexpected free caps: %+v", free)
	}
	if free.OwnedWorkspaces != 1 {
		t.Fatalf("free owned workspaces=%d want 1", free.OwnedWorkspaces)
	}
	if free.MaxUploadBytes != 25<<20 {
		t.Fatalf("free max upload=%d want 25MiB", free.MaxUploadBytes)
	}
	if free.KnowledgeAnswersMonthly != 0 || free.KnowledgeDesk {
		t.Fatalf("free knowledge desk must be off: %+v", free)
	}
	if free.CustomDomain || free.Watermark || free.NDA || free.VisitorAskAI || free.Branding || free.AccessControls ||
		free.Webhooks || free.HubSpot || free.DailyDigest || free.SlackAlerts || free.RoomInsights || free.RoomAnalytics ||
		free.FormalAsk {
		t.Fatalf("free must not include paid features: %+v", free)
	}
	if !OverLimit(1, 1, free.Rooms) {
		t.Fatal("second room must be over free cap")
	}
	if OverLimit(0, 1, free.Rooms) {
		t.Fatal("first room must be allowed")
	}
	if !OverLimit(20, 1, free.Links) {
		t.Fatal("21st link must be over free cap")
	}
	if !OverLimit(50, 1, free.Documents) {
		t.Fatal("51st document must be over free cap")
	}
	if OverLimit((2<<30)-1, 1, free.StorageBytes) {
		t.Fatal("upload that fits must be allowed")
	}
	if !OverLimit(2<<30, 1, free.StorageBytes) {
		t.Fatal("upload past 2GiB must be blocked")
	}
}

func TestProVsBusinessFeatureSplit(t *testing.T) {
	t.Parallel()
	pro := Lookup(CodePro)
	if pro.CustomDomain || pro.NDA || pro.AccessControls || pro.Webhooks || pro.HubSpot ||
		pro.DailyDigest || pro.SlackAlerts || pro.RoomInsights {
		t.Fatal("pro must not include custom domain / NDA / access controls / business integrations")
	}
	if !pro.Watermark || !pro.VisitorAskAI || !pro.Branding || !pro.RoomAnalytics {
		t.Fatal("pro must include watermark, ask AI, branding, room analytics")
	}
	if pro.FormalAsk {
		t.Fatal("pro must not include Formal Ask")
	}
	if !Lookup(CodeEnterprise).FormalAsk {
		t.Fatal("enterprise must include Formal Ask")
	}
	if pro.Rooms != 5 || pro.Documents != 200 || pro.VisitorAskAIMonthly != 200 || pro.KnowledgeAnswersMonthly != 100 {
		t.Fatalf("unexpected pro caps: %+v", pro)
	}
	if pro.OwnedWorkspaces != 3 {
		t.Fatalf("pro owned workspaces=%d want 3", pro.OwnedWorkspaces)
	}
	if Lookup(CodeEnterprise).OwnedWorkspaces != 0 {
		t.Fatal("enterprise owned workspaces must be unlimited")
	}
	if !pro.KnowledgeDesk {
		t.Fatal("pro must include knowledge desk")
	}
	if !Lookup(CodeBusiness).CustomDomain || !Lookup(CodeBusiness).NDA {
		t.Fatal("business must allow custom domain and NDA")
	}
	if Lookup(CodeFree).VisitorAskAI {
		t.Fatal("free must not include visitor ask AI")
	}
	if !Lookup(CodeTrial).VisitorAskAI {
		t.Fatal("trial must allow visitor ask AI")
	}
	if Lookup(CodeEnterprise).KnowledgeAnswersMonthly != 0 || !Lookup(CodeEnterprise).KnowledgeDesk {
		t.Fatal("enterprise knowledge desk must be unlimited")
	}
	if Lookup(CodeBusiness).KnowledgeAnswersMonthly != 200 || !Lookup(CodeBusiness).KnowledgeDesk {
		t.Fatal("business knowledge monthly must be 200")
	}
}

func TestOverLimitUnlimited(t *testing.T) {
	t.Parallel()
	if OverLimit(99, 1, 0) {
		t.Fatal("limit 0 is unlimited")
	}
}

func TestOffersMatchCatalogAndExcludeTrial(t *testing.T) {
	t.Parallel()
	offers := Offers()
	if len(offers) != 4 {
		t.Fatalf("offers=%d want 4", len(offers))
	}
	wantCodes := []string{CodeFree, CodePro, CodeBusiness, CodeEnterprise}
	wantPrices := []int{0, 49, 99, 0}
	for i, code := range wantCodes {
		if offers[i].Code != code {
			t.Fatalf("offers[%d]=%q want %q", i, offers[i].Code, code)
		}
		if offers[i].PriceMonthlyUSD != wantPrices[i] {
			t.Fatalf("offer %s price=%d want %d", code, offers[i].PriceMonthlyUSD, wantPrices[i])
		}
		lim := Lookup(code)
		if offers[i].Rooms != lim.Rooms || offers[i].Links != lim.Links ||
			offers[i].StorageBytes != lim.StorageBytes || offers[i].InternalSeats != lim.InternalSeats ||
			offers[i].Documents != lim.Documents || offers[i].VisitorAskAIMonthly != lim.VisitorAskAIMonthly ||
			offers[i].KnowledgeAnswersMonthly != lim.KnowledgeAnswersMonthly ||
			offers[i].KnowledgeDesk != lim.KnowledgeDesk ||
			offers[i].OwnedWorkspaces != lim.OwnedWorkspaces ||
			offers[i].CustomDomain != lim.CustomDomain || offers[i].NDA != lim.NDA ||
			offers[i].Webhooks != lim.Webhooks || offers[i].HubSpot != lim.HubSpot ||
			offers[i].DailyDigest != lim.DailyDigest || offers[i].SlackAlerts != lim.SlackAlerts ||
			offers[i].RoomInsights != lim.RoomInsights || offers[i].RoomAnalytics != lim.RoomAnalytics ||
			offers[i].FormalAsk != lim.FormalAsk {
			t.Fatalf("offer %s caps mismatch: %+v vs %+v", code, offers[i], lim)
		}
		if !Purchasable(code) {
			t.Fatalf("%s must be purchasable", code)
		}
	}
	if Purchasable(CodeTrial) {
		t.Fatal("trial must not be purchasable")
	}
	if SelfServe(CodeEnterprise) {
		t.Fatal("enterprise must not be self-serve")
	}
	if !SelfServe(CodePro) || !SelfServe(CodeFree) {
		t.Fatal("free and pro must be self-serve SKUs")
	}
	if !offers[2].Highlighted || !offers[3].CustomPricing {
		t.Fatal("business highlighted / enterprise custom pricing expected")
	}
}

func TestNormalizePeriod(t *testing.T) {
	t.Parallel()
	if NormalizePeriod("") != PeriodMonthly || NormalizePeriod("YEARLY") != PeriodYearly {
		t.Fatal("normalize period failed")
	}
	if NormalizePeriod("weekly") != "" {
		t.Fatal("invalid period must be empty")
	}
}
