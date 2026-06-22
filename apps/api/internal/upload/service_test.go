package upload

import (
	"mime/multipart"
	"testing"
)

func TestValidateFileHeader(t *testing.T) {
	cases := []struct {
		name     string
		fileName string
		size     int64
		wantErr  error
		wantType string
	}{
		{"valid pdf", "deck.pdf", 1024, nil, "pdf"},
		{"valid docx", "report.docx", 1024, nil, "docx"},
		{"invalid exe", "virus.exe", 1024, ErrInvalidFileType, ""},
		{"too large", "big.pdf", maxFileSize + 1, ErrFileTooLarge, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &multipart.FileHeader{Filename: tc.fileName, Size: tc.size}
			typ, err := ValidateFileHeader(h)
			if err != tc.wantErr {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
			if typ != tc.wantType {
				t.Fatalf("expected source type %q, got %q", tc.wantType, typ)
			}
		})
	}
}
