package link

import (
	"context"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

// AnalyticsSecurityRecorder adapts analytics.Service for Ask security events.
type AnalyticsSecurityRecorder struct {
	Svc *analytics.Service
}

func (r AnalyticsSecurityRecorder) RecordSecurityEvent(
	ctx context.Context,
	link db.Link,
	eventType, visitorID, email, ip, ua, reason string,
) error {
	if r.Svc == nil {
		return nil
	}
	return r.Svc.RecordSecurityEvent(ctx, link, eventType, visitorID, email, ip, ua, reason)
}
