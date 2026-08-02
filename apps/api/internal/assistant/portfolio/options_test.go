package portfolio

import "testing"

func TestOptionsFromEnv_D14Quotas(t *testing.T) {
	t.Setenv("ASK_DOCS_PORTFOLIO", "")
	t.Setenv("ASK_DOCS_PORTFOLIO_MAX_VIEWS", "")
	t.Setenv("ASK_DOCS_PORTFOLIO_MAX_ROOMS", "")
	o := OptionsFromEnv("staging")
	if !o.Enabled {
		t.Fatal("staging default on")
	}
	if o.MaxViews != 5 || o.MaxRooms != 20 {
		t.Fatalf("quotas=%d/%d want 5/20 (D14)", o.MaxViews, o.MaxRooms)
	}
	prod := OptionsFromEnv("production")
	if prod.Enabled {
		t.Fatal("production default off")
	}
}
