package action

import "testing"

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
		"link":     SourceTypeLinkAccessRequest,
		"roomLink": SourceTypeDealRoomLinkAccessRequest,
		"roomMem":  SourceTypeRoomAccessRequest,
		"nda":      SourceTypeRoomNDA,
	} {
		if other, ok := seen[value]; ok {
			t.Fatalf("source type collision: %s and %s both %q", name, other, value)
		}
		seen[value] = name
	}
}
