package radar

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
)

// NoiseHint explains why a product band was soft-demoted from closed-loop outcomes.
type NoiseHint struct {
	Product           Product `json:"product"`
	FalsePositiveRate float64 `json:"falsePositiveRate"`
	Sample            int     `json:"sample"`
	DemoteBoost       int     `json:"demoteBoost"`
}

// OutcomeRow is one aggregated done-outcome bucket from the DB.
type OutcomeRow struct {
	Kind    string
	Outcome string
	Count   int
}

// LearnFromOutcomes turns 30d completion outcomes into per-product demotion boosts.
// Thresholds require real volume so a few clicks cannot silence a product.
func LearnFromOutcomes(rows []OutcomeRow) (demote map[Product]int, hints []NoiseHint) {
	type agg struct {
		fp, total int
	}
	byProduct := map[Product]*agg{}

	for _, r := range rows {
		p := productFromOutcomeKind(r.Kind)
		if p == "" {
			continue
		}
		a := byProduct[p]
		if a == nil {
			a = &agg{}
			byProduct[p] = a
		}
		a.total += r.Count
		if r.Outcome == string(OutcomeFalsePositive) {
			a.fp += r.Count
		}
	}

	demote = map[Product]int{}
	for p, a := range byProduct {
		if a.total < 3 {
			continue
		}
		rate := float64(a.fp) / float64(a.total)
		boost := 0
		switch {
		case a.total >= 5 && rate >= 0.5:
			// Push past same-band P0/P2 ties on founder lens (leak=0 → +3 clears buying=2).
			boost = 3
		case rate >= 0.35:
			boost = 2
		}
		if boost == 0 {
			continue
		}
		demote[p] = boost
		hints = append(hints, NoiseHint{
			Product:           p,
			FalsePositiveRate: round2(rate),
			Sample:            a.total,
			DemoteBoost:       boost,
		})
	}
	return demote, hints
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
