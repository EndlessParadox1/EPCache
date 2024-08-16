// Package ratelimit implements a token bucket used for request rate limiting.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

type TokenBucket struct {
	tokens   int
	rate     int
	capacity int
	mu       sync.Mutex
	clsCh    chan struct{}
}

func New(rate, capacity int) *TokenBucket {
	tb := &TokenBucket{
		tokens:   capacity,
		rate:     rate,
		capacity: capacity,
		clsCh:    make(chan struct{}),
	}
	go tb.refill()
	return tb
}

func (tb *TokenBucket) refill() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tb.mu.Lock()
			tb.tokens += tb.rate
			if tb.tokens > tb.capacity {
				tb.tokens = tb.capacity
			}
			tb.mu.Unlock()
		case <-tb.clsCh:
			return
		}
	}
}

func (tb *TokenBucket) Close() {
	close(tb.clsCh)
}

func (tb *TokenBucket) Allow(tokens int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if tb.tokens >= tokens {
		tb.tokens -= tokens
		return true
	}
	return false
}

func (tb *TokenBucket) Wait(ctx context.Context, tokens int) error {
	for {
		tb.mu.Lock()
		if tb.tokens >= tokens {
			tb.tokens -= tokens
			tb.mu.Unlock()
			return nil
		}
		tb.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
