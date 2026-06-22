package integration

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

func TestRandomStateUnique(t *testing.T) {
	a, err := randomState()
	if err != nil {
		t.Fatalf("random state: %v", err)
	}
	b, err := randomState()
	if err != nil {
		t.Fatalf("random state: %v", err)
	}
	if a == b {
		t.Fatal("expected unique states")
	}
	if len(a) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(a))
	}
}

func TestSlackAuthURLRequiresClientID(t *testing.T) {
	s := &Service{cfg: &config.Config{AppBaseURL: "http://localhost:8080", SlackClientID: "client-id"}}
	url := s.slackAuthURL("state-123")
	if url == "" {
		t.Fatal("expected non-empty url")
	}
	if url == "state-123" {
		t.Fatal("url should contain state")
	}
}
