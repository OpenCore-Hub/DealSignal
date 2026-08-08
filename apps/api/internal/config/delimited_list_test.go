package config

import (
	"reflect"
	"testing"
)

func TestParseDelimitedList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"enterprise trial", []string{"enterprise", "trial"}},
		{"enterprise,trial", []string{"enterprise", "trial"}},
		{" Enterprise, Trial  pro ", []string{"enterprise", "trial", "pro"}},
		{"enterprise,,  trial", []string{"enterprise", "trial"}},
	}
	for _, tc := range cases {
		got := parseDelimitedList(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("parseDelimitedList(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}
