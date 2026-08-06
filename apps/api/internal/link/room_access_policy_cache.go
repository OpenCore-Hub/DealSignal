package link

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const roomAccessPolicyBlockedCacheTTL = 5 * time.Minute

func roomAccessPolicyBlockedCacheKey(dealRoomID string) string {
	return "room_access_policy:blocked:" + dealRoomID
}

// cachedRoomBlockedEmails returns the configured room blocklist for runtime
// access evaluation. Results are cached in Redis and invalidated on policy upsert.
func (s *Service) cachedRoomBlockedEmails(
	ctx context.Context,
	workspaceID pgtype.UUID,
	dealRoomID pgtype.UUID,
) ([]string, error) {
	if !dealRoomID.Valid {
		return nil, nil
	}
	roomIDStr := uuid.UUID(dealRoomID.Bytes).String()
	cacheKey := roomAccessPolicyBlockedCacheKey(roomIDStr)

	if s.redisClient != nil {
		if raw, err := s.redisClient.Get(ctx, cacheKey); err == nil {
			var blocked []string
			if json.Unmarshal([]byte(raw), &blocked) == nil {
				return blocked, nil
			}
		}
	}

	wsID := uuid.UUID(workspaceID.Bytes).String()
	row, hasPolicy, err := s.loadRoomAccessPolicyRow(ctx, wsID, roomIDStr)
	if err != nil {
		return nil, err
	}
	blocked := []string{}
	if hasPolicy && row.Configured && row.BlockedEmails != nil {
		blocked = append(blocked, row.BlockedEmails...)
	}

	if s.redisClient != nil {
		if raw, err := json.Marshal(blocked); err == nil {
			_ = s.redisClient.Set(ctx, cacheKey, string(raw), roomAccessPolicyBlockedCacheTTL)
		}
	}
	return blocked, nil
}

func (s *Service) invalidateRoomAccessPolicyCache(ctx context.Context, dealRoomID string) {
	if s.redisClient == nil || dealRoomID == "" {
		return
	}
	_ = s.redisClient.Del(ctx, roomAccessPolicyBlockedCacheKey(dealRoomID))
}
