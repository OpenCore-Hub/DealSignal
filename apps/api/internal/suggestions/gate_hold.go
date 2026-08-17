package suggestions

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

func visitorEmailFromGateHold(subtype string, events []db.ListRecentSecurityEventsByLinkRow) string {
	if subtype != SubtypeBlockedAttempt {
		return ""
	}
	for _, ev := range events {
		if !gateHoldEventType(ev.EventType) {
			continue
		}
		email := strings.TrimSpace(ev.Email.String)
		if ev.Email.Valid && email != "" {
			return email
		}
	}
	return ""
}

func gateHoldEventType(eventType string) bool {
	switch eventType {
	case "blocked_email", "blocked_domain", "not_in_allow_list", "no_allow_match":
		return true
	default:
		return false
	}
}
