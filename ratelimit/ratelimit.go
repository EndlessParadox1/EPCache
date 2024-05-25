// Package ratelimit implements a token bucket used for request rate limiting.
package ratelimit

import (
	"context"
	"errors"
	"sync"
	"time"
)

// RateLimiter adopts a non-goroutine approach.
type RateLimiter struct {
	tokens     int
	rate       int
	capacity   int
	mu         sync.Mutex
	lastRefill time.Time
}

func NewRateLimit(rate, capacity int) *RateLimiter {
	return &RateLimiter{
		tokens:     capacity,
		rate:       rate,
		capacity:   capacity,
		lastRefill: time.Now(),
	}
}
func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += int(elapsed * float64(r.rate))
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}
	r.lastRefill = now
}

func (r *RateLimiter) Wait(ctx context.Context, tokens int) error {
	for {
		r.mu.Lock()
		if r.tokens >= tokens {
			r.tokens -= tokens
			r.mu.Unlock()
			return nil
		}
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return errors.New("wait for tokens timeout")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (r *RateLimiter) Allow(tokens int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	if r.tokens >= tokens {
		r.tokens -= tokens
		return true
	}
	return false
}
