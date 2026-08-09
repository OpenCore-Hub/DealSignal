package radar

import (
	"fmt"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
)

// Stress / scale gates for the six high-value products. Keep deterministic and
// budgeted so CI catches O(n²) regressions in Compile coalesce/rank.

func buildMixedCompileInput(n int, now time.Time) CompileInput {
	if n < 6 {
		n = 6
	}
	rooms := map[string]RoomMeta{}
	links := map[string]LinkMeta{}
	signals := make([]db.Signal, 0, n)
	actions := make([]db.ActionItem, 0, n)

	products := []Product{
		ProductBuyingWindow, ProductDiligenceGate, ProductCommitmentAsk,
		ProductLeakWatch, ProductAccessDecay, ProductAbuseGuard,
	}

	for i := 0; i < n; i++ {
		p := products[i%len(products)]
		roomID := fmt.Sprintf("room-%d", i%17)
		linkID := uuid.New()
		rooms[roomID] = RoomMeta{Name: "Room " + roomID, Scenario: ScenarioStartupFundraising}
		links[linkID.String()] = LinkMeta{ID: linkID.String(), Name: "Link", DealRoomID: roomID}
		created := now.Add(-time.Duration(i) * time.Minute)

		switch p {
		case ProductBuyingWindow:
			sigID := uuid.New()
			signals = append(signals, db.Signal{
				ID: mustUUID(sigID), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Hot", Priority: "high", LinkID: mustUUID(linkID), CreatedAt: pgTime(created),
			})
			actions = append(actions, pendingAction(sigID, "email", "", "", created))
		case ProductDiligenceGate:
			actions = append(actions, db.ActionItem{
				ID: mustUUID(uuid.New()), Title: "Approve gate", Impact: "high", Status: "pending",
				ActionType: "approve", SourceType: pgText(action.SourceTypeDealRoomLinkAccessRequest),
				SourceID: pgText(linkID.String()), TargetID: pgText(roomID),
				CreatedAt: pgTime(created), DueAt: pgTime(created.Add(2 * time.Hour)), UpdatedAt: pgTime(now),
			})
		case ProductCommitmentAsk:
			actions = append(actions, db.ActionItem{
				ID: mustUUID(uuid.New()), Title: "Answer ask", Impact: "high", Status: "pending",
				ActionType: "answer", SourceType: pgText(action.SourceTypeDealRoomLinkQuestion),
				SourceID: pgText(uuid.New().String()), TargetID: pgText(roomID + "/" + linkID.String()),
				CreatedAt: pgTime(created), DueAt: pgTime(created.Add(4 * time.Hour)), UpdatedAt: pgTime(now),
			})
		case ProductLeakWatch:
			sigID := uuid.New()
			signals = append(signals, db.Signal{
				ID: mustUUID(sigID), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeForward),
				Title: "Forward", Priority: "high", LinkID: mustUUID(linkID), CreatedAt: pgTime(created),
			})
			actions = append(actions, pendingAction(sigID, "review", "", "", created))
		case ProductAccessDecay:
			actions = append(actions, db.ActionItem{
				ID: mustUUID(uuid.New()), Title: "Renew link", Impact: "high", Status: "pending",
				ActionType: "renew", SourceType: pgText(action.SourceTypeExpiringLink),
				SourceID: pgText(linkID.String()),
				CreatedAt: pgTime(created), DueAt: pgTime(created.Add(48 * time.Hour)), UpdatedAt: pgTime(now),
			})
		case ProductAbuseGuard:
			sigID := uuid.New()
			signals = append(signals, db.Signal{
				ID: mustUUID(sigID), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeAnomaly),
				Title: "Ask abuse", Description: "ask_ai rate limit", Priority: "high",
				LinkID: mustUUID(linkID), CreatedAt: pgTime(created),
			})
			actions = append(actions, pendingAction(sigID, "review", "", "", created))
		}
	}

	return CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         rooms,
		Links:         links,
		Signals:       signals,
		Actions:       actions,
	}
}

func TestCompileStressMixedProductsBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("stress budget skipped in -short")
	}
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	const n = 3000
	in := buildMixedCompileInput(n, now)

	start := time.Now()
	feed := Compile(in)
	elapsed := time.Since(start)
	if elapsed > 750*time.Millisecond {
		t.Fatalf("Compile(%d) took %s (budget 750ms)", n, elapsed)
	}
	if feed.Counts["all"] < n/2 {
		// Coalesce may reduce buying_window cards; still expect substantial volume.
		t.Fatalf("all=%d unexpectedly low for n=%d", feed.Counts["all"], n)
	}
	for _, p := range []Product{
		ProductBuyingWindow, ProductDiligenceGate, ProductCommitmentAsk,
		ProductLeakWatch, ProductAccessDecay, ProductAbuseGuard,
	} {
		if feed.Counts[string(p)] == 0 {
			t.Fatalf("missing product %s in stress mix counts=%v", p, feed.Counts)
		}
	}
	sum := 0
	for _, p := range []string{
		"buying_window", "diligence_gate", "commitment_ask",
		"leak_watch", "access_decay", "abuse_guard",
	} {
		sum += feed.Counts[p]
	}
	if sum != feed.Counts["all"] {
		t.Fatalf("product counts sum=%d all=%d", sum, feed.Counts["all"])
	}
	if feed.NextUp == nil {
		t.Fatal("expected NextUp under load")
	}
	if len(feed.Strands) == 0 {
		t.Fatal("expected strands under load")
	}
}

func TestCompileStressDeterministicNextUp(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	in := buildMixedCompileInput(600, now)
	a := Compile(in)
	b := Compile(in)
	if a.NextUp == nil || b.NextUp == nil {
		t.Fatal("missing NextUp")
	}
	if a.NextUp.ID != b.NextUp.ID || a.NextUp.Product != b.NextUp.Product {
		t.Fatalf("non-deterministic NextUp: %+v vs %+v", a.NextUp, b.NextUp)
	}
	if len(a.Items) != len(b.Items) {
		t.Fatalf("item len %d vs %d", len(a.Items), len(b.Items))
	}
	for i := range a.Items {
		if a.Items[i].ID != b.Items[i].ID {
			t.Fatalf("rank order drift at %d: %s vs %s", i, a.Items[i].ID, b.Items[i].ID)
		}
	}
}

func TestCompileStressBuyingWindowCoalesceUnderLoad(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	linkID := uuid.New()
	contact := uuid.New()
	const pairs = 40
	signals := make([]db.Signal, 0, pairs)
	actions := make([]db.ActionItem, 0, pairs)
	ctxEmail, _ := jsonMarshalContact("same@vc.com")

	for i := 0; i < pairs; i++ {
		sigID := uuid.New()
		signals = append(signals, db.Signal{
			ID: mustUUID(sigID), Type: "hot_signal",
			Subtype: pgText(suggestions.SubtypeHot), Title: "Hot", Priority: "high",
			LinkID: mustUUID(linkID), ContactID: mustUUID(contact),
			Context: ctxEmail, CreatedAt: pgTime(now.Add(-time.Duration(i) * time.Minute)),
		})
		actions = append(actions, pendingAction(sigID, "email", "", "", now.Add(-time.Duration(i)*time.Minute)))
	}

	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Links:         map[string]LinkMeta{linkID.String(): {ID: linkID.String(), Name: "Deck"}},
		Signals:       signals,
		Actions:       actions,
	})
	if feed.Counts[string(ProductBuyingWindow)] != 1 {
		t.Fatalf("expected coalesce to 1 buying_window, got %d", feed.Counts[string(ProductBuyingWindow)])
	}
	if len(feed.Items[0].CoalescedFrom) < pairs-1 {
		t.Fatalf("coalescedFrom=%d want >= %d", len(feed.Items[0].CoalescedFrom), pairs-1)
	}
}

func TestCompileStressIgnoresUploadedFileFlood(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	actions := make([]db.ActionItem, 0, 500)
	for i := 0; i < 500; i++ {
		actions = append(actions, db.ActionItem{
			ID: mustUUID(uuid.New()), Title: "Review file", Impact: "medium", Status: "pending",
			ActionType: "review", SourceType: pgText(action.SourceTypeUploadedFile),
			SourceID: pgText(uuid.New().String()),
			CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		})
	}
	// One real diligence item must still surface.
	roomID := uuid.New().String()
	actions = append(actions, db.ActionItem{
		ID: mustUUID(uuid.New()), Title: "Approve", Impact: "high", Status: "pending",
		ActionType: "approve", SourceType: pgText(action.SourceTypeRoomAccessRequest),
		SourceID: pgText(roomID), CreatedAt: pgTime(now), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
	})
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms:         map[string]RoomMeta{roomID: {Name: "R"}},
		Actions:       actions,
	})
	if feed.Counts["all"] != 1 || feed.Counts[string(ProductDiligenceGate)] != 1 {
		t.Fatalf("uploaded flood must not enter radar; counts=%v", feed.Counts)
	}
}

func BenchmarkCompileMixed3000(b *testing.B) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	in := buildMixedCompileInput(3000, now)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Compile(in)
	}
}

func jsonMarshalContact(email string) ([]byte, error) {
	return []byte(`{"contactEmail":"` + email + `"}`), nil
}
