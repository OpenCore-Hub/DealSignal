package ingestion

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestSignConverterRequest(t *testing.T) {
	secret := []byte("test-secret")
	body := []byte(`{"filetype":"docx","outputtype":"pdf"}`)

	token := signConverterRequest(body, secret)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if string(payloadJSON) != string(body) {
		t.Fatalf("expected payload %s, got %s", string(body), string(payloadJSON))
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header["alg"] != "HS256" {
		t.Fatalf("expected alg HS256, got %s", header["alg"])
	}
}
