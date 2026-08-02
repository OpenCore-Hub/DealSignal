package assistant

import (
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

// polishExtractiveAnswer is a P1+ hook for optional LLM polish of extractive answers.
// P0 keeps zero answer-LLM: identity only (G3).
func polishExtractiveAnswer(answer string, _ IntentDecision) string {
	return answer
}

// buildExtractiveAnswer builds a template locator sentence + quotes (zero answer LLM).
func buildExtractiveAnswer(lang string, decision IntentDecision, evidence []search.Evidence) string {
	zh := strings.HasPrefix(strings.ToLower(lang), "zh")
	if len(evidence) == 0 {
		if zh {
			return "未找到可跳转核验的原文摘录。"
		}
		return "No verifiable excerpt was found."
	}

	var b strings.Builder
	if decision.Intent == DocIntentLocate && decision.FallbackFrom == "" {
		if zh {
			b.WriteString("已定位到以下原文：")
		} else {
			b.WriteString("Located the following excerpt:")
		}
	} else {
		if zh {
			b.WriteString("在授权材料中找到以下相关摘录：")
		} else {
			b.WriteString("Found these related excerpts in the authorized materials:")
		}
	}
	b.WriteString("\n\n")
	for i, ev := range evidence {
		quote := strings.TrimSpace(ev.Quote)
		if quote == "" {
			continue
		}
		loc := formatEvidenceLocator(zh, ev)
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, loc))
		if zh {
			b.WriteString("「")
			b.WriteString(quote)
			b.WriteString("」")
		} else {
			b.WriteString("\"")
			b.WriteString(quote)
			b.WriteString("\"")
		}
		b.WriteString("\n\n")
	}
	return polishExtractiveAnswer(strings.TrimSpace(b.String()), decision)
}

func formatEvidenceLocator(zh bool, ev search.Evidence) string {
	page := ev.PageNumber
	if page <= 0 {
		page = 1
	}
	if zh {
		if ev.DocumentID != "" {
			return fmt.Sprintf("文档 %s · 第 %d 页", shortID(ev.DocumentID), page)
		}
		return fmt.Sprintf("第 %d 页", page)
	}
	if ev.DocumentID != "" {
		return fmt.Sprintf("Document %s · page %d", shortID(ev.DocumentID), page)
	}
	return fmt.Sprintf("Page %d", page)
}

func shortID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
