package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ErrOkfLockHeld is returned by BundleLock.Acquire when another caller already
// holds the lock for a bundle. Handlers map this onto HTTP 409 Conflict so
// concurrent writers can back off rather than overwrite each other.
var ErrOkfLockHeld = errors.New("okf: bundle lock held by another caller")

// BundleLock is a per-bundle advisory lock used to serialize writers. The
// release function returned by Acquire must be called when the holder is done
// (typically via defer). Implementations must be safe for concurrent use.
type BundleLock interface {
	// Acquire takes the lock for bundleID. On success it returns a non-nil
	// release function the caller must invoke. On conflict it returns
	// ErrOkfLockHeld.
	Acquire(ctx context.Context, bundleID uint64) (release func(), err error)
}

// NoOpBundleLock is the last-writer-wins fallback used when Redis is not
// configured. Acquire always succeeds and release is a no-op. Operators that
// accept the lost-update risk in single-writer deployments can wire this in
// place of the Redis lock; the warn-on-init path in router.go records the
// degradation so it is not silent.
type NoOpBundleLock struct{}

// Acquire returns a no-op release function. It never fails.
func (NoOpBundleLock) Acquire(_ context.Context, _ uint64) (func(), error) {
	return func() {}, nil
}

// RedisBundleLock implements BundleLock on top of Redis SET NX EX + a Lua
// compare-and-delete release. The TTL caps the worst-case hold time so a
// crashed writer does not pin the bundle forever: a holder that takes longer
// than LockTTLSeconds risks having its release silently no-op (the key
// expired and another caller acquired), which is the standard Redlock-style
// trade-off for short writes.
type RedisBundleLock struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisBundleLock constructs a Redis-backed BundleLock. ttlSeconds is
// clamped to a minimum of 1 second so a misconfiguration cannot produce an
// immediately-expiring lock. Pass the same *redis.Client the rest of the
// process uses; if the deployment does not run Redis, use NoOpBundleLock.
func NewRedisBundleLock(client *redis.Client, ttlSeconds int) *RedisBundleLock {
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}
	return &RedisBundleLock{client: client, ttl: time.Duration(ttlSeconds) * time.Second}
}

// Acquire attempts SET key token NX EX ttl. The token is a random UUID we
// match on release so we only ever delete our own key (compare-and-delete via
// Lua). On conflict the function returns ErrOkfLockHeld so callers can branch
// without parsing error strings.
func (l *RedisBundleLock) Acquire(ctx context.Context, bundleID uint64) (func(), error) {
	if l.client == nil {
		// Defensive: if the constructor was bypassed, fall back to no-op so
		// the writer path is never blocked by a nil dereference.
		return func() {}, nil
	}
	token := uuid.NewString()
	key := okfLockKey(bundleID)
	ok, err := l.client.SetNX(ctx, key, token, l.ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire okf lock: %w", err)
	}
	if !ok {
		return nil, ErrOkfLockHeld
	}
	released := false
	return func() {
		if released {
			return
		}
		released = true
		// Best-effort release; a failure here means either the key already
		// expired (TTL breach) or Redis is briefly unreachable. Either way
		// the writer has already committed its transaction, so we drop the
		// error rather than retrying — the TTL will reclaim the key.
		_, _ = l.client.Eval(ctx, luaCompareDelete, []string{key}, token).Result()
	}, nil
}

// okfLockKey formats the Redis key for a bundle lock. The "okf:lock:" prefix
// keeps the keyspace readable in redis-cli and avoids collisions with other
// Redis users on a shared cluster.
func okfLockKey(bundleID uint64) string {
	return fmt.Sprintf("okf:lock:%d", bundleID)
}

// luaCompareDelete atomically deletes the key only if its value equals the
// token. This is the canonical Redis pattern for safe lock release and
// matches the OKF spec example verbatim.
const luaCompareDelete = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) end
return 0`
