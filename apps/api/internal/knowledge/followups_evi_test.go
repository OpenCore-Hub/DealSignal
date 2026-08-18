package knowledge

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
)

func grossMarginTurn() QATurn {
	return QATurn{
		Question:     "毛利率是多少？",
		Answer:       "毛利率为 62%。",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{ChunkID: "h1", SourceName: "单元经济.xlsx", Text: "毛利率 62%"},
		},
		Claims: []AnswerClaim{{
			Text:       "毛利率为 62%",
			HitIDs:     []string{"h1"},
			Confidence: claimConfidenceGrounded,
		}},
	}
}

func TestBuildSplitFollowUpsSlot0FromGroundedClaim(t *testing.T) {
	t.Parallel()
	items := buildSplitFollowUps(SessionState{}, grossMarginTurn(), nil, "zh-CN")
	if !splitHasSlot0(items) {
		t.Fatalf("expected slot0, got %#v", items)
	}
	if items[0].Kind != followUpKindVerify {
		t.Fatalf("slot0 kind=%s want verify", items[0].Kind)
	}
	text := items[0].Text
	if !strings.Contains(text, "62") && !strings.Contains(text, "单元经济") {
		t.Fatalf("slot0 must cite the number or source: %q", text)
	}
}

func TestBuildSplitFollowUpsSlot0FromUnresolved(t *testing.T) {
	t.Parallel()
	turn := QATurn{
		Question:     "责任上限？",
		Answer:       "材料未写清例外情形。",
		ResultStatus: "answered",
		Unresolved:   []string{"本室 NDA 未写明责任上限的例外条款。"},
		Hits:         []QueryHit{{SourceName: "NDA.pdf", Text: "Each party’s liability is limited."}},
	}
	items := buildSplitFollowUps(SessionState{}, turn, nil, "zh-CN")
	if !splitHasSlot0(items) {
		t.Fatalf("expected slot0, got %#v", items)
	}
	if items[0].Kind != followUpKindConflict {
		t.Fatalf("slot0 kind=%s want conflict", items[0].Kind)
	}
	if !strings.Contains(items[0].Text, "例外") && !strings.Contains(items[0].Text, "责任上限") {
		t.Fatalf("slot0 must continue the unresolved gap: %q", items[0].Text)
	}
}

func TestBuildSplitFollowUpsCoverRewritesPackNotDump(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}
	items := buildSplitFollowUps(SessionState{
		Entities: []SessionEntity{{Name: "估值上限"}},
	}, grossMarginTurn(), &pack, "zh-CN")
	if !splitHasSlot0(items) {
		t.Fatalf("expected slot0, got %#v", items)
	}
	var cover *FollowUpSuggestion
	for i := range items {
		if items[i].Kind == followUpKindCover {
			cover = &items[i]
			break
		}
	}
	if cover == nil {
		t.Fatalf("expected a rewritten cover chip, got %#v", items)
	}
	if !strings.Contains(cover.Text, "毛利率") && !strings.Contains(cover.Text, "62") {
		t.Fatalf("cover must keep this-turn 毛利率 anchor: %q", cover.Text)
	}
	if !strings.Contains(cover.Text, "期权") {
		t.Fatalf("cover must mention option-pool topic: %q", cover.Text)
	}
	if looksLikePackPromptDump(cover.Text, &pack) {
		t.Fatalf("cover copied YAML prompt: %q", cover.Text)
	}
	if strings.Contains(cover.Text, "期权池规模如何约定") {
		t.Fatalf("unrewritten checklist dump: %q", cover.Text)
	}
}

func TestLooksLikePackPromptDumpGolden(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}
	var dump string
	for _, item := range pack.Items {
		if item.ID == "option_pool" {
			dump = item.Prompts.ZhCN
			break
		}
	}
	if dump == "" {
		t.Fatal("missing option_pool zh prompt")
	}
	if !looksLikePackPromptDump(dump, &pack) {
		t.Fatalf("YAML original must be rejected: %q", dump)
	}
	rewritten := "刚提到的毛利率 62，与本室文档中的期权池如何对得上？"
	if looksLikePackPromptDump(rewritten, &pack) {
		t.Fatalf("rewritten cover must pass: %q", rewritten)
	}
}

func TestTurnAnchorSplitsCJKLatinAndPrefersNumericRun(t *testing.T) {
	t.Parallel()
	got := turnAnchor(QATurn{
		Question: "年增长GMV多少？",
		Answer:   "提供的上下文未包含2025年GMV年增长数据。材料中可见 Managed Ad Spend 约 4.8 亿元。",
	})
	if strings.Contains(got, "提供的上下文") {
		t.Fatalf("anchor must not glue the refusal sentence: %q", got)
	}
	if !strings.Contains(got, "年增长") {
		t.Fatalf("anchor should keep question topic: %q", got)
	}
	if !strings.Contains(got, "2025") && !strings.Contains(got, "4.8") {
		t.Fatalf("anchor should keep a numeric run: %q", got)
	}
}

func TestComposeFollowUpsContextMissingUsesNarrowing(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}
	turn := QATurn{
		Question:     "年增长GMV多少？",
		Answer:       "提供的上下文未包含2025年GMV年增长数据。材料中可见「Managed Ad Spend」约 4.8 亿元人民币，但缺少 GMV 基数或同比口径，无法据此计算年增长。",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "00_财务口径统一说明.pdf", Text: "2023-2025 收入"},
			{SourceName: "01_商业计划书_Pitch_Deck_v2026_财务口径已修订.pdf", Text: "Managed Ad Spend 4.8 亿"},
		},
		Unresolved: []string{"提供的上下文未包含2025年GMV年增长数据。"},
	}
	res := composeFollowUps(turn, SessionState{}, &pack, "zh-CN", nil, false)
	assertComposerNarrowNoOffTopicSpend(t, res, &pack)
}

func TestBuildSplitFollowUpsNoSlot0WithoutAnchor(t *testing.T) {
	t.Parallel()
	items := buildSplitFollowUps(SessionState{}, QATurn{ResultStatus: "answered"}, nil, "en")
	if len(items) != 0 {
		t.Fatalf("empty turn must not invent pack chips: %#v", items)
	}
}

func TestStrongestGroundedClaimIgnoresOffTopic(t *testing.T) {
	t.Parallel()
	turn := QATurn{
		Question:     "年增长GMV多少？",
		Answer:       "材料可见 Managed Ad Spend 约 4.8 亿元。",
		ResultStatus: "answered",
		Hits:         []QueryHit{{ChunkID: "h1", SourceName: "口径.pdf", Text: "4.8 亿"}},
		Claims: []AnswerClaim{{
			Text:       "Managed Ad Spend 约 4.8 亿元",
			HitIDs:     []string{"h1"},
			Confidence: claimConfidenceGrounded,
		}},
	}
	claim, _ := strongestGroundedClaim(turn)
	if claim.Text != "" {
		t.Fatalf("off-topic 4.8亿 must not win slot0: %#v", claim)
	}
}

func TestComposeFollowUpsOffTopicClaimUsesNarrowing(t *testing.T) {
	t.Parallel()
	res := composeFollowUps(QATurn{
		Question:     "年增长GMV多少？",
		Answer:       "材料可见 Managed Ad Spend 约 4.8 亿元，但未给出 GMV 年增长。",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{ChunkID: "h1", SourceName: "口径.pdf", Text: "4.8 亿"},
			{SourceName: "Pitch.pdf", Text: "广告流水"},
		},
		Claims: []AnswerClaim{{
			Text:       "Managed Ad Spend 约 4.8 亿元",
			HitIDs:     []string{"h1"},
			Confidence: claimConfidenceGrounded,
		}},
	}, SessionState{}, nil, "zh-CN", nil, false)
	assertComposerNarrowNoOffTopicSpend(t, res, nil)
}

func TestComposeFollowUpsGroundedSplitKeepsVerifyAndRewrittenCover(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}
	res := composeFollowUps(grossMarginTurn(), SessionState{
		Entities: []SessionEntity{{Name: "估值上限"}},
	}, &pack, "zh-CN", nil, false)
	if res.Source != "gap" {
		t.Fatalf("source=%s want gap", res.Source)
	}
	if !splitHasSlot0(res.Items) || res.Items[0].Kind != followUpKindVerify {
		t.Fatalf("slot0 must verify this-turn claim: %#v", res.Items)
	}
	if !strings.Contains(res.Items[0].Text, "62") && !strings.Contains(res.Items[0].Text, "单元经济") {
		t.Fatalf("slot0 must cite 62 or source: %q", res.Items[0].Text)
	}
	var cover *FollowUpSuggestion
	for i := range res.Items {
		if res.Items[i].Kind == followUpKindCover {
			cover = &res.Items[i]
			break
		}
	}
	if cover == nil {
		t.Fatalf("expected rewritten cover, got %#v", res.Items)
	}
	if !strings.Contains(cover.Text, "毛利率") && !strings.Contains(cover.Text, "62") {
		t.Fatalf("cover must keep 毛利率 anchor: %q", cover.Text)
	}
	if !strings.Contains(cover.Text, "期权") {
		t.Fatalf("cover must mention option-pool topic: %q", cover.Text)
	}
	if looksLikePackPromptDump(cover.Text, &pack) || strings.Contains(cover.Text, "期权池规模如何约定") {
		t.Fatalf("YAML pack dump in cover: %q", cover.Text)
	}
}

func TestQuestionTopicGroundedPeelsSuffix(t *testing.T) {
	t.Parallel()
	if !questionTopicGrounded(grossMarginTurn()) {
		t.Fatal("「毛利率是多少」must overlap claim 「毛利率为 62%」 after peeling 是多少")
	}
	if peelQuestionSuffix("毛利率是多少") != "毛利率" {
		t.Fatalf("peel=%q want 毛利率", peelQuestionSuffix("毛利率是多少"))
	}
}

func assertComposerNarrowNoOffTopicSpend(t *testing.T, res FollowUpsResponse, pack *missions.Pack) {
	t.Helper()
	if res.Source != "template" {
		t.Fatalf("source=%s want template", res.Source)
	}
	if len(res.Items) != 2 || res.Items[0].Kind != followUpKindNarrow {
		t.Fatalf("want two narrow chips, got %#v", res.Items)
	}
	for _, it := range res.Items {
		tl := strings.ToLower(it.Text)
		if strings.Contains(it.Text, "4.8") || strings.Contains(tl, "ad spend") {
			t.Fatalf("off-topic spend leaked into composer: %q", it.Text)
		}
		if strings.Contains(it.Text, "提供的上下文未包含") {
			t.Fatalf("meta dump in composer: %q", it.Text)
		}
		if looksLikePackPromptDump(it.Text, pack) {
			t.Fatalf("pack dump in narrowing: %q", it.Text)
		}
	}
}

func TestStrongestGroundedClaimIgnoresWeak(t *testing.T) {
	t.Parallel()
	turn := QATurn{
		Question:     "valuation cap?",
		Answer:       "see the memo.",
		ResultStatus: "answered",
		Hits:         []QueryHit{{ChunkID: "c1", SourceName: "Memo.pdf", Text: "maybe 10M"}},
		Claims: []AnswerClaim{{
			Text:       "The cap is maybe 10M",
			HitIDs:     []string{"c1"},
			Confidence: claimConfidenceWeak,
		}},
	}
	claim, _ := strongestGroundedClaim(turn)
	if claim.Text != "" {
		t.Fatalf("weak claim must not win slot0: %#v", claim)
	}
	items := buildSplitFollowUps(SessionState{}, turn, nil, "en")
	if !splitHasSlot0(items) {
		t.Fatalf("expected question-anchor slot0, got %#v", items)
	}
	if items[0].ID == "gap-verify-claim" {
		t.Fatalf("weak claim must not become verify-claim: %#v", items[0])
	}
}

func TestSplitContinuationExtrasSkipsSameKind(t *testing.T) {
	t.Parallel()
	turn := QATurn{
		Question:     "责任上限？",
		Answer:       "材料未写清例外情形。",
		ResultStatus: "answered",
		Unresolved:   []string{"本室 NDA 未写明责任上限的例外条款。"},
		Hits: []QueryHit{
			{SourceName: "NDA.pdf", Text: "Each party’s liability is limited."},
			{SourceName: "SPA.pdf", Text: "Liability is capped at the purchase price."},
		},
	}
	items := buildSplitFollowUps(SessionState{}, turn, nil, "zh-CN")
	if !splitHasSlot0(items) {
		t.Fatalf("expected slot0, got %#v", items)
	}
	if items[0].Kind != followUpKindConflict {
		t.Fatalf("slot0 kind=%s want conflict", items[0].Kind)
	}
	conflicts := 0
	for _, it := range items {
		if it.Kind == followUpKindConflict {
			conflicts++
		}
	}
	if conflicts != 1 {
		t.Fatalf("must not stack same-kind extras, got %#v", items)
	}
}

func TestCoverTopicDoesNotDumpPrompt(t *testing.T) {
	t.Parallel()
	item := missions.Item{
		ID: "option_pool",
		Prompts: missions.LocalizedString{
			EN:   "How is the employee option pool sized in this room’s financing docs?",
			ZhCN: "本室融资文件里的员工期权池规模如何约定？",
		},
	}
	if got := coverTopic(item, "zh-CN"); got != "" {
		t.Fatalf("empty keywords must not fall back to YAML prompt: %q", got)
	}
	if got := coverTopic(item, "en"); got != "" {
		t.Fatalf("empty keywords must not fall back to YAML prompt: %q", got)
	}
}
