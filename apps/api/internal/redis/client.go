package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// hashToken returns a SHA256 hex digest of a token string.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Client wraps go-redis with application-specific helpers.
type Client struct {
	rdb *redis.Client
}

// GoRedis returns the underlying go-redis client for advanced operations.
func (c *Client) GoRedis() *redis.Client {
	return c.rdb
}

// NewClient creates a Redis client from a URL string (e.g. redis:6379 or redis://user:pass@host:port/db).
func NewClient(rawURL string) (*Client, error) {
	var opt *redis.Options
	if strings.Contains(rawURL, "://") {
		parsed, err := redis.ParseURL(rawURL)
		if err != nil {
			return nil, err
		}
		opt = parsed
	} else {
		// Simple host:port form used in docker-compose, e.g. "redis:6379".
		opt = &redis.Options{
			Addr: rawURL,
		}
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// RDB exposes the underlying go-redis client for advanced usage.
func (c *Client) RDB() *redis.Client {
	return c.rdb
}

// SetNX sets a key only if it does not exist, with a TTL.
func (c *Client) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, errors.New("redis client not available")
	}
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

// BlocklistToken stores a token hash with its remaining TTL.
func (c *Client) BlocklistToken(ctx context.Context, token string, ttl time.Duration) error {
	return c.rdb.Set(ctx, "token:blocklist:"+hashToken(token), "1", ttl).Err()
}

// IsTokenBlocklisted checks whether a token has been revoked.
func (c *Client) IsTokenBlocklisted(ctx context.Context, token string) (bool, error) {
	n, err := c.rdb.Exists(ctx, "token:blocklist:"+hashToken(token)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// StoreRefreshToken stores a refresh token bound to a user.
func (c *Client) StoreRefreshToken(ctx context.Context, userID, refreshToken string, ttl time.Duration) error {
	return c.rdb.Set(ctx, "refresh:"+userID+":"+hashToken(refreshToken), "1", ttl).Err()
}

// ValidateRefreshToken checks whether a refresh token is still valid.
func (c *Client) ValidateRefreshToken(ctx context.Context, userID, refreshToken string) (bool, error) {
	n, err := c.rdb.Exists(ctx, "refresh:"+userID+":"+hashToken(refreshToken)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RevokeRefreshToken removes a refresh token.
func (c *Client) RevokeRefreshToken(ctx context.Context, userID, refreshToken string) error {
	return c.rdb.Del(ctx, "refresh:"+userID+":"+hashToken(refreshToken)).Err()
}

// RevokeAllUserRefreshTokens removes all refresh tokens for a user.
func (c *Client) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	iter := c.rdb.Scan(ctx, 0, "refresh:"+userID+":*", 0).Iterator()
	for iter.Next(ctx) {
		if err := c.rdb.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// CreateVerificationToken creates a single-use email-verification token.
func (c *Client) CreateVerificationToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	token := uuid.NewString()
	if err := c.rdb.Set(ctx, "verify:"+token, userID, ttl).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// UserIDByVerificationToken resolves a verification token to a user ID.
func (c *Client) UserIDByVerificationToken(ctx context.Context, token string) (string, error) {
	return c.rdb.Get(ctx, "verify:"+token).Result()
}

// DeleteVerificationToken removes a verification token after use.
func (c *Client) DeleteVerificationToken(ctx context.Context, token string) error {
	return c.rdb.Del(ctx, "verify:"+token).Err()
}

// GetIdempotencyResponse retrieves a cached idempotent response if it exists.
func (c *Client) GetIdempotencyResponse(ctx context.Context, key string) (*middleware.IdempotencyResponse, error) {
	data, err := c.rdb.Get(ctx, "idempotency:"+key).Bytes()
	if err != nil {
		return nil, err
	}
	var resp middleware.IdempotencyResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StoreIdempotencyResponse caches a successful response for future idempotent replays.
func (c *Client) StoreIdempotencyResponse(ctx context.Context, key string, resp *middleware.IdempotencyResponse, ttl time.Duration) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, "idempotency:"+key, data, ttl).Err()
}

// SetEmailCode stores a short-lived email verification code.
func (c *Client) SetEmailCode(ctx context.Context, key, code string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, code, ttl).Err()
}

// GetEmailCode retrieves an email verification code.
func (c *Client) GetEmailCode(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// DeleteEmailCode removes an email verification code after successful verification.
func (c *Client) DeleteEmailCode(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// AllowEmailCodeSend returns true if the caller has not exceeded max sends in the window.
func (c *Client) AllowEmailCodeSend(ctx context.Context, key string, maxSends int, window time.Duration) (bool, error) {
	pipe := c.rdb.Pipeline()
	now := time.Now().UnixMilli()
	windowStart := now - window.Milliseconds()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, key, window)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	countCmd := cmds[1].(*redis.IntCmd)
	count := countCmd.Val()
	return count < int64(maxSends), nil
}

// RateLimitAllow checks whether a key is within its limit using token bucket approximation.
func (c *Client) RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	pipe := c.rdb.Pipeline()
	now := time.Now().UnixMilli()
	windowStart := now - window.Milliseconds()
	zkey := "ratelimit:" + key
	pipe.ZRemRangeByScore(ctx, zkey, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZCard(ctx, zkey)
	pipe.ZAdd(ctx, zkey, redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, zkey, window)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, err
	}
	countCmd := cmds[1].(*redis.IntCmd)
	count := countCmd.Val()
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}
	return count <= int64(limit), remaining, nil
}
