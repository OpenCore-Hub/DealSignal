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
