package radar

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
)

func TestCompileDropsWorkspaceMemberActors(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	linkID := uuid.New()
	ownerEmail := "owner@acme.com"
	lpEmail := "lp@vc.com"

	sigOwner := uuid.New()
	sigLP := uuid.New()
	ctxOwner, _ := json.Marshal(map[string]any{"contactEmail": ownerEmail})
	ctxLP, _ := json.Marshal(map[string]any{"contactEmail": lpEmail})

	feed := Compile(CompileInput{
		WorkspaceSlug:  "acme",
		Now:            now,
		InternalEmails: action.NewMemberEmailSet([]string{ownerEmail, "ops@acme.com"}),
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Fundraise", Scenario: ScenarioStartupFundraising},
		},
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "Deck", DealRoomID: roomID},
		},
		Signals: []db.Signal{
			{
				ID: mustUUID(sigOwner), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Hot", Priority: "high", LinkID: mustUUID(linkID), Context: ctxOwner,
				CreatedAt: pgTime(now.Add(-time.Hour)),
			},
			{
				ID: mustUUID(sigLP), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Hot", Priority: "high", LinkID: mustUUID(linkID), Context: ctxLP,
				CreatedAt: pgTime(now.Add(-30 * time.Minute)),
			},
		},
		Actions: []db.ActionItem{
			pendingAction(sigOwner, "email", "", "", now.Add(-time.Hour)),
			pendingAction(sigLP, "email", "", "", now.Add(-30*time.Minute)),
			{
				ID: mustUUID(uuid.New()), Title: "Approve access request from owner@acme.com for Deck",
				Impact: "high", Status: "pending", ActionType: "approve",
				SourceType: pgText(action.SourceTypeLinkAccessRequest),
				SourceID:   pgText(linkID.String()),
				CreatedAt:  pgTime(now.Add(-20 * time.Minute)),
			},
			{
				ID: mustUUID(uuid.New()), Title: "Approve access request from lp@vc.com for Deck",
				Impact: "high", Status: "pending", ActionType: "approve",
				SourceType: pgText(action.SourceTypeLinkAccessRequest),
				SourceID:   pgText(linkID.String()),
				CreatedAt:  pgTime(now.Add(-10 * time.Minute)),
			},
			{
				ID: mustUUID(uuid.New()), Title: "Link Deck expires soon",
				Impact: "high", Status: "pending", ActionType: "renew",
				SourceType: pgText(action.SourceTypeExpiringLink),
				SourceID:   pgText(linkID.String()),
				CreatedAt:  pgTime(now.Add(-5 * time.Minute)),
			},
		},
	})

	// Hot/Buying Window must NOT drop when signal.contactEmail is a member
	// (that's first link contact, not the event actor) — both heat cards stay.
	if feed.Counts["buying_window"] != 2 {
		t.Fatalf("buying_window=%d want 2 (heat cards keep signal contactEmail)", feed.Counts["buying_window"])
	}
	if feed.Counts["diligence_gate"] != 1 {
		t.Fatalf("diligence_gate=%d want 1 (LP access only; owner access dropped)", feed.Counts["diligence_gate"])
	}
	if feed.Counts["access_decay"] != 1 {
		t.Fatalf("access_decay=%d want 1 (expiring host ops kept)", feed.Counts["access_decay"])
	}
	for _, it := range feed.Items {
		if it.Product == ProductDiligenceGate && (it.ContactEmail == ownerEmail || it.Actor == ownerEmail) {
			t.Fatalf("owner access request leaked into diligence_gate: %+v", it)
		}
		if it.Product == ProductDiligenceGate && strings.Contains(strings.ToLower(it.Headline), ownerEmail) {
			t.Fatalf("owner email in diligence headline: %q", it.Headline)
		}
	}
}

func TestEmailFromActionTitle(t *testing.T) {
	got := emailFromActionTitle("NDA signature required from lp@vc.com for Startup Fundraising")
	if got != "lp@vc.com" {
		t.Fatalf("got %q", got)
	}
	got = emailFromActionTitle(`Approve access request from "lp@vc.com" for Deck`)
	if got != "lp@vc.com" {
		t.Fatalf("quoted email: got %q", got)
	}
	if emailFromActionTitle("Link Deck expires soon") != "" {
		t.Fatal("expiring titles have no actor email")
	}
	if emailFromActionTitle("Approve access request from unknown visitor for Deck") != "" {
		t.Fatal("non-email token after from must not match")
	}
	// Must not attribute the target clause (email-shaped room/link name).
	if emailFromActionTitle("NDA signature required from visitor for team@acme.com Room") != "" {
		t.Fatal("target email must not become actor")
	}
}
