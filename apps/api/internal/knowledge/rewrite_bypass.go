package knowledge

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// tryDeterministicRewrite resolves pure deixis follow-ups without an LLM hop.
// Only fires when a unique document/source anchor exists; still must pass rewriteIsGrounded.
func tryDeterministicRewrite(
	userQuery string,
	prior QATurn,
	state SessionState,
	evidence []followUpLLMEvidence,
) (query, basis string, ok bool) {
	userQuery = strings.TrimSpace(userQuery)
	if userQuery == "" || !looksLikeConversationalFollowUp(userQuery) || !isPureDeixisFollowUp(userQuery) {
		return "", "", false
	}
	anchor := uniqueRewriteAnchor(state, evidence, prior)
	if anchor == "" {
		return "", "", false
	}
	topic := strings.TrimSpace(prior.Question)
	if topic == "" {
		topic = truncateRunes(prior.Answer, 80)
	}
	q := strings.TrimSpace(anchor)
	if topic != "" && !strings.Contains(strings.ToLower(topic), strings.ToLower(anchor)) {
		q = strings.TrimSpace(anchor + " " + truncateRunes(topic, 100))
	} else if topic != "" {
		q = truncateRunes(topic, knowledgeQARewriteMaxRunes)
		if !strings.Contains(strings.ToLower(q), strings.ToLower(anchor)) {
			q = strings.TrimSpace(anchor + " " + q)
		}
	}
	q = truncateRunes(q, knowledgeQARewriteMaxRunes)
	if q == "" || strings.EqualFold(q, userQuery) {
		return "", "", false
	}
	basis = rewriteBasisPriorOnly
	if sessionStateHasRewriteHints(state) {
		basis = rewriteBasisState
	}
	return q, basis, true
}

func uniqueRewriteAnchor(state SessionState, evidence []followUpLLMEvidence, prior QATurn) string {
	seen := map[string]string{} // lower → display
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; !ok {
			seen[key] = name
		}
	}
	for _, e := range state.Entities {
		if e.Type == "document" || e.Type == "clause" || e.Type == "" {
			add(e.Name)
		}
	}
	if len(seen) == 1 {
		for _, v := range seen {
			return v
		}
	}
	// Fall back to unique evidence / coverage / prior hit sources.
	seen = map[string]string{}
	for _, e := range evidence {
		add(e.SourceName)
	}
	for _, h := range state.CoverageHints {
		for _, n := range h.SourceNames {
			add(n)
		}
	}
	for _, h := range prior.Hits {
		add(h.SourceName)
	}
	if len(seen) == 1 {
		for _, v := range seen {
			return v
		}
	}
	return ""
}

// isPureDeixisFollowUp is true when the ask is essentially a pronoun/pointer with no new topic.
func isPureDeixisFollowUp(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	lower := strings.ToLower(q)
	lower = strings.TrimRightFunc(lower, func(r rune) bool {
		return r == '?' || r == '？' || r == '!' || r == '！' || r == '.' || r == '。'
	})
	pure := []string{
		"那份文件呢", "该文件呢", "那个文件呢", "这份文件呢", "那份呢", "该文件",
		"那个呢", "这个呢", "它呢", "他们呢", "她们呢", "还有呢", "上述呢", "本条款呢", "此条款呢",
		"what about that", "what about it", "what about those", "what about this",
		"how about that", "how about it", "and that", "and that file", "same for that",
		"that one", "this one", "that file", "this file",
	}
	for _, p := range pure {
		if lower == p {
			return true
		}
	}
	if utf8.RuneCountInString(q) <= 10 && looksLikeConversationalFollowUp(q) {
		// Short ask with only deixis/stopwords remaining after stripping pronouns.
		stripped := stripDeixisTokens(lower)
		return stripped == "" || utf8.RuneCountInString(stripped) <= 2
	}
	return false
}

func stripDeixisTokens(lower string) string {
	replacer := []string{
		"那份文件", "该文件", "那个文件", "这份文件", "那份", "那个", "这个", "这些", "那些",
		"上述", "本条款", "此条款", "他们", "她们", "还有", "呢", "吗", "啊",
		"what about", "how about", "same for", "and the", "and that", "and this",
		"they", "them", "their", "this", "that", "those", "these", "it", "it's",
		"file", "one", "the", "a", "an",
	}
	out := lower
	for _, r := range replacer {
		out = strings.ReplaceAll(out, r, " ")
	}
	var b strings.Builder
	for _, r := range out {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
