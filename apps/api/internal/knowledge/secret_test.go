package knowledge

import "testing"

func TestSealOpenSecretRoundTrip(t *testing.T) {
	sealed, err := sealSecret("test-passphrase", "rag-tenant-key")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "rag-tenant-key" || sealed == "" {
		t.Fatalf("expected sealed ciphertext, got %q", sealed)
	}
	plain, err := openSecret("test-passphrase", sealed)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "rag-tenant-key" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestOpenSecretPlaintextCompat(t *testing.T) {
	plain, err := openSecret("unused", "legacy-plaintext-key")
	if err != nil {
		t.Fatal(err)
	}
	if plain != "legacy-plaintext-key" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestOpenSecretWrongPassphrase(t *testing.T) {
	sealed, err := sealSecret("right", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openSecret("wrong", sealed); err == nil {
		t.Fatal("expected decrypt failure")
	}
}
