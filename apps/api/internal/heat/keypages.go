package heat

// IsKeyPage reports whether a page title matches any keyword configured for the
// given circle. Matching is case-insensitive substring match. Empty titles never
// match.
func IsKeyPage(title string, circle Circle) bool {
	return NewRuleSet(circle, nil).IsKeyPage(title)
}

// MatchKeyPageCategory returns the first key-page category whose keywords match
// title (case-insensitive substring). Categories are checked in stable sorted
// order so results are deterministic across runs. Empty titles never match.
func MatchKeyPageCategory(title string, circle Circle) string {
	return NewRuleSet(circle, nil).MatchCategory(title)
}

// KeyPageCategories returns the category keys configured for the circle
// (stable sorted order).
func KeyPageCategories(circle Circle) []string {
	return NewRuleSet(circle, nil).Categories()
}

// KeyPageRule is one category and its title-substring keywords for a circle.
type KeyPageRule struct {
	Category string
	Keywords []string
}

// KeyPageRules returns category→keyword rules for the circle (stable sorted
// categories). Used by Insights to disclose the same rules heat scoring uses.
func KeyPageRules(circle Circle) []KeyPageRule {
	return NewRuleSet(circle, nil).Rules()
}

// KeyPagePatterns returns SQL LIKE patterns for the given circle, suitable for
// PostgreSQL's LIKE ANY operator.
func KeyPagePatterns(circle Circle) []string {
	return NewRuleSet(circle, nil).Patterns()
}

// KeywordsForCircle returns the flattened keyword list for a circle, falling
// back to the default circle when the requested circle is unknown.
func KeywordsForCircle(circle Circle) []string {
	return NewRuleSet(circle, nil).Keywords()
}
