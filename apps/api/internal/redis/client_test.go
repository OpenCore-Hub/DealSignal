package redis

import (
	"testing"
)

func TestHashToken(t *testing.T) {
	got := hashToken("token-1")
	want := hashToken("token-1")
	if got != want {
		t.Fatalf("hashToken not deterministic")
	}
	if got == hashToken("token-2") {
		t.Fatal("hashToken should produce different digests for different tokens")
	}
	if got == "" {
		t.Fatal("hashToken should not return empty")
	}
}

func TestNewClientInvalidURL(t *testing.T) {
	_, err := NewClient("redis://invalid:wrong")
	if err == nil {
		t.Fatal("expected error for invalid redis URL")
	}
}

func TestClientCloseNil(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Fatalf("Close on nil client should not error, got %v", err)
	}
}
