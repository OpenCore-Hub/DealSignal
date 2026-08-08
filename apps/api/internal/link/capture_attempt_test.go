package link

import "testing"

func TestNormalizeCaptureAttemptReason(t *testing.T) {
	cases := map[string]string{
		"printscreen":  "printscreen",
		"Print":        "print",
		"MAC_CAPTURE":  "mac_capture",
		"win_snip":     "win_snip",
		"":             "other",
		"unknown-key":  "other",
	}
	for in, want := range cases {
		if got := normalizeCaptureAttemptReason(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}
