package app

import (
	"sync"
	"time"
)

// Nomes dos eventos emitidos ao frontend.
const (
	EvScanProgress  = "scan:progress"
	EvScanDone      = "scan:done"
	EvCleanProgress = "clean:progress"
	EvCleanDone     = "clean:done"
	EvAIDone        = "ai:done"
)

// Throttle permite no máximo uma passagem por intervalo.
type Throttle struct {
	mu       sync.Mutex
	interval time.Duration
	now      func() time.Time
	last     time.Time
	started  bool
}

// NewThrottle cria um Throttle; now é time.Now em produção.
func NewThrottle(interval time.Duration, now func() time.Time) *Throttle {
	if now == nil {
		now = time.Now
	}
	return &Throttle{interval: interval, now: now}
}

// Allow retorna true se já passou um intervalo desde a última passagem.
func (t *Throttle) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := t.now()
	if t.started && n.Sub(t.last) < t.interval {
		return false
	}
	t.started, t.last = true, n
	return true
}
