package knowledge

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	claimConfidenceGrounded = "grounded"
	claimConfidenceWeak     = "weak"
	boundAnswerMaxClaims    = 24
	boundAnswerMinOverlap   = 2
)

var citeMarkerRe = regexp.MustCompile(`\[(\d+)\]`)

// AnswerClaim is one provenanced sentence in a bound desk answer (ceiling §3.4).
type AnswerClaim struct {
	Text       string   `json:"text"`
	HitIDs     []string `json:"hitIds,omitempty"`
	Confidence string   `json:"confidence,omitempty"` // grounded | weak
}

// BoundAnswer is the sentence↔hit binding envelope persisted on the turn.
type BoundAnswer struct {
	Claims     []AnswerClaim  `json:"claims,omitempty"`
	Unresolved []string       `json:"unresolved,omitempty"`
	Conflicts  []HitConflict  `json:"conflicts,omitempty"`
	MultiHop   *MultiHopAudit `json:"multiHop,omitempty"`
	Refusal    *RefusalInfo   `json:"refusal,omitempty"`
	Judgment   *JudgmentInfo  `json:"judgment,omitempty"`
	// CostUnits is a deterministic evidence+answer volume proxy (ceiling Phase M).
	CostUnits int `json:"costUnits,omitempty"`
}

func (b BoundAnswer) empty() bool {
	return len(b.Claims) == 0 && len(b.Unresolved) == 0 && len(b.Conflicts) == 0 &&
		(b.MultiHop == nil || (!b.MultiHop.Applied && len(b.MultiHop.Queries) == 0)) &&
		b.Refusal == nil && b.Judgment == nil && b.CostUnits <= 0
}

func marshalBoundAnswer(b BoundAnswer) []byte {
	if b.empty() {
		return nil
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return nil
	}
	return raw
}

func parseBoundAnswer(raw []byte) BoundAnswer {
	if len(raw) == 0 || string(raw) == "null" {
		return BoundAnswer{}
	}
	var b BoundAnswer
	if err := json.Unmarshal(raw, &b); err != nil {
		return BoundAnswer{}
	}
	return b
}

// bindAnswerClaims splits an audited answer into claims and binds them to hits.
// Deterministic — uses [n] citation markers first, then token overlap (weak).
// Refused / empty answers yield an empty binding.
func bindAnswerClaims(answer string, hits []QueryHit, refused bool) BoundAnswer {
	answer = strings.TrimSpace(answer)
	if answer == "" || refused || len(hits) == 0 || isUngroundedAnswer(answer) {
		return BoundAnswer{}
	}
	sentences := splitAnswerSentences(answer)
	if len(sentences) == 0 {
		return BoundAnswer{}
	}

	hitByIndex := make(map[int]QueryHit, len(hits))
	for i, h := range hits {
		hitByIndex[i+1] = h
	}
	hitTokens := make([][]string, len(hits))
	for i, h := range hits {
		hitTokens[i] = distinctiveEvidenceTokens(strings.ToLower(h.Text + " " + h.SourceName))
	}

	out := BoundAnswer{}
	for _, sent := range sentences {
		claimText, citeNums := extractCiteMarkers(sent)
		claimText = strings.TrimSpace(claimText)
		if claimText == "" {
			continue
		}
		if isConnectiveClaim(claimText) {
			out.Claims = append(out.Claims, AnswerClaim{Text: claimText})
			continue
		}

		var hitIDs []string
		seen := map[string]struct{}{}
		for _, n := range citeNums {
			h, ok := hitByIndex[n]
			if !ok {
				continue
			}
			id := strings.TrimSpace(h.ChunkID)
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			hitIDs = append(hitIDs, id)
		}

		confidence := ""
		if len(hitIDs) > 0 {
			confidence = claimConfidenceGrounded
		} else if len(hits) > 0 {
			if ids, ok := overlapBindHitIDs(claimText, hits, hitTokens); ok {
				hitIDs = ids
				confidence = claimConfidenceWeak
			}
		}

		claim := AnswerClaim{
			Text:       claimText,
			HitIDs:     hitIDs,
			Confidence: confidence,
		}
		out.Claims = append(out.Claims, claim)
		// Only promote complete, actionable factual sentences as desk gaps.
		if len(hitIDs) == 0 && isActionableUnresolvedGap(claimText) {
			out.Unresolved = append(out.Unresolved, claimText)
		}
		if len(out.Claims) >= boundAnswerMaxClaims {
			break
		}
	}
	return out
}

func extractCiteMarkers(sentence string) (clean string, nums []int) {
	matches := citeMarkerRe.FindAllStringSubmatchIndex(sentence, -1)
	if len(matches) == 0 {
		return sentence, nil
	}
	seen := map[int]struct{}{}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(sentence[last:m[0]])
		n := 0
		for _, r := range sentence[m[2]:m[3]] {
			n = n*10 + int(r-'0')
		}
		if n > 0 {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				nums = append(nums, n)
			}
		}
		last = m[1]
	}
	b.WriteString(sentence[last:])
	clean = strings.Join(strings.Fields(b.String()), " ")
	return clean, nums
}

func overlapBindHitIDs(claim string, hits []QueryHit, hitTokens [][]string) ([]string, bool) {
	claimToks := distinctiveEvidenceTokens(strings.ToLower(claim))
	if len(claimToks) == 0 {
		return nil, false
	}
	bestIdx := -1
	bestScore := 0
	for i, toks := range hitTokens {
		score := 0
		for _, ct := range claimToks {
			for _, ht := range toks {
				if ct == ht {
					score++
					break
				}
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	threshold := boundAnswerMinOverlap
	// Short CJK claims: a single distinctive token overlap can bind weakly.
	if utf8.RuneCountInString(claim) <= 24 {
		threshold = 1
	}
	if bestIdx < 0 || bestScore < threshold {
		return nil, false
	}
	id := strings.TrimSpace(hits[bestIdx].ChunkID)
	if id == "" {
		return nil, false
	}
	return []string{id}, true
}

func isConnectiveClaim(text string) bool {
	t := strings.TrimSpace(text)
	if utf8.RuneCountInString(t) <= 4 {
		return true
	}
	lower := strings.ToLower(t)
	connectives := []string{
		"根据以上", "综上", "如下", "见下", "如下所述",
		"according to", "as follows", "see below", "in summary",
	}
	for _, c := range connectives {
		if lower == c || strings.HasPrefix(lower, c+" ") || strings.HasPrefix(lower, c+"，") || strings.HasPrefix(lower, c+",") {
			return true
		}
	}
	return false
}

func looksLikeFactualClaim(text string) bool {
	t := strings.TrimSpace(text)
	if utf8.RuneCountInString(t) < 8 {
		return false
	}
	hasDigit := false
	hasLetter := false
	for _, r := range t {
		if unicode.IsDigit(r) {
			hasDigit = true
		}
		if unicode.IsLetter(r) || (r >= 0x4e00 && r <= 0x9fff) {
			hasLetter = true
		}
	}
	return hasDigit || hasLetter
}

const (
	unresolvedGapMinRunes = 12
	unresolvedGapMaxRunes = 180
)

// isActionableUnresolvedGap is the L2 quality gate for desk gaps / mission chips.
// Rejects markdown scaffolding, mid-token fragments, and list/heading debris.
func isActionableUnresolvedGap(text string) bool {
	t := strings.TrimSpace(text)
	n := utf8.RuneCountInString(t)
	if n < unresolvedGapMinRunes || n > unresolvedGapMaxRunes {
		return false
	}
	if !looksLikeFactualClaim(t) {
		return false
	}
	if isConnectiveClaim(t) {
		return false
	}
	if looksLikeMarkdownScaffold(t) {
		return false
	}
	if looksLikeBrokenFragment(t) {
		return false
	}
	// Red line: refusal / “cannot answer” meta is not a room-fact gap.
	if looksLikeNonRoomFactMeta(t) {
		return false
	}
	// Red line: industry/market trivia is not a this-room fact gap.
	if looksLikeOutOfRoomGeneralKnowledge(t) {
		return false
	}
	return true
}

func looksLikeMarkdownScaffold(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	// Heading / list / emphasis-only lines.
	if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
		return true
	}
	if strings.Contains(t, "###") || strings.Contains(t, "## ") {
		return true
	}
	// Ordered-list debris: "1." / "1. " / "12." with little else.
	runes := []rune(t)
	if orderedListMarkerPrefixLen(runes) > 0 {
		rest := strings.TrimSpace(string(runes[orderedListMarkerPrefixLen(runes):]))
		if rest == "" || utf8.RuneCountInString(rest) < 8 {
			return true
		}
	}
	// "如下：" scaffolding without a concrete clause.
	lower := strings.ToLower(t)
	scaffolds := []string{
		"如下：", "如下:", "如下所述", "需注意的内容", "对应风险如下",
		"as follows:", "as follows", "see below",
	}
	for _, s := range scaffolds {
		if lower == s || strings.HasSuffix(lower, s) && utf8.RuneCountInString(t) <= utf8.RuneCountInString(s)+6 {
			return true
		}
	}
	return false
}

func looksLikeBrokenFragment(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	runes := []rune(t)
	first := runes[0]
	// Mid-phrase after a bad split on ".docx" / quotes / closers.
	switch first {
	case '）', ')', '」', '』', '】', '〉', '>', ',', '，', ';', '；', ':', '：', '"', '\'', '”', '’':
		return true
	}
	// File-extension leftovers: "docx…", "pdf…", "xlsx…".
	lower := strings.ToLower(t)
	for _, ext := range []string{"docx", "pdf", "xlsx", "pptx", "doc", "xls", "csv"} {
		if strings.HasPrefix(lower, ext) {
			return true
		}
	}
	// Ends on an ordered-list marker ("…内容 1.") — incomplete.
	if endsWithOrderedListMarker(runes) {
		return true
	}
	// No terminal punctuation and looks truncated (ends with colon /、).
	last := runes[len(runes)-1]
	switch last {
	case ':', '：', '、', ',', '，', ';', '；', '-', '—', '/':
		return true
	}
	return false
}

func orderedListMarkerPrefixLen(runes []rune) int {
	// Matches "1." / "12. " at start.
	i := 0
	for i < len(runes) && unicode.IsDigit(runes[i]) && i < 3 {
		i++
	}
	if i == 0 || i >= len(runes) || runes[i] != '.' {
		return 0
	}
	return i + 1
}

func endsWithOrderedListMarker(runes []rune) bool {
	if len(runes) < 2 || runes[len(runes)-1] != '.' {
		return false
	}
	i := len(runes) - 2
	digits := 0
	for i >= 0 && unicode.IsDigit(runes[i]) && digits < 3 {
		i--
		digits++
	}
	if digits == 0 {
		return false
	}
	// Marker is at end, optionally preceded by whitespace.
	return i < 0 || unicode.IsSpace(runes[i])
}

// splitAnswerSentences splits on sentence terminators while keeping order.
// Does not split on decimals, file extensions (.docx), or ordered-list markers (1.).
// Newlines are soft boundaries so markdown headings/lists do not glue into one claim.
func splitAnswerSentences(answer string) []string {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	flush := func() {
		s := strings.TrimSpace(b.String())
		b.Reset()
		if s != "" {
			out = append(out, s)
		}
	}
	runes := []rune(answer)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\n' || r == '\r' {
			flush()
			continue
		}
		b.WriteRune(r)
		term := r == '。' || r == '！' || r == '？' || r == '!' || r == '?'
		if r == '.' {
			term = isSentencePeriod(runes, i)
		}
		if term {
			flush()
		}
	}
	flush()
	return out
}

// isSentencePeriod reports whether runes[i]=='.' ends a sentence.
func isSentencePeriod(runes []rune, i int) bool {
	if i < 0 || i >= len(runes) || runes[i] != '.' {
		return false
	}
	// Decimal: 10.5
	if i > 0 && unicode.IsDigit(runes[i-1]) && i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
		return false
	}
	// File extension: Memo.docx / report.PDF
	if isFileExtensionDot(runes, i) {
		return false
	}
	// Ordered list marker: "1." / "12. " (incl. trailing end)
	if isOrderedListMarkerDot(runes, i) {
		return false
	}
	return true
}

func isFileExtensionDot(runes []rune, i int) bool {
	if i == 0 || i+1 >= len(runes) {
		return false
	}
	prev := runes[i-1]
	if !unicode.IsLetter(prev) && !unicode.IsDigit(prev) {
		return false
	}
	j := i + 1
	n := 0
	for j < len(runes) && unicode.IsLetter(runes[j]) && n < 5 {
		j++
		n++
	}
	// Common doc extensions are 2–5 letters.
	if n < 2 || n > 5 {
		return false
	}
	// Extension ends the token (or is followed by quote/closer/CJK).
	if j >= len(runes) {
		return true
	}
	next := runes[j]
	if unicode.IsLetter(next) || unicode.IsDigit(next) {
		return false
	}
	return true
}

func isOrderedListMarkerDot(runes []rune, i int) bool {
	if i == 0 {
		return false
	}
	j := i - 1
	digits := 0
	for j >= 0 && unicode.IsDigit(runes[j]) && digits < 3 {
		j--
		digits++
	}
	if digits == 0 {
		return false
	}
	// Must be at line/segment start (or after whitespace).
	if j >= 0 && !unicode.IsSpace(runes[j]) {
		return false
	}
	// After '.': end, whitespace — not another digit (decimal already handled).
	if i+1 >= len(runes) {
		return true
	}
	return unicode.IsSpace(runes[i+1])
}

// citeIndexForHitID maps a hit chunk id to 1-based evidence index (UI cite number).
func citeIndexForHitID(hitID string, hits []QueryHit) int {
	id := strings.TrimSpace(hitID)
	if id == "" {
		return 0
	}
	for i, h := range hits {
		if strings.TrimSpace(h.ChunkID) == id {
			return i + 1
		}
	}
	return 0
}
