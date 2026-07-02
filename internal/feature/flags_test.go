package feature

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentdisk/agent-disk/config"
)

// freshConfig returns a Config with the Features section set to all-on,
// matching the default state operators get when they don't set features.*
// in config.yaml.
func freshConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Features.OkfReader = true
	cfg.Features.OkfWriter = true
	cfg.Features.OkfGraphBFS = true
	return cfg
}

// writeYAML writes a minimal config.yaml with the given features block. Used
// to exercise the parse + persist + hot-reload paths against a real file
// rather than mocks.
func writeYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return path
}

func TestRegistry_DefaultsOn(t *testing.T) {
	reg, err := NewRegistry(freshConfig(), "")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close()

	for _, name := range allFlags {
		if !reg.IsEnabled(name) {
			t.Errorf("flag %s default = false, want true", name)
		}
	}
}

func TestRegistry_SetFlips(t *testing.T) {
	reg, err := NewRegistry(freshConfig(), "")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close()

	if err := reg.Set(FlagOkfReader, false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if reg.IsEnabled(FlagOkfReader) {
		t.Errorf("after Set false, IsEnabled = true, want false")
	}
	// Other flags untouched.
	if !reg.IsEnabled(FlagOkfWriter) {
		t.Errorf("Set on reader flipped writer")
	}
}

func TestRegistry_SetUnknownFlag(t *testing.T) {
	reg, err := NewRegistry(freshConfig(), "")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close()

	if err := reg.Set(FlagName("nope"), true); err == nil {
		t.Errorf("Set unknown flag returned nil error")
	}
}

func TestRegistry_GetAtomicSnapshot(t *testing.T) {
	// Two reads in a row from the same goroutine must observe a consistent
	// snapshot. The atomic.Pointer swap guarantees no torn reads; this test
	// is mostly a documentation + smoke check.
	reg, _ := NewRegistry(freshConfig(), "")
	defer reg.Close()

	a := reg.Get()
	b := reg.Get()
	if a != b {
		// atomic.Pointer.Load returns the same *Flags pointer when no Set
		// happened between calls — which is the common case in the hot path.
		t.Errorf("Get returned different snapshots without a Set")
	}
}

func TestRegistry_PersistRoundTrip(t *testing.T) {
	// Start with a config.yaml that has features all ON.
	path := writeYAML(t, `
okf:
  enabled: true
features:
  okfReader: true
  okfWriter: true
  okfGraphBFS: true
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	reg, err := NewRegistry(cfg, path)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close()

	// Flip two flags; this should persist to disk.
	if err := reg.Set(FlagOkfReader, false); err != nil {
		t.Fatalf("Set reader: %v", err)
	}
	if err := reg.Set(FlagOkfGraphBFS, false); err != nil {
		t.Fatalf("Set bfs: %v", err)
	}

	// Re-read the file into a fresh config and confirm the values stuck.
	cfg2, err := config.Load(path)
	if err != nil {
		t.Fatalf("re-Load: %v", err)
	}
	if cfg2.Features.OkfReader {
		t.Errorf("persisted okfReader = true, want false")
	}
	if !cfg2.Features.OkfWriter {
		t.Errorf("persisted okfWriter = false, want true (untouched)")
	}
	if cfg2.Features.OkfGraphBFS {
		t.Errorf("persisted okfGraphBFS = true, want false")
	}

	// Other keys (okf.enabled) must survive the persist round-trip.
	if !cfg2.Okf.Enabled {
		t.Errorf("okf.enabled got clobbered by features persist")
	}
}

func TestRegistry_HotReloadFromDisk(t *testing.T) {
	// Initial config: all flags ON.
	path := writeYAML(t, `
okf:
  enabled: true
features:
  okfReader: true
  okfWriter: true
  okfGraphBFS: true
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	reg, err := NewRegistry(cfg, path)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer reg.Close()

	// Externally edit the file (operator vi/echo), simulating out-of-process
	// config change. The watcher should pick it up after debounce.
	newBody := `
okf:
  enabled: true
features:
  okfReader: false
  okfWriter: false
  okfGraphBFS: false
`
	if err := os.WriteFile(path, []byte(newBody), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	// Wait for debounce + fsnotify propagation. 1s is plenty on a fast machine.
	deadline := waitFor(t, 2_000_000_000, func() bool {
		return !reg.IsEnabled(FlagOkfReader) && !reg.IsEnabled(FlagOkfWriter) && !reg.IsEnabled(FlagOkfGraphBFS)
	})
	if !deadline {
		t.Fatalf("hot-reload did not propagate within timeout")
	}
}

func TestRegistry_FlagsListOrder(t *testing.T) {
	reg, _ := NewRegistry(freshConfig(), "")
	defer reg.Close()

	got := reg.FlagsList()
	if len(got) != len(allFlags) {
		t.Fatalf("FlagsList len = %d, want %d", len(got), len(allFlags))
	}
	for i, want := range allFlags {
		if got[i].Name != string(want) {
			t.Errorf("FlagsList[%d].Name = %q, want %q", i, got[i].Name, want)
		}
		if !got[i].Enabled {
			t.Errorf("FlagsList[%d].Enabled = false, want true (default)", i)
		}
	}
}

// waitFor polls cond every 25ms until it returns true or the timeout elapses.
// Returns true if cond succeeded, false on timeout. Avoids time.Sleep in
// tests — keeps them fast on quick systems and accurate on slow ones.
func waitFor(t *testing.T, timeoutNs int64, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().UnixNano() + timeoutNs
	for time.Now().UnixNano() < deadline {
		if cond() {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return cond()
}
