package pronunciation

import (
	"sync"
	"time"
)

// Limiter is a small in-memory sliding window per student. It keeps the cost of
// a provider call bounded even when a client retries in a loop. A single API
// instance is assumed; with several instances the effective limit multiplies.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
	now    func() time.Time
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:  limit,
		window: window,
		hits:   map[string][]time.Time{},
		now:    time.Now,
	}
}

// Allow reports whether the caller may make another attempt now.
func (l *Limiter) Allow(studentID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	recent := l.hits[studentID][:0]
	for _, t := range l.hits[studentID] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.limit {
		l.hits[studentID] = recent
		return false
	}
	l.hits[studentID] = append(recent, now)
	return true
}

// Forget drops a student's window when they sign out or are removed.
func (l *Limiter) Forget(studentID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, studentID)
}
