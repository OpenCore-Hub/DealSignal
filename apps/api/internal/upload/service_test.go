package upload

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCreateDocument_AgreementRequiresPDF(t *testing.T) {
	s := NewService(nil, nil, nil)
	h := &multipart.FileHeader{Filename: "nda.docx", Size: 1024}
	_, err := s.CreateDocument(t.Context(), uuid.NewString(), uuid.NewString(), uuid.NewString(), "agreement", h, false)
	if !errors.Is(err, ErrAgreementRequiresPDF) {
		t.Fatalf("expected ErrAgreementRequiresPDF, got %v", err)
	}
}

func TestCreateDocument_RejectsDealRoomCategory(t *testing.T) {
	s := NewService(nil, nil, nil)
	h := &multipart.FileHeader{Filename: "room.pdf", Size: 1024}
	_, err := s.CreateDocument(t.Context(), uuid.NewString(), uuid.NewString(), uuid.NewString(), "deal_room", h, false)
	if !errors.Is(err, ErrCategoryDealRoomViaAPI) {
		t.Fatalf("expected ErrCategoryDealRoomViaAPI, got %v", err)
	}
}

func TestErrIfAgreementNotPDF(t *testing.T) {
	cases := []struct {
		category, sourceType string
		wantErr              bool
	}{
		{"agreement", "pdf", false},
		{"agreement", "PDF", false},
		{"agreement", "docx", true},
		{"general", "docx", false},
		{"", "docx", false},
	}
	for _, tc := range cases {
		err := errIfAgreementNotPDF(tc.category, tc.sourceType)
		if tc.wantErr && !errors.Is(err, ErrAgreementRequiresPDF) {
			t.Fatalf("category=%q source=%q: want ErrAgreementRequiresPDF, got %v", tc.category, tc.sourceType, err)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("category=%q source=%q: unexpected err %v", tc.category, tc.sourceType, err)
		}
	}
}

func TestValidateUploadFilename(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"report.pdf", false},
		{"~$report.xlsx", true},
		{"._report.xlsx", true},
		{".DS_Store", true},
		{"", true},
	}
	for _, tc := range cases {
		err := ValidateUploadFilename(tc.name)
		if tc.wantErr && err == nil {
			t.Fatalf("expected error for %q", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("unexpected error for %q: %v", tc.name, err)
		}
	}
}

func TestValidateFileHeader_RejectsLockFile(t *testing.T) {
	h := &multipart.FileHeader{Filename: "~$report.xlsx", Size: 1024}
	_, err := ValidateFileHeader(h)
	if !errors.Is(err, ErrUnsupportedUpload) {
		t.Fatalf("expected ErrUnsupportedUpload, got %v", err)
	}
}

func TestValidateFileHeader_RejectsEmptyFile(t *testing.T) {
	h := &multipart.FileHeader{Filename: "report.xlsx", Size: 0}
	_, err := ValidateFileHeader(h)
	if !errors.Is(err, ErrEmptyFile) {
		t.Fatalf("expected ErrEmptyFile, got %v", err)
	}
}

func TestValidateFileContent_EmptyStream(t *testing.T) {
	err := validateFileContent(emptyFileReader{}, "xlsx")
	if !errors.Is(err, ErrEmptyFile) {
		t.Fatalf("expected ErrEmptyFile, got %v", err)
	}
}

func TestValidateFileContent_ValidXlsxMagic(t *testing.T) {
	// PK zip header for Office Open XML.
	data := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	err := validateFileContent(nopSeekFile{Reader: bytes.NewReader(data)}, "xlsx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type emptyFileReader struct{}

func (emptyFileReader) Read([]byte) (int, error)          { return 0, io.EOF }
func (emptyFileReader) ReadAt([]byte, int64) (int, error) { return 0, io.EOF }
func (emptyFileReader) Seek(int64, int) (int64, error)    { return 0, nil }
func (emptyFileReader) Close() error                      { return nil }

type nopSeekFile struct{ *bytes.Reader }

func (n nopSeekFile) Close() error                            { return nil }
func (n nopSeekFile) ReadAt(p []byte, off int64) (int, error) { return n.Reader.ReadAt(p, off) }

func TestNormalizeUploadFilename(t *testing.T) {
	if got := NormalizeUploadFilename(`financials/report.xlsx`); got != "report.xlsx" {
		t.Fatalf("got %q", got)
	}
}

func TestReplacedSnapshotTitle(t *testing.T) {
	at := time.Date(2026, 8, 13, 8, 32, 0, 0, time.UTC)
	got := replacedSnapshotTitle("Pitch Deck.pdf", at, "")
	want := "Pitch Deck (20260813-083200).pdf"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	withNonce := replacedSnapshotTitle("Pitch Deck.pdf", at, "ab12cd34")
	if withNonce != "Pitch Deck (20260813-083200-ab12cd34).pdf" {
		t.Fatalf("nonce title %q", withNonce)
	}
}

func TestLookupLiveByTitle_EmptyFilename(t *testing.T) {
	s := NewService(nil, nil, nil)
	_, _, _, err := s.LookupLiveByTitle(t.Context(), uuid.NewString(), "   ")
	if !errors.Is(err, ErrUnsupportedUpload) {
		t.Fatalf("expected ErrUnsupportedUpload, got %v", err)
	}
}

func TestValidateFileHeader(t *testing.T) {
	cases := []struct {
		name      string
		filename  string
		size      int64
		wantType  string
		wantError bool
	}{
		{"pdf", "report.pdf", 1024, "pdf", false},
		{"docx", "report.docx", 2048, "docx", false},
		{"unsupported", "report.txt", 100, "", true},
		{"too large", "report.pdf", MaxFileSize + 1, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &multipart.FileHeader{Filename: tc.filename, Size: tc.size}
			got, err := ValidateFileHeader(h)
			if tc.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantType {
				t.Fatalf("expected %s, got %s", tc.wantType, got)
			}
		})
	}
}

func TestExistingDocumentError(t *testing.T) {
	err := &ExistingDocumentError{ID: "abc", Title: "nda.docx"}
	if err.Error() == "" {
		t.Fatal("expected message")
	}
	if err.ID != "abc" || err.Title != "nda.docx" {
		t.Fatalf("unexpected fields: %+v", err)
	}
}

func TestDeleteDocument_InvalidIDs(t *testing.T) {
	s := NewService(nil, nil, nil)
	if err := s.DeleteDocument(t.Context(), "not-a-uuid", "also-bad"); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(errors.New("nope")) {
		t.Fatal("expected false")
	}
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected unique violation")
	}
}

func TestDocumentFromDB(t *testing.T) {
	now := time.Now()
	docID := uuid.New()
	d := db.CreateDocumentRow{
		ID:         pgtype.UUID{Bytes: docID, Valid: true},
		Title:      "report.pdf",
		SourceType: "pdf",
		Status:     "uploaded",
		CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
	}
	got := documentFromDB(d)
	if got.ID != docID.String() {
		t.Fatalf("expected id %s, got %s", docID.String(), got.ID)
	}
	if got.Title != "report.pdf" {
		t.Fatalf("expected title report.pdf, got %s", got.Title)
	}
}
