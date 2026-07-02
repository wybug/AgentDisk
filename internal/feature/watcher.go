package feature

import (
	"log"
	"time"

	"github.com/agentdisk/agent-disk/config"
	"github.com/fsnotify/fsnotify"
)

// debounceWait coalesces rapid file mutations (some editors write multiple
// times per save: tmp file + atomic rename). 200ms is long enough to cover
// that without delaying legitimate admin updates perceptibly.
const debounceWait = 200 * time.Millisecond

// watchLoop consumes fsnotify events. On a Write event we wait debounceWait
// before reloading, and any subsequent event in that window resets the timer.
// The reload reads config.yaml afresh (not via cached viper) so we pick up
// the operator's exact bytes.
func (r *Registry) watchLoop() {
	var debounce *time.Timer
	for {
		select {
		case <-r.closed:
			return
		case ev, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(debounceWait, r.reloadFromDisk)
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("feature: fsnotify error: %v", err)
		}
	}
}

// reloadFromDisk reads config.yaml, builds a fresh Flags, and atomically
// swaps it in. On error the previous flags stay in effect — a corrupt
// config.yaml should never silently flip flags.
func (r *Registry) reloadFromDisk() {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, err := config.Load(r.cfgPath)
	if err != nil {
		log.Printf("feature: reload parse failed (keeping current flags): %v", err)
		return
	}
	next := snapshotFromConfig(cfg)
	prev := r.Get()
	// Self-event from our own Set() persist lands here. State-compare skips
	// the swap + log line so we don't spam the log on every admin PATCH.
	if *next == *prev {
		return
	}
	r.cur.Store(next)
	log.Printf("feature: hot-reloaded from disk: reader=%v writer=%v bfs=%v",
		next.OkfReader, next.OkfWriter, next.OkfGraphBFS)
}
