package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisTestAddr is the address tests use to talk to the locally-running
// redis-server required by CLAUDE.md §3.1. Hardcoded rather than read from
// config so a misconfigured CI environment fails loudly with a clear cause
// rather than a silent skip.
const redisTestAddr = "localhost:6379"

// skipIfNoRedis short-circuits a test when redis-server is not reachable.
// CLAUDE.md §3.1 forbids mocking Redis, so the lock tests require a real
// instance. We still skip rather than hard-fail so a developer running just
// `go test ./internal/service/...` without Redis gets a clear skip message
// instead of a confusing failure cascade.
func skipIfNoRedis(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: redisTestAddr})
	if pErr := c.Ping(context.Background()).Err(); pErr != nil {
		t.Skipf("redis-server not reachable at %s: %v", redisTestAddr, pErr)
	}
	return c
}

// TestNoOpBundleLock_AlwaysSucceeds documents the no-op fallback behavior.
// Acquire returns a non-nil release and never errors.
func TestNoOpBundleLock_AlwaysSucceeds(t *testing.T) {
	l := NoOpBundleLock{}
	release, err := l.Acquire(context.Background(), 1)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if release == nil {
		t.Fatal("release = nil, want non-nil")
	}
	// Calling release twice must be safe.
	release()
	release()
}

// TestRedisBundleLock_AcquireRelease covers the happy path: acquire, release,
// then acquire again on the same bundle succeeds.
func TestRedisBundleLock_AcquireRelease(t *testing.T) {
	c := skipIfNoRedis(t)
	defer func() { _ = c.Close() }()
	// Clean slate so a leftover key from a prior run does not interfere.
	_, _ = c.Del(context.Background(), okfLockKey(101)).Result()

	l := NewRedisBundleLock(c, 5)
	release, err := l.Acquire(context.Background(), 101)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if release == nil {
		t.Fatal("release = nil")
	}
	release()

	// Second acquire must succeed immediately after release.
	release2, err := l.Acquire(context.Background(), 101)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	release2()
}

// TestRedisBundleLock_SecondAcquireFails verifies the lock is exclusive: a
// second concurrent acquire on the same bundle returns ErrOkfLockHeld.
func TestRedisBundleLock_SecondAcquireFails(t *testing.T) {
	c := skipIfNoRedis(t)
	defer func() { _ = c.Close() }()
	_, _ = c.Del(context.Background(), okfLockKey(102)).Result()

	l := NewRedisBundleLock(c, 5)
	release, err := l.Acquire(context.Background(), 102)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()

	if _, err := l.Acquire(context.Background(), 102); !errors.Is(err, ErrOkfLockHeld) {
		t.Fatalf("second Acquire err = %v, want ErrOkfLockHeld", err)
	}
}

// TestRedisBundleLock_TTLExpires verifies the lock auto-releases after the
// TTL elapses, so a crashed writer does not pin the bundle forever.
func TestRedisBundleLock_TTLExpires(t *testing.T) {
	c := skipIfNoRedis(t)
	defer func() { _ = c.Close() }()
	_, _ = c.Del(context.Background(), okfLockKey(103)).Result()

	// 1-second TTL keeps the test fast.
	l := NewRedisBundleLock(c, 1)
	release, err := l.Acquire(context.Background(), 103)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	// Don't call release — let the TTL expire.

	// Wait a bit longer than the TTL. Redis SET EX is precise enough that
	// 1.5s reliably lands past the expiry.
	time.Sleep(1500 * time.Millisecond)

	// We can now re-acquire. The original release must also be a no-op (the
	// key is gone, or it's a different token).
	release2, err := l.Acquire(context.Background(), 103)
	if err != nil {
		t.Fatalf("Acquire after TTL: %v", err)
	}
	release() // late release — must not delete the new holder's key
	// The new holder's key must still be present; a late release on the old
	// token should be a Lua no-op.
	exists, _ := c.Exists(context.Background(), okfLockKey(103)).Result()
	if exists != 1 {
		t.Errorf("late release clobbered new holder: exists=%d", exists)
	}
	release2()
}

// TestRedisBundleLock_LateReleaseNoOp verifies that a release called with a
// stale token does not delete a key owned by a different holder. This is the
// core safety property of the Lua compare-and-delete pattern.
func TestRedisBundleLock_LateReleaseNoOp(t *testing.T) {
	c := skipIfNoRedis(t)
	defer func() { _ = c.Close() }()
	_, _ = c.Del(context.Background(), okfLockKey(104)).Result()

	l := NewRedisBundleLock(c, 30)
	release1, _ := l.Acquire(context.Background(), 104)
	// Force a second holder by deleting the key (simulates TTL expiry).
	_, _ = c.Del(context.Background(), okfLockKey(104)).Result()
	release2, err := l.Acquire(context.Background(), 104)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	// release1 is the old holder; calling it must not drop release2's lock.
	release1()
	exists, _ := c.Exists(context.Background(), okfLockKey(104)).Result()
	if exists != 1 {
		t.Errorf("late release1 dropped the live lock: exists=%d", exists)
	}
	release2()
}

// TestRedisBundleLock_NilClientFallback guards the defensive path where the
// constructor was bypassed. Acquire must not panic.
func TestRedisBundleLock_NilClientFallback(t *testing.T) {
	l := &RedisBundleLock{client: nil, ttl: time.Second}
	release, err := l.Acquire(context.Background(), 1)
	if err != nil {
		t.Fatalf("nil client Acquire: %v", err)
	}
	if release == nil {
		t.Fatal("release = nil")
	}
	release()
}

// TestRedisBundleLock_ClampsTTL verifies a 0 / negative TTL is clamped to 1s.
// Constructing the lock with a bad value must not produce a sub-second TTL
// that expires immediately.
func TestRedisBundleLock_ClampsTTL(t *testing.T) {
	c := skipIfNoRedis(t)
	defer func() { _ = c.Close() }()
	l := NewRedisBundleLock(c, 0)
	if l.ttl != time.Second {
		t.Errorf("ttl = %v, want 1s", l.ttl)
	}
	l2 := NewRedisBundleLock(c, -5)
	if l2.ttl != time.Second {
		t.Errorf("negative ttl = %v, want 1s", l2.ttl)
	}
}
