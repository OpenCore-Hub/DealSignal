package knowledge

import "strings"

// isUngroundedAnswer detects docling-rag style refusals.
// Mirrored by apps/web isUngroundedKnowledgeAnswer — keep needles in sync.
func isUngroundedAnswer(answer string) bool {
	text := strings.ToLower(strings.TrimSpace(answer))
	if text == "" {
		return false
	}
	needles := []string{
		"does not contain an answer",
		"do not contain an answer",
		"no relevant information",
		"cannot answer based on the",
		"can't answer based on the",
		"未找到相关",
		"没有匹配",
		"无法从提供的",
		"上下文中没有",
		"资料中没有",
	}
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

func classifyTurnResult(answer string, hitCount int) (refused bool, status string) {
	if isUngroundedAnswer(answer) {
		return true, "refused"
	}
	if strings.TrimSpace(answer) == "" && hitCount == 0 {
		return false, "no_hits"
	}
	if hitCount == 0 {
		return false, "no_hits"
	}
	return false, "answered"
}
