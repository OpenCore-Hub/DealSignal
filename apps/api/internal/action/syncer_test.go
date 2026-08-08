package action

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestTitleForSurfaceIsolation(t *testing.T) {
	doc := titleFor(SourceTypeLinkAccessRequest, "a@x.com", "Pitch Deck")
	roomShare := titleFor(SourceTypeDealRoomLinkAccessRequest, "a@x.com", "Investor Link")
	roomMember := titleFor(SourceTypeRoomAccessRequest, "a@x.com", "Series A")

	if doc == roomShare {
		t.Fatalf("document and deal-room share titles must differ: %q", doc)
	}
	if roomShare == roomMember {
		t.Fatalf("deal-room share and membership titles must differ: %q", roomShare)
	}
	if got := titleFor(SourceTypeRoomNDA, "a@x.com", "Series A"); got == "" {
		t.Fatal("expected NDA title")
	}
}

func TestImpactForAccessSurfaces(t *testing.T) {
	for _, st := range []string{
		SourceTypeLinkAccessRequest,
		SourceTypeDealRoomLinkAccessRequest,
		SourceTypeRoomAccessRequest,
		SourceTypeRoomNDA,
	} {
		if impactFor(st) != "high" {
			t.Fatalf("%s impact want high", st)
		}
	}
}

func TestSourceTypeConstantsStayDistinct(t *testing.T) {
	seen := map[string]string{}
	for name, value := range map[string]string{
		"link":       SourceTypeLinkAccessRequest,
		"roomLink":   SourceTypeDealRoomLinkAccessRequest,
		"roomMem":    SourceTypeRoomAccessRequest,
		"nda":        SourceTypeRoomNDA,
		"roomAsk":    SourceTypeDealRoomLinkQuestion,
	} {
		if other, ok := seen[value]; ok {
			t.Fatalf("source type collision: %s and %s both %q", name, other, value)
		}
		seen[value] = name
	}
}

func TestDealRoomAskTargetID(t *testing.T) {
	room := uuid.New()
	link := uuid.New()
	got := dealRoomAskTargetID(
		pgtype.UUID{Bytes: room, Valid: true},
		pgtype.UUID{Bytes: link, Valid: true},
	)
	want := room.String() + "/" + link.String()
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTitleForDealRoomLinkQuestion(t *testing.T) {
	got := titleFor(SourceTypeDealRoomLinkQuestion, "a@x.com", "Investor Link")
	if got == "" || got == titleFor(SourceTypeLinkQuestion, "a@x.com", "Investor Link") {
		t.Fatalf("deal-room ask title should differ from legacy link_question: %q", got)
	}
}

func TestTitleForFormalAskReview(t *testing.T) {
	got := titleForAction(SourceTypeDealRoomLinkQuestion, operationalActionTypeReview, "a@x.com", "Investor Link")
	if got == "" || got == titleFor(SourceTypeDealRoomLinkQuestion, "a@x.com", "Investor Link") {
		t.Fatalf("formal review title should differ from host answer: %q", got)
	}
	if !strings.Contains(got, "formal") {
		t.Fatalf("expected formal wording, got %q", got)
	}
	lib := titleForAction(SourceTypeLinkQuestion, operationalActionTypeReview, "a@x.com", "Pitch Deck Link")
	if !strings.Contains(lib, "formal") {
		t.Fatalf("library formal review should use formal wording, got %q", lib)
	}
}
