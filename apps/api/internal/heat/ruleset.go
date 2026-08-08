package heat

import (
	"sort"
	"strings"
)

// RuleSet is the effective key-page matching rules for a workspace:
// circle built-ins merged with additive extra keywords (never replaces defaults).
// Lang filters built-in EN/ZH keywords (Settings → Language / Accept-Language);
// workspace extras always apply in both languages.
type RuleSet struct {
	Circle Circle
	Extra  map[string][]string
	Lang   KeywordLang
}

// NewRuleSet builds a RuleSet. Unknown circles fall back to CircleDefault.
// Extra keywords are normalized (trim, drop empties) per category.
// Built-in language filter defaults to KeywordLangAny (both EN + ZH).
func NewRuleSet(circle Circle, extra map[string][]string) RuleSet {
	switch circle {
	case CircleFounder, CircleInvestor, CircleSales:
	default:
		circle = CircleDefault
	}
	return RuleSet{Circle: circle, Extra: normalizeExtra(extra), Lang: KeywordLangAny}
}

// WithLang returns a copy with the built-in keyword language filter applied.
func (r RuleSet) WithLang(lang KeywordLang) RuleSet {
	r.Lang = lang
	return r
}

func normalizeExtra(extra map[string][]string) map[string][]string {
	if len(extra) == 0 {
		return nil
	}
	out := make(map[string][]string, len(extra))
	for cat, kws := range extra {
		cat = strings.TrimSpace(strings.ToLower(cat))
		if cat == "" {
			continue
		}
		seen := make(map[string]struct{})
		var cleaned []string
		for _, kw := range kws {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			key := strings.ToLower(kw)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			cleaned = append(cleaned, kw)
		}
		if len(cleaned) > 0 {
			out[cat] = cleaned
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r RuleSet) baseConfig() Config {
	cfg, ok := configs[r.Circle]
	if !ok {
		cfg = configs[CircleDefault]
	}
	return cfg
}

// mergedKeyPages returns circle defaults (language-filtered) + extras (always).
func (r RuleSet) mergedKeyPages() map[string][]string {
	base := r.baseConfig().KeyPages
	out := make(map[string][]string, len(base)+len(r.Extra))
	for cat, kws := range base {
		out[cat] = append([]string(nil), filterKeywordsByLang(kws, r.Lang)...)
	}
	for cat, extras := range r.Extra {
		seen := make(map[string]struct{}, len(out[cat]))
		for _, kw := range out[cat] {
			seen[strings.ToLower(kw)] = struct{}{}
		}
		for _, kw := range extras {
			key := strings.ToLower(kw)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out[cat] = append(out[cat], kw)
		}
	}
	return out
}

// Categories returns stable sorted category keys in the merged set.
func (r RuleSet) Categories() []string {
	pages := r.mergedKeyPages()
	cats := make([]string, 0, len(pages))
	for cat := range pages {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	return cats
}

// MatchCategory returns the first matching category for title, or "".
func (r RuleSet) MatchCategory(title string) string {
	if strings.TrimSpace(title) == "" {
		return ""
	}
	pages := r.mergedKeyPages()
	lower := strings.ToLower(title)
	for _, cat := range r.Categories() {
		for _, kw := range pages[cat] {
			if kw == "" {
				continue
			}
			if strings.Contains(lower, strings.ToLower(kw)) {
				return cat
			}
		}
	}
	return ""
}

// IsKeyPage reports whether title matches any keyword in the rule set.
func (r RuleSet) IsKeyPage(title string) bool {
	return r.MatchCategory(title) != ""
}

// Keywords returns the flattened deduplicated keyword list.
func (r RuleSet) Keywords() []string {
	pages := r.mergedKeyPages()
	var out []string
	seen := make(map[string]struct{})
	for _, cat := range r.Categories() {
		for _, kw := range pages[cat] {
			key := strings.ToLower(kw)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, kw)
		}
	}
	return out
}

// Patterns returns SQL LIKE patterns for PostgreSQL LIKE ANY.
func (r RuleSet) Patterns() []string {
	kws := r.Keywords()
	patterns := make([]string, 0, len(kws))
	for _, kw := range kws {
		patterns = append(patterns, "%"+strings.ToLower(kw)+"%")
	}
	return patterns
}

// Rules returns category→keywords for Insights disclosure (merged effective set).
func (r RuleSet) Rules() []KeyPageRule {
	pages := r.mergedKeyPages()
	cats := r.Categories()
	out := make([]KeyPageRule, 0, len(cats))
	for _, cat := range cats {
		out = append(out, KeyPageRule{
			Category: cat,
			Keywords: append([]string(nil), pages[cat]...),
		})
	}
	return out
}
