package link

import (
	"context"
	"errors"
	"testing"
)

func TestResolvedViewerHostPrefersLinkCustomDomain(t *testing.T) {
	s := &Service{}
	if got := s.ResolvedViewerHost(context.Background(), "", "view.example.com"); got != "view.example.com" {
		t.Fatalf("got %q", got)
	}
	if got := s.ResolvedViewerHost(context.Background(), "ws", "  "); got != "" {
		t.Fatalf("empty queries should return empty, got %q", got)
	}
}

func TestNormalizeLinkCustomDomain(t *testing.T) {
	ctx := context.Background()
	s := &Service{} // nil queries → no verified Brand host

	got, err := s.normalizeLinkCustomDomain(ctx, "ws", "", "")
	if err != nil || got != "" {
		t.Fatalf("empty should pass, got %q err=%v", got, err)
	}

	got, err = s.normalizeLinkCustomDomain(ctx, "ws", "Legacy.Example.com", "legacy.example.com")
	if err != nil || got != "legacy.example.com" {
		t.Fatalf("legacy unchanged should pass, got %q err=%v", got, err)
	}

	_, err = s.normalizeLinkCustomDomain(ctx, "ws", "not-verified.example.com", "")
	if !errors.Is(err, ErrInvalidCustomDomain) {
		t.Fatalf("expected ErrInvalidCustomDomain, got %v", err)
	}

	_, err = s.normalizeLinkCustomDomain(ctx, "ws", "other.example.com", "legacy.example.com")
	if !errors.Is(err, ErrInvalidCustomDomain) {
		t.Fatalf("expected ErrInvalidCustomDomain when changing away from legacy, got %v", err)
	}
}

func TestPublicLinkURLUnchanged(t *testing.T) {
	if got := publicLinkURL("https://app.example.com", "tok", ""); got != "https://app.example.com/l/tok" {
		t.Fatalf("default: %s", got)
	}
	if got := publicLinkURL("https://app.example.com", "tok", "invest.example.com"); got != "https://invest.example.com/l/tok" {
		t.Fatalf("custom: %s", got)
	}
}
