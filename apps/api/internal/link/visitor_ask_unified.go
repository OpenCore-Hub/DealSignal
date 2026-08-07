package link

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

// VisitorAskUnifiedEnabled reports whether the unified visitor Ask UI is enabled
// for a link. Deal-room links with qa_enabled use unified Ask by default; other
// links require VISITOR_ASK_UNIFIED=1 globally.
func VisitorAskUnifiedEnabled(link db.Link, cfg *config.Config) bool {
	if !link.QaEnabled {
		return false
	}
	if link.DealRoomID.Valid {
		return true
	}
	return cfg != nil && cfg.VisitorAskUnifiedEnabled
}
