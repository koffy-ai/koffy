package aigateway

import (
	"testing"
	"time"
)

func TestFixedWindowLimiter(t *testing.T) {
	limiter := newFixedWindowLimiter()
	now := time.Date(2026, 5, 28, 10, 0, 10, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		ok, retry := limiter.Allow("app:demo", 2)
		if !ok || retry != 0 {
			t.Fatalf("request %d allowed=%v retry=%s", i, ok, retry)
		}
	}
	ok, retry := limiter.Allow("app:demo", 2)
	if ok {
		t.Fatal("third request should be limited")
	}
	if retry <= 0 {
		t.Fatalf("retry = %s", retry)
	}

	now = now.Add(time.Minute)
	ok, retry = limiter.Allow("app:demo", 2)
	if !ok || retry != 0 {
		t.Fatalf("new window allowed=%v retry=%s", ok, retry)
	}
}

func TestFixedWindowLimiterDisabled(t *testing.T) {
	limiter := newFixedWindowLimiter()
	for i := 0; i < 10; i++ {
		ok, retry := limiter.Allow("app:demo", 0)
		if !ok || retry != 0 {
			t.Fatalf("disabled limiter allowed=%v retry=%s", ok, retry)
		}
	}
}
