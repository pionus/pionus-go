package handler

import (
	"sync"
	"time"
)

// rateLimiter is a small in-memory fixed-window limiter used to slow comment
// floods. It is per process; a multi-instance deployment needs a shared store.
type rateLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*rateEntry
}

type rateEntry struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, entries: make(map[string]*rateEntry)}
}

func (l *rateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok || time.Now().After(entry.resetAt) {
		return true
	}
	return entry.count < l.max
}

func (l *rateLimiter) Record(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, ok := l.entries[key]
	if !ok || now.After(entry.resetAt) {
		entry = &rateEntry{resetAt: now.Add(l.window)}
		l.entries[key] = entry
	}
	entry.count++

	// Avoid unbounded growth from spoofed/varied keys.
	if len(l.entries) > 4096 {
		for k, v := range l.entries {
			if now.After(v.resetAt) {
				delete(l.entries, k)
			}
		}
	}
}
