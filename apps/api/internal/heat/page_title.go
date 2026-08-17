package heat

import (
	"regexp"
	"strings"
)

// Keep in lockstep with apps/web/src/lib/insights/pageTitleDisplay.ts.
// Display-only: heat matching still uses the stored pages.title.
var (
	jsonObjectKey    = regexp.MustCompile(`"[^"]{1,80}"\s*:`)
	truncatedJSONKey = regexp.MustCompile(`^\w{1,40}":`)
)

// DisplayablePageTitle hides PDF text dumps that were stored as page titles.
func DisplayablePageTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" || looksLikeStructuredPageDump(t) {
		return ""
	}
	return t
}

func looksLikeStructuredPageDump(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return false
	}
	switch t[0] {
	case '{', '[':
		return true
	}
	if truncatedJSONKey.MatchString(t) {
		return true
	}
	keys := jsonObjectKey.FindAllStringIndex(t, 3)
	n := len(keys)
	if n >= 2 {
		return true
	}
	return n >= 1 && strings.ContainsAny(t, "{}[]")
}
