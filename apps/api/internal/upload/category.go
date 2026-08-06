package upload

import (
	"errors"
	"strings"
)

// Document category tri-state (documents.category CHECK).
const (
	CategoryGeneral   = "general"
	CategoryAgreement = "agreement"
	CategoryDealRoom  = "deal_room"
)

var (
	// ErrCategoryImmutable is returned when a deal_room or frozen category cannot be PATCH'd.
	ErrCategoryImmutable = errors.New("document category cannot be changed")
	// ErrCategoryDealRoomViaAPI rejects writing deal_room through UpdateCategory.
	ErrCategoryDealRoomViaAPI = errors.New("deal_room category is managed by deal-room membership")
	// ErrCategoryWhileInRoom rejects library category changes while still attached to a room.
	ErrCategoryWhileInRoom = errors.New("document is in a deal room; remove it before changing category")
)

// ValidateCreateCategory rejects deal_room on POST upload — membership promotes it.
func ValidateCreateCategory(category string) error {
	if strings.EqualFold(strings.TrimSpace(category), CategoryDealRoom) {
		return ErrCategoryDealRoomViaAPI
	}
	return nil
}

// NormalizeCreateCategory maps upload form category to a persisted value.
// deal_room is never accepted here; use deal-room attach to promote.
func NormalizeCreateCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case CategoryAgreement:
		return CategoryAgreement
	default:
		return CategoryGeneral
	}
}

// ValidateManualCategoryChange enforces UpdateCategory rules for general↔agreement only.
func ValidateManualCategoryChange(current, next string, roomMemberships int64) error {
	next = strings.ToLower(strings.TrimSpace(next))
	current = strings.ToLower(strings.TrimSpace(current))
	if next != CategoryGeneral && next != CategoryAgreement {
		if next == CategoryDealRoom {
			return ErrCategoryDealRoomViaAPI
		}
		return ErrCategoryImmutable
	}
	if current == CategoryDealRoom {
		return ErrCategoryImmutable
	}
	if roomMemberships > 0 {
		return ErrCategoryWhileInRoom
	}
	return nil
}
