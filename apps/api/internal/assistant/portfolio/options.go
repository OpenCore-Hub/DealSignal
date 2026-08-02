// Package portfolio implements Ask Docs cross-room DD coverage portfolio views (P3).
// Aggregation is snapshot-only: never runs cross-room chunk retrieval into Ask Docs.
package portfolio

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

const (
	defaultMaxViews = 5
	defaultMaxRooms = 20
)

// Options controls portfolio availability and soft quotas (D14).
type Options struct {
	Enabled  bool
	MaxViews int
	MaxRooms int
}

// OptionsFromConfig maps config.AskDocsConfig (D8 / D14) into portfolio options.
func OptionsFromConfig(c config.AskDocsConfig) Options {
	o := Options{
		Enabled:  c.PortfolioEnabled,
		MaxViews: c.PortfolioMaxViews,
		MaxRooms: c.PortfolioMaxRooms,
	}
	if o.MaxViews <= 0 {
		o.MaxViews = defaultMaxViews
	}
	if o.MaxRooms <= 0 {
		o.MaxRooms = defaultMaxRooms
	}
	return o
}

// OptionsFromEnv builds options from ASK_DOCS_PORTFOLIO_* + APP_ENV.
// Prefer OptionsFromConfig(cfg.AskDocs) at server wire-up (D8).
func OptionsFromEnv(appEnv string) Options {
	return OptionsFromConfig(config.AskDocsFromEnv(appEnv))
}
