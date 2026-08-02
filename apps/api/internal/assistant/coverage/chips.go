package coverage

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
)

// Chip is a visitor-facing suggested checklist label (no status / no gap table).
type Chip struct {
	ItemID string `json:"item_id"`
	Label  string `json:"label"`
}

// VisitorChips returns pack labels for visitor suggested-check chips.
func VisitorChips(reg *jobs.PackRegistry, packID, lang string) []Chip {
	if reg == nil {
		return nil
	}
	if packID == "" {
		packID = jobs.FinancingDDV1
	}
	pack, ok := reg.Get(packID)
	if !ok {
		return nil
	}
	out := make([]Chip, 0, len(pack.Items))
	for _, it := range pack.Items {
		label := it.LabelFor(lang)
		if strings.TrimSpace(label) == "" {
			continue
		}
		out = append(out, Chip{ItemID: it.ID, Label: label})
	}
	return out
}

// ValidChecklistItemID reports whether itemID exists in the pack.
func ValidChecklistItemID(reg *jobs.PackRegistry, packID, itemID string) bool {
	if reg == nil || strings.TrimSpace(itemID) == "" {
		return false
	}
	if packID == "" {
		packID = jobs.FinancingDDV1
	}
	pack, ok := reg.Get(packID)
	if !ok {
		return false
	}
	for _, it := range pack.Items {
		if it.ID == itemID {
			return true
		}
	}
	return false
}

// ChipPrefillQuestion builds the visitor Ask Docs message for a chip click (absence-style).
func ChipPrefillQuestion(lang, label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		return "有没有" + label
	}
	return "Is there documentation covering " + label + "?"
}
