package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(2, time.Minute).WithClock(func() time.Time { return now })
	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("first two should pass")
	}
	if l.Allow("k") {
		t.Fatal("third should be limited")
	}
	if !l.Allow("other") {
		t.Fatal("keys must be independent")
	}
	now = now.Add(time.Minute)
	if !l.Allow("k") {
		t.Fatal("window should reset")
	}
}

func TestLimiterDisabled(t *testing.T) {
	l := New(0, time.Minute)
	for range 100 {
		if !l.Allow("k") {
			t.Fatal("disabled limiter blocked")
		}
	}
}
