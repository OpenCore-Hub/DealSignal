package assistant

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

// CluePipelineResult is the exported pipeline output for one turn.
type CluePipelineResult struct {
	Evidence []search.Evidence
	Decision IntentDecision
}

// CluePipeline applies intent-aware literal ladder, rerank, and optional LLM filter (P1 export).
type CluePipeline struct {
	registry *Registry
}

// NewCluePipeline constructs a pipeline using the default intent registry.
func NewCluePipeline() *CluePipeline {
	return &CluePipeline{registry: DefaultRegistry()}
}

// NewCluePipelineWithRegistry constructs a pipeline with an explicit registry.
func NewCluePipelineWithRegistry(reg *Registry) *CluePipeline {
	if reg == nil {
		reg = DefaultRegistry()
	}
	return &CluePipeline{registry: reg}
}

// Registry returns the JobProfile registry bound to this pipeline.
func (p *CluePipeline) Registry() *Registry {
	if p == nil || p.registry == nil {
		return DefaultRegistry()
	}
	return p.registry
}

// Run applies the intent-aware clue pipeline to hybrid search hits.
// Locate/PreferLiteral runs the literal ladder on the full hybrid Top-K first so
// scoreRerank's strong-literal truncation cannot drop soft-literal candidates (G4/G5).
func (p *CluePipeline) Run(ctx context.Context, completer ChatCompleter, query string, evidence []search.Evidence, decision IntentDecision, cfg AskDocsOptions) CluePipelineResult {
	cfg = cfg.normalized()
	if len(evidence) == 0 {
		return CluePipelineResult{Decision: decision}
	}

	if decision.PreferLiteral || decision.Intent == DocIntentLocate {
		out, dec := applyLocateLiteralLadder(query, evidence, decision, cfg)
		if dec.FallbackFrom == string(DocIntentLocate) {
			reranked := scoreRerankEvidence(query, evidence)
			out = capEvidence(reranked, dec.MaxEvidence)
		}
		return CluePipelineResult{Evidence: out, Decision: dec}
	}

	reranked := scoreRerankEvidence(query, evidence)
	if len(reranked) == 0 {
		return CluePipelineResult{Decision: decision}
	}

	out := reranked
	dec := decision

	skipFilter := decision.SkipLLMFilter ||
		cfg.EvidenceFilterMode == "off" ||
		decision.LLMCalled ||
		decision.Mode == GenerationExtractive ||
		decision.Mode == GenerationRefuse

	if !skipFilter && completer != nil {
		filtered, err := filterEvidenceByLLM(ctx, completer, query, out)
		if err != nil {
			out = capEvidence(out, dec.MaxEvidence)
		} else {
			out = capEvidence(filtered, dec.MaxEvidence)
		}
	} else {
		out = capEvidence(out, dec.MaxEvidence)
	}

	return CluePipelineResult{Evidence: out, Decision: dec}
}

// cluePipelineRun is the package-local entry used by tests.
func cluePipelineRun(ctx context.Context, completer ChatCompleter, query string, evidence []search.Evidence, decision IntentDecision, cfg AskDocsOptions) CluePipelineResult {
	return NewCluePipeline().Run(ctx, completer, query, evidence, decision, cfg)
}

func applyLocateLiteralLadder(query string, evidence []search.Evidence, decision IntentDecision, cfg AskDocsOptions) ([]search.Evidence, IntentDecision) {
	if len(evidence) == 0 {
		return nil, decision
	}
	cfg = cfg.normalized()
	normQuery := normalizeEvidenceText(query)

	type scored struct {
		ev      search.Evidence
		hard    bool
		soft    bool
		softJac bool
		softLCS bool
		jac     float64
		lcs     float64
	}
	items := make([]scored, 0, len(evidence))
	for _, ev := range evidence {
		normQuote := normalizeEvidenceText(ev.Quote)
		hard := normQuery != "" && normQuote != "" && strings.Contains(normQuote, normQuery)
		jac := 0.0
		lcs := 0.0
		if normQuery != "" && normQuote != "" {
			jac = runeJaccard(normQuery, normQuote)
			lcs = lcsRatio(normQuery, normQuote)
		}
		// Soft literal (H9): Jaccard ≥ 0.72 OR LCS ratio ≥ threshold (default 0.85), in parallel.
		softJac := !hard && jac >= highOverlapThreshold
		softLCS := !hard && lcs >= cfg.LCSRatioThreshold
		soft := softJac || softLCS
		if softLCS && !softJac {
			slog.Debug("ask_docs_lcs_soft",
				"lcs", lcs,
				"jaccard", jac,
				"threshold", cfg.LCSRatioThreshold,
				"chunk_id", ev.ChunkID,
			)
		}
		items = append(items, scored{
			ev: ev, hard: hard, soft: soft, softJac: softJac, softLCS: softLCS, jac: jac, lcs: lcs,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].hard != items[j].hard {
			return items[i].hard
		}
		if items[i].soft != items[j].soft {
			return items[i].soft
		}
		if items[i].soft && items[j].soft {
			if items[i].jac != items[j].jac {
				return items[i].jac > items[j].jac
			}
			if items[i].lcs != items[j].lcs {
				return items[i].lcs > items[j].lcs
			}
		}
		if items[i].ev.Score != items[j].ev.Score {
			return items[i].ev.Score > items[j].ev.Score
		}
		return items[i].ev.ChunkID < items[j].ev.ChunkID
	})

	top := items[0]
	if top.hard || top.soft {
		dec := decision
		dec.Intent = DocIntentLocate
		dec.Mode = GenerationExtractive
		dec.MaxEvidence = 1
		dec.PreferLiteral = true
		dec.SkipLLMFilter = true
		return []search.Evidence{top.ev}, dec
	}

	dec := decision
	dec.FallbackFrom = string(DocIntentLocate)
	topic := ProfileFor(DocIntentTopic)
	dec.Intent = DocIntentTopic
	dec.Mode = topic.Mode
	dec.MaxEvidence = topic.MaxEvidence
	dec.PreferLiteral = topic.PreferLiteral
	dec.SkipLLMFilter = topic.SkipLLMFilter
	return capEvidence(evidence, topic.MaxEvidence), dec
}

// lcsRatio is normalized LCS length / max(len(a), len(b)) on runes (locate soft tier, H9).
func lcsRatio(a, b string) float64 {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	const maxN = 400
	if len(ra) > maxN {
		ra = ra[:maxN]
	}
	if len(rb) > maxN {
		rb = rb[:maxN]
	}
	n, m := len(ra), len(rb)
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if ra[i-1] == rb[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
		for j := range cur {
			cur[j] = 0
		}
	}
	lcs := prev[m]
	den := n
	if m > den {
		den = m
	}
	if den == 0 {
		return 0
	}
	return float64(lcs) / float64(den)
}
