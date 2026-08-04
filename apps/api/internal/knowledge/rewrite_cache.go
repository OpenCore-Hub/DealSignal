package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	knowledgeQARewriteCacheTTL     = 15 * time.Minute
	knowledgeQARewriteCacheMaxMem  = 2048
	knowledgeQARewriteCacheKeyPref = "knowledge:qa:rewrite:v1:"
)

// rewriteCacheEntry is a provenanced rewrite result (never served without re-grounding).
type rewriteCacheEntry struct {
	Query string `json:"q"`
	Basis string `json:"b"` // state | prior_only
}

// rewriteKV is a minimal string KV used by Redis/memory rewrite caches.
type rewriteKV interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// rewriteCache stores applied rewrite results keyed by provenance fingerprint.
type rewriteCache interface {
	Get(ctx context.Context, key string) (rewriteCacheEntry, bool)
	Set(ctx context.Context, key string, entry rewriteCacheEntry)
}

type memoryRewriteCache struct {
	mu    sync.Mutex
	items map[string]memoryRewriteItem
}

type memoryRewriteItem struct {
	entry rewriteCacheEntry
	exp   time.Time
}

// NewMemoryRewriteCache builds a process-local rewrite cache (tests / no Redis).
func NewMemoryRewriteCache() rewriteCache {
	return &memoryRewriteCache{items: map[string]memoryRewriteItem{}}
}

func (c *memoryRewriteCache) Get(_ context.Context, key string) (rewriteCacheEntry, bool) {
	if c == nil || key == "" {
		return rewriteCacheEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	it, ok := c.items[key]
	if !ok {
		return rewriteCacheEntry{}, false
	}
	if time.Now().After(it.exp) {
		delete(c.items, key)
		return rewriteCacheEntry{}, false
	}
	return it.entry, true
}

func (c *memoryRewriteCache) Set(_ context.Context, key string, entry rewriteCacheEntry) {
	if c == nil || key == "" || strings.TrimSpace(entry.Query) == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= knowledgeQARewriteCacheMaxMem {
		// Cheap eviction: drop expired, else clear half.
		now := time.Now()
		for k, it := range c.items {
			if now.After(it.exp) {
				delete(c.items, k)
			}
		}
		if len(c.items) >= knowledgeQARewriteCacheMaxMem {
			n := 0
			for k := range c.items {
				delete(c.items, k)
				n++
				if n >= knowledgeQARewriteCacheMaxMem/2 {
					break
				}
			}
		}
	}
	c.items[key] = memoryRewriteItem{entry: entry, exp: time.Now().Add(knowledgeQARewriteCacheTTL)}
}

type kvRewriteCache struct {
	kv rewriteKV
}

// NewKVRewriteCache wraps a Redis-like string KV for cross-replica rewrite cache.
func NewKVRewriteCache(kv rewriteKV) rewriteCache {
	if kv == nil {
		return nil
	}
	return &kvRewriteCache{kv: kv}
}

func (c *kvRewriteCache) Get(ctx context.Context, key string) (rewriteCacheEntry, bool) {
	if c == nil || c.kv == nil || key == "" {
		return rewriteCacheEntry{}, false
	}
	raw, err := c.kv.Get(ctx, knowledgeQARewriteCacheKeyPref+key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return rewriteCacheEntry{}, false
	}
	var entry rewriteCacheEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil || strings.TrimSpace(entry.Query) == "" {
		return rewriteCacheEntry{}, false
	}
	return entry, true
}

func (c *kvRewriteCache) Set(ctx context.Context, key string, entry rewriteCacheEntry) {
	if c == nil || c.kv == nil || key == "" || strings.TrimSpace(entry.Query) == "" {
		return
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = c.kv.Set(ctx, knowledgeQARewriteCacheKeyPref+key, string(raw), knowledgeQARewriteCacheTTL)
}

// rewriteCacheKey fingerprints session + prior turn + user wording + grounding surface.
// Corpus/entity drift changes the key so stale rewrites cannot replay.
func rewriteCacheKey(
	sessionID pgtype.UUID,
	priorTurnID string,
	userQuery string,
	state SessionState,
	evidence []followUpLLMEvidence,
) string {
	var b strings.Builder
	if sessionID.Valid {
		b.WriteString(uuid.UUID(sessionID.Bytes).String())
	}
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(priorTurnID))
	b.WriteByte('|')
	b.WriteString(strings.ToLower(strings.TrimSpace(userQuery)))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(sessionStateRewriteSurface(state)))
	b.WriteByte('|')
	for _, e := range evidence {
		b.WriteString(strings.ToLower(strings.TrimSpace(e.SourceName)))
		b.WriteByte(';')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}

func (s *Service) storeRewriteCache(
	ctx context.Context,
	sessionID pgtype.UUID,
	priorTurnID, userQuery string,
	state SessionState,
	evidence []followUpLLMEvidence,
	rewritten, basis string,
) {
	if s == nil || s.rewriteCache == nil {
		return
	}
	key := rewriteCacheKey(sessionID, priorTurnID, userQuery, state, evidence)
	s.rewriteCache.Set(ctx, key, rewriteCacheEntry{Query: rewritten, Basis: basis})
}
