package link

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeLinkDocumentAccessQuerier struct {
	fakeAuthorizedDocQuerier
	roomDocsByID map[string]db.DealRoomDocument
	hasLinkDoc   bool
}

func (f *fakeLinkDocumentAccessQuerier) GetDealRoomDocumentByDocumentID(_ context.Context, arg db.GetDealRoomDocumentByDocumentIDParams) (db.DealRoomDocument, error) {
	id := uuid.UUID(arg.DocumentID.Bytes).String()
	row, ok := f.roomDocsByID[id]
	if !ok {
		return db.DealRoomDocument{}, context.Canceled
	}
	return row, nil
}

func (f *fakeLinkDocumentAccessQuerier) HasLinkDocument(_ context.Context, _ db.HasLinkDocumentParams) (bool, error) {
	return f.hasLinkDoc, nil
}

func TestEvaluateDealRoomDocumentAccess_LockedFolder(t *testing.T) {
	roomID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	docID := uuid.New()
	q := &fakeLinkDocumentAccessQuerier{
		fakeAuthorizedDocQuerier: fakeAuthorizedDocQuerier{
			roomOK: true,
			room: db.DealRoom{
				ID:       roomID,
				Settings: []byte(`{"folders":[{"path":"/legal","locked":true}]}`),
			},
		},
		roomDocsByID: map[string]db.DealRoomDocument{
			docID.String(): {FolderPath: "/legal"},
		},
	}
	link := db.Link{DealRoomID: roomID, FolderScopeMode: FolderScopeModeFull}

	got := evaluateDealRoomDocumentAccess(context.Background(), q, link, docID)
	if got != linkDocAccessLocked {
		t.Fatalf("expected locked denial, got %v", got)
	}
}

func TestEvaluateDealRoomDocumentAccess_OutOfScope(t *testing.T) {
	roomID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	docID := uuid.New()
	q := &fakeLinkDocumentAccessQuerier{
		fakeAuthorizedDocQuerier: fakeAuthorizedDocQuerier{
			roomOK: true,
			room:   db.DealRoom{ID: roomID, Settings: []byte(`{}`)},
		},
		roomDocsByID: map[string]db.DealRoomDocument{
			docID.String(): {FolderPath: "/legal"},
		},
	}
	link := db.Link{
		DealRoomID:       roomID,
		FolderScopeMode:  FolderScopeModeAllowlist,
		FolderScopePaths: []string{"/general"},
	}

	got := evaluateDealRoomDocumentAccess(context.Background(), q, link, docID)
	if got != linkDocAccessOutOfScope {
		t.Fatalf("expected out-of-scope denial, got %v", got)
	}
}

func TestEvaluateLinkDocumentAccess_DeniesArchivedDocument(t *testing.T) {
	docID := uuid.New()
	wsID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	q := &fakeLinkDocumentAccessQuerier{
		fakeAuthorizedDocQuerier: fakeAuthorizedDocQuerier{
			legacy: db.GetDocumentByIDRow{
				ID:     pgtype.UUID{Bytes: docID, Valid: true},
				Status: "archived",
			},
			legacyOK: true,
		},
	}
	link := db.Link{
		DocumentID:  pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: wsID,
	}

	got := evaluateLinkDocumentAccess(context.Background(), q, link, docID)
	if got != linkDocAccessDenied {
		t.Fatalf("expected archived denial, got %v", got)
	}
}

func TestEvaluateLinkDocumentAccess_DeniesArchivedDocumentCaseInsensitive(t *testing.T) {
	docID := uuid.New()
	wsID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	q := &fakeLinkDocumentAccessQuerier{
		fakeAuthorizedDocQuerier: fakeAuthorizedDocQuerier{
			legacy: db.GetDocumentByIDRow{
				ID:     pgtype.UUID{Bytes: docID, Valid: true},
				Status: "Archived",
			},
			legacyOK: true,
		},
	}
	link := db.Link{
		DocumentID:  pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: wsID,
	}

	got := evaluateLinkDocumentAccess(context.Background(), q, link, docID)
	if got != linkDocAccessDenied {
		t.Fatalf("expected archived denial for mixed-case status, got %v", got)
	}
}

func TestEvaluateLinkDocumentAccess_AllowsReadyDocument(t *testing.T) {
	docID := uuid.New()
	wsID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	q := &fakeLinkDocumentAccessQuerier{
		fakeAuthorizedDocQuerier: fakeAuthorizedDocQuerier{
			legacy: db.GetDocumentByIDRow{
				ID:     pgtype.UUID{Bytes: docID, Valid: true},
				Status: "ready",
			},
			legacyOK: true,
		},
	}
	link := db.Link{
		DocumentID:  pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: wsID,
	}

	got := evaluateLinkDocumentAccess(context.Background(), q, link, docID)
	if got != linkDocAccessAllowed {
		t.Fatalf("expected allowed, got %v", got)
	}
}

func TestEvaluateLinkDocumentAccess_DeniesArchivedLinkDocumentMembership(t *testing.T) {
	docID := uuid.New()
	wsID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	linkID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	q := &fakeLinkDocumentAccessQuerier{
		fakeAuthorizedDocQuerier: fakeAuthorizedDocQuerier{
			legacy: db.GetDocumentByIDRow{
				ID:     pgtype.UUID{Bytes: docID, Valid: true},
				Status: "archived",
			},
			legacyOK: true,
		},
		hasLinkDoc: true,
	}
	link := db.Link{
		ID:          linkID,
		WorkspaceID: wsID,
		// No primary DocumentID — membership via link_documents only.
	}

	got := evaluateLinkDocumentAccess(context.Background(), q, link, docID)
	if got != linkDocAccessDenied {
		t.Fatalf("expected archived denial for link_documents membership, got %v", got)
	}
}

func TestEvaluateLinkDocumentAccess_DeniesArchivedDealRoomDocument(t *testing.T) {
	docID := uuid.New()
	wsID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	roomID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	q := &fakeLinkDocumentAccessQuerier{
		fakeAuthorizedDocQuerier: fakeAuthorizedDocQuerier{
			roomOK: true,
			room:   db.DealRoom{ID: roomID, WorkspaceID: wsID, Settings: []byte(`{}`)},
			legacy: db.GetDocumentByIDRow{
				ID:     pgtype.UUID{Bytes: docID, Valid: true},
				Status: "archived",
			},
			legacyOK: true,
		},
		roomDocsByID: map[string]db.DealRoomDocument{
			docID.String(): {FolderPath: "/general"},
		},
	}
	link := db.Link{
		DealRoomID:      roomID,
		WorkspaceID:     wsID,
		FolderScopeMode: FolderScopeModeFull,
	}

	got := evaluateLinkDocumentAccess(context.Background(), q, link, docID)
	if got != linkDocAccessDenied {
		t.Fatalf("expected archived denial for deal-room membership, got %v", got)
	}
}
