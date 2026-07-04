package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryTokenStore is an in-memory implementation of TokenStore for tests and local dev.
type MemoryTokenStore struct {
	mu                 sync.RWMutex
	blocklist          map[string]time.Time
	refreshTokens      map[string]time.Time
	verificationTokens map[string]verificationEntry
}

type verificationEntry struct {
	userID string
	exp    time.Time
}

// NewMemoryTokenStore creates a new in-memory token store. A background goroutine
// periodically evicts expired entries to prevent unbounded memory growth.
func NewMemoryTokenStore() *MemoryTokenStore {
	s := &MemoryTokenStore{
		blocklist:          make(map[string]time.Time),
		refreshTokens:      make(map[string]time.Time),
		verificationTokens: make(map[string]verificationEntry),
	}
	go s.cleanupExpired()
	return s
}

// cleanupExpired evicts expired entries from all maps every 5 minutes.
func (m *MemoryTokenStore) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, exp := range m.blocklist {
			if now.After(exp) {
				delete(m.blocklist, k)
			}
		}
		for k, exp := range m.refreshTokens {
			if now.After(exp) {
				delete(m.refreshTokens, k)
			}
		}
		for k, e := range m.verificationTokens {
			if now.After(e.exp) {
				delete(m.verificationTokens, k)
			}
		}
		m.mu.Unlock()
	}
}

// BlocklistToken stores a token with its remaining TTL.
func (m *MemoryTokenStore) BlocklistToken(ctx context.Context, token string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocklist[token] = time.Now().Add(ttl)
	return nil
}

// IsTokenBlocklisted checks whether a token has been revoked.
func (m *MemoryTokenStore) IsTokenBlocklisted(ctx context.Context, token string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	exp, ok := m.blocklist[token]
	if !ok {
		return false, nil
	}
	return time.Now().Before(exp), nil
}

// StoreRefreshToken stores a refresh token bound to a user.
func (m *MemoryTokenStore) StoreRefreshToken(ctx context.Context, userID, refreshToken string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshTokens[userID+":"+refreshToken] = time.Now().Add(ttl)
	return nil
}

// ValidateRefreshToken checks whether a refresh token is still valid.
func (m *MemoryTokenStore) ValidateRefreshToken(ctx context.Context, userID, refreshToken string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	exp, ok := m.refreshTokens[userID+":"+refreshToken]
	if !ok {
		return false, nil
	}
	return time.Now().Before(exp), nil
}

// RevokeRefreshToken removes a refresh token.
func (m *MemoryTokenStore) RevokeRefreshToken(ctx context.Context, userID, refreshToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.refreshTokens, userID+":"+refreshToken)
	return nil
}

// RevokeAllUserRefreshTokens removes all refresh tokens for a user.
func (m *MemoryTokenStore) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := userID + ":"
	for k := range m.refreshTokens {
		if strings.HasPrefix(k, prefix) {
			delete(m.refreshTokens, k)
		}
	}
	return nil
}

// CreateVerificationToken creates a single-use email-verification token.
func (m *MemoryTokenStore) CreateVerificationToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	token := uuid.NewString()
	m.verificationTokens[token] = verificationEntry{userID: userID, exp: time.Now().Add(ttl)}
	return token, nil
}

// UserIDByVerificationToken resolves a verification token to a user ID.
func (m *MemoryTokenStore) UserIDByVerificationToken(ctx context.Context, token string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.verificationTokens[token]
	if !ok || time.Now().After(entry.exp) {
		return "", errors.New("invalid or expired token")
	}
	return entry.userID, nil
}

// DeleteVerificationToken removes a verification token after use.
func (m *MemoryTokenStore) DeleteVerificationToken(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.verificationTokens, token)
	return nil
}
