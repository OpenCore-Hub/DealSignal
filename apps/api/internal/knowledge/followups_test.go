package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
)

func TestNeedsFollowUpNarrowing(t *testing.T) {
	t.Parallel()
	if !needsFollowUpNarrowing(QATurn{Refused: true, ResultStatus: "answered"}) {
		t.Fatal("refused should narrow")
	}
	if !needsFollowUpNarrowing(QATurn{ResultStatus: "no_hits"}) {
		t.Fatal("no_hits should narrow")
	}
	if needsFollowUpNarrowing(QATurn{ResultStatus: "answered"}) {
		t.Fatal("answered should not narrow")
	}
	gmvMissing := QATurn{
		ResultStatus: "answered",
		Question:     "年增长GMV多少？",
		Answer:       "提供的上下文未包含2025年GMV年增长数据。材料中可见「Managed Ad Spend」约 4.8 亿元人民币，但缺少 GMV 基数或同比口径，无法据此计算年增长。",
		Hits: []QueryHit{
			{SourceName: "00_财务口径统一说明.pdf"},
			{SourceName: "01_商业计划书_Pitch_Deck_v2026_财务口径已修订.pdf"},
		},
		Unresolved: []string{"提供的上下文未包含2025年GMV年增长数据。"},
	}
	if !needsFollowUpNarrowing(gmvMissing) {
		t.Fatal("RAG context-missing mixed answer must narrow, not split")
	}
	offTopic := QATurn{
		ResultStatus: "answered",
		Question:     "年增长GMV多少？",
		Answer:       "材料可见 Managed Ad Spend 约 4.8 亿元，但未给出 GMV 年增长。",
		Hits:         []QueryHit{{ChunkID: "h1", SourceName: "口径.pdf", Text: "Managed Ad Spend 4.8 亿"}},
		Claims: []AnswerClaim{{
			Text:       "Managed Ad Spend 约 4.8 亿元",
			HitIDs:     []string{"h1"},
			Confidence: claimConfidenceGrounded,
		}},
	}
	if !needsFollowUpNarrowing(offTopic) {
		t.Fatal("off-topic grounded number must not open a split")
	}
	if needsFollowUpNarrowing(grossMarginTurn()) {
		t.Fatal("question-grounded 毛利率 claim must still split")
	}
}

func TestTemplateFollowUpsNarrowingOnly(t *testing.T) {
	t.Parallel()
	items := templateFollowUps(QATurn{
		ResultStatus: "answered",
		Hits:         []QueryHit{{SourceName: "NDA.pdf", Text: "liability"}},
	}, "en")
	if len(items) != 2 {
		t.Fatalf("got %d", len(items))
	}
	if items[0].ID != "narrow-scope" || items[0].Kind != followUpKindNarrow {
		t.Fatalf("chip A: %#v", items[0])
	}
	if items[1].ID != "name-clause" || items[1].Kind != followUpKindNarrow {
		t.Fatalf("chip B: %#v", items[1])
	}
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Text), "liability") {
			t.Fatalf("narrowing must not use liability templates: %#v", item)
		}
	}
}

func TestFilterCoverageDiverseRejectsAllTop1(t *testing.T) {
	t.Parallel()
	coverage := []string{"Acme_SPA.pdf", "Disclosure_Schedule.xlsx"}
	stuck := []string{
		"What is the purchase price in Acme_SPA.pdf?",
		"How does Acme_SPA.pdf define working capital adjustment?",
		"What exceptions for purchase price appear in Acme_SPA.pdf?",
	}
	if got := filterCoverageDiverse(stuck, coverage); got != nil {
		t.Fatalf("want nil for all-stuck-on-top-1, got %#v", got)
	}
	diverse := []string{
		"What is the purchase price in Acme_SPA.pdf?",
		"Which material contracts appear in Disclosure_Schedule.xlsx?",
	}
	if got := filterCoverageDiverse(diverse, coverage); len(got) != 2 {
		t.Fatalf("want diverse kept, got %#v", got)
	}
}

func TestFilterGroundedFollowUps(t *testing.T) {
	t.Parallel()
	evidence := []followUpLLMEvidence{
		{SourceName: "NDA.pdf", Excerpt: "The liability of each party is limited."},
		{SourceName: "Memo.docx", Excerpt: "Confidential Information means non-public data."},
	}
	got := filterGroundedFollowUps(
		[]string{
			"Ask about liability in NDA.pdf?",
			"What is market standard indemnity?",
			"How does Memo.docx define Confidential Information?",
		},
		evidence,
	)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseFollowUpLLMQuestions(t *testing.T) {
	t.Parallel()
	raw := "```json\n{\"questions\":[\"Q1 about NDA.pdf?\",\"Q2 about NDA.pdf?\"]}\n```"
	qs, err := parseFollowUpLLMQuestions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("got %#v", qs)
	}
}

type stubFollowUpLLM struct {
	raw string
	err error
}

func (s stubFollowUpLLM) ChatCompletion(context.Context, string, []llm.Message) (string, error) {
	return s.raw, s.err
}

func TestGenerateLLMFollowUpsFiltersUngrounded(t *testing.T) {
	t.Parallel()
	payload, _ := json.Marshal(followUpLLMOutput{
		Questions: []string{
			"Continue with liability in Acme_NDA.pdf?",
			"What do competitors usually require?",
		},
	})
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: string(payload)}}
	items, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		Question:     "What is the liability cap in Acme_NDA.pdf?",
		Answer:       "See the original clause.",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "Acme_NDA.pdf", Text: "Each party’s liability under this NDA is limited."},
		},
	}, "en", nil, SessionState{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(items[0].Text, "Acme_NDA.pdf") {
		t.Fatalf("got %#v", items)
	}
	if strings.Contains(strings.ToLower(items[0].Text), "competitor") {
		t.Fatalf("industry trivia leaked: %#v", items)
	}
}

func TestGenerateLLMFollowUpsDropsOffTopicNumber(t *testing.T) {
	t.Parallel()
	payload, _ := json.Marshal(followUpLLMOutput{
		Slots: []followUpLLMSlot{
			{Slot: 0, Kind: "verify", Text: "《口径.pdf》里如何支持「Managed Ad Spend 约 4.8 亿元」？"},
			{Slot: 1, Kind: "consequence", Text: "若 4.8 亿成立，对估值意味着什么？"},
		},
	})
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: string(payload)}}
	_, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		Question:     "年增长GMV多少？",
		Answer:       "材料可见 Managed Ad Spend 约 4.8 亿元，但未给出 GMV 年增长。",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "口径.pdf", Text: "Managed Ad Spend 4.8 亿"},
		},
		Claims: []AnswerClaim{{
			Text:       "Managed Ad Spend 约 4.8 亿元",
			HitIDs:     []string{"h1"},
			Confidence: claimConfidenceGrounded,
		}},
	}, "zh-CN", nil, SessionState{})
	if err == nil {
		t.Fatal("off-topic 4.8亿 slot0 must not pass the LLM filter")
	}
}

func TestComposeFollowUpsWaterfall(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}

	t.Run("narrowing uses template not pack", func(t *testing.T) {
		t.Parallel()
		res := composeFollowUps(QATurn{ResultStatus: "no_hits"}, SessionState{}, &pack, "en", nil, false)
		if res.Source != "template" {
			t.Fatalf("source=%s", res.Source)
		}
		if len(res.Items) != 2 || res.Items[0].Kind != followUpKindNarrow {
			t.Fatalf("got %#v", res.Items)
		}
	})

	t.Run("llm with slot0 wins over gap", func(t *testing.T) {
		t.Parallel()
		llmItems := []FollowUpSuggestion{
			{ID: "llm-1", Text: "How does Unit_Economics.xlsx support 62% gross margin?", Kind: followUpKindVerify, Slot: 0},
			{ID: "llm-2", Text: "Given 62% margin, how do this room’s docs treat option pool?", Kind: followUpKindCover, Slot: 1},
		}
		res := composeFollowUps(grossMarginTurn(), SessionState{}, &pack, "en", llmItems, true)
		if res.Source != "llm" {
			t.Fatalf("source=%s want llm", res.Source)
		}
		if res.Items[0].ID != "llm-1" {
			t.Fatalf("got %#v", res.Items)
		}
	})

	t.Run("llm without slot0 falls to gap split", func(t *testing.T) {
		t.Parallel()
		llmItems := []FollowUpSuggestion{
			{ID: "llm-cover", Text: "How is the employee option pool sized?", Kind: followUpKindCover, Slot: 0},
		}
		res := composeFollowUps(grossMarginTurn(), SessionState{}, &pack, "zh-CN", llmItems, true)
		if res.Source != "gap" {
			t.Fatalf("source=%s want gap", res.Source)
		}
		if !splitHasSlot0(res.Items) {
			t.Fatalf("gap must still produce slot0: %#v", res.Items)
		}
		if res.Items[0].Kind == followUpKindCover || res.Items[0].Kind == followUpKindNarrow {
			t.Fatalf("slot0 must continue this turn: %#v", res.Items[0])
		}
	})

	t.Run("gap split when llm fails", func(t *testing.T) {
		t.Parallel()
		res := composeFollowUps(grossMarginTurn(), SessionState{}, &pack, "zh-CN", nil, false)
		if res.Source != "gap" {
			t.Fatalf("source=%s want gap", res.Source)
		}
		if res.Items[0].Kind != followUpKindVerify {
			t.Fatalf("slot0 kind=%s", res.Items[0].Kind)
		}
	})

	t.Run("no slot0 uses template not pack dump", func(t *testing.T) {
		t.Parallel()
		res := composeFollowUps(QATurn{ResultStatus: "answered"}, SessionState{}, &pack, "en", nil, false)
		if res.Source != "template" {
			t.Fatalf("source=%s want template", res.Source)
		}
		for _, it := range res.Items {
			if it.Kind != followUpKindNarrow {
				t.Fatalf("expected only narrowing, got %#v", it)
			}
			if looksLikePackPromptDump(it.Text, &pack) {
				t.Fatalf("pack dump in template path: %q", it.Text)
			}
		}
	})

	t.Run("never source=mission", func(t *testing.T) {
		t.Parallel()
		cases := []FollowUpsResponse{
			composeFollowUps(QATurn{ResultStatus: "refused"}, SessionState{}, &pack, "en", nil, false),
			composeFollowUps(grossMarginTurn(), SessionState{}, &pack, "zh-CN", nil, false),
			composeFollowUps(QATurn{ResultStatus: "answered"}, SessionState{}, &pack, "en", nil, false),
		}
		for i, res := range cases {
			if res.Source == "mission" {
				t.Fatalf("case %d returned source=mission", i)
			}
		}
	})
}

func TestParseFollowUpLLMSuggestionsSkipsSlot0Cover(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(followUpLLMOutput{
		Slots: []followUpLLMSlot{
			{Slot: 0, Kind: "cover", Text: "本室融资文件里的员工期权池规模如何约定？"},
			{Slot: 1, Kind: "verify", Text: "本室文档如何支持毛利率 62%？"},
		},
	})
	items, err := parseFollowUpLLMSuggestions(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != followUpKindVerify {
		t.Fatalf("slot0 cover must be skipped not coerced: %#v", items)
	}
	if strings.Contains(items[0].Text, "期权池规模如何约定") {
		t.Fatalf("cover dump survived: %#v", items)
	}
}

func TestGenerateLLMFollowUpsErrorsWithoutSources(t *testing.T) {
	t.Parallel()
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: `{"questions":["hi"]}`}}
	_, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		ResultStatus: "answered",
		Hits:         []QueryHit{{Text: "no name"}},
	}, "en", nil, SessionState{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateLLMFollowUpsPropagatesLLMError(t *testing.T) {
	t.Parallel()
	svc := &Service{followUpLLM: stubFollowUpLLM{err: errors.New("boom")}}
	_, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		ResultStatus: "answered",
		Hits:         []QueryHit{{SourceName: "A.pdf", Text: "x"}},
	}, "en", nil, SessionState{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateLLMFollowUpsKeepsTurnGroundedSameFile(t *testing.T) {
	t.Parallel()
	payload, _ := json.Marshal(followUpLLMOutput{
		Questions: []string{
			"What is the purchase price in Acme_SPA.pdf?",
			"How does Acme_SPA.pdf define working capital?",
			"What purchase exceptions appear in Acme_SPA.pdf?",
		},
	})
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: string(payload)}}
	items, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		Question:     "price?",
		Answer:       "ten million",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "Acme_SPA.pdf", Text: "Purchase price is ten million USD subject to working capital adjustment."},
			{SourceName: "Disclosure_Schedule.xlsx", Text: "Material contracts include the master supply agreement with Beta Logistics."},
		},
	}, "en", nil, SessionState{})
	if err != nil {
		t.Fatal(err)
	}
	if !splitHasSlot0(items) {
		t.Fatalf("expected slot0 continuation, got %#v", items)
	}
	joined := strings.ToLower(items[0].Text)
	if !strings.Contains(joined, "price") && !strings.Contains(joined, "million") {
		t.Fatalf("slot0 must stay on this turn, got %#v", items[0])
	}
}

func TestGenerateLLMFollowUpsRejectsPackPromptDump(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}
	dump := ""
	for _, item := range pack.Items {
		if item.ID == "option_pool" {
			dump = item.Prompts.ZhCN
			break
		}
	}
	if dump == "" {
		t.Fatal("missing option_pool prompt")
	}
	payload, _ := json.Marshal(followUpLLMOutput{
		Slots: []followUpLLMSlot{
			{Slot: 0, Kind: "verify", Text: "本室文档如何支持刚才问的「毛利率 62」？"},
			{Slot: 1, Kind: "cover", Text: dump},
		},
	})
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: string(payload)}}
	items, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		Question:     "毛利率是多少？",
		Answer:       "毛利率为 62%。",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "单元经济.xlsx", Text: "毛利率 62%"},
		},
		Claims: []AnswerClaim{{
			Text:       "毛利率为 62%",
			HitIDs:     []string{"h1"},
			Confidence: claimConfidenceGrounded,
		}},
	}, "zh-CN", &pack, SessionState{})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if looksLikePackPromptDump(it.Text, &pack) {
			t.Fatalf("YAML pack dump leaked into composer: %#v", it)
		}
		if strings.Contains(it.Text, "期权池规模如何约定") {
			t.Fatalf("unrewritten option-pool prompt: %#v", it)
		}
	}
	if !splitHasSlot0(items) {
		t.Fatalf("expected slot0, got %#v", items)
	}
}

func TestGenerateThenComposeFollowUpsKeepsOnTopicVerifyAndCover(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}
	turn := grossMarginTurn()
	anchor := turnAnchor(turn)
	if anchor == "" {
		t.Fatal("expected turn anchor")
	}
	payload, err := json.Marshal(followUpLLMOutput{
		Slots: []followUpLLMSlot{
			{Slot: 0, Kind: "verify", Text: "本室文档如何支持毛利率 62%？"},
			{Slot: 1, Kind: "cover", Text: fmt.Sprintf("%s 与本室文档中的期权池如何对得上？", anchor)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: string(payload)}}
	items, genErr := svc.generateLLMFollowUps(context.Background(), turn, "zh-CN", &pack, SessionState{})
	if genErr != nil {
		t.Fatal(genErr)
	}
	res := composeFollowUps(turn, SessionState{}, &pack, "zh-CN", items, true)
	if res.Source != "llm" {
		t.Fatalf("source=%s want llm", res.Source)
	}
	if !splitHasSlot0(res.Items) || res.Items[0].Kind != followUpKindVerify {
		t.Fatalf("slot0 must stay verify: %#v", res.Items)
	}
	if !strings.Contains(res.Items[0].Text, "62") && !strings.Contains(res.Items[0].Text, "毛利率") {
		t.Fatalf("slot0 must stay on 毛利率 62: %q", res.Items[0].Text)
	}
	var cover *FollowUpSuggestion
	for i := range res.Items {
		if res.Items[i].Kind == followUpKindCover {
			cover = &res.Items[i]
			break
		}
	}
	if cover == nil {
		t.Fatalf("expected rewritten cover to survive filter: %#v", res.Items)
	}
	if !strings.Contains(cover.Text, "毛利率") && !strings.Contains(cover.Text, "62") {
		t.Fatalf("cover must keep this-turn 毛利率: %q", cover.Text)
	}
	if !strings.Contains(cover.Text, "期权") {
		t.Fatalf("cover must keep pack topic: %q", cover.Text)
	}
	if looksLikePackPromptDump(cover.Text, &pack) || strings.Contains(cover.Text, "期权池规模如何约定") {
		t.Fatalf("YAML dump leaked: %q", cover.Text)
	}
}
