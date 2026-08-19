package radar

import (
	"net/url"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
)

// navigatePath builds the host deep-link for an operational or signal-backed item.
// Paths mirror apps/web/src/lib/actionNavigation.ts so FE and API stay aligned.
func navigatePath(workspaceSlug string, sourceType, sourceID, targetID string, formalAsk bool) string {
	slug := strings.TrimSpace(workspaceSlug)
	if slug == "" || sourceType == "" || sourceID == "" {
		return ""
	}
	switch sourceType {
	case action.SourceTypeLinkAccessRequest:
		return documentsSharePath(slug, sourceID)
	case action.SourceTypeDealRoomLinkAccessRequest:
		if targetID == "" {
			return ""
		}
		return dealRoomAccessPath(slug, targetID, sourceID)
	case action.SourceTypeRoomAccessRequest:
		return dealRoomAccessPath(slug, sourceID, "")
	case action.SourceTypeRoomNDA:
		roomID := targetID
		if roomID == "" {
			roomID = sourceID // legacy room-keyed rows
		}
		return dealRoomAccessPath(slug, roomID, "")
	case action.SourceTypeExpiringRoom:
		return "/" + slug + "/deal-rooms/" + sourceID
	case action.SourceTypeDealRoomLinkQuestion:
		if targetID == "" {
			return ""
		}
		roomID, linkID := parseDealRoomAskTarget(targetID)
		return dealRoomAskPath(slug, roomID, linkID, formalAsk)
	case action.SourceTypeLinkQuestion:
		if targetID == "" {
			return ""
		}
		return libraryAskPath(slug, targetID, formalAsk)
	case action.SourceTypeUploadedFile:
		return "/" + slug + "/links/" + sourceID
	case action.SourceTypeExpiringLink:
		// Room vs library is decided in buildItem (needs LinkMeta.DealRoomID).
		return "/" + slug + "/links/" + sourceID
	default:
		return ""
	}
}

// expiringLinkPath is the host destination for operational link renew.
// Library shares open the existing expiry editor. Deal-room shares stay on
// link detail — the document-library bundle pipeline must not edit them.
func expiringLinkPath(workspaceSlug, linkID, dealRoomID string) string {
	slug := strings.TrimSpace(workspaceSlug)
	link := strings.TrimSpace(linkID)
	if slug == "" || link == "" {
		return ""
	}
	if strings.TrimSpace(dealRoomID) != "" {
		return "/" + slug + "/links/" + link
	}
	return "/" + slug + "/links/" + link + "/edit?focus=expiry"
}

// diligenceRemediationPath is the host destination for a Diligence gate item
// that has no operational sourceType (blocked_attempt / allowlist hold).
// Rooms go to Access (allowlist). Library goes to the share link detail
// (access log), not the Share request inbox — that inbox is empty unless
// there is a pending link_access_request (those keep documentsSharePath via
// navigatePath / sourceType). Document library and deal room must not cross.
func diligenceRemediationPath(workspaceSlug, dealRoomID, linkID string) string {
	slug := strings.TrimSpace(workspaceSlug)
	roomID := strings.TrimSpace(dealRoomID)
	link := strings.TrimSpace(linkID)
	if slug == "" {
		return ""
	}
	if roomID != "" {
		return dealRoomAccessPath(slug, roomID, link)
	}
	if link != "" {
		return insightsPath(slug, link, "")
	}
	return ""
}

func evidencePath(workspaceSlug, documentID, linkID, contactID, page string) string {
	slug := strings.TrimSpace(workspaceSlug)
	if slug == "" {
		return ""
	}
	if documentID != "" {
		path := "/" + slug + "/documents/" + documentID + "?tab=analytics"
		if page != "" {
			path = "/" + slug + "/documents/" + documentID + "?tab=content&page=" + url.QueryEscape(page)
		}
		return path
	}
	if linkID != "" {
		return "/" + slug + "/links/" + linkID
	}
	if contactID != "" {
		return "/" + slug + "/contacts/" + contactID
	}
	return ""
}

func documentsSharePath(slug, linkID string) string {
	return "/" + slug + "/documents?tab=shared&linkId=" + url.QueryEscape(linkID)
}

func dealRoomAccessPath(slug, roomID, linkID string) string {
	path := "/" + slug + "/deal-rooms/" + roomID + "?tab=access"
	if linkID != "" {
		path += "&linkId=" + url.QueryEscape(linkID)
	}
	return path
}

func dealRoomAskPath(slug, roomID, linkID string, formalQueue bool) string {
	q := url.Values{}
	// DealRoomQATab mounts only when tab=qa. Missing tab defaults to documents.
	q.Set("tab", "qa")
	if formalQueue {
		q.Set("askInbox", "formal_queue")
	} else {
		q.Set("askInbox", "needs_host")
	}
	if linkID != "" {
		q.Set("linkId", linkID)
	}
	return "/" + slug + "/deal-rooms/" + roomID + "?" + q.Encode()
}

func libraryAskPath(slug, linkID string, formalQueue bool) string {
	q := url.Values{}
	if formalQueue {
		q.Set("askInbox", "formal_queue")
	} else {
		q.Set("askInbox", "needs_host")
	}
	return "/" + slug + "/links/" + linkID + "?" + q.Encode()
}

// parseDealRoomAskTarget accepts "roomId" or "roomId/linkId".
func parseDealRoomAskTarget(target string) (roomID, linkID string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", ""
	}
	if i := strings.IndexByte(target, '/'); i >= 0 {
		return target[:i], target[i+1:]
	}
	return target, ""
}
