package handler

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	l := newRateLimiter(2, time.Minute)
	if !l.Allow("a") {
		t.Fatal("first request should be allowed")
	}
	l.Record("a")
	l.Record("a")
	if l.Allow("a") {
		t.Fatal("third request should be blocked")
	}
	if !l.Allow("b") {
		t.Fatal("different key should be allowed")
	}

	short := newRateLimiter(1, time.Millisecond)
	short.Record("x")
	time.Sleep(2 * time.Millisecond)
	if !short.Allow("x") {
		t.Fatal("window should reset")
	}
}
