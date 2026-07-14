package storage

import (
	"strconv"
	"testing"
)

func TestObjectKey(t *testing.T) {
	cases := []struct {
		tenantID, workspaceID, documentID, filename, want string
	}{
		{"t1", "w1", "d1", "file.pdf", "tenants/t1/workspaces/w1/documents/d1/file.pdf"},
		{"t1", "w1", "d1", "path/to/file.pdf", "tenants/t1/workspaces/w1/documents/d1/path/to/file.pdf"},
	}
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			if got := ObjectKey(tc.tenantID, tc.workspaceID, tc.documentID, tc.filename); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"false", false},
		{"", false},
		{"no", false},
		{"FALSE", false},
		{"0", false},
	}
	for _, tc := range cases {
		t.Run(strconv.FormatBool(tc.want)+"_"+tc.in, func(t *testing.T) {
			if got := ParseBool(tc.in); got != tc.want {
				t.Fatalf("ParseBool(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
