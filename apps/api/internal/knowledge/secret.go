package knowledge

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const sealedPrefix = "v1:"

// deriveSecretKey turns a configured passphrase into a 32-byte AES key.
func deriveSecretKey(passphrase string) []byte {
	sum := sha256.Sum256([]byte(passphrase))
	return sum[:]
}

// sealSecret encrypts plaintext with AES-GCM. Empty passphrase stores plaintext
// only in development misconfig; callers should always pass a production secret.
func sealSecret(passphrase, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.TrimSpace(passphrase) == "" {
		return plaintext, nil
	}
	block, err := aes.NewCipher(deriveSecretKey(passphrase))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return sealedPrefix + base64.RawStdEncoding.EncodeToString(out), nil
}

// openSecret decrypts sealSecret output. Plaintext values without the v1: prefix
// are returned as-is for backward compatibility with earlier rows.
func openSecret(passphrase, sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	if !strings.HasPrefix(sealed, sealedPrefix) {
		return sealed, nil
	}
	if strings.TrimSpace(passphrase) == "" {
		return "", errors.New("secret key required to decrypt tenant api key")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(sealed, sealedPrefix))
	if err != nil {
		return "", fmt.Errorf("decode sealed secret: %w", err)
	}
	block, err := aes.NewCipher(deriveSecretKey(passphrase))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("sealed secret too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
