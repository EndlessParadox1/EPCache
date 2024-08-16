package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAllow(t *testing.T) {
	limiter := New(2, 10)
	var allows atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			if limiter.Allow(1) {
				allows.Add(1)
			}
			wg.Done()
		}()
		time.Sleep(50 * time.Millisecond)
	}
	wg.Wait()
	if got := allows.Load(); got != 12 {
		t.Errorf("allows = %d; want 12", got)
	}
}

func TestWait(t *testing.T) {
	limiter := New(2, 10)
	var allows atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			if err := limiter.Wait(context.Background(), 1); err == nil {
				allows.Add(1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if got := allows.Load(); got != 20 {
		t.Errorf("allows = %d; want 20", got)
	}
}

func TestWaitTimeout(t *testing.T) {
	limiter := New(2, 10)
	var allows atomic.Int32
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for range 20 {
		wg.Add(1)
		go func() {
			if err := limiter.Wait(ctx, 1); err == nil {
				allows.Add(1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if got := allows.Load(); got != 12 {
		t.Errorf("allows = %d; want 20", got)
	}
}
