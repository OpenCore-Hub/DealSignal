package ingestion

import "testing"

func TestEmbedOnIngest_DealRoomSkipsEmbedding(t *testing.T) {
	if embedOnIngest(true, true) {
		t.Fatal("deal-room documents must not auto-embed during ingest")
	}
}

func TestEmbedOnIngest_NonRoomEmbedsWhenConfigured(t *testing.T) {
	if !embedOnIngest(true, false) {
		t.Fatal("non-room documents should embed when an embedder is configured")
	}
}

func TestEmbedOnIngest_NoEmbedder(t *testing.T) {
	if embedOnIngest(false, false) {
		t.Fatal("must not embed when embedder is not configured")
	}
	if embedOnIngest(false, true) {
		t.Fatal("must not embed when embedder is not configured, even for deal-room docs")
	}
}
