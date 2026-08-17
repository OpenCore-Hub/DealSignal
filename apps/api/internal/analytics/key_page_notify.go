package analytics

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// Engaged dwell threshold shared with key-page compliance KPIs and heat.
const keyPageEngagedMinSeconds int32 = 3

// KeyPageNotification is a gated notification for a real key-page read.
type KeyPageNotification struct {
	EventType string // key_page | repeat_key_page
	Metadata  map[string]string
}

// ResolveKeyPageNotification decides whether a just-recorded page view should
// notify the link owner. Requires engaged dwell (≥3s), a resolvable page title
// on the viewed document (event document_id or link primary), and a workspace
// key-page keyword match (circle defaults + workspace extras).
//
// EventType is key_page on the visitor's first engaged key-page view for the
// link, and repeat_key_page on subsequent engaged key-page views.
func (s *Service) ResolveKeyPageNotification(
	ctx context.Context,
	link db.Link,
	visitorID string,
	pageNumber int32,
	durationSeconds int32,
	documentID string,
) (KeyPageNotification, bool, error) {
	if durationSeconds < keyPageEngagedMinSeconds {
		return KeyPageNotification{}, false, nil
	}

	docUUID, err := resolvePageViewDocumentID(link, documentID)
	if err != nil || !docUUID.Valid {
		// Bundle / room links need an explicit document_id — do not fake a match.
		return KeyPageNotification{}, false, nil
	}

	title, err := s.queries.GetPageTitleByDocumentAndNumber(ctx, db.GetPageTitleByDocumentAndNumberParams{
		DocumentID: docUUID,
		PageNumber: pageNumber,
	})
	if err != nil || title == "" {
		return KeyPageNotification{}, false, nil
	}

	rs, err := s.loadWorkspaceRuleSet(ctx, workspaceIDFromLink(link), nil)
	if err != nil {
		return KeyPageNotification{}, false, err
	}
	category := rs.MatchCategory(title)
	if category == "" {
		return KeyPageNotification{}, false, nil
	}

	patterns := rs.Patterns()
	if len(patterns) == 0 {
		return KeyPageNotification{}, false, nil
	}

	count, err := s.queries.CountVisitorEngagedKeyPageViews(ctx, db.CountVisitorEngagedKeyPageViewsParams{
		LinkID:    link.ID,
		VisitorID: pgtype.Text{String: visitorID, Valid: visitorID != ""},
		Patterns:  patterns,
	})
	if err != nil {
		return KeyPageNotification{}, false, fmt.Errorf("count visitor key page views: %w", err)
	}

	eventType := "key_page"
	if count >= 2 {
		eventType = "repeat_key_page"
	}

	meta := map[string]string{
		"page_number":            strconv.Itoa(int(pageNumber)),
		"page_title":             title,
		"category":               category,
		"duration_seconds":       strconv.Itoa(int(durationSeconds)),
		"engaged_key_page_views": strconv.FormatInt(count, 10),
		"circle":                 string(rs.Circle),
		"document_id":            uuidToString(docUUID),
	}
	if docTitle := bundleMemberDocumentTitle(ctx, s, link, docUUID); docTitle != "" {
		meta["document_title"] = docTitle
	}
	return KeyPageNotification{EventType: eventType, Metadata: meta}, true, nil
}

// bundleMemberDocumentTitle is set only when the viewed file is not the share
// primary, so solo / primary-document emails stay unchanged.
func bundleMemberDocumentTitle(ctx context.Context, s *Service, link db.Link, docUUID pgtype.UUID) string {
	if !docUUID.Valid {
		return ""
	}
	if link.DocumentID.Valid && link.DocumentID.Bytes == docUUID.Bytes {
		return ""
	}
	if !link.WorkspaceID.Valid {
		return ""
	}
	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docUUID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Title)
}
