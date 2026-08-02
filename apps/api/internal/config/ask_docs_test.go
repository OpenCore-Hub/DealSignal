package config

import (
	"os"
	"testing"
)

func TestAskDocsFromEnv_IntentFirstDefaultsOn(t *testing.T) {
	t.Setenv("ASK_DOCS_INTENT_FIRST", "")
	for _, env := range []string{"production", "prod", "staging", "development"} {
		o := AskDocsFromEnv(env)
		if !o.IntentFirstEnabled {
			t.Fatalf("APP_ENV=%q IntentFirst must default on", env)
		}
	}
}

func TestAskDocsFromEnv_DDAndPortfolioProdDefaultOff(t *testing.T) {
	t.Setenv("ASK_DOCS_DD_COVERAGE", "")
	t.Setenv("ASK_DOCS_PORTFOLIO", "")
	prod := AskDocsFromEnv("production")
	if prod.DDCoverageEnabled || prod.PortfolioEnabled {
		t.Fatalf("prod defaults: dd=%v portfolio=%v", prod.DDCoverageEnabled, prod.PortfolioEnabled)
	}
	stg := AskDocsFromEnv("staging")
	if !stg.DDCoverageEnabled || !stg.PortfolioEnabled {
		t.Fatalf("staging defaults: dd=%v portfolio=%v", stg.DDCoverageEnabled, stg.PortfolioEnabled)
	}
}

func TestAskDocsFromEnv_D9D12D14Defaults(t *testing.T) {
	clearAskDocsEnv(t)
	o := AskDocsFromEnv("staging")
	if o.LiteralRRFWeight != 1.75 {
		t.Fatalf("LiteralRRFWeight=%v want 1.75 (D9)", o.LiteralRRFWeight)
	}
	if o.BoundaryScoreLow != 0.35 || o.BoundaryScoreHigh != 0.75 || o.BoundaryMinJaccard != 0.5 {
		t.Fatalf("boundary defaults=%v/%v/%v want 0.35/0.75/0.5 (D12)", o.BoundaryScoreLow, o.BoundaryScoreHigh, o.BoundaryMinJaccard)
	}
	if o.PortfolioMaxViews != 5 || o.PortfolioMaxRooms != 20 {
		t.Fatalf("portfolio quotas=%d/%d want 5/20 (D14)", o.PortfolioMaxViews, o.PortfolioMaxRooms)
	}
	if o.QueryRewriteEnabled {
		t.Fatal("QueryRewrite must default off")
	}
}

func TestAskDocsFromEnv_TableIngestDefaults(t *testing.T) {
	clearAskDocsEnv(t)
	prod := AskDocsFromEnv("production")
	if prod.TableIngestEnabled || prod.TabularEnabled {
		t.Fatalf("prod table flags: ingest=%v tabular=%v", prod.TableIngestEnabled, prod.TabularEnabled)
	}
	stg := AskDocsFromEnv("staging")
	if !stg.TableIngestEnabled {
		t.Fatal("staging TABLE_INGEST must default on")
	}
	if stg.TabularEnabled {
		t.Fatal("TABULAR must default off")
	}
	if stg.TableMaxSheets != 20 || stg.TableMaxRowsPerSheet != 5000 || stg.TableMaxRowsPerFile != 20000 {
		t.Fatalf("limits=%d/%d/%d", stg.TableMaxSheets, stg.TableMaxRowsPerSheet, stg.TableMaxRowsPerFile)
	}
}

func TestAskDocsFromEnv_ExplicitOverrides(t *testing.T) {
	t.Setenv("ASK_DOCS_INTENT_FIRST", "false")
	t.Setenv("ASK_DOCS_DD_COVERAGE", "true")
	t.Setenv("ASK_DOCS_PORTFOLIO", "true")
	t.Setenv("ASK_DOCS_INTENT_LITERAL_RRF_WEIGHT", "2.0")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_SCORE_LOW", "0.4")
	t.Setenv("ASK_DOCS_PORTFOLIO_MAX_VIEWS", "3")
	t.Setenv("ASK_DOCS_QUERY_REWRITE", "on")
	t.Setenv("ASK_DOCS_TABLE_INGEST", "true")
	o := AskDocsFromEnv("production")
	if o.IntentFirstEnabled {
		t.Fatal("intent must be off")
	}
	if !o.DDCoverageEnabled || !o.PortfolioEnabled {
		t.Fatal("dd/portfolio must be on")
	}
	if o.LiteralRRFWeight != 2.0 || o.BoundaryScoreLow != 0.4 || o.PortfolioMaxViews != 3 {
		t.Fatalf("overrides got weight=%v low=%v views=%d", o.LiteralRRFWeight, o.BoundaryScoreLow, o.PortfolioMaxViews)
	}
	if !o.QueryRewriteEnabled {
		t.Fatal("query rewrite must be on")
	}
	if !o.TableIngestEnabled {
		t.Fatal("table ingest must be on")
	}
}

func clearAskDocsEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"ASK_DOCS_INTENT_FIRST",
		"ASK_DOCS_EVIDENCE_FILTER",
		"ASK_DOCS_INTENT_LOCATE_MIN_RUNES",
		"ASK_DOCS_INTENT_LOCATE_MIN_WORDS",
		"ASK_DOCS_INTENT_LCS_RATIO",
		"ASK_DOCS_INTENT_LITERAL_RRF_WEIGHT",
		"ASK_DOCS_QUERY_REWRITE",
		"ASK_DOCS_DD_COVERAGE",
		"ASK_DOCS_DD_BOUNDARY_LLM_MAX",
		"ASK_DOCS_DD_BOUNDARY_SCORE_LOW",
		"ASK_DOCS_DD_BOUNDARY_SCORE_HIGH",
		"ASK_DOCS_DD_BOUNDARY_MIN_JACCARD",
		"ASK_DOCS_PORTFOLIO",
		"ASK_DOCS_PORTFOLIO_MAX_VIEWS",
		"ASK_DOCS_PORTFOLIO_MAX_ROOMS",
		"ASK_DOCS_TABLE_INGEST",
		"ASK_DOCS_TABULAR",
		"ASK_DOCS_TABLE_MAX_SHEETS",
		"ASK_DOCS_TABLE_MAX_ROWS_PER_SHEET",
		"ASK_DOCS_TABLE_MAX_ROWS_PER_FILE",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
		t.Setenv(k, "")
	}
}
