package admin

import (
	"strings"
	"sync"
	"time"
)

// loginLimiter throttles failed logins per account. Keying on the username
// rather than the client IP avoids spoofable X-Forwarded-For headers.
type loginLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	attempts map[string]*loginAttempt
}

type loginAttempt struct {
	count   int
	resetAt time.Time
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		max:      max,
		window:   window,
		attempts: make(map[string]*loginAttempt),
	}
}

// Allow reports whether another attempt may be made.
func (l *loginLimiter) Allow(username string) bool {
	key := strings.ToLower(strings.TrimSpace(username))

	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[key]
	if !ok {
		return true
	}
	if time.Now().After(a.resetAt) {
		delete(l.attempts, key)
		return true
	}
	return a.count < l.max
}

// Fail records a failed attempt.
func (l *loginLimiter) Fail(username string) {
	key := strings.ToLower(strings.TrimSpace(username))

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	a, ok := l.attempts[key]
	if !ok || now.After(a.resetAt) {
		l.attempts[key] = &loginAttempt{count: 1, resetAt: now.Add(l.window)}
		return
	}
	a.count++
}

// Reset clears the failure count after a successful login.
func (l *loginLimiter) Reset(username string) {
	key := strings.ToLower(strings.TrimSpace(username))

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}
