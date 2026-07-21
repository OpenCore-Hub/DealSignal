package link

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// Folder scope modes for deal-room share links.
//
//	full      — whole room (legacy empty-paths semantics; preserved for existing links)
//	allowlist — only selected folder paths; empty allowlist denies all documents
const (
	FolderScopeModeFull      = "full"
	FolderScopeModeAllowlist = "allowlist"
)

func normalizeFolderScopeMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case FolderScopeModeAllowlist:
		return FolderScopeModeAllowlist
	case FolderScopeModeFull:
		return FolderScopeModeFull
	default:
		return ""
	}
}

// dealRoomUsesFolderAllowlist reports whether a deal-room link enforces an
// explicit folder allowlist. Non-deal-room links never use folder allowlists.
func dealRoomUsesFolderAllowlist(link db.Link) bool {
	if !link.DealRoomID.Valid {
		return false
	}
	mode := normalizeFolderScopeMode(link.FolderScopeMode)
	if mode == "" {
		// Defensive mid-migration fallback: non-empty paths imply allowlist.
		if len(link.FolderScopePaths) > 0 {
			return true
		}
		return false
	}
	return mode == FolderScopeModeAllowlist
}

// folderPathInDealRoomScope reports whether folderPath is visible for a
// deal-room link under the link's folder scope mode.
func folderPathInDealRoomScope(link db.Link, folderPath string) bool {
	if !link.DealRoomID.Valid {
		return true
	}
	if !dealRoomUsesFolderAllowlist(link) {
		return true
	}
	if len(link.FolderScopePaths) == 0 {
		return false
	}
	for _, scopePath := range link.FolderScopePaths {
		if folderPath == scopePath || strings.HasPrefix(folderPath, scopePath+"/") {
			return true
		}
	}
	return false
}

// linkSessionInvalidatingChange reports whether an UpdateLink write should bump
// security_version (forcing visitor sessions to re-authenticate). Folder scope
// and other live-enforced fields are excluded so owners can tighten/widen
// authorization scope and visitors only need a refresh to pick up the new set.
func linkSessionInvalidatingChange(
	existing db.Link,
	requireEmail, requireEmailVerification, requireNDA, requirePassword bool,
	passwordHash pgtype.Text,
	perm string,
	expiresAt pgtype.Timestamptz,
	maxAccess pgtype.Int4,
	ndaDocumentID pgtype.UUID,
	ndaTemplateID pgtype.UUID,
) bool {
	if existing.RequireEmail != requireEmail ||
		existing.RequireEmailVerification != requireEmailVerification ||
		existing.RequireNda != requireNDA ||
		existing.RequirePassword != requirePassword ||
		existing.PermissionType != perm {
		return true
	}
	if existing.PasswordHash.String != passwordHash.String || existing.PasswordHash.Valid != passwordHash.Valid {
		return true
	}
	if existing.NdaDocumentID.Valid != ndaDocumentID.Valid ||
		(ndaDocumentID.Valid && existing.NdaDocumentID.Bytes != ndaDocumentID.Bytes) {
		return true
	}
	if existing.NdaTemplateID.Valid != ndaTemplateID.Valid ||
		(ndaTemplateID.Valid && existing.NdaTemplateID.Bytes != ndaTemplateID.Bytes) {
		return true
	}
	if existing.ExpiresAt.Valid != expiresAt.Valid ||
		(expiresAt.Valid && !existing.ExpiresAt.Time.Equal(expiresAt.Time)) {
		return true
	}
	if existing.MaxAccessCount.Valid != maxAccess.Valid ||
		(maxAccess.Valid && existing.MaxAccessCount.Int32 != maxAccess.Int32) {
		return true
	}
	return false
}
