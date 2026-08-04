package knowledge

import (
	"strings"
	"unicode/utf8"
)

const (
	// costUnitRunes is the rune budget per cost unit (evidence + answer work proxy).
	costUnitRunes = 1000
)

// estimateCostUnits approximates ask cost from answer + hit text volume.
// Deterministic, no LLM tokens required — 1 unit ≈ 1k runes of desk work.
func estimateCostUnits(answer string, hits []QueryHit) int {
	n := utf8.RuneCountInString(strings.TrimSpace(answer))
	for _, h := range hits {
		n += utf8.RuneCountInString(strings.TrimSpace(h.Text))
	}
	if n <= 0 {
		return 0
	}
	units := (n + costUnitRunes - 1) / costUnitRunes
	if units < 1 {
		units = 1
	}
	return units
}
