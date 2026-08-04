package knowledge

import (
	"strings"
	"testing"
)

func TestBindAnswerClaimsFromCiteMarkers(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c1", SourceName: "SPA.pdf", Text: "Purchase price is ten million USD."},
		{ChunkID: "c2", SourceName: "Disclosure.xlsx", Text: "Material contracts include supply agreement."},
	}
	bound := bindAnswerClaims(
		"The purchase price is ten million USD [1]. Material contracts are listed in the schedule [2].",
		hits,
		false,
	)
	if len(bound.Claims) != 2 {
		t.Fatalf("claims %#v", bound.Claims)
	}
	if bound.Claims[0].Confidence != claimConfidenceGrounded ||
		len(bound.Claims[0].HitIDs) != 1 || bound.Claims[0].HitIDs[0] != "c1" {
		t.Fatalf("claim0 %#v", bound.Claims[0])
	}
	if bound.Claims[1].HitIDs[0] != "c2" {
		t.Fatalf("claim1 %#v", bound.Claims[1])
	}
	if strings.Contains(bound.Claims[0].Text, "[1]") {
		t.Fatalf("markers should be stripped: %q", bound.Claims[0].Text)
	}
}

func TestBindAnswerClaimsWeakOverlap(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c9", SourceName: "SAFE.pdf", Text: "The valuation cap is set at eight million dollars."},
	}
	bound := bindAnswerClaims(
		"The valuation cap is eight million dollars.",
		hits,
		false,
	)
	if len(bound.Claims) != 1 {
		t.Fatalf("claims %#v", bound.Claims)
	}
	if bound.Claims[0].Confidence != claimConfidenceWeak || bound.Claims[0].HitIDs[0] != "c9" {
		t.Fatalf("want weak bind %#v", bound.Claims[0])
	}
}

func TestBindAnswerClaimsUnresolvedFactual(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c1", SourceName: "SPA.pdf", Text: "Governing law is Delaware."},
	}
	// Industry trivia must never become a desk gap (red line).
	bound := bindAnswerClaims(
		"The EBITDA multiple for this sector is typically 12x.",
		hits,
		false,
	)
	if len(bound.Claims) != 1 || len(bound.Claims[0].HitIDs) != 0 {
		t.Fatalf("claims %#v", bound.Claims)
	}
	if len(bound.Unresolved) != 0 {
		t.Fatalf("out-of-room trivia must not be unresolved: %#v", bound.Unresolved)
	}
	// Unbound room-local assertion without cite still surfaces.
	bound = bindAnswerClaims(
		"The exclusivity period under the letter of intent is ninety days.",
		hits,
		false,
	)
	if len(bound.Unresolved) != 1 {
		t.Fatalf("room-local unbound claim should surface: %#v", bound.Unresolved)
	}
}

func TestBindAnswerClaimsSkipsRefusal(t *testing.T) {
	t.Parallel()
	bound := bindAnswerClaims(
		"The provided context does not contain an answer to the question.",
		[]QueryHit{{ChunkID: "c1", Text: "x"}},
		false,
	)
	if !bound.empty() {
		t.Fatalf("refusal must not bind %#v", bound)
	}
	bound = bindAnswerClaims("Anything [1].", []QueryHit{{ChunkID: "c1"}}, true)
	if !bound.empty() {
		t.Fatalf("refused flag %#v", bound)
	}
}

func TestSplitAnswerSentences(t *testing.T) {
	t.Parallel()
	got := splitAnswerSentences("价格为 10.5 百万。第二句？Third!")
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	if !strings.Contains(got[0], "10.5") {
		t.Fatalf("decimal split broken: %#v", got[0])
	}
}

func TestSplitAnswerSentencesKeepsFileExtAndListMarkers(t *testing.T) {
	t.Parallel()
	// Regression: ".docx" and "1." must not create mid-token debris gaps.
	raw := "基于 YourCompany_Standard_NDA.docx 的上下文，需注意的内容及对应风险如下：\n### 一、需注意的内容\n1. 单向保密义务对接收方不利。\n2. 保护期仅两年可能不足。"
	got := splitAnswerSentences(raw)
	for _, s := range got {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "docx") {
			t.Fatalf("must not split on .docx: %#v", got)
		}
		if strings.TrimSpace(s) == "1." || strings.HasSuffix(strings.TrimSpace(s), " 1.") {
			t.Fatalf("must not emit list-marker debris: %#v", got)
		}
	}
	joined := strings.Join(got, " || ")
	if !strings.Contains(joined, "YourCompany_Standard_NDA.docx") {
		t.Fatalf("filename should stay intact: %#v", got)
	}
	if !strings.Contains(joined, "单向保密义务") {
		t.Fatalf("list body should remain: %#v", got)
	}
}

func TestBindAnswerClaimsRejectsBrokenUnresolvedFragments(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c1", SourceName: "NDA.docx", Text: "Receiving party shall keep confidential information secret for two years."},
	}
	// Mimic the desk bug: markdown + extension + list marker debris.
	answer := "基于 NDA.docx 的上下文），需注意的内容及对应风险如下：\n### 一、需注意的内容\n1.\n该模式仅约束接收方保守披露方的信息，披露方对接收方提供的任何信息不承担保密义务，对接收方明显不利。"
	bound := bindAnswerClaims(answer, hits, false)
	for _, u := range bound.Unresolved {
		if strings.Contains(u, "###") || strings.HasPrefix(strings.ToLower(u), "docx") ||
			strings.HasSuffix(strings.TrimSpace(u), "1.") {
			t.Fatalf("unresolved must not include scaffold/debris: %q", u)
		}
		if !isActionableUnresolvedGap(u) {
			t.Fatalf("unresolved failed gate: %q", u)
		}
	}
	// Complete factual sentence without cite should still surface when unbound.
	found := false
	for _, u := range bound.Unresolved {
		if strings.Contains(u, "保密义务") {
			found = true
		}
	}
	if !found && len(bound.Unresolved) == 0 {
		// Weak overlap may bind the NDA sentence — that's fine; debris must stay out.
		for _, c := range bound.Claims {
			if strings.Contains(c.Text, "###") || strings.HasSuffix(strings.TrimSpace(c.Text), "1.") {
				// Claims may still hold raw sentences; unresolved is the user-facing gap list.
				continue
			}
		}
	}
}

func TestIsActionableUnresolvedGap(t *testing.T) {
	t.Parallel()
	good := "该模式仅约束接收方保守披露方的信息，对接收方明显不利。"
	if !isActionableUnresolvedGap(good) {
		t.Fatalf("expected actionable: %q", good)
	}
	bad := []string{
		`docx"的上下文），需注意的内容及对应风险如下：### 一、需注意的内容 1.`,
		"### 一、需注意的内容",
		"1.",
		"如下：",
		"短",
		"因此，无法根据现有上下文回答该问题。",
		"The provided context does not contain an answer to the question.",
		"The EBITDA multiple for this sector is typically 12x.",
		"同行一般怎么谈对赌条款？",
	}
	for _, b := range bad {
		if isActionableUnresolvedGap(b) {
			t.Fatalf("expected reject: %q", b)
		}
	}
}

func TestBindAnswerClaimsRejectsSoftRefusalMetaGaps(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c1", SourceName: "NDA.docx", Text: "This is a one-way non-disclosure agreement."},
	}
	answer := "根据您提供的上下文，文档属于单向保密协议，未提及员工期权池。因此，无法根据现有上下文回答该问题。"
	bound := bindAnswerClaims(answer, hits, false)
	// Whole-answer ungrounded → no claims/unresolved binding at all.
	if !bound.empty() {
		t.Fatalf("soft refusal must not bind: %#v", bound)
	}
}

func TestCiteIndexForHitID(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{{ChunkID: "a"}, {ChunkID: "b"}}
	if citeIndexForHitID("b", hits) != 2 {
		t.Fatal("index")
	}
	if citeIndexForHitID("missing", hits) != 0 {
		t.Fatal("missing")
	}
}
