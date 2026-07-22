package link

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeAuthorizedDocQuerier struct {
	roomDocs []db.ListDealRoomDocumentsWithMetaRow
	linkDocs []db.ListLinkDocumentsByPublicTokenRow
	legacy   db.GetDocumentByIDRow
	legacyOK bool
}

func (f *fakeAuthorizedDocQuerier) ListDealRoomDocumentsWithMeta(_ context.Context, _ pgtype.UUID) ([]db.ListDealRoomDocumentsWithMetaRow, error) {
	return f.roomDocs, nil
}

func (f *fakeAuthorizedDocQuerier) ListLinkDocumentsByPublicToken(_ context.Context, _ string) ([]db.ListLinkDocumentsByPublicTokenRow, error) {
	return f.linkDocs, nil
}

func (f *fakeAuthorizedDocQuerier) GetDocumentByID(_ context.Context, _ db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error) {
	if !f.legacyOK {
		return db.GetDocumentByIDRow{}, context.Canceled
	}
	return f.legacy, nil
}

func TestAuthorizedDocumentIDs_DealRoomAllowlistExcludesOutOfScope(t *testing.T) {
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	roomID := pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true}

	q := &fakeAuthorizedDocQuerier{
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: inScope, Valid: true}, FolderPath: "/general"},
			{DocumentID: pgtype.UUID{Bytes: outOfScope, Valid: true}, FolderPath: "/legal"},
		},
	}
	link := db.Link{
		DealRoomID:       roomID,
		FolderScopeMode:  FolderScopeModeAllowlist,
		FolderScopePaths: []string{"/general"},
		PublicToken:      "tok",
	}

	got, err := AuthorizedDocumentIDs(context.Background(), q, link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != inScope {
		t.Fatalf("expected only in-scope doc %s, got %v", inScope, got)
	}
}

func TestAuthorizedDocumentIDs_EmptyAllowlistReturnsEmpty(t *testing.T) {
	docID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	roomID := pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true}

	q := &fakeAuthorizedDocQuerier{
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: docID, Valid: true}, FolderPath: "/general"},
		},
	}
	link := db.Link{
		DealRoomID:       roomID,
		FolderScopeMode:  FolderScopeModeAllowlist,
		FolderScopePaths: nil,
		PublicToken:      "tok",
	}

	got, err := AuthorizedDocumentIDs(context.Background(), q, link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty authorized set, got %v", got)
	}
}

func TestAuthorizedDocumentIDs_SingleDocumentLink(t *testing.T) {
	docID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &fakeAuthorizedDocQuerier{
		legacy:   db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: docID, Valid: true}},
		legacyOK: true,
	}
	link := db.Link{
		DocumentID:  pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), Valid: true},
		PublicToken: "tok",
	}

	got, err := AuthorizedDocumentIDs(context.Background(), q, link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != docID {
		t.Fatalf("expected single doc %s, got %v", docID, got)
	}
}
