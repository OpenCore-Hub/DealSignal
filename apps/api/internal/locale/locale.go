package locale

import (
	"context"
	"strings"
)

type localeKey struct{}

// WithLocale returns a context that carries the requested email locale.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey{}, locale)
}

// FromContext returns the locale stored in the context, or empty.
func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(localeKey{}).(string); ok {
		return v
	}
	return ""
}

// Normalize collapses common locale variants into a canonical form.
// Unknown or empty input falls back to "en".
func Normalize(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "" {
		return "en"
	}

	switch locale {
	case "zh", "zh-cn", "zh-hans", "zh-hans-cn", "cmn":
		return "zh-CN"
	case "zh-tw", "zh-hant", "zh-hant-tw":
		return "zh-TW"
	case "en", "en-us", "en-gb", "en-ca", "en-au":
		return "en"
	}

	if before, _, ok := strings.Cut(locale, "-"); ok && before == "zh" {
		return "zh-CN"
	}
	return "en"
}
