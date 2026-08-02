package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultAskDocsLocateMinRunes    = 40
	defaultAskDocsLocateMinWords    = 20
	defaultAskDocsLCSRatio          = 0.85
	defaultAskDocsLiteralRRFWeight  = 1.75
	defaultAskDocsBoundaryLLMMax    = 8
	defaultAskDocsBoundaryScoreLow  = 0.35
	defaultAskDocsBoundaryScoreHigh = 0.75
	defaultAskDocsBoundaryJaccard   = 0.5
	defaultAskDocsPortfolioMaxViews = 5
	defaultAskDocsPortfolioMaxRooms = 20
	defaultAskDocsTableMaxSheets    = 20
	defaultAskDocsTableMaxRowsSheet = 5000
	defaultAskDocsTableMaxRowsFile  = 20000
)

// AskDocsConfig holds ASK_DOCS_* runtime flags (D8 / §7.2).
// Loaded via Config.Load so server wiring does not re-read os.Getenv ad hoc.
type AskDocsConfig struct {
	IntentFirstEnabled  bool
	EvidenceFilterMode  string // auto|off
	LocateMinRunes      int
	LocateMinWords      int
	LCSRatioThreshold   float64
	LiteralRRFWeight    float64
	QueryRewriteEnabled bool

	DDCoverageEnabled  bool
	BoundaryLLMMax     int
	BoundaryScoreLow   float64
	BoundaryScoreHigh  float64
	BoundaryMinJaccard float64

	PortfolioEnabled  bool
	PortfolioMaxViews int
	PortfolioMaxRooms int

	// TableIngestEnabled cuts xlsx/csv into table_row chunks (P3.1a).
	TableIngestEnabled bool
	// TabularEnabled reserves future struct.tabular recall (default off; search still excludes until wired).
	TabularEnabled       bool
	TableMaxSheets       int
	TableMaxRowsPerSheet int
	TableMaxRowsPerFile  int
}

// AskDocsFromEnv parses ASK_DOCS_* using the same semantics as §7.2 / package OptionsFromEnv.
// Intent-first: empty → on (including production).
// DD coverage / portfolio: empty → on in non-prod, off in production|prod.
func AskDocsFromEnv(appEnv string) AskDocsConfig {
	env := strings.ToLower(strings.TrimSpace(appEnv))
	prod := env == "production" || env == "prod"

	o := AskDocsConfig{
		IntentFirstEnabled:  true,
		EvidenceFilterMode:  "auto",
		LocateMinRunes:      defaultAskDocsLocateMinRunes,
		LocateMinWords:      defaultAskDocsLocateMinWords,
		LCSRatioThreshold:   defaultAskDocsLCSRatio,
		LiteralRRFWeight:    defaultAskDocsLiteralRRFWeight,
		QueryRewriteEnabled: false,

		DDCoverageEnabled:  !prod,
		BoundaryLLMMax:     defaultAskDocsBoundaryLLMMax,
		BoundaryScoreLow:   defaultAskDocsBoundaryScoreLow,
		BoundaryScoreHigh:  defaultAskDocsBoundaryScoreHigh,
		BoundaryMinJaccard: defaultAskDocsBoundaryJaccard,

		PortfolioEnabled:  !prod,
		PortfolioMaxViews: defaultAskDocsPortfolioMaxViews,
		PortfolioMaxRooms: defaultAskDocsPortfolioMaxRooms,

		TableIngestEnabled:   !prod,
		TabularEnabled:       false,
		TableMaxSheets:       defaultAskDocsTableMaxSheets,
		TableMaxRowsPerSheet: defaultAskDocsTableMaxRowsSheet,
		TableMaxRowsPerFile:  defaultAskDocsTableMaxRowsFile,
	}

	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_INTENT_FIRST")); v != "" {
		o.IntentFirstEnabled = envTruthy(v)
	}
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("ASK_DOCS_EVIDENCE_FILTER")))
	switch mode {
	case "off", "false", "0":
		o.EvidenceFilterMode = "off"
	case "auto", "":
		o.EvidenceFilterMode = "auto"
	default:
		o.EvidenceFilterMode = "auto"
	}
	if n, err := strconv.Atoi(os.Getenv("ASK_DOCS_INTENT_LOCATE_MIN_RUNES")); err == nil && n > 0 {
		o.LocateMinRunes = n
	}
	if n, err := strconv.Atoi(os.Getenv("ASK_DOCS_INTENT_LOCATE_MIN_WORDS")); err == nil && n > 0 {
		o.LocateMinWords = n
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_INTENT_LCS_RATIO")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 1 {
			o.LCSRatioThreshold = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_INTENT_LITERAL_RRF_WEIGHT")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			o.LiteralRRFWeight = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_QUERY_REWRITE")); v != "" {
		o.QueryRewriteEnabled = envTruthy(v)
	}

	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_DD_COVERAGE")); v != "" {
		o.DDCoverageEnabled = envTruthy(v)
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_DD_BOUNDARY_LLM_MAX")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			o.BoundaryLLMMax = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_DD_BOUNDARY_SCORE_LOW")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			o.BoundaryScoreLow = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_DD_BOUNDARY_SCORE_HIGH")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			o.BoundaryScoreHigh = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_DD_BOUNDARY_MIN_JACCARD")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			o.BoundaryMinJaccard = f
		}
	}
	if o.BoundaryScoreLow > o.BoundaryScoreHigh {
		o.BoundaryScoreLow, o.BoundaryScoreHigh = o.BoundaryScoreHigh, o.BoundaryScoreLow
	}

	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_PORTFOLIO")); v != "" {
		o.PortfolioEnabled = envTruthy(v)
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_PORTFOLIO_MAX_VIEWS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.PortfolioMaxViews = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_PORTFOLIO_MAX_ROOMS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.PortfolioMaxRooms = n
		}
	}

	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_TABLE_INGEST")); v != "" {
		o.TableIngestEnabled = envTruthy(v)
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_TABULAR")); v != "" {
		o.TabularEnabled = envTruthy(v)
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_TABLE_MAX_SHEETS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.TableMaxSheets = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_TABLE_MAX_ROWS_PER_SHEET")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.TableMaxRowsPerSheet = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ASK_DOCS_TABLE_MAX_ROWS_PER_FILE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.TableMaxRowsPerFile = n
		}
	}
	return o
}

func envTruthy(v string) bool {
	return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
}
