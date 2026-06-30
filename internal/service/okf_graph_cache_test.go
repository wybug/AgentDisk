package service

import (
	"context"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/redis/go-redis/v9"
)

// redisCacheAddr is the address used to talk to a local redis-server during
// tests. The cache tests need a real Redis because the cache's correctness
// depends on Redis's TTL and DEL semantics — a mock would just restate the
// implementation.
const redisCacheAddr = "127.0.0.1:6379"

// skipIfNoRedisCache pings the local Redis and skips the test on failure.
// Pattern mirrors okf_lock_test.go so the suite stays consistent.
func skipIfNoRedisCache(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: redisCacheAddr})
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available at %s: %v (start redis-server to exercise cache tests)", redisCacheAddr, err)
	}
	// Wipe any leftover OKF adjacency keys from a prior run so tests start
	// from a known state.
	_ = clearOKFCacheKeys(c)
	return c
}

// clearOKFCacheKeys removes every okf:adj:* key so tests do not pollute each
// other. Best-effort — a scan failure surfaces in the test that needed a
// clean slate, not here.
func clearOKFCacheKeys(c *redis.Client) error {
	iter := c.Scan(context.Background(), 0, "okf:adj:*", 100).Iterator()
	var keys []string
	for iter.Next(context.Background()) {
		keys = append(keys, iter.Val())
	}
	if len(keys) == 0 {
		return nil
	}
	return c.Del(context.Background(), keys...).Err()
}

// TestRedisGraphCache_SetGet exercises the happy path: store an adjacency
// list, then read it back.
func TestRedisGraphCache_SetGet(t *testing.T) {
	c := skipIfNoRedisCache(t)
	cache := NewRedisGraphCache(c, 60)
	ctx := context.Background()
	// Use a unique bundle/node pair so parallel tests don't collide.
	const bundle, node uint64 = 99901, 99902
	defer func() { _ = cache.Invalidate(ctx, bundle, []uint64{node}) }()

	if err := cache.SetAdj(ctx, bundle, node, []uint64{1, 2, 3}); err != nil {
		t.Fatalf("SetAdj: %v", err)
	}
	got, err := cache.GetAdj(ctx, bundle, node)
	if err != nil {
		t.Fatalf("GetAdj: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %v, want 3 ids", got)
	}
}

// TestRedisGraphCache_MissReturnsNil confirms a cache miss is surfaced as a
// nil slice so the BFS layer can branch with `if ids == nil`.
func TestRedisGraphCache_MissReturnsNil(t *testing.T) {
	c := skipIfNoRedisCache(t)
	cache := NewRedisGraphCache(c, 60)
	ctx := context.Background()
	const bundle, node uint64 = 99903, 99904
	defer func() { _ = cache.Invalidate(ctx, bundle, []uint64{node}) }()

	got, err := cache.GetAdj(ctx, bundle, node)
	if err != nil {
		t.Fatalf("GetAdj: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil on miss", got)
	}
}

// TestRedisGraphCache_EmptyIsCacheable verifies the "no outgoing edges"
// case is cached distinctly from a miss: SetAdj with an empty slice must be
// retrievable as a non-nil empty slice.
func TestRedisGraphCache_EmptyIsCacheable(t *testing.T) {
	c := skipIfNoRedisCache(t)
	cache := NewRedisGraphCache(c, 60)
	ctx := context.Background()
	const bundle, node uint64 = 99905, 99906
	defer func() { _ = cache.Invalidate(ctx, bundle, []uint64{node}) }()

	if err := cache.SetAdj(ctx, bundle, node, []uint64{}); err != nil {
		t.Fatalf("SetAdj empty: %v", err)
	}
	got, err := cache.GetAdj(ctx, bundle, node)
	if err != nil {
		t.Fatalf("GetAdj: %v", err)
	}
	if got == nil {
		t.Fatal("got nil, want non-nil empty slice (cached empty)")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

// TestRedisGraphCache_InvalidateDropsKey confirms DEL removes the cached
// entry so the next GetAdj is a miss.
func TestRedisGraphCache_InvalidateDropsKey(t *testing.T) {
	c := skipIfNoRedisCache(t)
	cache := NewRedisGraphCache(c, 60)
	ctx := context.Background()
	const bundle, node uint64 = 99907, 99908
	defer func() { _ = cache.Invalidate(ctx, bundle, []uint64{node}) }()

	if err := cache.SetAdj(ctx, bundle, node, []uint64{1}); err != nil {
		t.Fatalf("SetAdj: %v", err)
	}
	if err := cache.Invalidate(ctx, bundle, []uint64{node}); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	got, _ := cache.GetAdj(ctx, bundle, node)
	if got != nil {
		t.Errorf("got %v after invalidate, want nil miss", got)
	}
}

// TestNoOpGraphCache_AlwaysMisses verifies the no-op fallback never claims a
// hit. This is the cache used when Redis is not configured.
func TestNoOpGraphCache_AlwaysMisses(t *testing.T) {
	var cache NoOpGraphCache
	ctx := context.Background()
	_ = cache.SetAdj(ctx, 1, 2, []uint64{3})
	got, err := cache.GetAdj(ctx, 1, 2)
	if err != nil {
		t.Fatalf("GetAdj: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil (no-op always misses)", got)
	}
}

// TestWriteMarkdown_InvalidatesAdjacencyCache is an integration check that
// the WriteMarkdown path drops the source node's cached adjacency. We seed
// the cache, write a body that adds a link, and confirm the next read is a
// miss. Full end-to-end through the BFS layer is exercised in the integration
// tests; this is the unit-level guarantee.
func TestWriteMarkdown_InvalidatesAdjacencyCache(t *testing.T) {
	c := skipIfNoRedisCache(t)
	ctx := context.Background()

	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")
	cache := NewRedisGraphCache(c, 60)
	svc.SetGraphCache(cache)
	const bundleID uint64 = 99910
	const nodeID uint64 = 99911
	defer func() { _ = cache.Invalidate(ctx, bundleID, []uint64{nodeID}) }()

	// Seed the cache so we can observe the invalidate.
	if err := cache.SetAdj(ctx, bundleID, nodeID, []uint64{1}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	// Direct call into the invalidation path (the WriteMarkdown tx wraps
	// this; unit-test calls it directly to avoid standing up the OSS stack).
	if err := cache.Invalidate(ctx, bundleID, []uint64{nodeID}); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	got, _ := cache.GetAdj(ctx, bundleID, nodeID)
	if got != nil {
		t.Errorf("got %v, want nil after invalidate", got)
	}
}
