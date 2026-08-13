package plan

import (
	"errors"
	"net/http"
	"testing"
)

func TestHTTPError(t *testing.T) {
	t.Parallel()
	status, code, ok := HTTPError(ErrLimitRooms)
	if !ok || status != http.StatusForbidden || code != CodeLimitRooms {
		t.Fatalf("rooms: status=%d code=%q ok=%v", status, code, ok)
	}
	status, code, ok = HTTPError(ErrLimitLinks)
	if !ok || status != http.StatusForbidden || code != CodeLimitLinks {
		t.Fatalf("links: status=%d code=%q ok=%v", status, code, ok)
	}
	status, code, ok = HTTPError(ErrLimitStorage)
	if !ok || status != http.StatusForbidden || code != CodeLimitStorage {
		t.Fatalf("storage: status=%d code=%q ok=%v", status, code, ok)
	}
	status, code, ok = HTTPError(ErrLimitSeats)
	if !ok || status != http.StatusForbidden || code != CodeLimitSeats {
		t.Fatalf("seats: status=%d code=%q ok=%v", status, code, ok)
	}
	status, code, ok = HTTPError(ErrLimitWorkspaces)
	if !ok || status != http.StatusForbidden || code != CodeLimitWorkspaces {
		t.Fatalf("workspaces: status=%d code=%q ok=%v", status, code, ok)
	}
	status, code, ok = HTTPError(ErrFeatureCustomDomain)
	if !ok || status != http.StatusForbidden || code != CodeFeatureCustomDomain {
		t.Fatalf("custom domain: status=%d code=%q ok=%v", status, code, ok)
	}
	status, code, ok = HTTPError(ErrFeatureWebhooks)
	if !ok || status != http.StatusForbidden || code != CodeFeatureWebhooks {
		t.Fatalf("webhooks: status=%d code=%q ok=%v", status, code, ok)
	}
	for _, tc := range []struct {
		err  error
		code string
	}{
		{ErrFeatureHubSpot, CodeFeatureHubSpot},
		{ErrFeatureDailyDigest, CodeFeatureDailyDigest},
		{ErrFeatureSlackAlerts, CodeFeatureSlackAlerts},
		{ErrFeatureRoomInsights, CodeFeatureRoomInsights},
		{ErrFeatureRoomAnalytics, CodeFeatureRoomAnalytics},
		{ErrFeatureFormalAsk, CodeFeatureFormalAsk},
	} {
		status, code, ok = HTTPError(tc.err)
		if !ok || status != http.StatusForbidden || code != tc.code {
			t.Fatalf("%s: status=%d code=%q ok=%v", tc.code, status, code, ok)
		}
	}
	if _, _, ok = HTTPError(errors.New("other")); ok {
		t.Fatal("unrelated errors must not map")
	}
	if _, _, ok = HTTPError(nil); ok {
		t.Fatal("nil must not map")
	}
}
