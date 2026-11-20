// Package ratelimit provides a small in-memory fixed-window limiter.
// It is per-process; run behind a shared store if the API scales out.
package ratelimit

import (
	"sync"
	"time"
)

type window struct {
	start time.Time
	count int
}

// Limiter allows up to limit hits per key within each window.
type Limiter struct {
	limit     int
	window    time.Duration
	now       func() time.Time
	mu        sync.Mutex
	entries   map[string]*window
	lastSweep time.Time
}

// New returns a limiter. limit <= 0 disables limiting.
func New(limit int, w time.Duration) *Limiter {
	return &Limiter{limit: limit, window: w, now: time.Now, entries: map[string]*window{}}
}

// WithClock overrides the clock (tests).
func (l *Limiter) WithClock(now func() time.Time) *Limiter {
	l.now = now
	return l
}

// Allow records a hit for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	w, ok := l.entries[key]
	if !ok || now.Sub(w.start) >= l.window {
		l.entries[key] = &window{start: now, count: 1}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

// sweep drops expired windows at most once per window to bound memory.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	for k, w := range l.entries {
		if now.Sub(w.start) >= l.window {
			delete(l.entries, k)
		}
	}
	l.lastSweep = now
}
