package knowledge

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

const (
	multiHopKindDefinition = "definition"
	multiHopKindAttachment = "attachment"
	multiHopMaxQueries     = 2
	multiHopMaxScanHits    = 5
	multiHopModeSuffix     = "+hop"
	multiHopReserveSlots   = 2
)

var (
	hopQuotedTermRe = regexp.MustCompile(`(?i)[“"]([^”"]{2,80})[”"]\s+means\b`)
	hopAsDefinedRe  = regexp.MustCompile(`(?i)\bas\s+defined\s+in\s+((?:Exhibit|Schedule|Annex|Appendix)\s+[A-Z0-9][-A-Z0-9]{0,8})`)
	hopDefZHRe      = regexp.MustCompile(`「([^」]{2,40})」的?(?:定义|释义)`)
	hopAttachENRe   = regexp.MustCompile(`(?i)\b((?:Exhibit|Schedule|Annex|Appendix)\s+[A-Z0-9][-A-Z0-9]{0,8})\b`)
	hopAttachZHRe   = regexp.MustCompile(`((?:附件|附录|附表)\s*[A-Za-z0-9一二三四五六七八九十甲乙丙丁]{1,8})`)
)

// MultiHopQuery is one deterministic second-hop retrieve query (ceiling Phase I3).
type MultiHopQuery struct {
	Kind       string   `json:"kind"` // definition | attachment
	Query      string   `json:"query"`
	FromHitIDs []string `json:"fromHitIds,omitempty"`
	Anchor     string   `json:"anchor,omitempty"`
}

// MultiHopAudit is persisted on bound_answer for replay (no new DB column).
type MultiHopAudit struct {
	Applied     bool            `json:"applied"`
	Queries     []MultiHopQuery `json:"queries,omitempty"`
	AddedHitIDs []string        `json:"addedHitIds,omitempty"`
}

type hopAnchor struct {
	Kind   string
	Anchor string
	HitID  string
}

// extractHopAnchors finds definition/attachment anchors in first-hop hit text.
// Deterministic — no LLM. Scans top hits by input order (already score-sorted).
func extractHopAnchors(hits []QueryHit, state SessionState) []hopAnchor {
	if len(hits) == 0 {
		return nil
	}
	scan := hits
	if len(scan) > multiHopMaxScanHits {
		scan = scan[:multiHopMaxScanHits]
	}
	var out []hopAnchor
	seen := map[string]struct{}{}
	add := func(kind, anchor, hitID string) {
		anchor = strings.TrimSpace(anchor)
		if anchor == "" {
			return
		}
		key := kind + "|" + strings.ToLower(anchor)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, hopAnchor{Kind: kind, Anchor: anchor, HitID: hitID})
	}

	for _, h := range scan {
		text := strings.TrimSpace(h.Text)
		if text == "" {
			continue
		}
		hitID := strings.TrimSpace(h.ChunkID)
		for _, m := range hopQuotedTermRe.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				add(multiHopKindDefinition, m[1], hitID)
			}
		}
		for _, m := range hopDefZHRe.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				add(multiHopKindDefinition, m[1], hitID)
			}
		}
		for _, m := range hopAsDefinedRe.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				add(multiHopKindAttachment, m[1], hitID)
			}
		}
		for _, m := range hopAttachENRe.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				add(multiHopKindAttachment, m[1], hitID)
			}
		}
		for _, m := range hopAttachZHRe.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				add(multiHopKindAttachment, compactSpaces(m[1]), hitID)
			}
		}
		// Audited clause/other entities that appear in hit text become definition hops.
		for _, e := range state.Entities {
			name := strings.TrimSpace(e.Name)
			if name == "" || e.Type == "document" {
				continue
			}
			if len(name) < 2 {
				continue
			}
			if strings.Contains(strings.ToLower(text), strings.ToLower(name)) {
				add(multiHopKindDefinition, name, hitID)
			}
		}
	}
	return out
}

func compactSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// buildHopQueries turns anchors into grounded retrieve queries (≤ multiHopMaxQueries).
// Prefer one definition + one attachment hop when both exist.
func buildHopQueries(anchors []hopAnchor, hop1 []QueryHit) []MultiHopQuery {
	if len(anchors) == 0 {
		return nil
	}
	corpus := hopGroundingCorpus(hop1)
	var out []MultiHopQuery
	haveDef, haveAtt := false, false
	for _, a := range anchors {
		if len(out) >= multiHopMaxQueries {
			break
		}
		if a.Kind == multiHopKindDefinition && haveDef {
			continue
		}
		if a.Kind == multiHopKindAttachment && haveAtt {
			continue
		}
		q := hopQueryForAnchor(a)
		if q == "" || !hopQueryGrounded(a.Anchor, corpus) {
			continue
		}
		out = append(out, MultiHopQuery{
			Kind:       a.Kind,
			Query:      q,
			FromHitIDs: nonEmptyStrings(a.HitID),
			Anchor:     a.Anchor,
		})
		switch a.Kind {
		case multiHopKindDefinition:
			haveDef = true
		case multiHopKindAttachment:
			haveAtt = true
		}
	}
	return out
}

func hopQueryForAnchor(a hopAnchor) string {
	anchor := strings.TrimSpace(a.Anchor)
	if anchor == "" {
		return ""
	}
	switch a.Kind {
	case multiHopKindDefinition:
		if containsHan(anchor) {
			term := strings.Trim(anchor, "「」")
			return "「" + term + "」的定义或释义"
		}
		return `definition of "` + strings.Trim(anchor, `"'`) + `"`
	case multiHopKindAttachment:
		return strings.TrimSpace(anchor)
	default:
		return ""
	}
}

func hopGroundingCorpus(hits []QueryHit) string {
	var b strings.Builder
	for _, h := range hits {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(h.Text))
		b.WriteString(" ")
		b.WriteString(strings.ToLower(h.SourceName))
	}
	return b.String()
}

func hopQueryGrounded(anchor, corpus string) bool {
	anchor = strings.ToLower(strings.TrimSpace(anchor))
	if anchor == "" {
		return false
	}
	if strings.Contains(corpus, anchor) {
		return true
	}
	compact := strings.ReplaceAll(anchor, " ", "")
	if compact != "" && strings.Contains(strings.ReplaceAll(corpus, " ", ""), compact) {
		return true
	}
	return false
}

func containsHan(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// wantsMultiHop reports whether a second hop should be attempted.
func wantsMultiHop(enabled bool, hits []QueryHit, state SessionState) bool {
	if !enabled || len(hits) == 0 {
		return false
	}
	return len(extractHopAnchors(hits, state)) > 0
}

// mergeMultiHopHits keeps hop1 headroom, appends novel hop2 hits, then fills leftover hop1.
func mergeMultiHopHits(hop1, hop2 []QueryHit, topK int) ([]QueryHit, []string) {
	if topK <= 0 {
		topK = tableLaneDefaultLimit
	}
	if len(hop2) == 0 {
		return hop1, nil
	}
	reserve := multiHopReserveSlots
	if reserve > topK {
		reserve = topK
	}
	keep1 := topK - reserve
	if keep1 < 0 {
		keep1 = 0
	}
	if keep1 > len(hop1) {
		keep1 = len(hop1)
	}

	seenChunk := map[string]struct{}{}
	seenText := map[string]struct{}{}
	mark := func(h QueryHit) {
		if id := strings.TrimSpace(h.ChunkID); id != "" {
			seenChunk[id] = struct{}{}
		}
		key := strings.ToLower(h.DocumentID) + "|" + strings.ToLower(strings.TrimSpace(h.Text))
		seenText[key] = struct{}{}
	}
	dup := func(h QueryHit) bool {
		if id := strings.TrimSpace(h.ChunkID); id != "" {
			if _, ok := seenChunk[id]; ok {
				return true
			}
		}
		key := strings.ToLower(h.DocumentID) + "|" + strings.ToLower(strings.TrimSpace(h.Text))
		_, ok := seenText[key]
		return ok
	}

	out := make([]QueryHit, 0, topK)
	for _, h := range hop1[:keep1] {
		mark(h)
		out = append(out, h)
	}
	var added []string
	for _, h := range hop2 {
		if len(out) >= topK {
			break
		}
		if dup(h) {
			continue
		}
		mark(h)
		out = append(out, h)
		if id := strings.TrimSpace(h.ChunkID); id != "" {
			added = append(added, id)
		}
	}
	for _, h := range hop1[keep1:] {
		if len(out) >= topK {
			break
		}
		if dup(h) {
			continue
		}
		mark(h)
		out = append(out, h)
	}
	return out, added
}

func applyMultiHop(out *QueryResponse, hopHits []QueryHit, topK int, audit *MultiHopAudit) {
	if out == nil || len(hopHits) == 0 || audit == nil {
		return
	}
	merged, added := mergeMultiHopHits(out.Results, hopHits, topK)
	out.Results = merged
	audit.AddedHitIDs = added
	audit.Applied = len(added) > 0
	if audit.Applied && !strings.Contains(out.Mode, "hop") {
		if out.Mode == "" {
			out.Mode = "hop"
		} else {
			out.Mode = out.Mode + multiHopModeSuffix
		}
	}
}

// runMultiHop executes ≤2 Answer:false Search hops and merges novel hits into out.
func (s *Service) runMultiHop(
	ctx context.Context,
	cred ragCredentials,
	byExtID, byName map[string]string,
	lockedIDs map[string]bool,
	state SessionState,
	out *QueryResponse,
	topK int,
	searchMode string,
) *MultiHopAudit {
	if s == nil || !s.multiHopEnabled || out == nil || s.client == nil {
		return nil
	}
	if !wantsMultiHop(true, out.Results, state) {
		recordKnowledgeQAMultiHop("skipped")
		return nil
	}
	queries := buildHopQueries(extractHopAnchors(out.Results, state), out.Results)
	if len(queries) == 0 {
		recordKnowledgeQAMultiHop("skipped")
		return nil
	}
	audit := &MultiHopAudit{Queries: queries}
	var hopHits []QueryHit
	for _, hq := range queries {
		res, err := s.client.Search(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey, docling.SearchRequest{
			Query:  hq.Query,
			Mode:   searchMode,
			TopK:   topK,
			Answer: false,
		})
		if err != nil {
			var apiErr *docling.APIError
			if errors.As(err, &apiErr) && (apiErr.Code == "INDEX_NOT_READY" || apiErr.Status == http.StatusServiceUnavailable) {
				logger.InfoCtx(ctx, "knowledge multi-hop search unavailable",
					logger.Attr("error", err.Error()),
					logger.Attr("hop_kind", hq.Kind),
				)
			} else {
				logger.InfoCtx(ctx, "knowledge multi-hop search failed",
					logger.Attr("error", err.Error()),
					logger.Attr("hop_kind", hq.Kind),
				)
			}
			recordKnowledgeQAMultiHop("failed")
			continue
		}
		filtered := applyLockedSearchFilter(res, byExtID, byName, lockedIDs)
		hopHits = append(hopHits, filtered.Results...)
	}
	applyMultiHop(out, hopHits, topK, audit)
	if audit.Applied {
		recordKnowledgeQAMultiHop("applied")
	} else {
		recordKnowledgeQAMultiHop("skipped")
	}
	return audit
}
