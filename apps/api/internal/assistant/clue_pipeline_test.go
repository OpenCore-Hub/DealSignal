package assistant

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

func TestCluePipeline_LocateHardLiteral(t *testing.T) {
	clause := "受让方不得将本协议项下权利义务转让给任何第三方，除非事先取得转让方书面同意。"
	hits := []search.Evidence{
		{ChunkID: "a", Quote: "无关邻句内容一二三四五六七八九十", Score: 0.9},
		{ChunkID: "b", Quote: clause, Score: 0.5},
	}
	decision := decisionFromIntent(DocIntentLocate, "rule", false)
	got := cluePipelineRun(context.Background(), nil, clause, hits, decision, defaultAskDocsOptions())
	if len(got.Evidence) != 1 || got.Evidence[0].ChunkID != "b" {
		t.Fatalf("want hard literal top-1 chunk b, got %+v", got.Evidence)
	}
	if got.Decision.Intent != DocIntentLocate || got.Decision.Mode != GenerationExtractive {
		t.Fatalf("decision=%+v", got.Decision)
	}
	if got.Decision.FallbackFrom != "" {
		t.Fatalf("unexpected fallback %q", got.Decision.FallbackFrom)
	}
}

func TestCluePipeline_LocateSoftLiteralSurvivesRerankTruncation(t *testing.T) {
	// Soft-literal target (high Jaccard, low RRF) must win even when a high-RRF
	// neighbor would cause scoreRerank to truncate before the ladder runs.
	query := "甲乙丙丁戊己庚辛壬癸子丑寅卯辰巳午未申酉戌亥一二三四五六七八九十"
	softQuote := "甲乙丙丁戊己庚辛壬癸子丑寅卯辰巳午未申酉戌亥一二三四五六七八九九" // one-char diff → soft Jaccard
	hits := []search.Evidence{
		{ChunkID: "noise-high", Quote: "董事会关于预算与融资的完全无关决议全文填充占位", Score: 10.0},
		{ChunkID: "soft-low", Quote: softQuote, Score: 0.01},
		{ChunkID: "noise-mid", Quote: "员工手册考勤与报销制度细则补充说明段落", Score: 5.0},
	}
	// Prove the regression: scoreRerank alone would keep the high-score noise first.
	reranked := scoreRerankEvidence(query, hits)
	if len(reranked) == 0 || reranked[0].ChunkID == "soft-low" {
		t.Fatalf("precondition: scoreRerank should prefer high RRF noise, got %+v", reranked)
	}

	decision := decisionFromIntent(DocIntentLocate, "rule", false)
	got := cluePipelineRun(context.Background(), nil, query, hits, decision, defaultAskDocsOptions())
	if got.Decision.Intent != DocIntentLocate || got.Decision.FallbackFrom != "" {
		t.Fatalf("want locate soft hit, got decision=%+v", got.Decision)
	}
	if len(got.Evidence) != 1 || got.Evidence[0].ChunkID != "soft-low" {
		t.Fatalf("want soft-literal top-1 soft-low, got %+v", got.Evidence)
	}
}

func TestCluePipeline_LocateSoftLiteralViaLCS(t *testing.T) {
	// Low set-Jaccard (few shared unique runes) but high LCS ratio → soft tier (H9).
	query := strings.Repeat("甲", 30) + strings.Repeat("乙", 30)
	lcsQuote := query + "丙丁戊己庚辛壬癸" // LCS=60 / 68 ≈ 0.88; Jaccard = 2/10 = 0.2
	if jac := runeJaccard(normalizeEvidenceText(query), normalizeEvidenceText(lcsQuote)); jac >= highOverlapThreshold {
		t.Fatalf("precondition: jaccard must be below soft threshold, got %f", jac)
	}
	if lcs := lcsRatio(normalizeEvidenceText(query), normalizeEvidenceText(lcsQuote)); lcs < 0.85 {
		t.Fatalf("precondition: lcs must be ≥0.85, got %f", lcs)
	}

	hits := []search.Evidence{
		{ChunkID: "noise-high", Quote: "董事会关于预算与融资的完全无关决议全文填充占位", Score: 10.0},
		{ChunkID: "lcs-soft", Quote: lcsQuote, Score: 0.01},
	}
	decision := decisionFromIntent(DocIntentLocate, "rule", false)
	got := cluePipelineRun(context.Background(), nil, query, hits, decision, defaultAskDocsOptions())
	if got.Decision.Intent != DocIntentLocate || got.Decision.FallbackFrom != "" {
		t.Fatalf("want locate via LCS soft, got decision=%+v", got.Decision)
	}
	if len(got.Evidence) != 1 || got.Evidence[0].ChunkID != "lcs-soft" {
		t.Fatalf("want LCS soft top-1, got %+v", got.Evidence)
	}
}

func TestCluePipeline_LocateLCSBelowThresholdFallsBack(t *testing.T) {
	query := strings.Repeat("甲", 30) + strings.Repeat("乙", 30)
	// Noise inserted in the middle breaks hard contains; LCS ratio stays < 0.85.
	weakQuote := strings.Repeat("甲", 30) + "丙丁戊己庚辛壬癸子丑寅卯" + strings.Repeat("乙", 30)
	normQ := normalizeEvidenceText(query)
	normW := normalizeEvidenceText(weakQuote)
	if strings.Contains(normW, normQ) {
		t.Fatal("precondition: must not hard-contain query")
	}
	if jac := runeJaccard(normQ, normW); jac >= highOverlapThreshold {
		t.Fatalf("precondition: jaccard must be below soft threshold, got %f", jac)
	}
	if lcs := lcsRatio(normQ, normW); lcs >= 0.85 {
		t.Fatalf("precondition: lcs must be <0.85, got %f", lcs)
	}

	hits := []search.Evidence{
		{ChunkID: "weak", Quote: weakQuote, Score: 0.9},
		{ChunkID: "other", Quote: "董事会决议通过预算案", Score: 0.5},
	}
	decision := decisionFromIntent(DocIntentLocate, "rule", false)
	cfg := defaultAskDocsOptions()
	cfg.LCSRatioThreshold = 0.85
	got := cluePipelineRun(context.Background(), nil, query, hits, decision, cfg)
	if got.Decision.Intent != DocIntentTopic || got.Decision.FallbackFrom != string(DocIntentLocate) {
		t.Fatalf("want topic fallback, got decision=%+v", got.Decision)
	}
}

func TestCluePipeline_LocateFallbackTopic(t *testing.T) {
	hits := []search.Evidence{
		{ChunkID: "1", Quote: "财务报告显示营收增长", Score: 0.8},
		{ChunkID: "2", Quote: "董事会决议通过预算", Score: 0.7},
		{ChunkID: "3", Quote: "员工手册考勤制度", Score: 0.6},
		{ChunkID: "4", Quote: "额外第四条线索", Score: 0.5},
	}
	query := strings.Repeat("甲乙丙丁戊", 10) // locate-length, no overlap with quotes
	decision := decisionFromIntent(DocIntentLocate, "rule", false)
	got := cluePipelineRun(context.Background(), nil, query, hits, decision, defaultAskDocsOptions())
	if got.Decision.Intent != DocIntentTopic {
		t.Fatalf("intent=%s want topic", got.Decision.Intent)
	}
	if got.Decision.FallbackFrom != string(DocIntentLocate) {
		t.Fatalf("fallback_from=%q", got.Decision.FallbackFrom)
	}
	if len(got.Evidence) > 3 {
		t.Fatalf("max evidence 3, got %d", len(got.Evidence))
	}
}

func TestBuildExtractiveAnswer_NoDefinitionStyle(t *testing.T) {
	ev := []search.Evidence{{DocumentID: "doc-12345678", PageNumber: 2, Quote: "营收与毛利率摘要"}}
	ans := buildExtractiveAnswer("zh", decisionFromIntent(DocIntentTopic, "rule", false), ev)
	for _, bad := range []string{"是指", "定义为", "指的是"} {
		if strings.Contains(ans, bad) {
			t.Fatalf("extractive answer must not define: %q in %q", bad, ans)
		}
	}
	if !strings.Contains(ans, "营收与毛利率摘要") {
		t.Fatalf("missing quote: %q", ans)
	}
}

func TestMarshalDecodeEvidenceEnvelope(t *testing.T) {
	ev := []search.Evidence{{ChunkID: "c1", Quote: "hello"}}
	raw := marshalEvidenceWithAudit(ev, askDocsAuditSnapshot{
		DocIntent:      "topic",
		GenerationMode: "extractive",
		IntentSource:   "rule",
	})
	items, meta := decodeStoredEvidence(raw)
	if len(items) != 1 || items[0].ChunkID != "c1" {
		t.Fatalf("items=%+v", items)
	}
	if meta.DocIntent != "topic" || meta.GenerationMode != "extractive" {
		t.Fatalf("meta=%+v", meta)
	}
	// Legacy bare array still decodes.
	legacy := []byte(`[{"chunk_id":"x","quote":"q"}]`)
	items, meta = decodeStoredEvidence(legacy)
	if len(items) != 1 || items[0].ChunkID != "x" || meta.DocIntent != "" {
		t.Fatalf("legacy items=%+v meta=%+v", items, meta)
	}
}
