package radar

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Six high-value radar products — include / exclude boundaries through Compile
// (action+signal → WorkItem), the host productization contract.

func TestCompileSixProductIncludeBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	linkID := uuid.New()
	memberID := uuid.New().String()

	sigHot := uuid.New()
	sigFwd := uuid.New()
	sigAbuse := uuid.New()
	sigExpired := uuid.New()
	sigAsk := uuid.New()

	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Startup Fundraising", Scenario: ScenarioStartupFundraising},
		},
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "Investor link", DealRoomID: roomID},
		},
		Signals: []db.Signal{
			{
				ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Hot", Priority: "high", LinkID: mustUUID(linkID),
				CreatedAt: pgTime(now.Add(-2 * time.Hour)),
			},
			{
				ID: mustUUID(sigFwd), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeForward),
				Title: "Forward", Priority: "high", LinkID: mustUUID(linkID),
				CreatedAt: pgTime(now.Add(-90 * time.Minute)),
			},
			{
				ID: mustUUID(sigAbuse), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeAnomaly),
				Title: "Ask abuse", Description: "visitor ask rate limit exceeded",
				Priority: "high", LinkID: mustUUID(linkID),
				CreatedAt: pgTime(now.Add(-80 * time.Minute)),
			},
			{
				ID: mustUUID(sigExpired), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeExpired),
				Title: "Expired", Priority: "medium", LinkID: mustUUID(linkID),
				CreatedAt: pgTime(now.Add(-70 * time.Minute)),
			},
			{
				ID: mustUUID(sigAsk), Type: "follow_up", Subtype: pgText(suggestions.SubtypeQuestion),
				Title: "Question", Priority: "high", LinkID: mustUUID(linkID),
				CreatedAt: pgTime(now.Add(-60 * time.Minute)),
			},
		},
		Actions: []db.ActionItem{
			// buying_window
			pendingAction(sigHot, "email", "", "", now.Add(-2*time.Hour)),
			// leak_watch
			pendingAction(sigFwd, "review", "", "", now.Add(-90*time.Minute)),
			// abuse_guard
			pendingAction(sigAbuse, "review", "", "", now.Add(-80*time.Minute)),
			// access_decay (signal path)
			pendingAction(sigExpired, "review", "", "", now.Add(-70*time.Minute)),
			// commitment_ask (signal+answer)
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigAsk), Title: "Answer ask",
				Impact: "high", Status: "pending", ActionType: "answer",
				SourceType: pgText(action.SourceTypeDealRoomLinkQuestion),
				SourceID:   pgText(uuid.New().String()),
				TargetID:   pgText(roomID + "/" + linkID.String()),
				CreatedAt:  pgTime(now.Add(-60 * time.Minute)),
				DueAt:      pgTime(now.Add(2 * time.Hour)), UpdatedAt: pgTime(now),
			},
			// diligence_gate (member-keyed external NDA)
			{
				ID: mustUUID(uuid.New()), Title: "NDA signature required from lp@vc.com for Startup Fundraising",
				Impact: "high", Status: "pending", ActionType: "sign",
				SourceType: pgText(action.SourceTypeRoomNDA),
				SourceID:   pgText(memberID), TargetID: pgText(roomID),
				CreatedAt: pgTime(now.Add(-30 * time.Minute)),
				DueAt:     pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
			// access_decay (operational renew)
			{
				ID: mustUUID(uuid.New()), Title: "Link expires soon", Impact: "high", Status: "pending",
				ActionType: "renew", SourceType: pgText(action.SourceTypeExpiringLink),
				SourceID: pgText(linkID.String()),
				CreatedAt: pgTime(now.Add(-20 * time.Minute)),
				DueAt:     pgTime(now.Add(48 * time.Hour)), UpdatedAt: pgTime(now),
			},
		},
	})

	wantMin := map[Product]int{
		ProductBuyingWindow:  1,
		ProductLeakWatch:     1,
		ProductAbuseGuard:    1,
		ProductAccessDecay:   1, // expired +/or renew
		ProductCommitmentAsk: 1,
		ProductDiligenceGate: 1,
	}
	for p, n := range wantMin {
		if feed.Counts[string(p)] < n {
			t.Fatalf("%s count=%d want>=%d counts=%v", p, feed.Counts[string(p)], n, feed.Counts)
		}
	}
	if feed.Counts["all"] != len(feed.Items) {
		t.Fatalf("all=%d items=%d", feed.Counts["all"], len(feed.Items))
	}
	if feed.NextUp == nil {
		t.Fatal("expected NextUp")
	}
}

func TestCompileSixProductExcludeBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	linkID := uuid.New()
	links := map[string]LinkMeta{linkID.String(): {ID: linkID.String(), Name: "L"}}

	t.Run("bounce never productizes", func(t *testing.T) {
		sigID := uuid.New()
		feed := Compile(CompileInput{
			WorkspaceSlug: "acme", Now: now, Links: links,
			Signals: []db.Signal{{
				ID: mustUUID(sigID), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeBounce),
				Title: "Bounce", Priority: "medium", LinkID: mustUUID(linkID), CreatedAt: pgTime(now),
			}},
			Actions: []db.ActionItem{pendingAction(sigID, "review", "", "", now)},
		})
		if len(feed.Items) != 0 {
			t.Fatalf("bounce leaked: %+v", feed.Items)
		}
	})

	t.Run("uploaded_file never diligence_gate", func(t *testing.T) {
		feed := Compile(CompileInput{
			WorkspaceSlug: "acme", Now: now,
			Actions: []db.ActionItem{{
				ID: mustUUID(uuid.New()), Title: "Review uploaded file", Impact: "medium", Status: "pending",
				ActionType: "review", SourceType: pgText(action.SourceTypeUploadedFile),
				SourceID: pgText(uuid.New().String()),
				CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			}},
		})
		if len(feed.Items) != 0 {
			t.Fatalf("uploaded_file leaked: %+v", feed.Items)
		}
	})

	t.Run("unknown action type dropped", func(t *testing.T) {
		feed := Compile(CompileInput{
			WorkspaceSlug: "acme", Now: now,
			Actions: []db.ActionItem{{
				ID: mustUUID(uuid.New()), Title: "Mystery", Impact: "low", Status: "pending",
				ActionType: "upload", CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			}},
		})
		if len(feed.Items) != 0 {
			t.Fatalf("unknown action leaked: %+v", feed.Items)
		}
	})

	t.Run("anomaly without abuse keywords is leak_watch not abuse_guard", func(t *testing.T) {
		sigID := uuid.New()
		feed := Compile(CompileInput{
			WorkspaceSlug: "acme", Now: now, Links: links,
			Signals: []db.Signal{{
				ID: mustUUID(sigID), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeAnomaly),
				Title: "Odd pattern", Description: "unusual session", Priority: "high",
				LinkID: mustUUID(linkID), CreatedAt: pgTime(now),
			}},
			Actions: []db.ActionItem{pendingAction(sigID, "review", "", "", now)},
		})
		if feed.Counts[string(ProductLeakWatch)] != 1 || feed.Counts[string(ProductAbuseGuard)] != 0 {
			t.Fatalf("counts=%v", feed.Counts)
		}
	})

	t.Run("anomaly escalate routes to commitment_ask not abuse", func(t *testing.T) {
		sigID := uuid.New()
		feed := Compile(CompileInput{
			WorkspaceSlug: "acme", Now: now, Links: links,
			Signals: []db.Signal{{
				ID: mustUUID(sigID), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeAnomaly),
				Title: "Escalated ask", Description: "host escalated visitor question", Priority: "high",
				LinkID: mustUUID(linkID), CreatedAt: pgTime(now),
			}},
			Actions: []db.ActionItem{pendingAction(sigID, "review", "", "", now)},
		})
		if feed.Counts[string(ProductCommitmentAsk)] != 1 || feed.Counts[string(ProductAbuseGuard)] != 0 {
			t.Fatalf("counts=%v", feed.Counts)
		}
	})
}

func TestCompileSixProductsPresentNextUpIsHighestValue(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	linkID := uuid.New()

	// Fresh buying window vs overdue diligence — diligence must win NextUp.
	sigHot := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Room", Scenario: ScenarioStartupFundraising}},
		Signals: []db.Signal{{
			ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
			Title: "Hot", Priority: "high", LinkID: mustUUID(linkID),
			CreatedAt: pgTime(now.Add(-10 * time.Minute)),
		}},
		Actions: []db.ActionItem{
			pendingAction(sigHot, "email", "", "", now.Add(-10*time.Minute)),
			{
				ID: mustUUID(uuid.New()), Title: "Approve gate", Impact: "medium", Status: "pending",
				ActionType: "approve", SourceType: pgText(action.SourceTypeDealRoomLinkAccessRequest),
				SourceID: pgText(linkID.String()), TargetID: pgText(roomID),
				CreatedAt: pgTime(now.Add(-5 * time.Hour)),
				DueAt:     pgTime(now.Add(-3 * time.Hour)), UpdatedAt: pgTime(now),
			},
		},
	})
	if feed.NextUp == nil || feed.NextUp.Product != ProductDiligenceGate {
		t.Fatalf("NextUp=%+v want diligence_gate", feed.NextUp)
	}
	if feed.NextUp.WhyNowCode != "sla_overdue" {
		t.Fatalf("whyNow=%s", feed.NextUp.WhyNowCode)
	}
}

func TestCompileOperatorStyleEmptyNDATitleStillGatesButMemberKeyRequiredForNav(t *testing.T) {
	// Compile still accepts a malformed title (sync layer must not emit empty actors);
	// navigation for member-keyed rows requires target_id=room.
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	memberID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "Startup Fundraising", Scenario: ScenarioStartupFundraising}},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()),
			Title:      "NDA signature required from for Startup Fundraising",
			Impact:     "high",
			Status:     "pending",
			ActionType: "sign",
			SourceType: pgText(action.SourceTypeRoomNDA),
			SourceID:   pgText(memberID),
			TargetID:   pgText(roomID),
			CreatedAt:  pgTime(now),
			DueAt:      pgTime(now.Add(time.Hour)),
			UpdatedAt:  pgTime(now),
		}},
	})
	if len(feed.Items) != 1 || feed.Items[0].Product != ProductDiligenceGate {
		t.Fatalf("items=%+v", feed.Items)
	}
	want := "/acme/deal-rooms/" + roomID + "?tab=access"
	if feed.Items[0].NavigatePath != want {
		t.Fatalf("nav=%s want %s", feed.Items[0].NavigatePath, want)
	}
}

func TestCompileDoneAndSnoozedExcluded(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "R"}},
		Actions: []db.ActionItem{
			{
				ID: mustUUID(uuid.New()), Title: "Done gate", Impact: "high", Status: "done",
				ActionType: "approve", SourceType: pgText(action.SourceTypeRoomAccessRequest),
				SourceID: pgText(roomID), CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
			{
				ID: mustUUID(uuid.New()), Title: "Snoozed gate", Impact: "high", Status: "snoozed",
				ActionType: "approve", SourceType: pgText(action.SourceTypeRoomAccessRequest),
				SourceID: pgText(roomID), CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
				SnoozedUntil: pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true},
			},
		},
	})
	if len(feed.Items) != 0 {
		t.Fatalf("done/snoozed must not appear, got %+v", feed.Items)
	}
}

func pendingAction(sigID uuid.UUID, actionType, sourceType, sourceID string, created time.Time) db.ActionItem {
	a := db.ActionItem{
		ID:         mustUUID(uuid.New()),
		Title:      actionType + " item",
		Impact:     "high",
		Status:     "pending",
		ActionType: actionType,
		CreatedAt:  pgTime(created),
		DueAt:      pgTime(created.Add(4 * time.Hour)),
		UpdatedAt:  pgTime(created),
	}
	if sigID != uuid.Nil {
		a.SignalID = mustUUID(sigID)
	}
	if sourceType != "" {
		a.SourceType = pgText(sourceType)
	}
	if sourceID != "" {
		a.SourceID = pgText(sourceID)
	}
	return a
}

func TestReasonLooksLikeAskAbuseBoundary(t *testing.T) {
	cases := []struct {
		desc string
		want bool
	}{
		{"visitor ask rate limit hit", true},
		{"ask_ai monthly quota", true},
		{"ask abuse detected", true},
		{"unusual session pattern", false},
		{"forwarded to partner", false},
	}
	for _, tc := range cases {
		sig := &db.Signal{Description: tc.desc, Title: "", Suggestion: ""}
		if got := reasonLooksLikeAskAbuse(sig); got != tc.want {
			t.Fatalf("%q → %v want %v", tc.desc, got, tc.want)
		}
	}
}

func TestSignalContextJSONRoundTripForBuyingWindow(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	linkID := uuid.New()
	sigID := uuid.New()
	ctx, _ := json.Marshal(map[string]any{
		"contactEmail":  "lp@vc.com",
		"documentTitle": "Deck",
	})
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Signals: []db.Signal{{
			ID: mustUUID(sigID), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
			Title: "Hot", Priority: "high", LinkID: mustUUID(linkID), Context: ctx,
			CreatedAt: pgTime(now),
		}},
		Actions: []db.ActionItem{pendingAction(sigID, "email", "", "", now)},
	})
	if len(feed.Items) != 1 || feed.Items[0].ContactEmail != "lp@vc.com" {
		t.Fatalf("items=%+v", feed.Items)
	}
}
