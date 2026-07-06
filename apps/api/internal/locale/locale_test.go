package locale

import (
	"context"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "en"},
		{"en", "en"},
		{"EN-US", "en"},
		{"zh", "zh-CN"},
		{"zh-CN", "zh-CN"},
		{"zh-Hans", "zh-CN"},
		{"zh-TW", "zh-TW"},
		{"fr", "en"},
	}
	for _, tc := range cases {
		got := Normalize(tc.in)
		if got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestContext(t *testing.T) {
	ctx := context.Background()
	if got := FromContext(ctx); got != "" {
		t.Errorf("expected empty locale from background context, got %q", got)
	}
	ctx = WithLocale(ctx, "zh-CN")
	if got := FromContext(ctx); got != "zh-CN" {
		t.Errorf("expected locale zh-CN, got %q", got)
	}
}
