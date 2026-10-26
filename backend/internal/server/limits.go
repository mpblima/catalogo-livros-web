package server

import (
	"sync"
	"time"
)

type bucket struct {
	Started time.Time
	Count   int
}
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]bucket
}

func (l *rateLimiter) allow(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.buckets) >= 10000 {
		for key, b := range l.buckets {
			if now.Sub(b.Started) >= time.Minute {
				delete(l.buckets, key)
			}
		}
	}
	b, exists := l.buckets[id]
	if !exists && len(l.buckets) >= 10000 {
		return false
	}
	if now.Sub(b.Started) >= time.Minute {
		b = bucket{Started: now}
	}
	if b.Count >= 120 {
		return false
	}
	b.Count++
	l.buckets[id] = b
	return true
}
