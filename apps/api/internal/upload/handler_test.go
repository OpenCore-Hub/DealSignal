package upload

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestShouldLoadDealRoomMembershipExclude(t *testing.T) {
	cases := []struct {
		excludeDealRoom bool
		category        string
		want            bool
	}{
		{true, CategoryGeneral, false},
		{true, "", true},
		{true, CategoryAgreement, true},
		{false, "", false},
		{false, CategoryGeneral, false},
	}
	for _, tc := range cases {
		if got := shouldLoadDealRoomMembershipExclude(tc.excludeDealRoom, tc.category); got != tc.want {
			t.Fatalf("exclude=%v category=%q: got %v want %v", tc.excludeDealRoom, tc.category, got, tc.want)
		}
	}
}

func TestClassifyCreateDocumentError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantExists bool
	}{
		{"document exists", &ExistingDocumentError{ID: "1", Title: "a.pdf"}, http.StatusConflict, "document_exists", true},
		{"empty file", ErrEmptyFile, http.StatusBadRequest, "empty_file", false},
		{"lock file", fmt.Errorf("%w: excel lock files cannot be uploaded", ErrUnsupportedUpload), http.StatusBadRequest, "unsupported_upload", false},
		{"bad content", ErrInvalidFileContent, http.StatusUnsupportedMediaType, "unsupported_media_type", false},
		{"internal", errors.New("db down"), http.StatusInternalServerError, "internal_error", false},
	}
	for _, tc := range cases {
		status, code, exists := classifyCreateDocumentError(tc.err)
		if status != tc.wantStatus || code != tc.wantCode {
			t.Fatalf("%s: got status=%d code=%q want status=%d code=%q", tc.name, status, code, tc.wantStatus, tc.wantCode)
		}
		if (exists != nil) != tc.wantExists {
			t.Fatalf("%s: exists=%v wantExists=%v", tc.name, exists != nil, tc.wantExists)
		}
	}
}

func TestParseTruthyForm(t *testing.T) {
	cases := map[string]bool{
		"true":  true,
		"TRUE":  true,
		"1":     true,
		"yes":   true,
		"on":    true,
		"":      false,
		"false": false,
		"0":     false,
	}
	for in, want := range cases {
		if got := parseTruthyForm(in); got != want {
			t.Fatalf("parseTruthyForm(%q)=%v want %v", in, got, want)
		}
	}
}

func TestContentTypeForSourceType(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"pdf", "application/pdf"},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		{"xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"unknown", "application/octet-stream"},
	}

	for _, tc := range cases {
		got := contentTypeForSourceType(tc.input)
		if got != tc.expected {
			t.Fatalf("contentTypeForSourceType(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestDocumentMatchesCategoryListFilter(t *testing.T) {
	cases := []struct {
		status   string
		filter   string
		category string
		want     bool
	}{
		{"ready", "archived", CategoryGeneral, false},
		{"archived", "archived", CategoryGeneral, true},
		{"ready", "all", CategoryGeneral, true},
		{"archived", "all", CategoryGeneral, false},
		{"archived", "all", CategoryAgreement, true},
		{"ready", "", CategoryGeneral, true},
		{"archived", "shared", CategoryGeneral, false},
		{"ready", "shared", CategoryGeneral, false}, // dedicated branch only
		{"processing", "archived", CategoryGeneral, false},
	}
	for _, tc := range cases {
		if got := documentMatchesCategoryListFilter(tc.status, tc.filter, tc.category); got != tc.want {
			t.Fatalf("status=%q filter=%q category=%q: got %v want %v", tc.status, tc.filter, tc.category, got, tc.want)
		}
	}
}
