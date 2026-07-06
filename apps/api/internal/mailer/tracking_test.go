package mailer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTrackerSignVerify(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "secret", time.Hour)
	if !tr.Enabled() {
		t.Fatal("expected tracker to be enabled")
	}

	token := TrackingToken{LogID: "550e8400-e29b-41d4-a716-446655440000", Type: "open", Exp: time.Now().Add(time.Hour).Unix()}
	signed := tr.sign(token)

	got, err := tr.verify(signed)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if got.LogID != token.LogID || got.Type != token.Type {
		t.Fatalf("token mismatch: got %+v", got)
	}
}

func TestTrackerVerifyExpired(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "secret", time.Hour)
	token := TrackingToken{LogID: "550e8400-e29b-41d4-a716-446655440000", Type: "open", Exp: time.Now().Add(-time.Hour).Unix()}
	_, err := tr.verify(tr.sign(token))
	if err == nil {
		t.Fatal("expected expired token to fail verification")
	}
}

func TestTrackerVerifyTampered(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "secret", time.Hour)
	token := TrackingToken{LogID: "550e8400-e29b-41d4-a716-446655440000", Type: "open", Exp: time.Now().Add(time.Hour).Unix()}
	signed := tr.sign(token) + "x"
	_, err := tr.verify(signed)
	if err == nil {
		t.Fatal("expected tampered token to fail verification")
	}
}

func TestTrackerDisabledWithoutSecret(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "", time.Hour)
	if tr.Enabled() {
		t.Fatal("expected tracker without secret to be disabled")
	}
	_, err := tr.verify("anything")
	if err == nil {
		t.Fatal("expected verify to fail when tracking disabled")
	}
}

func TestInjectOpenPixel(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "secret", time.Hour)
	logID := "550e8400-e29b-41d4-a716-446655440000"
	html := "<html><body>Hello</body></html>"
	out := injectOpenPixel(html, logID, tr)
	if !strings.Contains(out, "/api/v1/public/emails/track/open.png?token=") {
		t.Fatalf("expected open pixel URL, got %s", out)
	}
	if !strings.Contains(out, "</body>") {
		t.Fatal("expected body tag to remain")
	}
}

func TestInjectOpenPixelDisabled(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "", time.Hour)
	logID := "550e8400-e29b-41d4-a716-446655440000"
	html := "<html><body>Hello</body></html>"
	out := injectOpenPixel(html, logID, tr)
	if out != html {
		t.Fatalf("expected unchanged html when disabled, got %s", out)
	}
}

func TestRewriteClickLinks(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "secret", time.Hour)
	logID := "550e8400-e29b-41d4-a716-446655440000"
	html := `<p><a href="https://example.com/page">Click</a></p><p><a href="mailto:test@example.com">Email</a></p>`
	out := rewriteClickLinks(html, logID, tr)
	if !strings.Contains(out, "/api/v1/public/emails/track/click?token=") {
		t.Fatalf("expected tracking click URL, got %s", out)
	}
	if !strings.Contains(out, "mailto:test@example.com") {
		t.Fatal("expected mailto link to remain unchanged")
	}
}

func TestRewriteClickLinksDisabled(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "", time.Hour)
	logID := "550e8400-e29b-41d4-a716-446655440000"
	html := `<p><a href="https://example.com/page">Click</a></p>`
	out := rewriteClickLinks(html, logID, tr)
	if out != html {
		t.Fatalf("expected unchanged html when disabled, got %s", out)
	}
}

func TestShouldTrackURL(t *testing.T) {
	cases := []struct {
		url   string
		track bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"mailto:a@b.com", false},
		{"#anchor", false},
		{"/relative", false},
	}
	for _, c := range cases {
		if got := shouldTrackURL(c.url); got != c.track {
			t.Errorf("shouldTrackURL(%q) = %v, want %v", c.url, got, c.track)
		}
	}
}

type mockTrackingRedis struct {
	keys map[string]struct{}
}

func (m *mockTrackingRedis) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if _, ok := m.keys[key]; ok {
		return false, nil
	}
	m.keys[key] = struct{}{}
	return true, nil
}

func TestConsumeTokenReplayProtection(t *testing.T) {
	tr := NewTracker(nil, "http://localhost:8080", "secret", time.Hour, WithRedis(&mockTrackingRedis{keys: make(map[string]struct{})}))
	token := TrackingToken{LogID: "550e8400-e29b-41d4-a716-446655440000", Type: "open", Exp: time.Now().Add(time.Hour).Unix()}
	tokenStr := tr.sign(token)

	if !tr.consumeToken(context.Background(), tokenStr, token) {
		t.Fatal("expected first consume to succeed")
	}
	if tr.consumeToken(context.Background(), tokenStr, token) {
		t.Fatal("expected second consume to be rejected as replay")
	}
}
