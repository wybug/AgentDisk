package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// GraphCache stores per-node adjacency lists so BFS expansions do not have to
// hit the edge table on every hop. A cache miss falls through to the batched
// MySQL/SQLite lookup; a hit short-circuits it.
//
// The cache stores only *live* outgoing edges (dst_exists=true) because BFS
// treats dead links as leaves. Storing them here would just inflate the
// payload without changing the walk.
type GraphCache interface {
	// GetAdj returns the cached outgoing-node-id list for (bundleID, nodeID).
	// A nil slice and nil error mean "not cached" — the caller must go to the
	// DB. An empty (non-nil) slice means "cached empty" (the node has no live
	// outgoing edges) and the caller should NOT hit the DB.
	GetAdj(ctx context.Context, bundleID, nodeID uint64) ([]uint64, error)
	// SetAdj stores the adjacency list with the cache's configured TTL. An
	// empty ids slice is cached as "empty" (a sentinel) so subsequent lookups
	// still hit.
	SetAdj(ctx context.Context, bundleID, nodeID uint64, ids []uint64) error
	// Invalidate drops the cached adjacency for one or more nodes. Called by
	// the WriteMarkdown path after the edge table is mutated.
	Invalidate(ctx context.Context, bundleID uint64, nodeIDs []uint64) error
}

// NoOpGraphCache is the cache disabled. Every GetAdj returns a miss. Wire
// this in when Redis is not configured — the BFS layer still works, just
// slower, since every hop goes to the DB.
type NoOpGraphCache struct{}

// GetAdj always returns a miss so the BFS layer falls through to the DB.
func (NoOpGraphCache) GetAdj(_ context.Context, _, _ uint64) ([]uint64, error) { return nil, nil }

// SetAdj is a no-op; without a backing store there is nothing to cache.
func (NoOpGraphCache) SetAdj(_ context.Context, _, _ uint64, _ []uint64) error { return nil }

// Invalidate is a no-op; nothing was cached so nothing needs dropping.
func (NoOpGraphCache) Invalidate(_ context.Context, _ uint64, _ []uint64) error { return nil }

// RedisGraphCache is the Redis-backed GraphCache. Values are JSON-encoded
// uint64 slices under key `okf:adj:{bundleID}:{nodeID}`. The TTL caps the
// worst-case staleness window — a writer that fails to DEL (Redis briefly
// unreachable, process crash mid-write) still has its stale entry age out
// within adjTTL.
//
// The cache is best-effort: any Redis error from Get/Set/Invalidate is
// swallowed and surfaced as a miss, since the BFS layer has a correct
// fallback. This keeps a Redis hiccup from turning into reader-facing 500s.
type RedisGraphCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisGraphCache constructs a Redis-backed GraphCache. ttlSeconds is
// clamped to a minimum of 1 second so a misconfiguration cannot produce an
// immediately-expiring entry.
func NewRedisGraphCache(client *redis.Client, ttlSeconds int) *RedisGraphCache {
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}
	return &RedisGraphCache{client: client, ttl: time.Duration(ttlSeconds) * time.Second}
}

// adjCacheMissSentinel is the JSON payload cached for a node whose live
// adjacency is empty. Without it, an empty slice and a cache miss would be
// indistinguishable: both decode to a nil []uint64. The sentinel is the
// empty array form, which json.Unmarshal decodes to a non-nil empty slice.
const adjCacheMissSentinel = `[]`

// GetAdj implements GraphCache. A Redis miss (redis.Nil) is surfaced as a
// nil slice so the caller can branch with `if ids == nil`.
func (c *RedisGraphCache) GetAdj(ctx context.Context, bundleID, nodeID uint64) ([]uint64, error) {
	if c.client == nil {
		return nil, nil
	}
	raw, err := c.client.Get(ctx, adjKey(bundleID, nodeID)).Bytes()
	if err != nil {
		// Miss or transient outage — either way the BFS layer must fall back
		// to the DB lookup. Returning (nil, nil) is the miss contract.
		return nil, nil //nolint:nilerr // best-effort cache: redis errors are treated as misses
	}
	var ids []uint64
	if uErr := json.Unmarshal(raw, &ids); uErr != nil {
		return nil, nil //nolint:nilerr // corrupt payload: treat as miss, DB is authoritative
	}
	return ids, nil
}

// SetAdj stores the adjacency list. An empty ids slice is stored as the
// sentinel `[]` so the "no outgoing edges" case is cacheable.
func (c *RedisGraphCache) SetAdj(ctx context.Context, bundleID, nodeID uint64, ids []uint64) error {
	if c.client == nil {
		return nil
	}
	payload := adjCacheMissSentinel
	if len(ids) > 0 {
		buf, err := json.Marshal(ids)
		if err != nil {
			return fmt.Errorf("encode adjacency: %w", err)
		}
		payload = string(buf)
	}
	if err := c.client.Set(ctx, adjKey(bundleID, nodeID), payload, c.ttl).Err(); err != nil {
		// Best-effort: a Set failure means the next Get will miss and the DB
		// path will run. Swallowing avoids turning a Redis blip into a 500.
		return nil //nolint:nilerr // best-effort cache: swallow set failures
	}
	return nil
}

// Invalidate DELs each node's cached adjacency in one Redis call (DEL takes
// multiple keys). nodeIDs may be empty.
func (c *RedisGraphCache) Invalidate(ctx context.Context, bundleID uint64, nodeIDs []uint64) error {
	if c.client == nil || len(nodeIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		keys = append(keys, adjKey(bundleID, id))
	}
	_ = c.client.Del(ctx, keys...).Err()
	return nil
}

// adjKey formats the Redis key for a node's adjacency entry.
func adjKey(bundleID, nodeID uint64) string {
	return fmt.Sprintf("okf:adj:%d:%d", bundleID, nodeID)
}
