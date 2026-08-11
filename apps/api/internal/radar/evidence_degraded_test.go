package radar

import (
	"testing"
)

func TestEvidencePackNoteDegradedDedupes(t *testing.T) {
	var pack EvidencePack
	pack.noteDegraded("metrics")
	pack.noteDegraded("metrics")
	pack.noteDegraded("top_pages")
	if len(pack.DegradedSections) != 2 {
		t.Fatalf("degraded=%v", pack.DegradedSections)
	}
	if pack.DegradedSections[0] != "metrics" || pack.DegradedSections[1] != "top_pages" {
		t.Fatalf("order=%v", pack.DegradedSections)
	}
	pack.noteDegraded("")
	if len(pack.DegradedSections) != 2 {
		t.Fatalf("empty section should be ignored: %v", pack.DegradedSections)
	}
}
