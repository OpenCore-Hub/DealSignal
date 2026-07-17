package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/redis"
)

// RedisCache implements Cache on top of the application Redis client.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a cache backed by Redis.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// Get retrieves a JSON-encoded value and decodes it into dest.
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if c == nil || c.client == nil {
		return errors.New("redis cache not available")
	}
	val, err := c.client.RDB().Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// Set stores a JSON-encoded value with the given TTL.
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return errors.New("redis cache not available")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.RDB().Set(ctx, key, data, ttl).Err()
}
