package heat

import "strings"

// IsKeyPage reports whether a page title matches any keyword configured for the
// given circle. Matching is case-insensitive substring match. Empty titles never
// match.
func IsKeyPage(title string, circle Circle) bool {
	if strings.TrimSpace(title) == "" {
		return false
	}
	lower := strings.ToLower(title)
	for _, kw := range KeywordsForCircle(circle) {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// KeyPagePatterns returns SQL LIKE patterns for the given circle, suitable for
// PostgreSQL's LIKE ANY operator.
func KeyPagePatterns(circle Circle) []string {
	patterns := make([]string, 0, len(KeywordsForCircle(circle)))
	for _, kw := range KeywordsForCircle(circle) {
		patterns = append(patterns, "%"+strings.ToLower(kw)+"%")
	}
	return patterns
}

// KeywordsForCircle returns the flattened keyword list for a circle, falling
// back to the default circle when the requested circle is unknown.
func KeywordsForCircle(circle Circle) []string {
	cfg, ok := configs[circle]
	if !ok {
		cfg = configs[CircleDefault]
	}
	var out []string
	seen := make(map[string]struct{})
	for _, kws := range cfg.KeyPages {
		for _, kw := range kws {
			if _, ok := seen[kw]; ok {
				continue
			}
			seen[kw] = struct{}{}
			out = append(out, kw)
		}
	}
	return out
}
