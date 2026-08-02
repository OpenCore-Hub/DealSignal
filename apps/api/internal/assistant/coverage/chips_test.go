package coverage

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
)

func TestVisitorChips_FinancingLabels(t *testing.T) {
	reg := jobs.MustLoadBuiltinPacks()
	chips := VisitorChips(reg, "", "en")
	if len(chips) != 20 {
		t.Fatalf("chips=%d", len(chips))
	}
	if chips[0].ItemID != "cap_table" || chips[0].Label == "" {
		t.Fatalf("first chip=%+v", chips[0])
	}
	zh := VisitorChips(reg, jobs.FinancingDDV1, "zh-CN")
	if zh[0].Label == chips[0].Label {
		t.Fatalf("expected zh label to differ from en for cap_table")
	}
}

func TestValidChecklistItemID(t *testing.T) {
	reg := jobs.MustLoadBuiltinPacks()
	if !ValidChecklistItemID(reg, "", "cap_table") {
		t.Fatal("cap_table should be valid")
	}
	if ValidChecklistItemID(reg, "", "nope") {
		t.Fatal("unknown id must be invalid")
	}
}

func TestChipPrefillQuestion(t *testing.T) {
	if got := ChipPrefillQuestion("zh-CN", "期权池"); got != "有没有期权池" {
		t.Fatalf("zh got %q", got)
	}
	if got := ChipPrefillQuestion("en", "Cap table"); !strings.Contains(got, "Cap table") {
		t.Fatalf("en got %q", got)
	}
}
