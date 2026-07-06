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
