package heat

import (
	"strings"
	"unicode"
)

// KeywordLang selects which built-in key-page keywords participate in matching.
// Workspace extras are never language-filtered.
type KeywordLang string

const (
	// KeywordLangAny keeps both English and Chinese built-ins (background jobs / no locale).
	KeywordLangAny KeywordLang = ""
	KeywordLangEN  KeywordLang = "en"
	KeywordLangZH  KeywordLang = "zh"
)

// KeywordLangFromLocale maps Accept-Language / UI locale to a keyword filter.
// Empty locale → Any (do not silently drop a language for workers).
func KeywordLangFromLocale(loc string) KeywordLang {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return KeywordLangAny
	}
	lower := strings.ToLower(loc)
	if lower == "zh" || strings.HasPrefix(lower, "zh-") || lower == "cmn" {
		return KeywordLangZH
	}
	return KeywordLangEN
}

// isCJKKeyword reports whether kw contains Han script (used as the ZH keyword signal).
func isCJKKeyword(kw string) bool {
	for _, r := range kw {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// filterKeywordsByLang keeps EN or ZH built-ins. Any returns the input unchanged.
func filterKeywordsByLang(kws []string, lang KeywordLang) []string {
	switch lang {
	case KeywordLangEN, KeywordLangZH:
	default:
		return kws
	}
	out := make([]string, 0, len(kws))
	for _, kw := range kws {
		cjk := isCJKKeyword(kw)
		if lang == KeywordLangZH && cjk {
			out = append(out, kw)
			continue
		}
		if lang == KeywordLangEN && !cjk {
			out = append(out, kw)
		}
	}
	return out
}
