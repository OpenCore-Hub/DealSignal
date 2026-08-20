package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("DOTENV_TEST_URL=from-file\nDOTENV_TEST_SECRET=file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTENV_TEST_URL", "already-set")
	t.Setenv("DOTENV_TEST_SECRET", "")
	_ = os.Unsetenv("DOTENV_TEST_SECRET")

	loadDotEnv(path)

	if got := os.Getenv("DOTENV_TEST_URL"); got != "already-set" {
		t.Fatalf("existing env must win, got %q", got)
	}
	if got := os.Getenv("DOTENV_TEST_SECRET"); got != "file-secret" {
		t.Fatalf("unset env should load file, got %q", got)
	}
}

func TestLoadDotEnvMissingFile(t *testing.T) {
	loadDotEnv(filepath.Join(t.TempDir(), "nope.env"))
}
