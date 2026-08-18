package knowledge

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
)

const (
	followUpKindVerify      = "verify"
	followUpKindConflict    = "conflict"
	followUpKindConsequence = "consequence"
	followUpKindCover       = "cover"
	followUpKindNarrow      = "narrow"

	followUpSlotMax           = 3
	followUpAnchorMaxRunes    = 24
	followUpClaimPreviewRunes = 48
)

// buildSplitFollowUps fills composer slots: slot0 must continue this turn;
// remaining slots may continue or rewrite an uncovered pack item against this turn.
func buildSplitFollowUps(
	state SessionState,
	turn QATurn,
	pack *missions.Pack,
	loc string,
) []FollowUpSuggestion {
	var out []FollowUpSuggestion
	seen := map[string]struct{}{}
	add := func(slot int, kind, id, text string) bool {
		text = strings.TrimSpace(text)
		if text == "" || !isPromotableFollowUpText(text) {
			return false
		}
		text = truncateRunes(text, followUpMaxQuestionRunes)
		key := strings.ToLower(text)
		if _, ok := seen[key]; ok {
			return false
		}
		seen[key] = struct{}{}
		out = append(out, FollowUpSuggestion{
			ID:   id,
			Text: text,
			Kind: kind,
			Slot: slot,
		})
		return true
	}

	slot0, kind0, id0 := splitSlot0(turn, loc)
	if !add(0, kind0, id0, slot0) {
		return nil
	}

	for _, c := range splitContinuationExtras(turn, loc, out[0].Text) {
		if len(out) >= followUpSlotMax {
			break
		}
		if c.kind == out[0].Kind {
			continue
		}
		add(len(out), c.kind, c.id, c.text)
	}

	if pack != nil && len(out) < followUpSlotMax {
		covered := missionCoverageCorpus(state, turn)
		for _, item := range pack.Items {
			if len(out) >= followUpSlotMax {
				break
			}
			if missionItemCovered(item, covered) {
				continue
			}
			rewritten, ok := rewriteCoverFollowUp(turn, item, loc)
			if !ok || looksLikePackPromptDump(rewritten, pack) {
				continue
			}
			add(len(out), followUpKindCover, "cover-"+item.ID, rewritten)
		}
	}

	return out
}

func continuationKind(kind string) bool {
	switch kind {
	case followUpKindVerify, followUpKindConflict, followUpKindConsequence:
		return true
	default:
		return false
	}
}

func splitHasSlot0(items []FollowUpSuggestion) bool {
	if len(items) == 0 {
		return false
	}
	first := items[0]
	if first.Kind == followUpKindCover || first.Kind == followUpKindNarrow {
		return false
	}
	return continuationKind(first.Kind) || (first.Kind == "" && strings.TrimSpace(first.Text) != "")
}

func arrangeSplitSlots(items []FollowUpSuggestion) []FollowUpSuggestion {
	var cont, cover []FollowUpSuggestion
	for _, it := range items {
		if strings.TrimSpace(it.Text) == "" {
			continue
		}
		if it.Kind == followUpKindCover {
			cover = append(cover, it)
			continue
		}
		if it.Kind == followUpKindNarrow {
			continue
		}
		if it.Kind == "" {
			it.Kind = followUpKindVerify
		}
		cont = append(cont, it)
	}
	if len(cont) == 0 {
		return nil
	}
	out := make([]FollowUpSuggestion, 0, followUpSlotMax)
	head := cont[0]
	head.Slot = 0
	out = append(out, head)
	rest := append(append([]FollowUpSuggestion{}, cont[1:]...), cover...)
	for i, it := range rest {
		if len(out) >= followUpSlotMax {
			break
		}
		it.Slot = i + 1
		out = append(out, it)
	}
	return out
}

func splitSlot0(turn QATurn, loc string) (text, kind, id string) {
	for i, u := range turn.Unresolved {
		if !isActionableUnresolvedGap(u) {
			continue
		}
		prompt := strings.TrimSpace(u)
		if utf8.RuneCountInString(prompt) > 120 {
			prompt = truncateRunes(prompt, 120)
		}
		if loc == "zh-CN" {
			return "请在本室文档中核对：" + prompt, followUpKindConflict, fmt.Sprintf("gap-unresolved-%d", i+1)
		}
		return "Verify in this room’s docs: " + prompt, followUpKindConflict, fmt.Sprintf("gap-unresolved-%d", i+1)
	}

	if claim, src := strongestGroundedClaim(turn); claim.Text != "" {
		preview := truncateRunes(strings.TrimSpace(claim.Text), followUpClaimPreviewRunes)
		if loc == "zh-CN" {
			if src != "" {
				return fmt.Sprintf("《%s》里如何支持「%s」？有无例外或口径限定？", src, preview),
					followUpKindVerify, "gap-verify-claim"
			}
			return fmt.Sprintf("本室文档如何支持「%s」？出处和口径是什么？", preview),
				followUpKindVerify, "gap-verify-claim"
		}
		if src != "" {
			return fmt.Sprintf("How does “%s” support “%s”? Any exception or definitional limit?", src, preview),
				followUpKindVerify, "gap-verify-claim"
		}
		return fmt.Sprintf("What in this room’s docs supports “%s”?", preview),
			followUpKindVerify, "gap-verify-claim"
	}

	sources := coverageSourceNames(turn.Hits, followUpCoverageMax)
	if len(sources) >= 2 && !answerMentionsSources(turn.Answer, sources[0], sources[1]) {
		if loc == "zh-CN" {
			return fmt.Sprintf("《%s》与《%s》对刚才这一点是否一致？", sources[0], sources[1]),
				followUpKindConflict, "gap-cross-file"
		}
		return fmt.Sprintf("Do “%s” and “%s” agree on the point just answered?", sources[0], sources[1]),
			followUpKindConflict, "gap-cross-file"
	}

	anchor := turnAnchor(turn)
	if anchor == "" {
		return "", "", ""
	}
	if loc == "zh-CN" {
		return fmt.Sprintf("本室文档如何支持刚才问的「%s」？", anchor),
			followUpKindVerify, "gap-verify-question"
	}
	return fmt.Sprintf("What in this room’s docs supports “%s”?", anchor),
		followUpKindVerify, "gap-verify-question"
}

type splitExtra struct {
	kind, id, text string
}

func splitContinuationExtras(turn QATurn, loc, slot0Text string) []splitExtra {
	var extras []splitExtra
	slot0l := strings.ToLower(slot0Text)
	sources := coverageSourceNames(turn.Hits, followUpCoverageMax)
	if len(sources) >= 2 {
		var text string
		if loc == "zh-CN" {
			text = fmt.Sprintf("《%s》与《%s》对刚才这一点是否一致？", sources[0], sources[1])
		} else {
			text = fmt.Sprintf("Do “%s” and “%s” agree on the point just answered?", sources[0], sources[1])
		}
		if !strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(slot0Text)) {
			extras = append(extras, splitExtra{kind: followUpKindConflict, id: "gap-cross-file", text: text})
		}
	}
	anchor := turnAnchor(turn)
	if anchor != "" && !strings.Contains(slot0l, strings.ToLower(anchor)) {
		var text string
		if loc == "zh-CN" {
			text = fmt.Sprintf("若「%s」成立，本室条款对估值或义务意味着什么？", anchor)
		} else {
			text = fmt.Sprintf("If “%s” holds, what do this room’s terms imply for value or obligations?", anchor)
		}
		extras = append(extras, splitExtra{kind: followUpKindConsequence, id: "gap-consequence", text: text})
	}
	return extras
}

func strongestGroundedClaim(turn QATurn) (AnswerClaim, string) {
	hitName := map[string]string{}
	for _, h := range turn.Hits {
		id := strings.TrimSpace(h.ChunkID)
		if id == "" {
			continue
		}
		hitName[id] = strings.TrimSpace(h.SourceName)
	}
	var best AnswerClaim
	bestHits := -1
	src := ""
	for _, c := range turn.Claims {
		if strings.TrimSpace(c.Text) == "" {
			continue
		}
		if c.Confidence != claimConfidenceGrounded || len(c.HitIDs) == 0 {
			continue
		}
		if !claimOverlapsQuestion(c.Text, turn.Question) {
			continue
		}
		if len(c.HitIDs) < bestHits {
			continue
		}
		name := ""
		if len(c.HitIDs) > 0 {
			name = hitName[c.HitIDs[0]]
		}
		best = c
		src = name
		bestHits = len(c.HitIDs)
	}
	return best, src
}

func questionTopicGrounded(turn QATurn) bool {
	for _, c := range turn.Claims {
		if c.Confidence != claimConfidenceGrounded || len(c.HitIDs) == 0 {
			continue
		}
		if claimOverlapsQuestion(c.Text, turn.Question) {
			return true
		}
	}
	return false
}

func hasActionableUnresolvedTurn(turn QATurn) bool {
	for _, u := range turn.Unresolved {
		if isActionableUnresolvedGap(u) {
			return true
		}
	}
	return false
}

func claimOverlapsQuestion(claimText, question string) bool {
	qTokens := questionTopicTokens(question)
	if len(qTokens) == 0 {
		return false
	}
	return containsAnyToken(strings.ToLower(claimText), qTokens)
}

func questionTopicTokens(question string) []string {
	weak := map[string]struct{}{
		"多少": {}, "什么": {}, "如何": {}, "怎样": {}, "哪个": {}, "是否": {},
		"much": {}, "many": {}, "which": {},
	}
	var out []string
	seen := map[string]struct{}{}
	add := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		if _, skip := weak[t]; skip {
			return
		}
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	for _, t := range splitTurnAnchorTokens(strings.ToLower(question)) {
		add(t)
		if peeled := peelQuestionSuffix(t); peeled != t {
			add(peeled)
		}
	}
	return out
}

func peelQuestionSuffix(tok string) string {
	suffixes := []string{"是多少", "是什么", "如何", "怎样", "多少", "什么", "吗"}
	for _, s := range suffixes {
		if !strings.HasSuffix(tok, s) {
			continue
		}
		rest := strings.TrimSuffix(tok, s)
		if utf8.RuneCountInString(rest) >= 2 {
			return rest
		}
	}
	return tok
}

func answerMentionsSources(answer, a, b string) bool {
	al := strings.ToLower(answer)
	return strings.Contains(al, strings.ToLower(a)) && strings.Contains(al, strings.ToLower(b))
}

func turnAnchor(turn QATurn) string {
	corpus := strings.TrimSpace(turn.Question + " " + turn.Answer)
	for _, c := range turn.Claims {
		corpus += " " + c.Text
	}
	tokens := splitTurnAnchorTokens(strings.ToLower(corpus))
	qTokens := splitTurnAnchorTokens(strings.ToLower(turn.Question))
	num := ""
	for _, tok := range tokens {
		if looksAnchorNumberToken(tok) {
			num = tok
			break
		}
	}
	topic := ""
	for _, tok := range qTokens {
		if looksAnchorNumberToken(tok) {
			continue
		}
		topic = tok
		break
	}
	if topic != "" && num != "" {
		return truncateRunes(topic+" "+num, followUpAnchorMaxRunes)
	}
	if num != "" {
		return truncateRunes(num, followUpAnchorMaxRunes)
	}
	if topic != "" {
		return truncateRunes(topic, followUpAnchorMaxRunes)
	}
	q := strings.TrimSpace(turn.Question)
	if q != "" {
		return truncateRunes(q, followUpAnchorMaxRunes)
	}
	if len(tokens) > 0 {
		return truncateRunes(tokens[0], followUpAnchorMaxRunes)
	}
	return ""
}

func looksNumericToken(tok string) bool {
	for _, r := range tok {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// looksAnchorNumberToken is true only for numeric-run tokens (62, 2025, 4.8, 10m),
// not a CJK sentence that happens to contain a year.
func looksAnchorNumberToken(tok string) bool {
	if tok == "" {
		return false
	}
	digits := 0
	for _, r := range tok {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.' || r == ',' || r == '%' || r == '+' || r == '-':
			continue
		case r >= 'a' && r <= 'z':
			if digits == 0 {
				return false
			}
		default:
			return false
		}
	}
	return digits > 0
}

const (
	anchorClassOther = iota
	anchorClassCJK
	anchorClassLatin
	anchorClassDigit
)

func runeAnchorClass(r rune) int {
	switch {
	case r >= 0x4e00 && r <= 0x9fff:
		return anchorClassCJK
	case r >= 'a' && r <= 'z':
		return anchorClassLatin
	case r >= '0' && r <= '9':
		return anchorClassDigit
	default:
		return anchorClassOther
	}
}

// splitTurnAnchorTokens splits CJK / latin / digit runs so “年增长GMV多少2025”
// does not become one token. Local to turnAnchor — do not reuse for retrieval.
func splitTurnAnchorTokens(corpus string) []string {
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "this": {}, "that": {},
		"from": {}, "are": {}, "was": {}, "how": {}, "what": {}, "does": {},
	}
	var out []string
	seen := map[string]struct{}{}
	var buf []rune
	class := anchorClassOther
	flush := func() {
		if len(buf) == 0 {
			return
		}
		tok := string(buf)
		buf = buf[:0]
		if _, skip := stop[tok]; skip {
			return
		}
		if _, ok := seen[tok]; ok {
			return
		}
		runes := []rune(tok)
		switch {
		case looksAnchorNumberToken(tok):
		case runeAnchorClass(runes[0]) == anchorClassCJK && len(runes) >= 2:
		case runeAnchorClass(runes[0]) == anchorClassLatin && len(runes) >= 3:
		default:
			return
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	for _, r := range corpus {
		c := runeAnchorClass(r)
		if c == anchorClassOther {
			if class == anchorClassDigit && (r == '.' || r == ',' || r == '%') && len(buf) > 0 {
				buf = append(buf, r)
				continue
			}
			flush()
			class = anchorClassOther
			continue
		}
		if len(buf) > 0 && c != class {
			flush()
		}
		class = c
		buf = append(buf, r)
	}
	flush()
	return out
}

func rewriteCoverFollowUp(turn QATurn, item missions.Item, loc string) (string, bool) {
	anchor := turnAnchor(turn)
	topic := coverTopic(item, loc)
	if anchor == "" || topic == "" {
		return "", false
	}
	if strings.EqualFold(anchor, topic) {
		return "", false
	}
	var text string
	if loc == "zh-CN" {
		text = fmt.Sprintf("刚提到的%s，与本室文档中的%s如何对得上？", anchor, topic)
	} else {
		text = fmt.Sprintf("Given %s, how do this room’s docs treat %s?", anchor, topic)
	}
	ql := strings.ToLower(text)
	if !strings.Contains(ql, strings.ToLower(anchor)) || !strings.Contains(ql, strings.ToLower(topic)) {
		return "", false
	}
	return text, true
}

func coverTopic(item missions.Item, loc string) string {
	for _, kw := range item.Keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if loc == "zh-CN" {
			for _, r := range kw {
				if r >= 0x4E00 && r <= 0x9FFF {
					return kw
				}
			}
			continue
		}
		if len(kw) >= 4 {
			return kw
		}
	}
	for _, kw := range item.Keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		for _, r := range kw {
			if r >= 0x4E00 && r <= 0x9FFF && utf8.RuneCountInString(kw) >= 2 {
				return kw
			}
		}
		if len(kw) >= 4 {
			return kw
		}
	}
	return ""
}

func looksLikePackPromptDump(text string, pack *missions.Pack) bool {
	if pack == nil {
		return false
	}
	norm := compactFollowUpText(text)
	if norm == "" {
		return false
	}
	for _, item := range pack.Items {
		if compactFollowUpText(item.Prompts.EN) == norm || compactFollowUpText(item.Prompts.ZhCN) == norm {
			return true
		}
	}
	return false
}

func compactFollowUpText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func turnGroundingCorpus(turn QATurn, evidence []followUpLLMEvidence) string {
	var b strings.Builder
	b.WriteString(" ")
	b.WriteString(strings.ToLower(turn.Question))
	b.WriteString(" ")
	b.WriteString(strings.ToLower(turn.Answer))
	for _, u := range turn.Unresolved {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(u))
	}
	for _, c := range turn.Claims {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(c.Text))
	}
	for _, e := range evidence {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(e.Excerpt))
	}
	return b.String()
}
