package domain

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"
)

// CertificateProvider issues or renews TLS certificates for a domain.
// Production deployments should replace the built-in providers with
// ACME / Let's Encrypt (cert-manager, Caddy, Traefik, etc.).
type CertificateProvider interface {
	Issue(ctx context.Context, domain string) (expiresAt time.Time, err error)
}

// NoopProvider simulates a successful issuance. Use it for tests and local dev.
type NoopProvider struct{}

func (NoopProvider) Issue(context.Context, string) (time.Time, error) {
	return time.Now().AddDate(1, 0, 0), nil
}

// SelfSignedProvider generates an in-memory self-signed certificate.
// The private key is never persisted.
type SelfSignedProvider struct{}

func (SelfSignedProvider) Issue(_ context.Context, domain string) (time.Time, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return time.Time{}, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	_, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return time.Time{}, err
	}
	return tmpl.NotAfter, nil
}
