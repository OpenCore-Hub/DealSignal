package assistant

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Absence slot detection and rule peel rewrite (P1b / H4–H5).
// Absence is a slot on DocIntentQA — never a primary DocIntent.

var (
	absenceENLeadRE = regexp.MustCompile(`(?i)^(is there(?:\s+any|\s+a|\s+an)?|are there(?:\s+any)?|do we have(?:\s+any|\s+a|\s+an)?|does (?:the )?(?:document|agreement|contract|nda|spa|materials?) have(?:\s+any|\s+a|\s+an)?|is there any mention of)\s+`)
	absenceENTailRE = regexp.MustCompile(`(?i)\s+in (?:the )?(?:docs|documents|materials|corpus|authorized materials)\s*[?.!]*$`)
)

// zhAbsenceShells are stripped in longest-first order.
var zhAbsenceShells = []string{
	"授权材料中有没有",
	"材料里有没有",
	"材料中有没有",
	"文档里有没有",
	"文档中有没有",
	"有没有关于",
	"有无关于",
	"是否存在",
	"存不存在",
	"是否有",
	"有没有",
	"有无",
}

func applyAbsenceSlot(d IntentDecision, message string) IntentDecision {
	if d.Intent != DocIntentQA || d.Mode == GenerationRefuse {
		return d
	}
	if detectAbsenceSlot(message) {
		d.Absence = true
	}
	return d
}

func detectAbsenceSlot(message string) bool {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)

	for _, shell := range zhAbsenceShells {
		if strings.Contains(msg, shell) {
			return true
		}
	}
	if absenceENLeadRE.MatchString(lower) {
		return true
	}
	if strings.HasPrefix(lower, "is there") || strings.HasPrefix(lower, "are there") {
		return true
	}
	if strings.Contains(lower, "is there a ") || strings.Contains(lower, "is there an ") ||
		strings.Contains(lower, "is there any ") || strings.Contains(lower, "are there any ") {
		return true
	}
	return false
}

// peelAbsenceQuery strips existence-question shells for a second retrieval pass.
// Returns ok=false when peel yields nothing useful or does not change the query.
func peelAbsenceQuery(message string) (string, bool) {
	original := strings.TrimSpace(message)
	if original == "" {
		return "", false
	}

	s := original

	for _, shell := range zhAbsenceShells {
		if idx := strings.Index(s, shell); idx >= 0 {
			s = strings.TrimSpace(s[:idx] + s[idx+len(shell):])
			break
		}
	}

	lower := strings.ToLower(s)
	if loc := absenceENLeadRE.FindStringIndex(lower); loc != nil {
		s = strings.TrimSpace(s[loc[1]:])
		lower = strings.ToLower(s)
	}
	if loc := absenceENTailRE.FindStringIndex(lower); loc != nil {
		s = strings.TrimSpace(s[:loc[0]])
	}

	s = strings.TrimSpace(s)
	s = strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '?' || r == '？' || r == '!' || r == '！' ||
			r == '。' || r == '.' || r == '吗' || r == '呢' || r == '呀' || r == '啊'
	})
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "关于")
	if len(s) >= 6 && strings.EqualFold(s[:6], "about ") {
		s = strings.TrimSpace(s[6:])
	}
	s = strings.TrimSpace(s)

	if s == "" || utf8.RuneCountInString(s) < 2 {
		return "", false
	}
	if normalizeAbsenceQuery(s) == normalizeAbsenceQuery(original) {
		return "", false
	}
	return s, true
}

func normalizeAbsenceQuery(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
