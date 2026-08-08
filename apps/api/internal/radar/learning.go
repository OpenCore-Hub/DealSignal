package radar

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
)

// NoiseHint explains why a product band was soft-demoted from closed-loop outcomes.
type NoiseHint struct {
	Scenario          string  `json:"scenario,omitempty"`
	Product           Product `json:"product"`
	FalsePositiveRate float64 `json:"falsePositiveRate"`
	Sample            int     `json:"sample"`
	DemoteBoost       int     `json:"demoteBoost"`
}

// OutcomeRow is one aggregated done-outcome bucket from the DB.
type OutcomeRow struct {
	// Scenario is the normalized deal-room scenario (empty = workspace-global / unknown).
	Scenario string
	Kind     string
	Outcome  string
	Count    int
}

type outcomeAgg struct {
	fp, total int
}

// LearnFromOutcomes turns 30d completion outcomes into demotion boosts.
// Phase C: prefers scenario×product buckets; also returns workspace-global demotes
// for items without a room scenario.
func LearnFromOutcomes(rows []OutcomeRow) (global map[Product]int, byScenario map[Scenario]map[Product]int, hints []NoiseHint) {
	globalAgg := map[Product]*outcomeAgg{}
	scenarioAgg := map[Scenario]map[Product]*outcomeAgg{}

	for _, r := range rows {
		p := productFromOutcomeKind(r.Kind)
		if p == "" {
			continue
		}
		ga := globalAgg[p]
		if ga == nil {
			ga = &outcomeAgg{}
			globalAgg[p] = ga
		}
		ga.total += r.Count
		if r.Outcome == string(OutcomeFalsePositive) {
			ga.fp += r.Count
		}

		sc := NormalizeScenario(r.Scenario)
		if sc == ScenarioUnknown {
			continue
		}
		byProd := scenarioAgg[sc]
		if byProd == nil {
			byProd = map[Product]*outcomeAgg{}
			scenarioAgg[sc] = byProd
		}
		sa := byProd[p]
		if sa == nil {
			sa = &outcomeAgg{}
			byProd[p] = sa
		}
		sa.total += r.Count
		if r.Outcome == string(OutcomeFalsePositive) {
			sa.fp += r.Count
		}
	}

	global = map[Product]int{}
	for p, a := range globalAgg {
		boost, rate := demoteFromAgg(a)
		if boost == 0 {
			continue
		}
		global[p] = boost
		hints = append(hints, NoiseHint{
			Product:           p,
			FalsePositiveRate: round2(rate),
			Sample:            a.total,
			DemoteBoost:       boost,
		})
	}

	byScenario = map[Scenario]map[Product]int{}
	for sc, byProd := range scenarioAgg {
		for p, a := range byProd {
			boost, rate := demoteFromAgg(a)
			if boost == 0 {
				continue
			}
			if byScenario[sc] == nil {
				byScenario[sc] = map[Product]int{}
			}
			byScenario[sc][p] = boost
			hints = append(hints, NoiseHint{
				Scenario:          string(sc),
				Product:           p,
				FalsePositiveRate: round2(rate),
				Sample:            a.total,
				DemoteBoost:       boost,
			})
		}
	}
	return global, byScenario, hints
}

func demoteFromAgg(a *outcomeAgg) (boost int, rate float64) {
	if a == nil || a.total < 3 {
		return 0, 0
	}
	rate = float64(a.fp) / float64(a.total)
	switch {
	case a.total >= 5 && rate >= 0.5:
		return 3, rate
	case rate >= 0.35:
		return 2, rate
	default:
		return 0, rate
	}
}

// demoteBoostForItem applies scenario×product demotion with hard isolation:
// a known scenario never inherits another scenario's (or workspace-global) noise.
// Workspace-global demotes apply only when the item has no room scenario.
func demoteBoostForItem(global map[Product]int, byScenario map[Scenario]map[Product]int, sc Scenario, p Product) int {
	if sc != ScenarioUnknown {
		if byScenario == nil {
			return 0
		}
		if m := byScenario[sc]; m != nil {
			return m[p]
		}
		return 0
	}
	if global == nil {
		return 0
	}
	return global[p]
}

func productFromOutcomeKind(kind string) Product {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case suggestions.SubtypeForward, suggestions.SubtypeDownload,
		suggestions.SubtypeBlockedAttempt, suggestions.SubtypeCaptureAttempt,
		suggestions.SubtypeAnomaly, "review":
		return ProductLeakWatch
	case "approve", "sign", "verify":
		return ProductDiligenceGate
	case suggestions.SubtypeQuestion, suggestions.SubtypeFormalAsk, "answer":
		return ProductCommitmentAsk
	case suggestions.SubtypeHot, suggestions.SubtypeKeyPage, suggestions.SubtypeRevisit,
		"email", "call", "open":
		return ProductBuyingWindow
	case suggestions.SubtypeExpired, suggestions.SubtypeAccessExhausted,
		suggestions.SubtypeAccessRevoked, "renew":
		return ProductAccessDecay
	case "rate_limit_exceeded", "ask_ai_rate_limited":
		return ProductAbuseGuard
	default:
		return ""
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// microRank breaks ties inside a product band (higher = sooner).
func microRank(product Product, aFormalAsk bool, subtype string, escalateAsk bool) int {
	switch product {
	case ProductCommitmentAsk:
		if aFormalAsk || subtype == suggestions.SubtypeFormalAsk {
			return 3
		}
		if escalateAsk {
			return 2
		}
		return 1
	case ProductBuyingWindow:
		switch subtype {
		case suggestions.SubtypeKeyPage:
			return 3
		case suggestions.SubtypeHot:
			return 2
		case suggestions.SubtypeRevisit:
			return 1
		default:
			return 0
		}
	default:
		return 0
	}
}
