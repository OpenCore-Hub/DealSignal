package link

import (
	"context"
	"errors"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrLockedFolderScope is returned when a share link allowlist includes a locked folder.
var ErrLockedFolderScope = errors.New("locked folder cannot be included in share link scope")

// AuthorizedDocumentQuerier is the DB surface needed to resolve the Access
// document set for a public link (deal-room folder scope or document links).
type AuthorizedDocumentQuerier interface {
	ListDealRoomDocumentsWithMeta(ctx context.Context, roomID pgtype.UUID) ([]db.ListDealRoomDocumentsWithMetaRow, error)
	ListLinkDocumentsByPublicToken(ctx context.Context, publicToken string) ([]db.ListLinkDocumentsByPublicTokenRow, error)
	GetDocumentByID(ctx context.Context, arg db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error)
	GetDealRoomByID(ctx context.Context, arg db.GetDealRoomByIDParams) (db.DealRoom, error)
}

// dealRoomLinkAccessQuerier resolves deal-room document membership for asset gates.
type dealRoomLinkAccessQuerier interface {
	GetDealRoomByID(ctx context.Context, arg db.GetDealRoomByIDParams) (db.DealRoom, error)
	GetDealRoomDocumentByDocumentID(ctx context.Context, arg db.GetDealRoomDocumentByDocumentIDParams) (db.DealRoomDocument, error)
}

// AuthorizedDocumentIDs returns document IDs visible to a public visitor for
// the given link — the same set Access exposes (including deal-room folder
// allowlists). An empty result means fail-closed: no documents are authorized.
func AuthorizedDocumentIDs(ctx context.Context, q AuthorizedDocumentQuerier, link db.Link) ([]uuid.UUID, error) {
	docs, err := listAuthorizedDocuments(ctx, q, link)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(docs))
	seen := make(map[uuid.UUID]struct{}, len(docs))
	for _, d := range docs {
		if _, ok := seen[d.ID]; ok {
			continue
		}
		seen[d.ID] = struct{}{}
		ids = append(ids, d.ID)
	}
	return ids, nil
}

// authorizedDocument is the Access-parity document projection used by listing
// and Ask Docs retrieval scope.
type authorizedDocument struct {
	ID            uuid.UUID
	Title         string
	SourceType    string
	PageCount     int32
	FolderPath    string
	Status        string
	FileSize      int64
	IncludeMeta   bool // legacy single-doc Access fields (status/fileSize)
	IncludeFolder bool
}

func listAuthorizedDocuments(ctx context.Context, q AuthorizedDocumentQuerier, link db.Link) ([]authorizedDocument, error) {
	if link.DealRoomID.Valid {
		room, err := q.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
			ID:          link.DealRoomID,
			WorkspaceID: link.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		lockedFolders := dealroom.LockedFolderPathSet(room.Settings)

		drDocs, err := q.ListDealRoomDocumentsWithMeta(ctx, link.DealRoomID)
		if err != nil {
			return nil, err
		}
		applyScope := dealRoomUsesFolderAllowlist(link)
		out := make([]authorizedDocument, 0, len(drDocs))
		for _, d := range drDocs {
			if dealroom.ResourceLockedOut(d.Locked, d.FolderPath, lockedFolders) {
				continue
			}
			if applyScope && !folderPathInDealRoomScope(link, d.FolderPath) {
				continue
			}
			out = append(out, authorizedDocument{
				ID:            uuid.UUID(d.DocumentID.Bytes),
				Title:         d.DocumentTitle,
				SourceType:    d.SourceType,
				PageCount:     d.PageCount.Int32,
				FolderPath:    d.FolderPath,
				IncludeFolder: true,
			})
		}
		return out, nil
	}

	linkDocs, err := q.ListLinkDocumentsByPublicToken(ctx, link.PublicToken)
	if err != nil {
		return nil, err
	}
	out := make([]authorizedDocument, 0, len(linkDocs))
	for _, ld := range linkDocs {
		out = append(out, authorizedDocument{
			ID:         uuid.UUID(ld.DocumentID.Bytes),
			Title:      ld.Title,
			SourceType: ld.SourceType,
			PageCount:  ld.PageCount,
		})
	}
	if len(out) > 0 {
		return out, nil
	}

	// Fallback: single-document legacy links.
	if !link.DocumentID.Valid {
		return out, nil
	}
	doc, err := q.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          link.DocumentID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		return out, nil
	}
	return []authorizedDocument{{
		ID:          uuid.UUID(doc.ID.Bytes),
		Title:       doc.Title,
		SourceType:  doc.SourceType,
		PageCount:   doc.PageCount.Int32,
		Status:      doc.Status,
		FileSize:    0,
		IncludeMeta: true,
	}}, nil
}

func dealRoomDocumentAuthorizedForLink(
	ctx context.Context,
	q dealRoomLinkAccessQuerier,
	link db.Link,
	docID uuid.UUID,
) bool {
	return evaluateDealRoomDocumentAccess(ctx, q, link, docID) == linkDocAccessAllowed
}
