package link

import (
	"context"
	"errors"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	errDocumentLocked       = errors.New("document is locked in this deal room")
	errDocumentOutOfScope   = errors.New("document is not included in this link")
	errDocumentAccessDenied = errors.New("document access denied")
)

// linkDocumentAccessQuerier resolves membership for public document asset gates.
type linkDocumentAccessQuerier interface {
	dealRoomLinkAccessQuerier
	HasLinkDocument(ctx context.Context, arg db.HasLinkDocumentParams) (bool, error)
}

// Link document access denial reasons for public asset endpoints.
type linkDocumentAccessDenial int

const (
	linkDocAccessAllowed linkDocumentAccessDenial = iota
	linkDocAccessDenied
	linkDocAccessLocked
	linkDocAccessOutOfScope
)

func evaluateDealRoomDocumentAccess(
	ctx context.Context,
	q dealRoomLinkAccessQuerier,
	link db.Link,
	docID uuid.UUID,
) linkDocumentAccessDenial {
	row, err := q.GetDealRoomDocumentByDocumentID(ctx, db.GetDealRoomDocumentByDocumentIDParams{
		RoomID:     link.DealRoomID,
		DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
	})
	if err != nil {
		return linkDocAccessDenied
	}
	room, err := q.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          link.DealRoomID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		return linkDocAccessDenied
	}
	lockedFolders := dealroom.LockedFolderPathSet(room.Settings)
	if dealroom.ResourceLockedOut(row.Locked, row.FolderPath, lockedFolders) {
		return linkDocAccessLocked
	}
	if !folderPathInDealRoomScope(link, row.FolderPath) {
		return linkDocAccessOutOfScope
	}
	return linkDocAccessAllowed
}

func evaluateLinkDocumentAccess(
	ctx context.Context,
	q linkDocumentAccessQuerier,
	link db.Link,
	docID uuid.UUID,
) linkDocumentAccessDenial {
	if uuid.UUID(link.DocumentID.Bytes) == docID {
		return linkDocAccessAllowed
	}
	if link.DealRoomID.Valid {
		return evaluateDealRoomDocumentAccess(ctx, q, link, docID)
	}
	inScope, err := q.HasLinkDocument(ctx, db.HasLinkDocumentParams{
		LinkID:     pgtype.UUID{Bytes: link.ID.Bytes, Valid: true},
		DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
	})
	if err != nil || !inScope {
		return linkDocAccessDenied
	}
	return linkDocAccessAllowed
}

// writeLinkDocumentAccessDenied responds with a visitor-safe denial when false.
func writeLinkDocumentAccessDenied(c *gin.Context, denial linkDocumentAccessDenial) bool {
	if denial == linkDocAccessAllowed {
		return true
	}
	switch denial {
	case linkDocAccessLocked:
		c.JSON(403, gin.H{
			"code":    "document_locked",
			"message": httpx.SafeMessage("document_locked", errDocumentLocked),
		})
	case linkDocAccessOutOfScope:
		c.JSON(403, gin.H{
			"code":    "document_out_of_scope",
			"message": httpx.SafeMessage("document_out_of_scope", errDocumentOutOfScope),
		})
	default:
		c.JSON(403, gin.H{
			"code":    "access_denied",
			"message": httpx.SafeMessage("access_denied", errDocumentAccessDenied),
		})
	}
	return false
}
