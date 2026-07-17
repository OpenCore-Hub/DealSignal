// Package heat implements the DealSignal heat-score algorithm in Go.
package heat

import "math"

// Circle selects the scoring configuration.
type Circle string

const (
	CircleFounder   Circle = "founder"
	CircleInvestor  Circle = "investor_ir"
	CircleSales     Circle = "sales"
	CircleDefault   Circle = CircleFounder
)

// Weights holds per-factor weights.
type Weights struct {
	Opens                int `json:"opens"`
	Revisits             int `json:"revisits"`
	AvgDurationMinutes   int `json:"avgDurationMinutes"`
	KeyPageViews         int `json:"keyPageViews"`
	ForwardSignals       int `json:"forwardSignals"`
	Downloads            int `json:"downloads"`
	BouncePenalty        int `json:"bouncePenalty"`
}

// Thresholds define heat level boundaries.
type Thresholds struct {
	Hot  int `json:"hot"`
	Warm int `json:"warm"`
	Cold int `json:"cold"`
}

// Config is a full scoring profile.
type Config struct {
	Name       string            `json:"name"`
	Weights    Weights           `json:"weights"`
	KeyPages   map[string][]string `json:"keyPages"`
	Thresholds Thresholds        `json:"thresholds"`
}

// Input contains the raw metrics for a link.
type Input struct {
	Opens              int
	Revisits           int
	AvgDurationMinutes float64
	KeyPageViews       int
	ForwardSignals     int
	Downloads          int
	BouncePenalty      int
	// DecayDays is the number of days since the link's last activity.
	// Used for exponential time decay of the final score. Zero means today.
	DecayDays float64
}

// DecayHalfLifeDays is the default half-life for time decay (7 days).
const DecayHalfLifeDays = 7.0

// MaxAvgDurationMinutes caps the duration factor so a single very long session
// cannot dominate the heat score.
const MaxAvgDurationMinutes = 15.0

// Result is the computed heat score output.
type Result struct {
	Score      int                `json:"score"`
	Level      string             `json:"level"`
	Trend      string             `json:"trend"`
	Breakdown  map[string]float64 `json:"breakdown"`
}

var configs = map[Circle]Config{
	CircleFounder: {
		Name: "founder",
		Weights: Weights{Opens: 3, Revisits: 18, AvgDurationMinutes: 12, KeyPageViews: 25, ForwardSignals: 15, Downloads: 8, BouncePenalty: 10},
		KeyPages: map[string][]string{
			"financials": {"financial", "revenue", "projection", "unit economics", "burn", "runway"},
			"team":       {"team", "founder", "advisor", "hiring"},
			"traction":   {"traction", "growth", "metric", "mrr", "arr", "customer"},
			"market":     {"market", "tam", "sam", "som", "opportunity"},
		},
		Thresholds: Thresholds{Hot: 75, Warm: 40, Cold: 0},
	},
	CircleInvestor: {
		Name: "investor_ir",
		Weights: Weights{Opens: 2, Revisits: 12, AvgDurationMinutes: 10, KeyPageViews: 20, ForwardSignals: 8, Downloads: 5, BouncePenalty: 10},
		KeyPages: map[string][]string{
			"performance":  {"performance", "return", "irr", "multiple", "nav"},
			"distribution": {"distribution", "dpi", "rvpi", "tvpi", "capital"},
			"strategy":     {"strategy", "thesis", "allocation", "outlook"},
			"portfolio":    {"portfolio", "company", "investment"},
		},
		Thresholds: Thresholds{Hot: 70, Warm: 35, Cold: 0},
	},
	CircleSales: {
		Name: "sales",
		Weights: Weights{Opens: 2, Revisits: 15, AvgDurationMinutes: 10, KeyPageViews: 28, ForwardSignals: 20, Downloads: 5, BouncePenalty: 12},
		KeyPages: map[string][]string{
			"pricing":        {"pricing", "price", "cost", "fee", "quote", "proposal"},
			"security":       {"security", "compliance", "soc2", "gdpr", "encryption"},
			"case_studies":   {"case study", "customer story", "testimonial", "roi"},
			"implementation": {"implementation", "onboarding", "deployment", "timeline"},
		},
		Thresholds: Thresholds{Hot: 72, Warm: 38, Cold: 0},
	},
}

// Compute calculates the heat score for the given circle.
func Compute(circle Circle, in Input) Result {
	cfg, ok := configs[circle]
	if !ok {
		cfg = configs[CircleDefault]
	}
	w := cfg.Weights

	if in.AvgDurationMinutes > MaxAvgDurationMinutes {
		in.AvgDurationMinutes = MaxAvgDurationMinutes
	}

	breakdown := map[string]float64{
		"opens":                component("opens", in, w),
		"revisits":             component("revisits", in, w),
		"avgDurationMinutes":   component("avgDurationMinutes", in, w),
		"keyPageViews":         component("keyPageViews", in, w),
		"forwardSignals":       component("forwardSignals", in, w),
		"downloads":            component("downloads", in, w),
		"bouncePenalty":        component("bouncePenalty", in, w),
	}

	var raw float64
	for _, v := range breakdown {
		raw += v
	}

	// Apply exponential time decay: factor = 2^(-decayDays / halfLifeDays)
	// A link with no recent activity gradually loses score weight.
	if in.DecayDays > 0 && DecayHalfLifeDays > 0 {
		decay := math.Pow(2, -in.DecayDays/DecayHalfLifeDays)
		raw *= decay
	}

	score := int(math.Max(0, math.Min(100, math.Round(raw))))

	level := "cold"
	if score >= cfg.Thresholds.Hot {
		level = "hot"
	} else if score >= cfg.Thresholds.Warm {
		level = "warm"
	}

	trend := "stable"
	if in.Revisits > 0 && in.AvgDurationMinutes > 1 {
		trend = "rising"
	} else if in.AvgDurationMinutes < 0.5 && in.Opens > 0 {
		trend = "falling"
	}

	return Result{Score: score, Level: level, Trend: trend, Breakdown: breakdown}
}

func component(key string, in Input, w Weights) float64 {
	var value int
	var weight int
	switch key {
	case "opens":
		value = in.Opens
		weight = w.Opens
		if value > 10 {
			value = 10
		}
	case "revisits":
		value = in.Revisits
		weight = w.Revisits
	case "avgDurationMinutes":
		return in.AvgDurationMinutes * float64(w.AvgDurationMinutes)
	case "keyPageViews":
		value = in.KeyPageViews
		weight = w.KeyPageViews
	case "forwardSignals":
		value = in.ForwardSignals
		weight = w.ForwardSignals
	case "downloads":
		value = in.Downloads
		weight = w.Downloads
	case "bouncePenalty":
		value = in.BouncePenalty
		if value > 5 {
			value = 5
		}
		weight = w.BouncePenalty
		return -float64(value * weight)
	}
	return float64(value * weight)
}
