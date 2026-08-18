package dealroom

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
)

const uploadConflictLocked = "locked"

// WorkspaceDocuments is the library upload surface used by room-scoped upload.
// Implemented by upload.Service. Kept narrow so dealroom does not open POST /documents.
type WorkspaceDocuments interface {
	CreateDealRoomDocument(ctx context.Context, userID, tenantID, workspaceID, roomID string, fileHeader *multipart.FileHeader, after upload.PersistHook) (upload.Document, error)
	ReplaceDocument(ctx context.Context, workspaceID, documentID string, fileHeader *multipart.FileHeader) (upload.Document, error)
}

// WithDocuments wires library document create/replace for room-scoped upload.
func WithDocuments(d WorkspaceDocuments) ServiceOption {
	return func(s *Service) { s.docs = d }
}

// RoomUploadExists is the preflight result for GET .../uploads/exists.
type RoomUploadExists struct {
	Exists      bool
	Replaceable bool
	Reason      string
	DocumentID  string
	Title       string
}

// RoomUploadRequest is the multipart form payload for POST .../uploads.
type RoomUploadRequest struct {
	FolderPath string
	SortOrder  int32
	Replace    bool
}

type roomTitleConflict struct {
	exists      bool
	replaceable bool
	reason      string
	live        db.GetLiveDealRoomDocumentByTitleRow
	membership  db.DealRoomDocument
}

// CheckRoomUpload reports whether a live title collides in this room, and
// whether this room may replace it. NeedContribute so room guests cannot probe titles.
func (s *Service) CheckRoomUpload(ctx context.Context, roomID, workspaceID, userID, filename string) (RoomUploadExists, error) {
	room, folders, err := s.roomUploadContext(ctx, roomID, workspaceID, userID)
	if err != nil {
		return RoomUploadExists{}, err
	}
	title := upload.NormalizeUploadFilename(filename)
	if title == "" {
		return RoomUploadExists{}, fmt.Errorf("%w: filename is required", upload.ErrUnsupportedUpload)
	}
	conflict, err := s.lookupRoomTitleConflict(ctx, room, folders, title)
	if err != nil {
		return RoomUploadExists{}, err
	}
	if !conflict.exists {
		return RoomUploadExists{}, nil
	}
	return RoomUploadExists{
		Exists:      true,
		Replaceable: conflict.replaceable,
		Reason:      conflict.reason,
		Title:       conflict.live.Title,
		DocumentID:  uuid.UUID(conflict.live.ID.Bytes).String(),
	}, nil
}

// UploadDocument creates or replaces a file inside a room. Workspace guests
// stay guest; the document is never minted as general. Library or other-room
// same-name files are ignored — this path always creates a new deal_room copy
// unless this room already has the title.
func (s *Service) UploadDocument(ctx context.Context, roomID, workspaceID, userID, tenantID string, fileHeader *multipart.FileHeader, req RoomUploadRequest) (upload.Document, error) {
	room, folders, err := s.roomUploadContext(ctx, roomID, workspaceID, userID)
	if err != nil {
		return upload.Document{}, err
	}
	folderPath := normalizeFolderPath(req.FolderPath)
	if folderPath == "/" || folderPath == "" {
		return upload.Document{}, ErrFolderPathRequired
	}
	if !folderExists(folders, folderPath) {
		return upload.Document{}, ErrFolderNotFound
	}
	if folderIsLocked(folders, folderPath) {
		return upload.Document{}, ErrResourceLocked
	}

	title := upload.NormalizeUploadFilename(fileHeader.Filename)
	if title == "" {
		return upload.Document{}, fmt.Errorf("%w: filename is required", upload.ErrUnsupportedUpload)
	}
	conflict, err := s.lookupRoomTitleConflict(ctx, room, folders, title)
	if err != nil {
		return upload.Document{}, err
	}
	if conflict.exists && conflict.reason == uploadConflictLocked {
		return upload.Document{}, ErrResourceLocked
	}
	if conflict.exists && !req.Replace {
		return upload.Document{}, &upload.ExistingDocumentError{
			ID:    uuid.UUID(conflict.live.ID.Bytes).String(),
			Title: conflict.live.Title,
		}
	}
	if s.docs == nil {
		return upload.Document{}, ErrDocumentUploadNotConfigured
	}

	if conflict.exists && req.Replace {
		doc, err := s.docs.ReplaceDocument(ctx, workspaceID, uuid.UUID(conflict.live.ID.Bytes).String(), fileHeader)
		if err != nil {
			return upload.Document{}, s.reclassifyExists(ctx, room, err)
		}
		if _, addErr := s.AddDocument(ctx, roomID, workspaceID, userID, doc.ID, folderPath, req.SortOrder); addErr != nil {
			return upload.Document{}, addErr
		}
		return doc, nil
	}

	after := func(ctx context.Context, q *db.Queries, created db.CreateDocumentRow) error {
		_, err := q.AddDealRoomDocument(ctx, db.AddDealRoomDocumentParams{
			TenantID:    room.TenantID,
			WorkspaceID: room.WorkspaceID,
			RoomID:      room.ID,
			DocumentID:  created.ID,
			FolderPath:  folderPath,
			SortOrder:   req.SortOrder,
		})
		return err
	}
	doc, err := s.docs.CreateDealRoomDocument(ctx, userID, tenantID, workspaceID, roomID, fileHeader, after)
	if err != nil {
		return upload.Document{}, s.reclassifyExists(ctx, room, err)
	}
	s.invalidateListCache(ctx, workspaceID)
	return doc, nil
}

func (s *Service) roomUploadContext(ctx context.Context, roomID, workspaceID, userID string) (db.DealRoom, []Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.DealRoom{}, nil, err
	}
	if err := s.requireRoomContribute(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return db.DealRoom{}, nil, err
	}
	folders, err := s.loadFolders(room)
	if err != nil {
		return db.DealRoom{}, nil, err
	}
	return room, folders, nil
}

func (s *Service) lookupRoomTitleConflict(ctx context.Context, room db.DealRoom, folders []Folder, title string) (roomTitleConflict, error) {
	live, err := s.queries.GetLiveDealRoomDocumentByTitle(ctx, db.GetLiveDealRoomDocumentByTitleParams{
		RoomID: room.ID,
		Title:  title,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return roomTitleConflict{}, nil
	}
	if err != nil {
		return roomTitleConflict{}, fmt.Errorf("lookup existing document: %w", err)
	}
	membership, memErr := s.queries.GetDealRoomDocumentByDocumentID(ctx, db.GetDealRoomDocumentByDocumentIDParams{
		RoomID:     room.ID,
		DocumentID: live.ID,
	})
	if memErr != nil {
		if errors.Is(memErr, pgx.ErrNoRows) {
			return roomTitleConflict{}, nil
		}
		return roomTitleConflict{}, memErr
	}
	locked := membership.Locked || folderIsLocked(folders, membership.FolderPath)
	if locked {
		return roomTitleConflict{
			exists:     true,
			reason:     uploadConflictLocked,
			live:       live,
			membership: membership,
		}, nil
	}
	return roomTitleConflict{
		exists:      true,
		replaceable: true,
		live:        live,
		membership:  membership,
	}, nil
}

func (s *Service) reclassifyExists(ctx context.Context, room db.DealRoom, err error) error {
	var existsErr *upload.ExistingDocumentError
	if !errors.As(err, &existsErr) {
		return err
	}
	docID, parseErr := uuid.Parse(existsErr.ID)
	if parseErr != nil {
		return existsErr
	}
	_, memErr := s.queries.GetDealRoomDocumentByDocumentID(ctx, db.GetDealRoomDocumentByDocumentIDParams{
		RoomID:     room.ID,
		DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
	})
	if memErr == nil {
		return existsErr
	}
	if !errors.Is(memErr, pgx.ErrNoRows) {
		return memErr
	}
	// Race: persist lock found a this-room title that Check missed. Keep
	// document_exists so the client can offer replace. Other ids are not
	// this-room occupants — surface as exists so the caller retries Check.
	return existsErr
}

type uploadedDocumentView struct {
	row    db.GetDocumentByIDAndTenantRow
	job    db.IngestionJob
	hasJob bool
}

func (s *Service) loadUploadedDocument(ctx context.Context, tenantID, workspaceID, documentID string) (uploadedDocumentView, error) {
	id, err := uuid.Parse(documentID)
	if err != nil {
		return uploadedDocumentView{}, fmt.Errorf("invalid document id")
	}
	ws, err := uuid.Parse(workspaceID)
	if err != nil {
		return uploadedDocumentView{}, fmt.Errorf("invalid workspace id")
	}
	ten, err := uuid.Parse(tenantID)
	if err != nil {
		return uploadedDocumentView{}, fmt.Errorf("invalid tenant id")
	}
	row, err := s.queries.GetDocumentByIDAndTenant(ctx, db.GetDocumentByIDAndTenantParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		TenantID:    pgtype.UUID{Bytes: ten, Valid: true},
	})
	if err != nil {
		return uploadedDocumentView{}, err
	}
	job, jobErr := s.queries.GetIngestionJobByDocument(ctx, row.ID)
	if jobErr != nil {
		return uploadedDocumentView{row: row}, nil
	}
	return uploadedDocumentView{row: row, job: job, hasJob: true}, nil
}