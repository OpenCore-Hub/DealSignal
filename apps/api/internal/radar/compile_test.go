package radar

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCompileExcludesBounce(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	actID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:          mustUUID(sigID),
			Type:        "risk_alert",
			Subtype:     pgText(suggestions.SubtypeBounce),
			Title:       "Bounce risk",
			Description: "left quickly",
			Priority:    "medium",
			CreatedAt:   pgTime(now.Add(-time.Hour)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(actID),
			SignalID:   mustUUID(sigID),
			Title:      "Review bounce",
			Impact:     "medium",
			Status:     "pending",
			ActionType: "review",
			CreatedAt:  pgTime(now.Add(-time.Hour)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 0 {
		t.Fatalf("bounce must not appear on radar, got %+v", feed.Items)
	}
}

func TestCompileDiligenceGateAndNextUp(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	actID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Series A Room"}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(actID),
			Title:      "Approve access",
			Impact:     "high",
			Status:     "pending",
			ActionType: "approve",
			SourceType: pgText(action.SourceTypeDealRoomLinkAccessRequest),
			SourceID:   pgText(uuid.New().String()),
			TargetID:   pgText(roomID),
			CreatedAt:  pgTime(now.Add(-30 * time.Minute)),
			DueAt:      pgTime(now.Add(2 * time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.Product != ProductDiligenceGate {
		t.Fatalf("product=%s", item.Product)
	}
	if item.Verb != VerbApprove {
		t.Fatalf("verb=%s", item.Verb)
	}
	if item.DealName != "Series A Room" {
		t.Fatalf("dealName=%s", item.DealName)
	}
	if feed.NextUp == nil || feed.NextUp.ID != item.ID {
		t.Fatalf("nextUp mismatch: %+v", feed.NextUp)
	}
	if feed.Counts[string(ProductDiligenceGate)] != 1 {
		t.Fatalf("counts=%v", feed.Counts)
	}
	if item.NavigatePath == "" || item.NavigatePath[:len("/acme/deal-rooms/")] != "/acme/deal-rooms/" {
		t.Fatalf("navigatePath=%s", item.NavigatePath)
	}
}

func TestCompileExcludesUploadedFileFromDiligenceGate(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			Title:      "Review uploaded file deck.pdf on Investor Link",
			Impact:     "medium",
			Status:     "pending",
			ActionType: "review",
			SourceType: pgText(action.SourceTypeUploadedFile),
			SourceID:   pgText(uuid.New().String()),
			CreatedAt:  pgTime(now.Add(-time.Hour)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 0 {
		t.Fatalf("uploaded_file must not enter radar products, got %+v", feed.Items)
	}
}

func TestCompileRoomNDAMemberKeyedNavigatesToRoom(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	memberID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Startup Fundraising", Scenario: ScenarioStartupFundraising},
		},
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			Title:      "NDA signature required from lp@vc.com for Startup Fundraising",
			Impact:     "high",
			Status:     "pending",
			ActionType: "sign",
			SourceType: pgText(action.SourceTypeRoomNDA),
			SourceID:   pgText(memberID),
			TargetID:   pgText(roomID),
			CreatedAt:  pgTime(now.Add(-30 * time.Minute)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.Product != ProductDiligenceGate {
		t.Fatalf("product=%s", item.Product)
	}
	if item.DealName != "Startup Fundraising" {
		t.Fatalf("dealName=%s", item.DealName)
	}
	wantPath := "/acme/deal-rooms/" + roomID + "?tab=access"
	if item.NavigatePath != wantPath {
		t.Fatalf("navigatePath=%s want %s", item.NavigatePath, wantPath)
	}
}

func TestCompileBuyingWindowCoalesce(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	linkID := uuid.New()
	contactID := uuid.New()
	sigHot := uuid.New()
	sigKey := uuid.New()
	actHot := uuid.New()
	actKey := uuid.New()
	ctx, _ := jsonContext(map[string]any{
		"contactEmail":  "investor@example.com",
		"documentTitle": "Deck",
	})

	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "Investor link"},
		},
		Signals: []db.Signal{
			{
				ID:         mustUUID(sigHot),
				Type:       "hot_signal",
				Subtype:    pgText(suggestions.SubtypeHot),
				Title:      "High-intent",
				Suggestion: "Reach out",
				Priority:   "high",
				LinkID:     mustUUID(linkID),
				ContactID:  mustUUID(contactID),
				Context:    ctx,
				CreatedAt:  pgTime(now.Add(-2 * time.Hour)),
			},
			{
				ID:         mustUUID(sigKey),
				Type:       "hot_signal",
				Subtype:    pgText(suggestions.SubtypeKeyPage),
				Title:      "Key-page deep read",
				Suggestion: "Follow up",
				Priority:   "high",
				LinkID:     mustUUID(linkID),
				ContactID:  mustUUID(contactID),
				Context:    ctx,
				CreatedAt:  pgTime(now.Add(-time.Hour)),
			},
		},
		Actions: []db.ActionItem{
			{
				ID:         mustUUID(actHot),
				SignalID:   mustUUID(sigHot),
				Title:      "Reach out",
				Impact:     "high",
				Status:     "pending",
				ActionType: "call",
				CreatedAt:  pgTime(now.Add(-2 * time.Hour)),
				DueAt:      pgTime(now.Add(4 * time.Hour)),
				UpdatedAt:  pgTime(now),
			},
			{
				ID:         mustUUID(actKey),
				SignalID:   mustUUID(sigKey),
				Title:      "Follow up",
				Impact:     "high",
				Status:     "pending",
				ActionType: "call",
				CreatedAt:  pgTime(now.Add(-time.Hour)),
				DueAt:      pgTime(now.Add(4 * time.Hour)),
				UpdatedAt:  pgTime(now),
			},
		},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("expected coalesce to 1 buying_window card, got %d (%+v)", len(feed.Items), feed.Items)
	}
	if feed.Items[0].Product != ProductBuyingWindow {
		t.Fatalf("product=%s", feed.Items[0].Product)
	}
	if len(feed.Items[0].CoalescedFrom) == 0 {
		t.Fatalf("expected coalescedFrom ids")
	}
	if feed.Items[0].ContactEmail != "investor@example.com" {
		t.Fatalf("email=%s", feed.Items[0].ContactEmail)
	}
}

func TestCompileLeakWatchRanksAboveBuyingWindow(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigRisk := uuid.New()
	sigHot := uuid.New()
	linkID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Metrics: map[string]LinkMetrics24h{
			linkID.String(): {ForwardSignals: 2, UniqueVisitors: 4},
		},
		Signals: []db.Signal{
			{
				ID:        mustUUID(sigRisk),
				Type:      "risk_alert",
				Subtype:   pgText(suggestions.SubtypeForward),
				Title:     "Possible forward",
				Priority:  "high",
				LinkID:    mustUUID(linkID),
				CreatedAt: pgTime(now.Add(-10 * time.Minute)),
			},
			{
				ID:        mustUUID(sigHot),
				Type:      "hot_signal",
				Subtype:   pgText(suggestions.SubtypeHot),
				Title:     "Hot",
				Priority:  "high",
				CreatedAt: pgTime(now.Add(-5 * time.Minute)),
			},
		},
		Actions: []db.ActionItem{
			{
				ID:         mustUUID(uuid.New()),
				SignalID:   mustUUID(sigHot),
				Title:      "Email",
				Impact:     "high",
				Status:     "pending",
				ActionType: "email",
				CreatedAt:  pgTime(now.Add(-5 * time.Minute)),
				DueAt:      pgTime(now.Add(time.Hour)),
				UpdatedAt:  pgTime(now),
			},
			{
				ID:         mustUUID(uuid.New()),
				SignalID:   mustUUID(sigRisk),
				Title:      "Review forward",
				Impact:     "high",
				Status:     "pending",
				ActionType: "review",
				CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
				DueAt:      pgTime(now.Add(time.Hour)),
				UpdatedAt:  pgTime(now),
			},
		},
	})
	if feed.NextUp == nil || feed.NextUp.Product != ProductLeakWatch {
		t.Fatalf("expected leak_watch nextUp, got %+v", feed.NextUp)
	}
	if feed.NextUp.Confidence != ConfidenceHigh {
		t.Fatalf("expected high confidence, got %s", feed.NextUp.Confidence)
	}
	if feed.Lens != "founder" {
		t.Fatalf("lens=%s", feed.Lens)
	}
}

func TestCompileSalesLensPrefersBuyingWindow(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigRisk := uuid.New()
	sigHot := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug:  "acme",
		Now:            now,
		Circle:         heat.CircleSales,
		CircleExplicit: true,
		Signals: []db.Signal{
			{
				ID:        mustUUID(sigRisk),
				Type:      "risk_alert",
				Subtype:   pgText(suggestions.SubtypeBlockedAttempt),
				Title:     "Blocked",
				Priority:  "medium",
				CreatedAt: pgTime(now.Add(-10 * time.Minute)),
			},
			{
				ID:        mustUUID(sigHot),
				Type:      "hot_signal",
				Subtype:   pgText(suggestions.SubtypeHot),
				Title:     "Hot",
				Priority:  "high",
				CreatedAt: pgTime(now.Add(-5 * time.Minute)),
			},
		},
		Actions: []db.ActionItem{
			{
				ID:         mustUUID(uuid.New()),
				SignalID:   mustUUID(sigHot),
				Title:      "Email",
				Impact:     "high",
				Status:     "pending",
				ActionType: "email",
				CreatedAt:  pgTime(now.Add(-5 * time.Minute)),
				DueAt:      pgTime(now.Add(time.Hour)),
				UpdatedAt:  pgTime(now),
			},
			{
				ID:         mustUUID(uuid.New()),
				SignalID:   mustUUID(sigRisk),
				Title:      "Review block",
				Impact:     "medium",
				Status:     "pending",
				ActionType: "review",
				CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
				DueAt:      pgTime(now.Add(time.Hour)),
				UpdatedAt:  pgTime(now),
			},
		},
	})
	if feed.Lens != "sales" {
		t.Fatalf("lens=%s", feed.Lens)
	}
	if feed.NextUp == nil || feed.NextUp.Product != ProductBuyingWindow {
		t.Fatalf("sales lens should prefer buying_window, got %+v", feed.NextUp)
	}
}

func TestCompileBlockedAttemptIsDiligenceGateNotLeakWatch(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:        mustUUID(sigID),
			Type:      "risk_alert",
			Subtype:   pgText(suggestions.SubtypeBlockedAttempt),
			Title:     "Blocked",
			Priority:  "medium",
			CreatedAt: pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			SignalID:   mustUUID(sigID),
			Title:      "Review block",
			Impact:     "medium",
			Status:     "pending",
			ActionType: "review",
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	got := feed.Items[0]
	if got.Product != ProductDiligenceGate {
		t.Fatalf("product=%s want diligence_gate", got.Product)
	}
	if got.Verb != VerbReview {
		t.Fatalf("verb=%s want review", got.Verb)
	}
	if got.Confidence != "" {
		t.Fatalf("gate hold must not carry leak_watch confidence, got %s", got.Confidence)
	}
	hasGate := false
	for _, c := range got.Evidence {
		if c.Kind == "gate" {
			hasGate = true
		}
		if c.Kind == "forward" || c.Kind == "download" {
			t.Fatalf("unexpected sharing chip %+v", c)
		}
	}
	if !hasGate {
		t.Fatal("expected gate chip")
	}
}

func TestCompileBlockedAttemptKeepsReviewVerbInFundraisingPack(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	sigID := uuid.New()
	linkID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Series A", Scenario: ScenarioStartupFundraising}},
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "Deck", DealRoomID: roomID},
		},
		Signals: []db.Signal{{
			ID:        mustUUID(sigID),
			Type:      "risk_alert",
			Subtype:   pgText(suggestions.SubtypeBlockedAttempt),
			Title:     "Blocked",
			Priority:  "medium",
			LinkID:    mustUUID(linkID),
			CreatedAt: pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			SignalID:   mustUUID(sigID),
			Title:      "Review block",
			Impact:     "medium",
			Status:     "pending",
			ActionType: "review",
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	got := feed.Items[0]
	if got.Product != ProductDiligenceGate {
		t.Fatalf("product=%s want diligence_gate", got.Product)
	}
	if got.Verb != VerbReview {
		t.Fatalf("fundraising pack must not turn a hold into approve, verb=%s", got.Verb)
	}
	if got.HeadlineCode != "unlock_investor_gate" {
		t.Fatalf("headlineCode=%s want unlock_investor_gate", got.HeadlineCode)
	}
	wantNav := "/acme/deal-rooms/" + roomID + "?tab=access&linkId=" + linkID.String()
	if got.NavigatePath != wantNav {
		t.Fatalf("navigatePath=%s want %s", got.NavigatePath, wantNav)
	}
}

func TestCompileBlockedAttemptDocumentLinkNavigatesToShare(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	linkID := uuid.New()
	docID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "Deck", DocumentID: docID.String()},
		},
		Signals: []db.Signal{{
			ID:         mustUUID(sigID),
			Type:       "risk_alert",
			Subtype:    pgText(suggestions.SubtypeBlockedAttempt),
			Title:      "Blocked",
			Priority:   "medium",
			LinkID:     mustUUID(linkID),
			DocumentID: mustUUID(docID),
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			SignalID:   mustUUID(sigID),
			Title:      "Review block",
			Impact:     "medium",
			Status:     "pending",
			ActionType: "review",
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	got := feed.Items[0]
	wantNav := "/acme/documents?tab=shared&linkId=" + linkID.String()
	if got.NavigatePath != wantNav {
		t.Fatalf("navigatePath=%s want %s (must not pick the document analytics tab)", got.NavigatePath, wantNav)
	}
}

func TestCompileGateHoldActorPrefersVisitorEmailOverShareContact(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	ctx, err := jsonContext(map[string]any{
		"contactName":  "张姐",
		"contactEmail": "zhang@share.example",
		"visitorEmail": "yqx-401@126.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:        mustUUID(sigID),
			Type:      "risk_alert",
			Subtype:   pgText(suggestions.SubtypeBlockedAttempt),
			Title:     "Blocked",
			Priority:  "medium",
			Context:   ctx,
			CreatedAt: pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			SignalID:   mustUUID(sigID),
			Title:      "Review block",
			Impact:     "medium",
			Status:     "pending",
			ActionType: "review",
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	got := feed.Items[0]
	if got.Actor != "yqx-401@126.com" {
		t.Fatalf("actor=%q want gated visitor email", got.Actor)
	}
	if got.ContactEmail != "zhang@share.example" {
		t.Fatalf("contactEmail=%q want share-contact email", got.ContactEmail)
	}
}

func TestCompileGateHoldActorKeepsShareContactWhenVisitorEmailMissing(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	ctx, err := jsonContext(map[string]any{
		"contactName": "张姐",
	})
	if err != nil {
		t.Fatal(err)
	}
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:        mustUUID(sigID),
			Type:      "risk_alert",
			Subtype:   pgText(suggestions.SubtypeBlockedAttempt),
			Title:     "Blocked",
			Priority:  "medium",
			Context:   ctx,
			CreatedAt: pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			SignalID:   mustUUID(sigID),
			Title:      "Review block",
			Impact:     "medium",
			Status:     "pending",
			ActionType: "review",
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	got := feed.Items[0]
	if got.Actor != "张姐" {
		t.Fatalf("actor=%q want share contact when visitorEmail is absent", got.Actor)
	}
}

func TestCompileOutcomeDemoteAndMicroRank(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigKey := uuid.New()
	sigHot := uuid.New()
	sigLeak := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		OutcomeDemote: map[Product]int{ProductLeakWatch: 3},
		Signals: []db.Signal{
			{
				ID:        mustUUID(sigKey),
				Type:      "hot_signal",
				Subtype:   pgText(suggestions.SubtypeKeyPage),
				Title:     "Key page",
				Priority:  "high",
				CreatedAt: pgTime(now.Add(-time.Hour)),
			},
			{
				ID:        mustUUID(sigHot),
				Type:      "hot_signal",
				Subtype:   pgText(suggestions.SubtypeHot),
				Title:     "Hot",
				Priority:  "high",
				CreatedAt: pgTime(now.Add(-30 * time.Minute)),
			},
			{
				ID:        mustUUID(sigLeak),
				Type:      "risk_alert",
				Subtype:   pgText(suggestions.SubtypeForward),
				Title:     "Forward",
				Priority:  "high",
				CreatedAt: pgTime(now.Add(-20 * time.Minute)),
			},
		},
		Actions: []db.ActionItem{
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigHot), Title: "Hot",
				Impact: "high", Status: "pending", ActionType: "email",
				CreatedAt: pgTime(now.Add(-30 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigKey), Title: "Key",
				Impact: "high", Status: "pending", ActionType: "email",
				CreatedAt: pgTime(now.Add(-time.Hour)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigLeak), Title: "Leak",
				Impact: "high", Status: "pending", ActionType: "review",
				CreatedAt: pgTime(now.Add(-20 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
		},
		NoiseHints: []NoiseHint{{Product: ProductLeakWatch, FalsePositiveRate: 0.8, Sample: 5, DemoteBoost: 2}},
	})
	if feed.NextUp == nil || feed.NextUp.Product != ProductBuyingWindow {
		t.Fatalf("demoted leak should not be nextUp, got %+v", feed.NextUp)
	}
	if feed.NextUp.Headline != "Key" {
		t.Fatalf("key_page should beat hot within buying window, got %+v", feed.NextUp)
	}
	if len(feed.NoiseHints) != 1 {
		t.Fatalf("noiseHints=%+v", feed.NoiseHints)
	}
}

func TestCompileClearedToday(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Actions: []db.ActionItem{{
			ID:         mustUUID(uuid.New()),
			Title:      "Done",
			Impact:     "high",
			Status:     "done",
			ActionType: "approve",
			CreatedAt:  pgTime(now.Add(-2 * time.Hour)),
			DueAt:      pgTime(now),
			UpdatedAt:  pgTime(now.Add(-time.Minute)),
		}},
	})
	if feed.ClearedToday != 1 {
		t.Fatalf("clearedToday=%d", feed.ClearedToday)
	}
}

func TestCompileCoalesceRespects24hWindow(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	linkID := uuid.New()
	contactID := uuid.New()
	sigNew := uuid.New()
	sigOld := uuid.New()
	ctx, _ := jsonContext(map[string]any{
		"contactEmail": "investor@example.com",
	})
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "Investor link"},
		},
		Signals: []db.Signal{
			{
				ID: mustUUID(sigNew), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "New", Priority: "high", LinkID: mustUUID(linkID), ContactID: mustUUID(contactID),
				Context: ctx, CreatedAt: pgTime(now.Add(-time.Hour)),
			},
			{
				ID: mustUUID(sigOld), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Old", Priority: "high", LinkID: mustUUID(linkID), ContactID: mustUUID(contactID),
				Context: ctx, CreatedAt: pgTime(now.Add(-48 * time.Hour)),
			},
		},
		Actions: []db.ActionItem{
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigNew), Title: "New",
				Impact: "high", Status: "pending", ActionType: "email",
				CreatedAt: pgTime(now.Add(-time.Hour)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigOld), Title: "Old",
				Impact: "high", Status: "pending", ActionType: "email",
				CreatedAt: pgTime(now.Add(-48 * time.Hour)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
		},
	})
	if len(feed.Items) != 2 {
		t.Fatalf("expected 2 items outside 24h window, got %d", len(feed.Items))
	}
}

func TestCompileSLAOverdueRanksFirst(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigHot := uuid.New()
	roomID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Room"}},
		Signals: []db.Signal{{
			ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
			Title: "Hot", Priority: "high", CreatedAt: pgTime(now.Add(-30 * time.Minute)),
		}},
		Actions: []db.ActionItem{
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigHot), Title: "Email hot",
				Impact: "high", Status: "pending", ActionType: "email",
				CreatedAt: pgTime(now.Add(-30 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
			{
				// Diligence gate created 5h ago → 2h SLA → overdue; must beat fresh buying_window.
				ID: mustUUID(uuid.New()), Title: "Approve overdue", Impact: "medium", Status: "pending",
				ActionType: "approve", SourceType: pgText(action.SourceTypeDealRoomLinkAccessRequest),
				SourceID: pgText(uuid.New().String()), TargetID: pgText(roomID),
				CreatedAt: pgTime(now.Add(-5 * time.Hour)), DueAt: pgTime(now.Add(-3 * time.Hour)), UpdatedAt: pgTime(now),
			},
		},
	})
	if feed.NextUp == nil || feed.NextUp.Product != ProductDiligenceGate {
		t.Fatalf("overdue diligence_gate should be nextUp, got %+v", feed.NextUp)
	}
	if feed.NextUp.WhyNowCode != "sla_overdue" {
		t.Fatalf("whyNowCode=%s", feed.NextUp.WhyNowCode)
	}
	if feed.NextUp.State != "open" {
		t.Fatalf("state=%s", feed.NextUp.State)
	}
}

func TestCompileCommitmentAskSLAEndOfDay(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Answer ask", Impact: "high", Status: "pending",
			ActionType: "answer", SourceType: pgText(action.SourceTypeLinkQuestion),
			SourceID: pgText(uuid.New().String()), TargetID: pgText(uuid.New().String()),
			CreatedAt: pgTime(now.Add(-time.Hour)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	due, err := time.Parse(time.RFC3339, feed.Items[0].SlaDueAt)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 8, 23, 59, 59, 0, time.UTC)
	if !due.Equal(want) {
		t.Fatalf("commitment SLA want end-of-day %s, got %s", want, due)
	}
}

func TestCompileUnknownActionDropped(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Mystery", Impact: "low", Status: "pending",
			ActionType: "upload", CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 0 {
		t.Fatalf("unknown action must not productize to buying_window, got %+v", feed.Items)
	}
}

func TestCompileDealNameHasNoEnglishFallback(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Approve", Impact: "high", Status: "pending",
			ActionType: "approve", SourceType: pgText(action.SourceTypeLinkAccessRequest),
			SourceID:  pgText(uuid.New().String()),
			CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	if feed.Items[0].DealName == "Deal" {
		t.Fatal("must not hardcode English Deal fallback")
	}
}

func TestCompileAccessRequestActorFromTitle(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Actions: []db.ActionItem{{
			ID:     mustUUID(uuid.New()),
			Title:  "Approve access request from buyer@acme.test for Pitch",
			Impact: "high", Status: "pending",
			ActionType: "approve", SourceType: pgText(action.SourceTypeLinkAccessRequest),
			SourceID:  pgText(uuid.New().String()),
			CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	if feed.Items[0].ContactEmail != "buyer@acme.test" {
		t.Fatalf("contactEmail=%q", feed.Items[0].ContactEmail)
	}
	if feed.Items[0].Actor != "buyer@acme.test" {
		t.Fatalf("actor=%q", feed.Items[0].Actor)
	}
}

// sameDualProductActions builds one diligence_gate + one buying_window pair for scenario rank tests.
func sameDualProductActions(now time.Time, roomID string, sigHot uuid.UUID) []db.ActionItem {
	return []db.ActionItem{
		{
			ID: mustUUID(uuid.New()), Title: "Approve access", Impact: "high", Status: "pending",
			ActionType: "approve", SourceType: pgText(action.SourceTypeDealRoomLinkAccessRequest),
			SourceID: pgText(uuid.New().String()), TargetID: pgText(roomID),
			CreatedAt: pgTime(now.Add(-20 * time.Minute)), DueAt: pgTime(now.Add(2 * time.Hour)), UpdatedAt: pgTime(now),
		},
		{
			ID: mustUUID(uuid.New()), SignalID: mustUUID(sigHot), Title: "Email hot", Impact: "high", Status: "pending",
			ActionType: "email",
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)), DueAt: pgTime(now.Add(2 * time.Hour)), UpdatedAt: pgTime(now),
		},
	}
}

func TestCompileScenarioPackChangesNextUp(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	sigHot := uuid.New()
	linkID := uuid.New()
	signals := []db.Signal{{
		ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
		Title: "Hot", Priority: "high", CreatedAt: pgTime(now.Add(-10 * time.Minute)),
		LinkID: mustUUID(linkID),
	}}
	links := map[string]LinkMeta{
		linkID.String(): {ID: linkID.String(), Name: "Deck", DealRoomID: roomID},
	}

	fund := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Series A", Scenario: ScenarioStartupFundraising}},
		Links:         links,
		Signals:       signals,
		Actions:       sameDualProductActions(now, roomID, sigHot),
	})
	if fund.NextUp == nil || fund.NextUp.Product != ProductDiligenceGate {
		t.Fatalf("fundraising pack should prefer diligence_gate, got %+v", fund.NextUp)
	}
	if fund.NextUp.Scenario != string(ScenarioStartupFundraising) {
		t.Fatalf("scenario=%s", fund.NextUp.Scenario)
	}
	if fund.DefaultLens != "founder" || fund.LensSource != "inferred" {
		t.Fatalf("defaultLens=%s lensSource=%s lens=%s", fund.DefaultLens, fund.LensSource, fund.Lens)
	}
	if fund.Lens != "founder" {
		t.Fatalf("inferred lens want founder, got %s", fund.Lens)
	}

	sales := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Enterprise Deal", Scenario: ScenarioSalesDataRoom}},
		Links:         links,
		Signals:       signals,
		Actions:       sameDualProductActions(now, roomID, sigHot),
	})
	if sales.NextUp == nil || sales.NextUp.Product != ProductBuyingWindow {
		t.Fatalf("sales pack should prefer buying_window, got %+v", sales.NextUp)
	}
	if sales.NextUp.Scenario != string(ScenarioSalesDataRoom) {
		t.Fatalf("scenario=%s", sales.NextUp.Scenario)
	}
	if sales.DefaultLens != "sales" || sales.Lens != "sales" || sales.LensSource != "inferred" {
		t.Fatalf("sales inferred lens=%s default=%s source=%s", sales.Lens, sales.DefaultLens, sales.LensSource)
	}
}

func TestCompileCircleQueryOverridesInferredLens(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	sigHot := uuid.New()
	linkID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug:  "acme",
		Now:            now,
		Circle:         heat.CircleFounder,
		CircleExplicit: true,
		Rooms:          map[string]RoomMeta{roomID: {Name: "Deal", Scenario: ScenarioSalesDataRoom}},
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "Deck", DealRoomID: roomID},
		},
		Signals: []db.Signal{{
			ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
			Title: "Hot", Priority: "high", CreatedAt: pgTime(now.Add(-10 * time.Minute)),
			LinkID: mustUUID(linkID),
		}},
		Actions: sameDualProductActions(now, roomID, sigHot),
	})
	if feed.LensSource != "query" || feed.Lens != "founder" {
		t.Fatalf("explicit founder override: lens=%s source=%s", feed.Lens, feed.LensSource)
	}
	if feed.DefaultLens != "sales" {
		t.Fatalf("defaultLens should still reflect scenario pack, got %s", feed.DefaultLens)
	}
	// Scenario rank still primary: sales pack ranks buying_window above diligence even under founder lens.
	if feed.NextUp == nil || feed.NextUp.Product != ProductBuyingWindow {
		t.Fatalf("scenario pack rank should still prefer buying_window, got %+v", feed.NextUp)
	}
}

func TestCompileHotSignalPrefersMetadataDocument(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	actID := uuid.New()
	primary := uuid.New()
	focus := uuid.New()
	md, err := json.Marshal(map[string]string{
		"page_number": "8",
		"document_id": focus.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:         mustUUID(sigID),
			Type:       "hot_signal",
			Subtype:    pgText(suggestions.SubtypeHot),
			Title:      "Hot",
			Priority:   "high",
			DocumentID: mustUUID(primary),
			Metadata:   md,
			CreatedAt:  pgTime(now.Add(-time.Hour)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(actID),
			SignalID:   mustUUID(sigID),
			Title:      "Email",
			Impact:     "high",
			Status:     "pending",
			ActionType: "email",
			CreatedAt:  pgTime(now.Add(-time.Hour)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.DocumentID != focus.String() {
		t.Fatalf("documentId=%s want focus %s", item.DocumentID, focus.String())
	}
	want := "/acme/documents/" + focus.String() + "?tab=content&page=8"
	if item.EvidencePath != want {
		t.Fatalf("evidencePath=%s want %s", item.EvidencePath, want)
	}
}

func TestCompileBuyingWindowEmailActDoesNotUseEvidencePath(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	actID := uuid.New()
	docID := uuid.New()
	ctx, err := jsonContext(map[string]any{
		"contactName":   "张总",
		"contactEmail":  "zhang@lp.example",
		"documentTitle": "Deck",
	})
	if err != nil {
		t.Fatal(err)
	}
	md, err := json.Marshal(map[string]string{
		"page_number": "8",
		"document_id": docID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:        mustUUID(sigID),
			Type:      "hot_signal",
			Subtype:   pgText(suggestions.SubtypeHot),
			Title:     "Hot",
			Priority:  "high",
			Context:   ctx,
			Metadata:  md,
			CreatedAt: pgTime(now.Add(-time.Hour)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(actID),
			SignalID:   mustUUID(sigID),
			Title:      "Email",
			Impact:     "high",
			Status:     "pending",
			ActionType: "email",
			CreatedAt:  pgTime(now.Add(-time.Hour)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.Verb != VerbEmail {
		t.Fatalf("verb=%s want email", item.Verb)
	}
	wantEv := "/acme/documents/" + docID.String() + "?tab=content&page=8"
	if item.EvidencePath != wantEv {
		t.Fatalf("evidencePath=%s want %s", item.EvidencePath, wantEv)
	}
	if item.NavigatePath != "" {
		t.Fatalf("navigatePath=%s want empty (email ACT is compose, not the document tab)", item.NavigatePath)
	}
}

func TestCompileBuyingWindowEmptyEmailDoesNotNavigateToDocument(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	actID := uuid.New()
	docID := uuid.New()
	ctx, err := jsonContext(map[string]any{
		"contactName":   "张姐",
		"documentTitle": "Deck",
	})
	if err != nil {
		t.Fatal(err)
	}
	md, err := json.Marshal(map[string]string{
		"page_number": "8",
		"document_id": docID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:        mustUUID(sigID),
			Type:      "hot_signal",
			Subtype:   pgText(suggestions.SubtypeHot),
			Title:     "Hot",
			Priority:  "high",
			Context:   ctx,
			Metadata:  md,
			CreatedAt: pgTime(now.Add(-time.Hour)),
		}},
		Actions: []db.ActionItem{{
			ID:         mustUUID(actID),
			SignalID:   mustUUID(sigID),
			Title:      "Email",
			Impact:     "high",
			Status:     "pending",
			ActionType: "email",
			CreatedAt:  pgTime(now.Add(-time.Hour)),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.Verb != VerbOpen {
		t.Fatalf("verb=%s want open", item.Verb)
	}
	wantEv := "/acme/documents/" + docID.String() + "?tab=content&page=8"
	if item.EvidencePath != wantEv {
		t.Fatalf("evidencePath=%s want %s", item.EvidencePath, wantEv)
	}
	if item.NavigatePath != "" {
		t.Fatalf("navigatePath=%s want empty (confirm-recipient is not the document tab)", item.NavigatePath)
	}
}

func TestCompileLeakWatchNavigatesToShareLinkNotDocument(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	linkID := uuid.New()
	docID := uuid.New()
	md, err := json.Marshal(map[string]string{
		"page_number": "3",
		"document_id": docID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:         mustUUID(sigID),
			Type:       "risk_alert",
			Subtype:    pgText(suggestions.SubtypeForward),
			Title:      "Possible forward",
			Priority:   "high",
			LinkID:     mustUUID(linkID),
			DocumentID: mustUUID(docID),
			Metadata:   md,
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), SignalID: mustUUID(sigID), Title: "Review forward",
			Impact: "high", Status: "pending", ActionType: "review",
			CreatedAt: pgTime(now.Add(-10 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.Product != ProductLeakWatch {
		t.Fatalf("product=%s", item.Product)
	}
	wantNav := "/acme/links/" + linkID.String()
	if item.NavigatePath != wantNav {
		t.Fatalf("navigatePath=%s want %s", item.NavigatePath, wantNav)
	}
	wantEv := "/acme/documents/" + docID.String() + "?tab=content&page=3"
	if item.EvidencePath != wantEv {
		t.Fatalf("evidencePath=%s want %s (page stays on the evidence rail)", item.EvidencePath, wantEv)
	}
}

func TestCompileAbuseGuardNavigatesToShareLinkNotDocument(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sigID := uuid.New()
	linkID := uuid.New()
	docID := uuid.New()
	md, err := json.Marshal(map[string]string{
		"eventType":   "ask_ai_rate_limited",
		"page_number": "2",
		"document_id": docID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID:         mustUUID(sigID),
			Type:       "risk_alert",
			Subtype:    pgText(suggestions.SubtypeAnomaly),
			Title:      "Ask rate limited",
			Priority:   "medium",
			LinkID:     mustUUID(linkID),
			DocumentID: mustUUID(docID),
			Metadata:   md,
			CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), SignalID: mustUUID(sigID), Title: "Review quota",
			Impact: "medium", Status: "pending", ActionType: "review",
			CreatedAt: pgTime(now.Add(-10 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.Product != ProductAbuseGuard {
		t.Fatalf("product=%s", item.Product)
	}
	wantNav := "/acme/links/" + linkID.String()
	if item.NavigatePath != wantNav {
		t.Fatalf("navigatePath=%s want %s", item.NavigatePath, wantNav)
	}
}

func TestCompileDealRoomAskNavigatesToQATab(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := "room-1"
	linkID := "link-9"
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Series A"}},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Answer ask", Impact: "high", Status: "pending",
			ActionType: "answer", SourceType: pgText(action.SourceTypeDealRoomLinkQuestion),
			SourceID: pgText(uuid.New().String()), TargetID: pgText(roomID + "/" + linkID),
			CreatedAt: pgTime(now.Add(-time.Hour)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	want := "/acme/deal-rooms/room-1?askInbox=needs_host&linkId=link-9&tab=qa"
	if feed.Items[0].NavigatePath != want {
		t.Fatalf("navigatePath=%s want %s", feed.Items[0].NavigatePath, want)
	}
}

func TestCompileLibraryAskNavigatesToLinkInbox(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	linkID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Answer ask", Impact: "high", Status: "pending",
			ActionType: "answer", SourceType: pgText(action.SourceTypeLinkQuestion),
			SourceID: pgText(uuid.New().String()), TargetID: pgText(linkID),
			CreatedAt: pgTime(now.Add(-time.Hour)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	want := "/acme/links/" + linkID + "?askInbox=needs_host"
	if feed.Items[0].NavigatePath != want {
		t.Fatalf("navigatePath=%s want %s", feed.Items[0].NavigatePath, want)
	}
}

func TestMetadataDocumentID(t *testing.T) {
	if got := metadataDocumentID(nil); got != "" {
		t.Fatalf("empty => %q", got)
	}
	if got := metadataDocumentID([]byte(`{"document_id":"doc-pdf"}`)); got != "doc-pdf" {
		t.Fatalf("document_id => %q", got)
	}
	if got := metadataDocumentID([]byte(`{"documentId":"doc-xlsx"}`)); got != "doc-xlsx" {
		t.Fatalf("documentId => %q", got)
	}
}

func mustUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func jsonContext(m map[string]any) ([]byte, error) {
	return json.Marshal(m)
}
