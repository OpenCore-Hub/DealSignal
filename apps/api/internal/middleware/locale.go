package middleware

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
)

var supportedLocales = []language.Tag{
	language.English,
	language.Chinese,
	language.MustParse("zh-CN"),
	language.MustParse("zh-TW"),
}

var localeMatcher = language.NewMatcher(supportedLocales)

// Locale parses the Accept-Language header and stores the normalized locale in
// the request context so downstream mailers can render localized templates.
func Locale() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Accept-Language")
		if loc := parseAcceptLanguage(header); loc != "" {
			c.Request = c.Request.WithContext(locale.WithLocale(c.Request.Context(), loc))
		}
		c.Next()
	}
}

// parseAcceptLanguage returns the first usable language tag from an
// Accept-Language header using BCP-47 parsing and supported-locale matching.
func parseAcceptLanguage(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	tags, _, err := language.ParseAcceptLanguage(header)
	if err != nil || len(tags) == 0 {
		return ""
	}
	matched, _, _ := localeMatcher.Match(tags...)
	return locale.Normalize(matched.String())
}
