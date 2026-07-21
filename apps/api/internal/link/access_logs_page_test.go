package link

import "testing"

func TestClampAccessLogsLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int
		want int
	}{
		{0, accessLogsDefaultLimit},
		{-1, accessLogsDefaultLimit},
		{10, 10},
		{200, 200},
		{201, accessLogsMaxLimit},
	}
	for _, tc := range cases {
		if got := clampAccessLogsLimit(tc.in); got != tc.want {
			t.Fatalf("clampAccessLogsLimit(%d)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestClampAccessCodeContactsLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int
		want int
	}{
		{0, accessCodeContactsPageSize},
		{-1, accessCodeContactsPageSize},
		{10, 10},
		{100, 100},
		{101, accessCodeContactsMaxPageSize},
	}
	for _, tc := range cases {
		if got := clampAccessCodeContactsLimit(tc.in); got != tc.want {
			t.Fatalf("clampAccessCodeContactsLimit(%d)=%d want %d", tc.in, got, tc.want)
		}
	}
}
