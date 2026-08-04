package upload

import (
	"testing"
)

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
