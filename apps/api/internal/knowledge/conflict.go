package knowledge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	conflictKindNumeric     = "numeric"
	conflictMaxPerTurn      = 6
	conflictExcerptMaxRunes = 160
	conflictTopicWindow     = 48 // runes around a number for topic tokens
)

// ConflictSide is one source's evidence in a conflict.
type ConflictSide struct {
	SourceName string `json:"sourceName"`
	HitID      string `json:"hitId,omitempty"`
	Value      string `json:"value,omitempty"`
	Excerpt    string `json:"excerpt"`
}

// HitConflict is a deterministic cross-file disagreement in the coverage set (ceiling §3.1 / Phase I).
type HitConflict struct {
	ID    string         `json:"id"`
	Kind  string         `json:"kind"` // numeric
	Topic string         `json:"topic,omitempty"`
	Sides []ConflictSide `json:"sides"`
}

var (
	// Captures currency/percent-aware numbers with an optional unit suffix.
	conflictNumberRe = regexp.MustCompile(
		`(?i)(?:\$|USD|RMB|CNY|¥|€|£)?\s*(\d{1,3}(?:,\d{3})+(?:\.\d+)?|\d+(?:\.\d+)?)\s*(%|percent|pct|bps|x|倍|万|亿|million|billion|bn|m)?`,
	)
	conflictMetricKeywords = []string{
		"valuation", "cap", "interest", "rate", "coupon", "maturity", "revenue",
		"ebitda", "ebit", "shares", "equity", "debt", "principal", "fee", "price",
		"amount", "threshold", "limit", "multiple", "irr", "moic",
		"估值", "上限", "利率", "票息", "期限", "营收", "收入", "股份", "股权",
		"债务", "本金", "费用", "价格", "金额", "门槛", "倍数",
	}
	// Language that picks a winner after (or instead of) dual listing — ceiling §3.1.
	conflictPickSideRe = regexp.MustCompile(
		`(?i)(?:\b(?:therefore|thus|hence|accordingly)\b|\bis correct\b|\bare correct\b|\bprefer\b|\bshould (?:use|follow|rely)\b|\bwe (?:use|rely|follow|take)\b|\bcorrect (?:value|figure|amount|cap|rate)\b|\b(?:the )?(?:correct|actual|true) (?:value|figure|amount|cap|rate)\b|\brely on\b|\bgo with\b|因此|所以|应以|为准|正确的是|采用|以[^。；\n]{0,24}为准)`,
	)
)

type numericFact struct {
	sourceName string
	hitID      string
	valueNorm  string
	valueDisp  string
	topicKey   string
	topicLabel string
	excerpt    string
	hitIndex   int // 1-based cite index in full hits slice
}

// detectHitConflicts finds numeric disagreements across coverage-set sources.
// Deterministic; no LLM. Requires ≥2 distinct sourceName hits.
func detectHitConflicts(hits []QueryHit) []HitConflict {
	sources := coverageSourceNames(hits, followUpCoverageMax)
	if len(sources) < 2 {
		return nil
	}
	topBySource := map[string]QueryHit{}
	indexByChunk := map[string]int{}
	for i, h := range hits {
		name := strings.TrimSpace(h.SourceName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := topBySource[key]; !ok {
			topBySource[key] = h
		}
		if id := strings.TrimSpace(h.ChunkID); id != "" {
			if _, ok := indexByChunk[id]; !ok {
				indexByChunk[id] = i + 1
			}
		}
	}
	if len(topBySource) < 2 {
		return nil
	}

	var facts []numericFact
	for _, src := range sources {
		h, ok := topBySource[strings.ToLower(src)]
		if !ok {
			continue
		}
		facts = append(facts, extractNumericFacts(h, src, indexByChunk[strings.TrimSpace(h.ChunkID)])...)
	}
	if len(facts) < 2 {
		return nil
	}

	// Group by topicKey; emit conflict when ≥2 sources disagree on valueNorm.
	type bucket struct {
		bySource map[string]numericFact
		label    string
	}
	groups := map[string]*bucket{}
	order := []string{}
	for _, f := range facts {
		if f.topicKey == "" || f.valueNorm == "" {
			continue
		}
		b, ok := groups[f.topicKey]
		if !ok {
			b = &bucket{bySource: map[string]numericFact{}, label: f.topicLabel}
			groups[f.topicKey] = b
			order = append(order, f.topicKey)
		}
		srcKey := strings.ToLower(f.sourceName)
		if prev, exists := b.bySource[srcKey]; exists {
			// Keep first (top hit) fact per source for this topic.
			_ = prev
			continue
		}
		b.bySource[srcKey] = f
		if b.label == "" {
			b.label = f.topicLabel
		}
	}

	var out []HitConflict
	for i, key := range order {
		b := groups[key]
		if len(b.bySource) < 2 {
			continue
		}
		norms := map[string]struct{}{}
		sides := make([]ConflictSide, 0, len(b.bySource))
		// Preserve coverage source order.
		for _, src := range sources {
			f, ok := b.bySource[strings.ToLower(src)]
			if !ok {
				continue
			}
			norms[f.valueNorm] = struct{}{}
			sides = append(sides, ConflictSide{
				SourceName: f.sourceName,
				HitID:      f.hitID,
				Value:      f.valueDisp,
				Excerpt:    f.excerpt,
			})
		}
		if len(sides) < 2 || len(norms) < 2 {
			continue
		}
		out = append(out, HitConflict{
			ID:    fmt.Sprintf("conflict-numeric-%d", i+1),
			Kind:  conflictKindNumeric,
			Topic: b.label,
			Sides: sides,
		})
		if len(out) >= conflictMaxPerTurn {
			break
		}
	}
	return out
}

func extractNumericFacts(h QueryHit, sourceName string, citeIndex int) []numericFact {
	text := strings.TrimSpace(h.Text)
	if text == "" {
		return nil
	}
	matches := conflictNumberRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	var out []numericFact
	seen := map[string]struct{}{}
	runes := []rune(text)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		rawNum := text[m[2]:m[3]]
		unit := ""
		if len(m) >= 6 && m[4] >= 0 {
			unit = strings.ToLower(strings.TrimSpace(text[m[4]:m[5]]))
		}
		norm, disp, ok := normalizeConflictNumber(rawNum, unit)
		if !ok {
			continue
		}
		// Topic window in rune space.
		startByte, endByte := m[0], m[1]
		startRune := utf8.RuneCountInString(text[:startByte])
		endRune := utf8.RuneCountInString(text[:endByte])
		winStart := startRune - conflictTopicWindow
		if winStart < 0 {
			winStart = 0
		}
		winEnd := endRune + conflictTopicWindow
		if winEnd > len(runes) {
			winEnd = len(runes)
		}
		window := string(runes[winStart:winEnd])
		topicLabel, topicKey := conflictTopicFromWindow(window, sourceName)
		if topicKey == "" {
			continue
		}
		dedupe := topicKey + "|" + norm + "|" + strings.ToLower(sourceName)
		if _, dup := seen[dedupe]; dup {
			continue
		}
		seen[dedupe] = struct{}{}
		out = append(out, numericFact{
			sourceName: sourceName,
			hitID:      strings.TrimSpace(h.ChunkID),
			valueNorm:  norm,
			valueDisp:  disp,
			topicKey:   topicKey,
			topicLabel: topicLabel,
			excerpt:    truncateRunes(strings.TrimSpace(window), conflictExcerptMaxRunes),
			hitIndex:   citeIndex,
		})
	}
	return out
}

func normalizeConflictNumber(raw, unit string) (norm, display string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	cleaned := strings.ReplaceAll(raw, ",", "")
	f, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return "", "", false
	}
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch unit {
	case "%", "percent", "pct":
		norm = fmt.Sprintf("%.6f%%", f)
		display = trimFloat(f) + "%"
	case "bps":
		norm = fmt.Sprintf("%.6f%%", f/100.0)
		display = trimFloat(f) + " bps"
	case "万":
		norm = fmt.Sprintf("%.6f", f*10_000)
		display = trimFloat(f) + "万"
	case "亿":
		norm = fmt.Sprintf("%.6f", f*100_000_000)
		display = trimFloat(f) + "亿"
	case "million", "m":
		norm = fmt.Sprintf("%.6f", f*1_000_000)
		display = trimFloat(f) + "M"
	case "billion", "bn":
		norm = fmt.Sprintf("%.6f", f*1_000_000_000)
		display = trimFloat(f) + "B"
	case "x", "倍":
		norm = fmt.Sprintf("%.6fx", f)
		display = trimFloat(f) + "x"
	default:
		norm = fmt.Sprintf("%.6f", f)
		display = trimFloat(f)
		if unit != "" {
			display += " " + unit
		}
	}
	return norm, display, true
}

func trimFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return s
}

func conflictTopicFromWindow(window, sourceName string) (label, key string) {
	lower := strings.ToLower(window)
	var hits []string
	for _, kw := range conflictMetricKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			hits = append(hits, strings.ToLower(kw))
		}
	}
	if len(hits) == 0 {
		// Fallback: use distinctive non-numeric tokens near the number.
		toks := distinctiveEvidenceTokens(lower)
		for _, t := range toks {
			if len(hits) >= 3 {
				break
			}
			if isMostlyDigits(t) {
				continue
			}
			hits = append(hits, t)
		}
	}
	if len(hits) == 0 {
		return "", ""
	}
	label = hits[0]
	key = strings.Join(hits, "+")
	_ = sourceName
	return label, key
}

func isMostlyDigits(s string) bool {
	if s == "" {
		return false
	}
	digits := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits*2 >= len([]rune(s))
}

// answerPicksConflictSide reports whether the answer chooses a winner (ceiling §3.1).
func answerPicksConflictSide(answer string) bool {
	return conflictPickSideRe.MatchString(strings.TrimSpace(answer))
}

// answerMentionsAllConflictSources reports whether the answer names every conflict side source.
func answerMentionsAllConflictSources(answer string, conflicts []HitConflict) bool {
	answer = strings.ToLower(answer)
	if strings.TrimSpace(answer) == "" {
		return false
	}
	for _, c := range conflicts {
		for _, side := range c.Sides {
			name := strings.ToLower(strings.TrimSpace(side.SourceName))
			if name == "" {
				return false
			}
			if !strings.Contains(answer, name) {
				// Also try basename without extension.
				base := name
				if i := strings.LastIndex(base, "."); i > 0 {
					base = base[:i]
				}
				if base == "" || !strings.Contains(answer, base) {
					return false
				}
			}
		}
	}
	return true
}

// formatConflictAnswer builds a dual-sided listing that does not pick a winner.
func formatConflictAnswer(conflicts []HitConflict, hits []QueryHit) string {
	if len(conflicts) == 0 {
		return ""
	}
	citeOf := map[string]int{}
	for i, h := range hits {
		if id := strings.TrimSpace(h.ChunkID); id != "" {
			citeOf[id] = i + 1
		}
	}
	var b strings.Builder
	b.WriteString("These room documents disagree — listing both sides without choosing:\n")
	for _, c := range conflicts {
		topic := strings.TrimSpace(c.Topic)
		if topic == "" {
			topic = "the same figure"
		}
		b.WriteString("- On ")
		b.WriteString(topic)
		b.WriteString(":\n")
		for _, side := range c.Sides {
			cite := ""
			if n := citeOf[strings.TrimSpace(side.HitID)]; n > 0 {
				cite = fmt.Sprintf(" [%d]", n)
			}
			val := strings.TrimSpace(side.Value)
			if val == "" {
				val = "…"
			}
			b.WriteString("  • ")
			b.WriteString(side.SourceName)
			b.WriteString(": ")
			b.WriteString(val)
			b.WriteString(cite)
			b.WriteByte('\n')
		}
	}
	b.WriteString("No single value is selected.")
	return strings.TrimSpace(b.String())
}

// applyConflictAnswerPolicy enforces ceiling §3.1: on conflict, list both sides and do not pick.
// Returns possibly rewritten answer and a BoundAnswer that always carries Conflicts when present.
func applyConflictAnswerPolicy(answer string, hits []QueryHit, refused bool) (string, BoundAnswer) {
	conflicts := detectHitConflicts(hits)
	if refused || len(hits) == 0 {
		return answer, BoundAnswer{}
	}
	if len(conflicts) == 0 {
		return answer, bindAnswerClaims(answer, hits, false)
	}

	rewritten := false
	// Keep only neutral dual listings; rewrite one-sided answers and any pick-a-winner prose.
	if !answerMentionsAllConflictSources(answer, conflicts) || answerPicksConflictSide(answer) {
		answer = formatConflictAnswer(conflicts, hits)
		rewritten = true
	}
	bound := bindAnswerClaims(answer, hits, false)
	bound.Conflicts = conflicts
	if rewritten {
		// Ensure unresolved does not re-flag the dual listing as gaps.
		bound.Unresolved = filterUnresolvedAgainstConflicts(bound.Unresolved, conflicts)
	} else {
		bound.Unresolved = appendUniqueUnresolved(bound.Unresolved, conflictUnresolvedNotes(conflicts))
	}
	return answer, bound
}

func conflictUnresolvedNotes(conflicts []HitConflict) []string {
	var out []string
	for _, c := range conflicts {
		topic := strings.TrimSpace(c.Topic)
		if topic == "" {
			topic = "figure"
		}
		out = append(out, "Cross-file conflict on "+topic+" — both sides listed; no value selected.")
	}
	return out
}

func filterUnresolvedAgainstConflicts(unresolved []string, conflicts []HitConflict) []string {
	if len(unresolved) == 0 {
		return conflictUnresolvedNotes(conflicts)
	}
	// Keep short; prefer conflict notes.
	return conflictUnresolvedNotes(conflicts)
}

func appendUniqueUnresolved(base, extra []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range base {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	for _, s := range extra {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	return out
}
