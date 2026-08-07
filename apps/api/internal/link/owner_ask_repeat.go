package link

import (
	"strings"
	"unicode"
)

const ownerAskRepeatPinThreshold = 3

// normalizeAskQuestionKey mirrors frontend repeat detection (same link scope).
func normalizeAskQuestionKey(question string) string {
	question = strings.TrimSpace(strings.ToLower(question))
	var b strings.Builder
	lastSpace := false
	for _, r := range question {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if b.Len() > 0 && !lastSpace {
			b.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

type ownerAskRepeatKey struct {
	linkID string
	qKey   string
}

func attachOwnerAskRepeatCounts(turns []OwnerAskTurn) []OwnerAskTurn {
	counts := make(map[ownerAskRepeatKey]int, len(turns))
	for _, t := range turns {
		qKey := normalizeAskQuestionKey(t.Question)
		if qKey == "" {
			continue
		}
		counts[ownerAskRepeatKey{linkID: t.LinkID, qKey: qKey}]++
	}
	out := make([]OwnerAskTurn, len(turns))
	for i, t := range turns {
		t.RepeatCount = counts[ownerAskRepeatKey{
			linkID: t.LinkID,
			qKey:   normalizeAskQuestionKey(t.Question),
		}]
		out[i] = t
	}
	return out
}
