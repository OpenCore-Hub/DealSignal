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
		WorkspaceSlug: "acme",
		Now:           now,
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
			SourceID: pgText(uuid.New().String()),
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
