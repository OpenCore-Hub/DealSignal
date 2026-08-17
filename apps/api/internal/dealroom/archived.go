package dealroom

import "strings"

const documentStatusArchived = "archived"

// IsArchivedDocumentStatus reports a library-archived document.
// Live room lists, AddDocument, visitor Access, and knowledge alignment skip
// these rows. Membership is kept so Unarchive restores the same placement.
func IsArchivedDocumentStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), documentStatusArchived)
}
