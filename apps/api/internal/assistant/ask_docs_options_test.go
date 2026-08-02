package assistant

import "testing"

func TestAskDocsOptionsFromEnv_ProductionDefaultOn(t *testing.T) {
	t.Setenv("ASK_DOCS_INTENT_FIRST", "")
	o := AskDocsOptionsFromEnv("production")
	if !o.IntentFirstEnabled {
		t.Fatal("production empty env must default Intent-first on")
	}
	o = AskDocsOptionsFromEnv("prod")
	if !o.IntentFirstEnabled {
		t.Fatal("prod empty env must default Intent-first on")
	}
}

func TestAskDocsOptionsFromEnv_ExplicitOff(t *testing.T) {
	for _, v := range []string{"false", "0", "off", "FALSE"} {
		t.Setenv("ASK_DOCS_INTENT_FIRST", v)
		o := AskDocsOptionsFromEnv("production")
		if o.IntentFirstEnabled {
			t.Fatalf("ASK_DOCS_INTENT_FIRST=%q must disable Intent-first", v)
		}
	}
}

func TestAskDocsOptionsFromEnv_ExplicitOn(t *testing.T) {
	t.Setenv("ASK_DOCS_INTENT_FIRST", "true")
	o := AskDocsOptionsFromEnv("production")
	if !o.IntentFirstEnabled {
		t.Fatal("explicit true must enable")
	}
}
