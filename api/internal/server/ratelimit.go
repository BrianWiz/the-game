package server

import (
	"math"
	"sync"
	"time"
)

// limiter is a keyed token bucket: each key may make `burst` requests at once and
// regains one every `per`. In-memory, so limits reset when the process restarts.
type limiter struct {
	burst float64
	per   time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(burst int, per time.Duration) *limiter {
	return &limiter{burst: float64(burst), per: per, buckets: map[string]*bucket{}}
}

// allow takes one token for key. When none is left it returns how long to wait.
func (l *limiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 50_000 {
		l.sweep(now)
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()/l.per.Seconds())
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) * float64(l.per))
	return false, wait
}

// sweep drops buckets that have refilled completely; they carry no state.
func (l *limiter) sweep(now time.Time) {
	full := time.Duration(l.burst * float64(l.per))
	for k, b := range l.buckets {
		if now.Sub(b.last) >= full {
			delete(l.buckets, k)
		}
	}
}
