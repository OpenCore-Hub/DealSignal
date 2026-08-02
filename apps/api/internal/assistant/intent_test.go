package assistant

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRouteIntent_Rules(t *testing.T) {
	cfg := defaultAskDocsOptions()
	cfg.IntentFirstEnabled = true

	cases := []struct {
		name   string
		msg    string
		intent DocIntent
		source string
	}{
		{name: "topic bare term", msg: "财务数据", intent: DocIntentTopic, source: "rule"},
		{name: "list", msg: "有哪些财务指标", intent: DocIntentList, source: "rule"},
		{name: "qa whether", msg: "是否可转让", intent: DocIntentQA, source: "rule"},
		{name: "refuse market", msg: "市场惯例是什么", intent: DocIntentRefuseEarly, source: "rule"},
		{name: "locate clause", msg: "请定位第 12 条关于转让的约定", intent: DocIntentLocate, source: "rule"},
		{name: "locate long cjk", msg: strings.Repeat("甲", 45), intent: DocIntentLocate, source: "rule"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := routeIntent(context.Background(), nil, tc.msg, cfg)
			if d.Intent != tc.intent {
				t.Fatalf("intent=%s want %s (source=%s)", d.Intent, tc.intent, d.Source)
			}
			if d.Source != tc.source {
				t.Fatalf("source=%s want %s", d.Source, tc.source)
			}
			if d.LLMCalled {
				t.Fatal("rules path must not call LLM")
			}
		})
	}
}

func TestRouteIntent_LLMFallbackToQA(t *testing.T) {
	cfg := defaultAskDocsOptions()
	llm := &mockLLM{err: errors.New("timeout")}
	// Mid-length English that avoids short-topic and list/qa prefixes.
	msg := "Regarding the capitalization table footnotes and related party notes"
	d := routeIntent(context.Background(), llm, msg, cfg)
	if d.Intent != DocIntentQA {
		t.Fatalf("intent=%s want qa", d.Intent)
	}
	if d.Source != "default" {
		t.Fatalf("source=%s want default", d.Source)
	}
	if !d.LLMCalled {
		t.Fatal("expected LLMCalled after failed classify")
	}
}

func TestParseDocIntentJSON(t *testing.T) {
	got, err := parseDocIntentJSON(`{"intent":"topic"}`)
	if err != nil || got != DocIntentTopic {
		t.Fatalf("got %q err %v", got, err)
	}
	got, err = parseDocIntentJSON("```json\n{\"intent\":\"list\"}\n```")
	if err != nil || got != DocIntentList {
		t.Fatalf("fenced got %q err %v", got, err)
	}
}
