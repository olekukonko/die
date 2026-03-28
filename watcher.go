package die

import (
	"context"
	"time"
)

// Watcher continuously monitors and kills processes
type Watcher struct {
	killer   *Killer
	interval time.Duration
	onKill   func(*KillResult)
}

// WatcherConfig holds watcher configuration
type WatcherConfig struct {
	Interval time.Duration
	OnKill   func(*KillResult)
}

// NewWatcher creates a new watcher
func NewWatcher(killer *Killer, config WatcherConfig) *Watcher {
	return &Watcher{
		killer:   killer,
		interval: config.Interval,
		onKill:   config.OnKill,
	}
}

// Run starts the watcher loop
func (w *Watcher) Run(ctx context.Context, target string, mode TargetMode) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately
	w.execute(ctx, target, mode)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.execute(ctx, target, mode)
		}
	}
}

func (w *Watcher) execute(ctx context.Context, target string, mode TargetMode) {
	result, err := w.killer.Kill(ctx, target, mode)
	if err == nil && w.onKill != nil {
		w.onKill(result)
	}
}
