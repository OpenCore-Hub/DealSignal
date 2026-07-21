package nda

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeSignerName(t *testing.T) {
	if _, err := NormalizeSignerName("  ", true); err == nil {
		t.Fatal("expected error for empty required name")
	}
	got, err := NormalizeSignerName("  Ada Lovelace  ", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Ada Lovelace" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildAuditTrailPDF(t *testing.T) {
	pdf, err := buildAuditTrailPDF(SealParams{
		TemplateName:  "One-Way NDA",
		CertificateID: "cert-123",
		SignerName:    "Ada Lovelace",
		SignerEmail:   "ada@example.com",
		ContentSHA256: "abc",
		LinkID:        "link-1",
		IPHash:        "deadbeef",
		UserAgent:     "vitest",
		SignedAt:      time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("buildAuditTrailPDF: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-1.4") {
		t.Fatalf("expected PDF header, got %q", string(pdf[:min(20, len(pdf))]))
	}
	if !strings.Contains(string(pdf), "Certificate ID: cert-123") {
		t.Fatal("expected certificate id in PDF content")
	}
}
