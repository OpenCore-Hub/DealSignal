package knowledge

import "strings"

// Typed refusal kinds (ceiling Phase J / L2). Persisted on bound_answer.refusal.
const (
	RefusalKindUngrounded = "ungrounded" // model/corpus refusal text; evidence rail hidden
	RefusalKindNoHits     = "no_hits"    // retrieve returned no usable passages / empty answer
	RefusalKindError      = "error"      // upstream / transport failure on the ask
)

// RefusalInfo is the auditable L2 refusal envelope (no new DB column).
type RefusalInfo struct {
	Kind     string `json:"kind"`               // ungrounded | no_hits | error
	HadHits  bool   `json:"hadHits,omitempty"`  // true when ungrounded cleared a non-empty hit set
	HitCount int    `json:"hitCount,omitempty"` // hit count before refuse clear (audit)
}

// ungroundedAnswerNeedles detects corpus/model refusal / “cannot answer” meta text.
// Mirrored by apps/web isUngroundedKnowledgeAnswer — keep needles in sync.
var ungroundedAnswerNeedles = []string{
	"does not contain an answer",
	"does not contain the answer",
	"do not contain an answer",
	"do not contain the answer",
	"context does not contain",
	"no relevant information",
	"cannot answer based on the",
	"can't answer based on the",
	"unable to answer based on",
	"cannot be answered based on",
	"can't be answered based on",
	"not enough information in the",
	"insufficient information in the",
	"未找到相关",
	"没有匹配",
	"无法从提供的",
	"资料中没有",
	// Soft Chinese refusals (often mixed with NDA/hit prose — still not room facts).
	"无法根据现有上下文",
	"无法根据提供的上下文",
	"无法根据上下文",
	"无法根据现有资料",
	"无法根据现有文档",
	"无法回答该问题",
	"不能根据现有上下文",
	"不能根据上下文",
	"不足以回答",
	"没有足够信息回答",
	"现有上下文回答",
	"现有材料不足以",
	"不足以确定",
	"无法确定该问题",
	"i cannot determine",
	"i can't determine",
	"cannot determine from",
	"can't determine from",
	"unable to determine from",
	"not possible to answer from",
}

// isUngroundedAnswer detects docling-rag / soft-refusal answers.
func isUngroundedAnswer(answer string) bool {
	text := strings.ToLower(strings.TrimSpace(answer))
	if text == "" {
		return false
	}
	for _, n := range ungroundedAnswerNeedles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

// looksLikeNonRoomFactMeta rejects assistant meta-discourse that must never become
// an unresolved gap or follow-up chip (ceiling: follow-ups stay inside room facts).
func looksLikeNonRoomFactMeta(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	if isUngroundedAnswer(t) {
		return true
	}
	lower := strings.ToLower(t)
	meta := []string{
		"无法回答",
		"不能回答",
		"i cannot answer",
		"i can't answer",
		"cannot be answered",
		"can't be answered",
		"based on the provided context, i",
		"based on the existing context, i",
		"根据现有资料无法",
		"没有在提供的上下文",
		"未在提供的上下文",
		// RAG template: “provided context does not include …” — not a room-fact gap.
		// Kept here (not in isUngroundedAnswer) so mixed answers still show citations.
		"提供的上下文未包含",
		"提供的上下文中未包含",
		"上下文中没有",
		"the provided context does not include",
		"provided context does not include",
		"现有材料不足以",
		"不足以确定",
		"i cannot determine",
		"i can't determine",
		"cannot determine from",
	}
	for _, m := range meta {
		if strings.Contains(lower, m) || strings.Contains(t, m) {
			return true
		}
	}
	return false
}

// looksLikeOutOfRoomGeneralKnowledge rejects industry/market trivia that must
// never become an unresolved gap or mission chip (red line: room facts only).
func looksLikeOutOfRoomGeneralKnowledge(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	lower := strings.ToLower(t)
	needles := []string{
		"ebitda multiple",
		"trading multiple",
		"valuation multiple",
		"typically 12x",
		"typically 10x",
		"market-standard",
		"market standard",
		"industry average",
		"industry standard",
		"industry norm",
		"market comps",
		"comparable companies",
		"saas m&a",
		"pe funds usually",
		"private equity usually",
		"nvca model",
		"how do pe funds",
		"how do sponsors usually",
		"同行一般",
		"市场一般怎么",
		"市场惯例",
		"行业惯例",
		"行业通常",
		"对标公司",
		"可比公司",
		"估值倍数通常",
		"倍数通常为",
		"倍数通常是",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) || strings.Contains(t, n) {
			return true
		}
	}
	return false
}

// classifyTurnResult returns refuse flag, result_status, and typed refusal audit.
func classifyTurnResult(answer string, hitCount int) (refused bool, status string, refusal *RefusalInfo) {
	if isUngroundedAnswer(answer) {
		return true, "refused", &RefusalInfo{
			Kind:     RefusalKindUngrounded,
			HadHits:  hitCount > 0,
			HitCount: hitCount,
		}
	}
	if hitCount == 0 {
		return false, "no_hits", &RefusalInfo{Kind: RefusalKindNoHits}
	}
	if strings.TrimSpace(answer) == "" {
		// Hits without an answer still count as a retrieval gap for the desk.
		return false, "no_hits", &RefusalInfo{
			Kind:     RefusalKindNoHits,
			HadHits:  true,
			HitCount: hitCount,
		}
	}
	return false, "answered", nil
}

func refusalForError() *RefusalInfo {
	return &RefusalInfo{Kind: RefusalKindError}
}

// hasGroundedClaim reports whether any claim is citation-grounded (not weak/empty).
func hasGroundedClaim(bound BoundAnswer) bool {
	for _, c := range bound.Claims {
		if c.Confidence == claimConfidenceGrounded && len(c.HitIDs) > 0 {
			return true
		}
	}
	return false
}
