package aigateway

import (
	"sync"
	"time"
)

type fixedWindowLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	windows map[string]rateWindow
}

type rateWindow struct {
	start time.Time
	count int
}

func newFixedWindowLimiter() *fixedWindowLimiter {
	return &fixedWindowLimiter{
		now:     time.Now,
		windows: make(map[string]rateWindow),
	}
}

func (l *fixedWindowLimiter) Allow(key string, limit int) (bool, time.Duration) {
	if limit <= 0 {
		return true, 0
	}

	now := l.now()
	windowStart := now.Truncate(time.Minute)

	l.mu.Lock()
	defer l.mu.Unlock()

	item := l.windows[key]
	if item.start.IsZero() || !item.start.Equal(windowStart) {
		l.windows[key] = rateWindow{start: windowStart, count: 1}
		l.cleanupLocked(windowStart)
		return true, 0
	}
	if item.count >= limit {
		return false, windowStart.Add(time.Minute).Sub(now)
	}
	item.count++
	l.windows[key] = item
	return true, 0
}

func (l *fixedWindowLimiter) cleanupLocked(currentWindow time.Time) {
	cutoff := currentWindow.Add(-5 * time.Minute)
	for key, item := range l.windows {
		if item.start.Before(cutoff) {
			delete(l.windows, key)
		}
	}
}
