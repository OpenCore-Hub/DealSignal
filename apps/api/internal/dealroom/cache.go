package dealroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// listCacheTTL keeps engagement counters fresh enough for commercial UX while
// absorbing list storms. Structural writes always invalidate.
const listCacheTTL = 20 * time.Second

const (
	dealRoomsDefaultPageSize = 24
	dealRoomsMaxPageSize     = 100
)

// ListCache stores serialized deal-room list payloads for a workspace.
type ListCache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisListCache implements ListCache on the application Redis client.
type RedisListCache struct {
	client *redis.Client
}

// NewRedisListCache creates a Redis-backed list cache.
func NewRedisListCache(client *redis.Client) *RedisListCache {
	return &RedisListCache{client: client}
}

// Get retrieves a JSON-encoded value and decodes it into dest.
func (c *RedisListCache) Get(ctx context.Context, key string, dest interface{}) error {
	if c == nil || c.client == nil {
		return errors.New("redis list cache not available")
	}
	val, err := c.client.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// Set stores a JSON-encoded value with the given TTL.
func (c *RedisListCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return errors.New("redis list cache not available")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, string(data), ttl)
}

// Del removes one or more cache keys.
func (c *RedisListCache) Del(ctx context.Context, keys ...string) error {
	if c == nil || c.client == nil {
		return errors.New("redis list cache not available")
	}
	return c.client.Del(ctx, keys...)
}

// TryAcquireDebounce returns true once per window for key (Redis SETNX).
func (c *RedisListCache) TryAcquireDebounce(ctx context.Context, key string, window time.Duration) bool {
	if c == nil || c.client == nil || window <= 0 {
		return true
	}
	ok, err := c.client.SetNX(ctx, key, "1", window)
	if err != nil {
		return true
	}
	return ok
}

func listCacheKey(workspaceID string) string {
	return fmt.Sprintf("dealrooms:list:v2:%s", workspaceID)
}

func roomAnalyticsCacheKey(workspaceID, roomID string) string {
	return fmt.Sprintf("dealrooms:analytics:v1:%s:%s", workspaceID, roomID)
}

// cachedRoomListItem is the slim Redis payload for list cards (no settings JSON).
type cachedRoomListItem struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	TemplateType     string     `json:"template_type,omitempty"`
	Status           string     `json:"status"`
	RequiresNDA      bool       `json:"requires_nda"`
	RequiresApproval bool       `json:"requires_approval"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DocumentCount    int64      `json:"document_count"`
	MemberCount      int64      `json:"member_count"`
	PendingApprovals int64      `json:"pending_approvals"`
	VisitorCount     int64      `json:"visitor_count"`
	UnreadQuestions  int64      `json:"unread_questions"`
	HeatScore        int32      `json:"heat_score"`
	LastAccessedAt   *time.Time `json:"last_accessed_at,omitempty"`
}

func roomSummariesToCached(items []RoomSummary) []cachedRoomListItem {
	out := make([]cachedRoomListItem, len(items))
	for i, item := range items {
		out[i] = cachedRoomListItem{
			ID:               uuid.UUID(item.Room.ID.Bytes).String(),
			Slug:             item.Room.Slug,
			Name:             item.Room.Name,
			Status:           item.Room.Status,
			RequiresNDA:      item.Room.RequiresNda,
			RequiresApproval: item.Room.RequiresApproval,
			DocumentCount:    item.DocumentCount,
			MemberCount:      item.MemberCount,
			PendingApprovals: item.PendingApprovals,
			VisitorCount:     item.VisitorCount,
			UnreadQuestions:  item.UnreadQuestions,
			HeatScore:        item.HeatScore,
		}
		if item.Room.Description.Valid {
			out[i].Description = item.Room.Description.String
		}
		if item.Room.TemplateType.Valid {
			out[i].TemplateType = item.Room.TemplateType.String
		}
		if item.Room.CreatedAt.Valid {
			out[i].CreatedAt = item.Room.CreatedAt.Time
		}
		if item.Room.UpdatedAt.Valid {
			out[i].UpdatedAt = item.Room.UpdatedAt.Time
		}
		if item.LastAccessedAt.Valid {
			t := item.LastAccessedAt.Time
			out[i].LastAccessedAt = &t
		}
	}
	return out
}

func cachedToRoomSummaries(items []cachedRoomListItem) []RoomSummary {
	out := make([]RoomSummary, len(items))
	for i, item := range items {
		out[i] = RoomSummary{
			Room: db.DealRoom{
				ID:               pgUUID(item.ID),
				Slug:             item.Slug,
				Name:             item.Name,
				Description:      pgtype.Text{String: item.Description, Valid: item.Description != ""},
				TemplateType:     pgtype.Text{String: item.TemplateType, Valid: item.TemplateType != ""},
				RequiresNda:      item.RequiresNDA,
				RequiresApproval: item.RequiresApproval,
				Status:           item.Status,
				CreatedAt:        pgtype.Timestamptz{Time: item.CreatedAt, Valid: !item.CreatedAt.IsZero()},
				UpdatedAt:        pgtype.Timestamptz{Time: item.UpdatedAt, Valid: !item.UpdatedAt.IsZero()},
			},
			DocumentCount:    item.DocumentCount,
			MemberCount:      item.MemberCount,
			PendingApprovals: item.PendingApprovals,
			VisitorCount:     item.VisitorCount,
			UnreadQuestions:  item.UnreadQuestions,
			HeatScore:        item.HeatScore,
		}
		if item.LastAccessedAt != nil {
			out[i].LastAccessedAt = pgtype.Timestamptz{Time: *item.LastAccessedAt, Valid: true}
		}
	}
	return out
}

func normalizeDealRoomsPaging(page, pageSize int, total int64) (normPage, normSize, offset int) {
	normPage = page
	normSize = pageSize
	if normPage < 1 {
		normPage = 1
	}
	if normSize < 1 {
		normSize = dealRoomsDefaultPageSize
	}
	if normSize > dealRoomsMaxPageSize {
		normSize = dealRoomsMaxPageSize
	}
	if total < 0 {
		total = 0
	}
	maxPage := 1
	if total > 0 {
		maxPage = int((total + int64(normSize) - 1) / int64(normSize))
		if maxPage < 1 {
			maxPage = 1
		}
	}
	if normPage > maxPage {
		normPage = maxPage
	}
	offset = (normPage - 1) * normSize
	return normPage, normSize, offset
}
