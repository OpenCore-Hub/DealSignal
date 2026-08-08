package radar

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
)

// Scenario is a deal-room template business scenario (not a role lens).
type Scenario string

const (
	ScenarioStartupFundraising   Scenario = "startup-fundraising"
	ScenarioRaisingFirstFund     Scenario = "raising-first-fund"
	ScenarioMAAcquisition        Scenario = "ma-acquisition"
	ScenarioSeriesAPlus          Scenario = "series-a-plus"
	ScenarioRealEstate           Scenario = "real-estate-transaction"
	ScenarioFundManagement       Scenario = "fund-management"
	ScenarioPortfolioManagement  Scenario = "portfolio-management"
	ScenarioProjectManagement    Scenario = "project-management"
	ScenarioSalesDataRoom        Scenario = "sales-dataroom"
	ScenarioCustom               Scenario = "custom"
	ScenarioUnknown              Scenario = ""
)

// PackDepth marks how far a scenario pack has been productized.
type PackDepth string

const (
	PackDepthBase PackDepth = "base" // Phase A: lens + rank + whyNow narrative
	PackDepthP0   PackDepth = "p0"   // Phase B: key pages + gates + CTA + insights KPI
	PackDepthP1   PackDepth = "p1"   // Phase C: IR packs + scenario×product learning
	PackDepthLite PackDepth = "lite" // Phase D: Real Estate / Project — key pages + Diligence/Access
)

// Pack is the Scenario Pack: default role lens + product urgency + Phase B deep fields.
// FE narrative lives under radar.scenario.<id>.* (ids/codes only). LabelEN/DigestLead
// are English-only for Insights daily digest email/Slack (no FE i18n there).
type Pack struct {
	Scenario      Scenario
	DefaultCircle heat.Circle
	Depth         PackDepth
	// LabelEN is the English scenario name for digests / server logs.
	LabelEN string
	// DigestLead is the English weekly-focus sentence for the digest skeleton.
	DigestLead string
	// ProductRank is absolute urgency (lower = more urgent), covering all six products.
	ProductRank map[Product]int
	// KeyPageExtra is additive to the default-circle key-page dictionary (heat RuleSet).
	KeyPageExtra map[string][]string
	// GateBoostSources elevate diligence micro-rank when action.SourceType matches.
	GateBoostSources []string
	// VerbByProduct overrides the primary CTA verb for a product in this scenario.
	VerbByProduct map[Product]Verb
	// HeadlineCodeByProduct emits i18n codes (FE: radar.scenario.<id>.headline.<code>).
	HeadlineCodeByProduct map[Product]string
	// SLAHours optionally shortens product SLA (hours from created). Commitment ask stays EOD.
	SLAHours map[Product]int
	// InsightsKPI is the ordered KPI strip ids for Insights overview (computed server-side).
	InsightsKPI []string
}

var scenarioPacks = map[Scenario]Pack{
	ScenarioStartupFundraising: {
		Scenario:      ScenarioStartupFundraising,
		DefaultCircle: heat.CircleFounder,
		LabelEN:       "Startup fundraising",
		ProductRank: map[Product]int{
			ProductDiligenceGate: 0,
			ProductLeakWatch:     0,
			ProductCommitmentAsk: 1,
			ProductBuyingWindow:  2,
			ProductAccessDecay:   3,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioSeriesAPlus: {
		Scenario:      ScenarioSeriesAPlus,
		DefaultCircle: heat.CircleFounder,
		LabelEN:       "Series A+",
		ProductRank: map[Product]int{
			ProductDiligenceGate: 0,
			ProductBuyingWindow:  1,
			ProductCommitmentAsk: 1,
			ProductLeakWatch:     2,
			ProductAccessDecay:   3,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioMAAcquisition: {
		Scenario:      ScenarioMAAcquisition,
		DefaultCircle: heat.CircleFounder,
		LabelEN:       "M&A acquisition",
		ProductRank: map[Product]int{
			ProductDiligenceGate: 0,
			ProductAccessDecay:   0,
			ProductLeakWatch:     1,
			ProductCommitmentAsk: 2,
			ProductBuyingWindow:  3,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioSalesDataRoom: {
		Scenario:      ScenarioSalesDataRoom,
		DefaultCircle: heat.CircleSales,
		LabelEN:       "Sales data room",
		ProductRank: map[Product]int{
			ProductBuyingWindow:  0,
			ProductAccessDecay:   1,
			ProductCommitmentAsk: 1,
			ProductDiligenceGate: 2,
			ProductLeakWatch:     2,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioRaisingFirstFund: {
		Scenario:      ScenarioRaisingFirstFund,
		DefaultCircle: heat.CircleInvestor,
		LabelEN:       "Raising first fund",
		ProductRank: map[Product]int{
			ProductBuyingWindow:  0,
			ProductLeakWatch:     0,
			ProductCommitmentAsk: 1,
			ProductDiligenceGate: 2,
			ProductAccessDecay:   2,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioFundManagement: {
		Scenario:      ScenarioFundManagement,
		DefaultCircle: heat.CircleInvestor,
		LabelEN:       "Fund management",
		ProductRank: map[Product]int{
			ProductAccessDecay:   0,
			ProductCommitmentAsk: 0,
			ProductLeakWatch:     1,
			ProductBuyingWindow:  2,
			ProductDiligenceGate: 2,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioPortfolioManagement: {
		Scenario:      ScenarioPortfolioManagement,
		DefaultCircle: heat.CircleInvestor,
		LabelEN:       "Portfolio management",
		ProductRank: map[Product]int{
			ProductBuyingWindow:  0,
			ProductDiligenceGate: 1,
			ProductCommitmentAsk: 1,
			ProductLeakWatch:     2,
			ProductAccessDecay:   2,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioRealEstate: {
		Scenario:      ScenarioRealEstate,
		DefaultCircle: heat.CircleFounder,
		LabelEN:       "Real estate transaction",
		// Phase D lite contract: Diligence · Access primary; Leak is secondary.
		ProductRank: map[Product]int{
			ProductDiligenceGate: 0,
			ProductAccessDecay:   1,
			ProductCommitmentAsk: 2,
			ProductBuyingWindow:  3,
			ProductLeakWatch:     3,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioProjectManagement: {
		Scenario:      ScenarioProjectManagement,
		DefaultCircle: heat.CircleFounder,
		LabelEN:       "Project management",
		ProductRank: map[Product]int{
			ProductCommitmentAsk: 0,
			ProductAccessDecay:   1,
			ProductDiligenceGate: 2,
			ProductBuyingWindow:  2,
			ProductLeakWatch:     3,
			ProductAbuseGuard:    3,
		},
	},
	ScenarioCustom: {
		Scenario:      ScenarioCustom,
		DefaultCircle: heat.CircleFounder,
		LabelEN:       "Custom room",
		ProductRank:   nil, // fall back to circle ranks
	},
}

// NormalizeScenario maps stored template_type / API template values onto Scenario.
// Accepts tmpl_startup_fundraising, tmpl-startup-fundraising, startup-fundraising, etc.
func NormalizeScenario(raw string) Scenario {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return ScenarioUnknown
	}
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.TrimPrefix(s, "tmpl-")
	switch Scenario(s) {
	case ScenarioStartupFundraising, ScenarioRaisingFirstFund, ScenarioMAAcquisition,
		ScenarioSeriesAPlus, ScenarioRealEstate, ScenarioFundManagement,
		ScenarioPortfolioManagement, ScenarioProjectManagement, ScenarioSalesDataRoom,
		ScenarioCustom:
		return Scenario(s)
	default:
		return ScenarioUnknown
	}
}

// PackFor returns the scenario pack (Custom pack when unknown).
func PackFor(s Scenario) Pack {
	if p, ok := scenarioPacks[s]; ok {
		return p
	}
	return scenarioPacks[ScenarioCustom]
}

// productRankForItem stacks scenario pack urgency (primary) with role lens (tie-break).
// When the pack has no ProductRank (custom/unknown), circle ranks stay unscaled so
// outcome demote boosts (same units as circle bands) remain effective.
func productRankForItem(circle heat.Circle, scenario Scenario, p Product) int {
	circleRank := productRankForCircle(circle, p)
	pack := PackFor(scenario)
	if len(pack.ProductRank) == 0 {
		return circleRank
	}
	scenarioRank, ok := pack.ProductRank[p]
	if !ok {
		scenarioRank = 9
	}
	return scenarioRank*10 + circleRank
}

// InferDefaultLens picks the majority DefaultCircle among open-item scenarios.
// Ties break founder → sales → investor_ir for stable UX.
func InferDefaultLens(scenarios []Scenario) heat.Circle {
	if len(scenarios) == 0 {
		return heat.CircleDefault
	}
	counts := map[heat.Circle]int{}
	for _, s := range scenarios {
		if s == ScenarioUnknown {
			continue
		}
		pack := PackFor(s)
		counts[pack.DefaultCircle]++
	}
	if len(counts) == 0 {
		return heat.CircleDefault
	}
	best := heat.CircleDefault
	bestN := -1
	order := []heat.Circle{heat.CircleFounder, heat.CircleSales, heat.CircleInvestor}
	for _, c := range order {
		n := counts[c]
		if n > bestN {
			best = c
			bestN = n
		}
	}
	return best
}

// scenarioOrder is the stable pack order used for UniqueScenarios / DominantScenario.
var scenarioOrder = []Scenario{
	ScenarioStartupFundraising, ScenarioSeriesAPlus, ScenarioMAAcquisition,
	ScenarioSalesDataRoom, ScenarioRaisingFirstFund, ScenarioFundManagement,
	ScenarioPortfolioManagement, ScenarioRealEstate, ScenarioProjectManagement,
	ScenarioCustom,
}

// applyDeepPacks merges Phase B/C/D depth fields onto base scenarioPacks.
func applyDeepPacks(depth PackDepth, packs map[Scenario]Pack) {
	for s, deep := range packs {
		base, ok := scenarioPacks[s]
		if !ok {
			continue
		}
		base.Depth = depth
		if deep.DigestLead != "" {
			base.DigestLead = deep.DigestLead
		}
		base.KeyPageExtra = deep.KeyPageExtra
		base.GateBoostSources = deep.GateBoostSources
		base.VerbByProduct = deep.VerbByProduct
		base.HeadlineCodeByProduct = deep.HeadlineCodeByProduct
		base.SLAHours = deep.SLAHours
		base.InsightsKPI = deep.InsightsKPI
		scenarioPacks[s] = base
	}
}

// UniqueScenarios returns stable-ordered distinct non-empty scenarios.
func UniqueScenarios(scenarios []Scenario) []string {
	seen := map[Scenario]struct{}{}
	var out []string
	present := map[Scenario]struct{}{}
	for _, s := range scenarios {
		if s == ScenarioUnknown {
			continue
		}
		present[s] = struct{}{}
	}
	for _, s := range scenarioOrder {
		if _, ok := present[s]; !ok {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, string(s))
	}
	return out
}
