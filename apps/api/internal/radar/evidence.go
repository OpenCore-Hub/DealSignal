package radar

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// ErrItemNotFound is returned when the radar work item (action) is missing.
var ErrItemNotFound = errors.New("radar item not found")

// EvidencePack is the right-rail payload for a selected work item.
type EvidencePack struct {
	ItemID         string            `json:"itemId"`
	Product        Product           `json:"product"`
	Headline       string            `json:"headline"`
	WhyNow         string            `json:"whyNow,omitempty"` // deprecated free-text; prefer WhyNowCode
	WhyNowCode     string            `json:"whyNowCode,omitempty"`
	WhyNowHours    int               `json:"whyNowHours,omitempty"`
	Actor          string            `json:"actor,omitempty"`
	DealName       string            `json:"dealName,omitempty"`
	LinkID         string            `json:"linkId,omitempty"`
	DocumentID     string            `json:"documentId,omitempty"`
	NavigatePath   string            `json:"navigatePath,omitempty"`
	EvidencePath   string            `json:"evidencePath,omitempty"`
	InsightsPath   string            `json:"insightsPath,omitempty"`
	Metrics        *EvidenceMetrics  `json:"metrics,omitempty"`
	KeyPageTitles  []string          `json:"keyPageTitles,omitempty"`
	TopPages       []EvidencePage    `json:"topPages,omitempty"`
	RecentVisitors []EvidenceVisitor `json:"recentVisitors,omitempty"`
	SecurityEvents []EvidenceEvent   `json:"securityEvents,omitempty"`
}

// EvidenceMetrics are rolling engagement counters for the linked share.
type EvidenceMetrics struct {
	Opens24h             int `json:"opens24h"`
	UniqueVisitors24h    int `json:"uniqueVisitors24h"`
	ForwardSignals24h    int `json:"forwardSignals24h"`
	Downloads24h         int `json:"downloads24h"`
	CaptureAttempts24h   int `json:"captureAttempts24h,omitempty"`
}

// EvidencePage is a top page-view row.
type EvidencePage struct {
	PageNumber         int32   `json:"pageNumber"`
	Views              int64   `json:"views"`
	AvgDurationSeconds float64 `json:"avgDurationSeconds"`
}

// EvidenceVisitor is a recent visitor chip.
type EvidenceVisitor struct {
	VisitorID     string `json:"visitorId"`
	Email         string `json:"email,omitempty"`
	TotalViews    int64  `json:"totalViews"`
	LastAccessAt  string `json:"lastAccessAt,omitempty"`
}

// EvidenceEvent is a recent security_events row.
type EvidenceEvent struct {
	EventType string `json:"eventType"`
	Reason    string `json:"reason,omitempty"`
	Email     string `json:"email,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// GetEvidence loads a real evidence pack for a radar work item (action UUID).
func (s *Service) GetEvidence(ctx context.Context, workspaceID, itemID, workspaceSlug string) (EvidencePack, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return EvidencePack{}, err
	}
	actionUUID, err := pgUUID(itemID)
	if err != nil {
		return EvidencePack{}, ErrItemNotFound
	}

	action, err := s.queries.GetActionItemByID(ctx, db.GetActionItemByIDParams{
		ID:          actionUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvidencePack{}, ErrItemNotFound
		}
		return EvidencePack{}, fmt.Errorf("get action: %w", err)
	}

	var sig *db.Signal
	if action.SignalID.Valid {
		got, serr := s.queries.GetSignalByID(ctx, db.GetSignalByIDParams{
			ID:          action.SignalID,
			WorkspaceID: wsUUID,
		})
		if serr == nil {
			sig = &got
		}
	}

	product, verb, ok := classify(action, sig)
	if !ok {
		// Still return a minimal pack for non-radar products (e.g. legacy bounce).
		product = ProductBuyingWindow
		verb = VerbOpen
	}

	item := buildItem(CompileInput{WorkspaceSlug: workspaceSlug, Now: s.now()}, action, sig, product, verb, s.now())

	// Prefer structured whyNowCode (FE i18n); keep WhyNow empty of free-text fallbacks.
	slaDue := parseRFC3339(item.SlaDueAt, s.now())
	whyCode, whyHours := whyNowCode(item.Product, slaDue, s.now(), len(item.CoalescedFrom))
	item.WhyNowCode = whyCode
	item.WhyNowHours = whyHours
	item.State = "open"

	pack := EvidencePack{
		ItemID:       item.ID,
		Product:      item.Product,
		Headline:     item.Headline,
		WhyNowCode:   whyCode,
		WhyNowHours:  whyHours,
		Actor:        item.Actor,
		DealName:     item.DealName,
		LinkID:       item.LinkID,
		DocumentID:   item.DocumentID,
		NavigatePath: item.NavigatePath,
		EvidencePath: item.EvidencePath,
		InsightsPath: insightsPath(workspaceSlug, item.LinkID, item.DocumentID),
	}

	if sig != nil {
		if titles := contextKeyPageTitles(sig.Context); len(titles) > 0 {
			pack.KeyPageTitles = titles
		}
	}

	if item.LinkID == "" {
		return pack, nil
	}
	linkUUID, err := pgUUID(item.LinkID)
	if err != nil {
		return pack, nil
	}

	if metrics, err := s.queries.GetLinkAccessMetrics24h(ctx, linkUUID); err == nil {
		pack.Metrics = &EvidenceMetrics{
			Opens24h:          int(metrics.Opens),
			UniqueVisitors24h: int(metrics.UniqueVisitors),
			ForwardSignals24h: int(metrics.ForwardSignals),
			Downloads24h:      int(metrics.Downloads),
		}
	}
	if captures, err := s.queries.CountCaptureAttemptsByLink24h(ctx, linkUUID); err == nil && captures > 0 {
		if pack.Metrics == nil {
			pack.Metrics = &EvidenceMetrics{}
		}
		pack.Metrics.CaptureAttempts24h = int(captures)
	}

	if pages, err := s.queries.ListTopPagesByLink(ctx, linkUUID); err == nil {
		limit := 5
		if len(pages) < limit {
			limit = len(pages)
		}
		for i := 0; i < limit; i++ {
			pack.TopPages = append(pack.TopPages, EvidencePage{
				PageNumber:         pages[i].PageNumber,
				Views:              pages[i].Views,
				AvgDurationSeconds: pages[i].AvgDurationSeconds,
			})
		}
	}

	if visitors, err := s.queries.ListRecentVisitorsByLink(ctx, db.ListRecentVisitorsByLinkParams{
		LinkID: linkUUID,
		Limit:  int32(5),
		Offset: int32(0),
	}); err == nil {
		for _, v := range visitors {
			ev := EvidenceVisitor{
				VisitorID:  textOrEmpty(v.VisitorID),
				Email:      v.VisitorEmail,
				TotalViews: v.TotalViews,
			}
			if v.LastAccessAt.Valid {
				ev.LastAccessAt = v.LastAccessAt.Time.UTC().Format(time.RFC3339)
			}
			pack.RecentVisitors = append(pack.RecentVisitors, ev)
		}
	}

	if product == ProductLeakWatch || product == ProductAbuseGuard || product == ProductAccessDecay {
		if events, err := s.queries.ListRecentSecurityEventsByLink(ctx, linkUUID); err == nil {
			limit := 5
			if len(events) < limit {
				limit = len(events)
			}
			for i := 0; i < limit; i++ {
				e := events[i]
				pack.SecurityEvents = append(pack.SecurityEvents, EvidenceEvent{
					EventType: e.EventType,
					Reason:    textOrEmpty(e.Reason),
					Email:     textOrEmpty(e.Email),
					CreatedAt: e.CreatedAt.Time.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	return pack, nil
}

func insightsPath(slug, linkID, documentID string) string {
	if slug == "" {
		return ""
	}
	if linkID != "" {
		return "/" + slug + "/links/" + linkID
	}
	if documentID != "" {
		return "/" + slug + "/documents/" + documentID + "?tab=analytics"
	}
	return "/" + slug + "/insights/overview"
}

func contextKeyPageTitles(raw []byte) []string {
	ctx, ok := unmarshalMap(raw)
	if !ok {
		return nil
	}
	rawTitles, ok := ctx["keyPageTitles"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rawTitles))
	for _, v := range rawTitles {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
