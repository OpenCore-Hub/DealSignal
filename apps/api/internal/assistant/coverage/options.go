// Package coverage implements Ask Docs DD checklist scan (P2 / financing_dd_v1).
package coverage

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

const (
	defaultBoundaryLLMMax     = 8
	defaultBoundaryScoreLow   = 0.35
	defaultBoundaryScoreHigh  = 0.75
	defaultBoundaryMinJaccard = 0.5
)

// Options controls DD Coverage availability (independent of Intent-first).
type Options struct {
	Enabled bool

	// Boundary LLM (P2.1a): weak supported hits may be reclassified.
	// LLMMax ≤ 0 disables boundary LLM even when a Completer is wired.
	BoundaryLLMMax     int
	BoundaryScoreLow   float64
	BoundaryScoreHigh  float64
	BoundaryMinJaccard float64
}

// OptionsFromConfig maps config.AskDocsConfig (D8 / D12) into coverage options.
func OptionsFromConfig(c config.AskDocsConfig) Options {
	o := Options{
		Enabled:            c.DDCoverageEnabled,
		BoundaryLLMMax:     c.BoundaryLLMMax,
		BoundaryScoreLow:   c.BoundaryScoreLow,
		BoundaryScoreHigh:  c.BoundaryScoreHigh,
		BoundaryMinJaccard: c.BoundaryMinJaccard,
	}
	if o.BoundaryScoreLow > o.BoundaryScoreHigh {
		o.BoundaryScoreLow, o.BoundaryScoreHigh = o.BoundaryScoreHigh, o.BoundaryScoreLow
	}
	return o
}

// OptionsFromEnv builds options from ASK_DOCS_DD_* + APP_ENV.
// Prefer OptionsFromConfig(cfg.AskDocs) at server wire-up (D8).
func OptionsFromEnv(appEnv string) Options {
	return OptionsFromConfig(config.AskDocsFromEnv(appEnv))
}
