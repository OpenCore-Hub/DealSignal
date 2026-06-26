package link

import (
	"testing"
	"time"
)

func TestLinkSessionRoundTrip(t *testing.T) {
	secret := "test-secret"
	s := LinkSession{
		PublicToken: "pub-token",
		Email:       "alice@example.com",
		Password:    "secret",
		NDAAgreed:   true,
		VisitorID:   "visitor-1",
	}
	token, err := signLinkSession(s, secret)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	got, ok := verifyLinkSession(token, secret)
	if !ok {
		t.Fatal("verify failed")
	}
	if got.PublicToken != s.PublicToken || got.Email != s.Email || got.VisitorID != s.VisitorID || !got.NDAAgreed {
		t.Fatalf("session mismatch: %+v", got)
	}
	if time.Now().Unix() > got.ExpiresAt {
		t.Fatal("session already expired")
	}
}

func TestLinkSessionTampered(t *testing.T) {
	secret := "test-secret"
	token, _ := signLinkSession(LinkSession{PublicToken: "tok"}, secret)
	if _, ok := verifyLinkSession(token+"x", secret); ok {
		t.Error("tampered token should fail verification")
	}
}

func TestLinkSessionWrongSecret(t *testing.T) {
	token, _ := signLinkSession(LinkSession{PublicToken: "tok"}, "secret-a")
	if _, ok := verifyLinkSession(token, "secret-b"); ok {
		t.Error("wrong secret should fail verification")
	}
}
