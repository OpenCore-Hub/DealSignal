package storage

import (
	"strconv"
	"testing"
)

func TestObjectKey(t *testing.T) {
	key := ObjectKey("t1", "w1", "d1", "file.pdf")
	want := "tenants/t1/workspaces/w1/documents/d1/file.pdf"
	if key != want {
		t.Fatalf("expected %q, got %q", want, key)
	}
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"false", false},
		{"", false},
		{"no", false},
	}
	for _, tc := range cases {
		t.Run(strconv.FormatBool(tc.want), func(t *testing.T) {
			if got := ParseBool(tc.in); got != tc.want {
				t.Fatalf("ParseBool(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
