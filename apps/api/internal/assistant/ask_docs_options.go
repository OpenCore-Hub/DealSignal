package assistant

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

// AskDocsOptions controls Intent-first Ask Docs behavior (P0+).
type AskDocsOptions struct {
	IntentFirstEnabled bool
	// EvidenceFilterMode: "auto" (profile-driven) or "off" (force skip LLM filter).
	EvidenceFilterMode string
	LocateMinRunes     int
	LocateMinWords     int
	// LCSRatioThreshold gates locate soft-literal LCS matches (H9; default 0.85).
	LCSRatioThreshold float64
	// LiteralRRFWeight is α for PreferLiteral FTS/trigram RRF tilt (D9); 0 → 1.75.
	LiteralRRFWeight float64
	// QueryRewriteEnabled enables optional qa/list retrieval rewrite (P2.1; default off).
	// Never applied to locate/topic. Skipped when Intent LLM already ran this turn (≤2 budget).
	QueryRewriteEnabled bool
}

func defaultAskDocsOptions() AskDocsOptions {
	return AskDocsOptions{
		IntentFirstEnabled:  false,
		EvidenceFilterMode:  "auto",
		LocateMinRunes:      40,
		LocateMinWords:      20,
		LCSRatioThreshold:   0.85,
		LiteralRRFWeight:    1.75,
		QueryRewriteEnabled: false,
	}
}

// AskDocsOptionsFromConfig maps config.AskDocsConfig (D8) into assistant options.
func AskDocsOptionsFromConfig(c config.AskDocsConfig) AskDocsOptions {
	return AskDocsOptions{
		IntentFirstEnabled:  c.IntentFirstEnabled,
		EvidenceFilterMode:  c.EvidenceFilterMode,
		LocateMinRunes:      c.LocateMinRunes,
		LocateMinWords:      c.LocateMinWords,
		LCSRatioThreshold:   c.LCSRatioThreshold,
		LiteralRRFWeight:    c.LiteralRRFWeight,
		QueryRewriteEnabled: c.QueryRewriteEnabled,
	}.normalized()
}

// AskDocsOptionsFromEnv builds options from env + APP_ENV defaults.
// Prefer AskDocsOptionsFromConfig(cfg.AskDocs) at server wire-up (D8).
func AskDocsOptionsFromEnv(appEnv string) AskDocsOptions {
	return AskDocsOptionsFromConfig(config.AskDocsFromEnv(appEnv))
}

func (o AskDocsOptions) normalized() AskDocsOptions {
	out := o
	if out.EvidenceFilterMode == "" {
		out.EvidenceFilterMode = "auto"
	}
	if out.LocateMinRunes <= 0 {
		out.LocateMinRunes = 40
	}
	if out.LocateMinWords <= 0 {
		out.LocateMinWords = 20
	}
	if out.LCSRatioThreshold <= 0 || out.LCSRatioThreshold > 1 {
		out.LCSRatioThreshold = 0.85
	}
	if out.LiteralRRFWeight <= 0 {
		out.LiteralRRFWeight = 1.75
	}
	return out
}
