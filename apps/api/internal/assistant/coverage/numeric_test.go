package coverage

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
)

func TestExtractFromText_Percent(t *testing.T) {
	v, ok := extractFromText(ValueTypePercent, "The option pool is reserved at 15% on a fully diluted basis.")
	if !ok || v != "15%" {
		t.Fatalf("got %q ok=%v", v, ok)
	}
	v, ok = extractFromText(ValueTypePercent, "期权池占比百分之12.5")
	if !ok || !strings.Contains(v, "12.5") {
		t.Fatalf("got %q ok=%v", v, ok)
	}
}

func TestExtractFromText_Money(t *testing.T) {
	v, ok := extractFromText(ValueTypeMoney, "ARR reached $1.2M in FY2025.")
	if !ok || v != "$1.2M" {
		t.Fatalf("got %q ok=%v", v, ok)
	}
	v, ok = extractFromText(ValueTypeMoney, "年度经常性收入约 人民币 850万。")
	if !ok {
		t.Fatalf("expected money extract, got %q", v)
	}
}

func TestExtractFromText_Share(t *testing.T) {
	v, ok := extractFromText(ValueTypeShare, "Fully diluted share capital is 12,345,678 shares.")
	if !ok || v != "12,345,678 shares" {
		t.Fatalf("got %q ok=%v", v, ok)
	}
}

func TestAttachExtractedValues(t *testing.T) {
	pack := mustFinancingPack(t)
	rows := []CoverageRow{
		{
			ItemID: "option_pool",
			Label:  "Option / ESOP pool",
			Status: StatusSupported,
			Clues: []search.Evidence{{
				ChunkID: uuid.NewString(),
				Quote:   "ESOP pool remains at 10% post-money.",
				Score:   0.9,
			}},
		},
		{
			ItemID: "revenue_metrics",
			Label:  "Revenue / ARR metrics",
			Status: StatusSupported,
			Clues: []search.Evidence{{
				ChunkID: uuid.NewString(),
				Quote:   "Trailing ARR is USD 4.5M.",
				Score:   0.8,
			}},
		},
		{
			ItemID: "cap_table",
			Label:  "Cap table",
			Status: StatusSupported,
			Clues: []search.Evidence{{
				Quote: "founder owns 40%",
				Score: 0.7,
			}},
		},
	}
	out := AttachExtractedValues(rows, pack)
	if out[0].ValueType != ValueTypePercent || out[0].ExtractedValue != "10%" {
		t.Fatalf("option_pool: %+v", out[0])
	}
	if out[1].ValueType != ValueTypeMoney || out[1].ExtractedValue == "" {
		t.Fatalf("revenue: %+v", out[1])
	}
	if out[2].ValueType != "" || out[2].ExtractedValue != "" {
		t.Fatalf("cap_table should have no value_type extract: %+v", out[2])
	}
}

func TestAttachExtractedValues_SkipsAbsent(t *testing.T) {
	pack := mustFinancingPack(t)
	rows := []CoverageRow{{
		ItemID: "option_pool",
		Status: StatusAbsentInScope,
		Clues:  []search.Evidence{},
	}}
	out := AttachExtractedValues(rows, pack)
	if out[0].ValueType != ValueTypePercent {
		t.Fatalf("value_type should still be set: %+v", out[0])
	}
	if out[0].ExtractedValue != "" {
		t.Fatalf("absent row must not extract: %+v", out[0])
	}
}
