package link

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

// VisitorAskUnifiedEnabled reports whether the unified visitor Ask UI is enabled
// for a link (requires qa_enabled on the link and VISITOR_ASK_UNIFIED=1 globally).
func VisitorAskUnifiedEnabled(link db.Link, cfg *config.Config) bool {
	if !link.QaEnabled {
		return false
	}
	return cfg != nil && cfg.VisitorAskUnifiedEnabled
}
