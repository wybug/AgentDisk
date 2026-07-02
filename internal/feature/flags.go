// Package feature provides a runtime-toggleable feature flag registry with
// hot-reload via fsnotify and admin API persistence.
//
// Design:
//   - Flags is a snapshot of all feature flags; reads return *Flags so callers
//     get a consistent point-in-time view (no torn reads).
//   - Registry stores the current snapshot via atomic.Pointer, so the hot path
//     (Get() from middleware) is lock-free.
//   - Set() writes the new snapshot atomically AND persists to config.yaml so
//     the toggle survives restart. A mutex serializes Set calls to keep the
//     file write linearizable.
//   - A fsnotify watcher reloads config.yaml on external edits (operator-
//     initiated changes from vi/echo). 200ms debounce coalesces rapid saves.
//
// Boot-time vs runtime:
//   - cfg.Okf.Enabled is read once at boot and gates whether OKF routes are
//     registered at all. If false at boot, the routes don't exist.
//   - features.* are read at every request via middleware. Operators can flip
//     them at runtime via the admin API; routes return 403 until re-enabled.
package feature

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/agentdisk/agent-disk/config"
	"github.com/fsnotify/fsnotify"
)

// FlagName is the stable identifier for a flag. Wire-compat: this string
// appears in admin API request/response payloads and in config.yaml keys.
type FlagName string

// Flag constants. Add a new flag here, then wire a case in applySet /
// flagEnabled, plus a Flags field and a FeaturesConfig field. The wire
// format (the string value) is part of the public admin API contract.
const (
	// FlagOkfReader gates OKF reader + maintenance routes.
	FlagOkfReader FlagName = "okfReader"
	// FlagOkfWriter gates OKF writer routes (write_markdown, register,
	// refresh, regenerate-index).
	FlagOkfWriter FlagName = "okfWriter"
	// FlagOkfGraphBFS gates the BFS-heavy routes (reachable, shortest,
	// subgraph, neighbors, stats).
	FlagOkfGraphBFS FlagName = "okfGraphBFS"
)

// allFlags lists every known flag. Adding a new flag = append here + add a
// Flags field + wire a case in applySet / flagEnabled.
var allFlags = []FlagName{FlagOkfReader, FlagOkfWriter, FlagOkfGraphBFS}

// Flags is the immutable snapshot returned by Get(). Fields are read directly
// (no methods) so callers can grab *Flags once and check several flags in a
// single atomic view.
type Flags struct {
	OkfReader   bool
	OkfWriter   bool
	OkfGraphBFS bool
}

// flagEnabled returns the value of the named flag on this snapshot.
func (f *Flags) flagEnabled(name FlagName) bool {
	switch name {
	case FlagOkfReader:
		return f.OkfReader
	case FlagOkfWriter:
		return f.OkfWriter
	case FlagOkfGraphBFS:
		return f.OkfGraphBFS
	}
	return false
}

// Registry holds the live feature flags and supports hot-reload + admin-
// initiated updates. Construct once at boot via NewRegistry; share across
// handlers + middleware.
type Registry struct {
	cur     atomic.Pointer[Flags]
	cfgPath string
	// mu serializes Set calls so two concurrent admin PATCHes can't tear the
	// file write. The hot-path Get() does not take this lock — it goes through
	// atomic.Pointer.
	mu      sync.Mutex
	watcher *fsnotify.Watcher
	closed  chan struct{}
}

// NewRegistry snapshots the initial flags from cfg and starts a fsnotify
// watcher on cfgPath. The cfgPath is also where Set() persists updates.
// Returns an error only if the fsnotify watcher cannot be created; missing
// cfgPath is logged but not fatal (admin API + Set still work, just no hot
// reload).
func NewRegistry(cfg *config.Config, cfgPath string) (*Registry, error) {
	r := &Registry{cfgPath: cfgPath, closed: make(chan struct{})}
	r.cur.Store(snapshotFromConfig(cfg))

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("feature: create fsnotify watcher: %w", err)
	}
	r.watcher = w
	if cfgPath != "" {
		if err := w.Add(cfgPath); err != nil {
			log.Printf("feature: fsnotify watch %s failed (hot-reload disabled): %v", cfgPath, err)
		} else {
			go r.watchLoop()
		}
	}
	return r, nil
}

// Get returns the current flag snapshot. Safe for concurrent use; the
// middleware hot path calls this once per request.
func (r *Registry) Get() *Flags {
	return r.cur.Load()
}

// IsEnabled returns the current value of a single flag. Convenience wrapper
// for callers that don't need a multi-flag snapshot.
func (r *Registry) IsEnabled(name FlagName) bool {
	return r.Get().flagEnabled(name)
}

// Set flips one flag and persists the change to config.yaml. The persistence
// is best-effort: a write failure is returned as an error, but the in-memory
// flag is already updated — the next process restart re-reads whatever is on
// disk. The fsnotify self-event is consumed by the watch loop without
// triggering a redundant reload (state-compare debounce).
func (r *Registry) Set(name FlagName, enabled bool) error {
	if !isKnownFlag(name) {
		return fmt.Errorf("feature: unknown flag %q", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	next := *r.Get()
	switch name {
	case FlagOkfReader:
		next.OkfReader = enabled
	case FlagOkfWriter:
		next.OkfWriter = enabled
	case FlagOkfGraphBFS:
		next.OkfGraphBFS = enabled
	}
	r.cur.Store(&next)

	if r.cfgPath != "" {
		if err := persistFlags(r.cfgPath, &next); err != nil {
			log.Printf("feature: persist %s=%v failed: %v", name, enabled, err)
			return fmt.Errorf("feature: persist %s: %w", name, err)
		}
	}
	log.Printf("feature: %s set to %v", name, enabled)
	return nil
}

// Close stops the fsnotify watcher. Safe to call multiple times.
func (r *Registry) Close() error {
	select {
	case <-r.closed:
		return nil
	default:
	}
	close(r.closed)
	if r.watcher != nil {
		return r.watcher.Close()
	}
	return nil
}

// FlagsList returns the current state of all known flags, sorted by the order
// they were declared in allFlags. Used by the admin List endpoint.
func (r *Registry) FlagsList() []FlagState {
	cur := r.Get()
	out := make([]FlagState, 0, len(allFlags))
	for _, name := range allFlags {
		out = append(out, FlagState{Name: string(name), Enabled: cur.flagEnabled(name)})
	}
	return out
}

// FlagState is the admin API's view of one flag.
type FlagState struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// snapshotFromConfig copies the Features section of cfg into a fresh Flags.
func snapshotFromConfig(cfg *config.Config) *Flags {
	return &Flags{
		OkfReader:   cfg.Features.OkfReader,
		OkfWriter:   cfg.Features.OkfWriter,
		OkfGraphBFS: cfg.Features.OkfGraphBFS,
	}
}

// isKnownFlag guards Set / admin API against typos.
func isKnownFlag(name FlagName) bool {
	for _, f := range allFlags {
		if f == name {
			return true
		}
	}
	return false
}
