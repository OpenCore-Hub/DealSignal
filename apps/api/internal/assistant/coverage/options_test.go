package coverage

import (
	"testing"
)

func TestStreamName_D11(t *testing.T) {
	if StreamName != "askdocs:dd_scan" {
		t.Fatalf("StreamName=%q", StreamName)
	}
}

func TestOptionsFromEnv_ProductionDefaultOff(t *testing.T) {
	t.Setenv("ASK_DOCS_DD_COVERAGE", "")
	o := OptionsFromEnv("production")
	if o.Enabled {
		t.Fatal("production default must disable DD coverage")
	}
	o = OptionsFromEnv("prod")
	if o.Enabled {
		t.Fatal("prod default must disable DD coverage")
	}
}

func TestOptionsFromEnv_NonProdDefaultOn(t *testing.T) {
	t.Setenv("ASK_DOCS_DD_COVERAGE", "")
	for _, env := range []string{"development", "staging", "test", ""} {
		o := OptionsFromEnv(env)
		if !o.Enabled {
			t.Fatalf("APP_ENV=%q empty ASK_DOCS_DD_COVERAGE must enable", env)
		}
	}
}

func TestOptionsFromEnv_Explicit(t *testing.T) {
	t.Setenv("ASK_DOCS_DD_COVERAGE", "true")
	if !OptionsFromEnv("production").Enabled {
		t.Fatal("explicit true must enable in production")
	}
	t.Setenv("ASK_DOCS_DD_COVERAGE", "false")
	if OptionsFromEnv("staging").Enabled {
		t.Fatal("explicit false must disable in staging")
	}
}

func TestOptionsFromEnv_D12Defaults(t *testing.T) {
	t.Setenv("ASK_DOCS_DD_BOUNDARY_SCORE_LOW", "")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_SCORE_HIGH", "")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_MIN_JACCARD", "")
	o := OptionsFromEnv("staging")
	if o.BoundaryScoreLow != 0.35 || o.BoundaryScoreHigh != 0.75 || o.BoundaryMinJaccard != 0.5 {
		t.Fatalf("D12 defaults=%v/%v/%v", o.BoundaryScoreLow, o.BoundaryScoreHigh, o.BoundaryMinJaccard)
	}
}
