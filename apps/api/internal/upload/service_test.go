package upload

import (
	"mime/multipart"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

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
		{"too large", "report.pdf", maxFileSize + 1, "", true},
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

func TestDocumentFromDB(t *testing.T) {
	now := time.Now()
	docID := uuid.New()
	d := db.Document{
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
