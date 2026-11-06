package ratelimit

import (
	"context"
	"sync"
	"testing"

	"github.com/gateflow/gateflow/internal/core"
)

func TestMemoryLimiter_BurstAndThrottle(t *testing.T) {
	limiter := NewMemoryLimiter()

	policy := &core.RateLimitPolicy{
		ID:        "test_rl",
		Limit:     5,
		WindowSec: 1,
		Burst:     0,
	}

	ctx := context.Background()
	key := "test_ip_1"

	// 5 requests allowed
	for i := 0; i < 5; i++ {
		res, err := limiter.Allow(ctx, key, policy)
		if err != nil || !res.Allowed {
			t.Fatalf("Request %d expected allowed, got rejected: %v", i+1, err)
		}
	}

	// 6th request rejected
	res, err := limiter.Allow(ctx, key, policy)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Allowed {
		t.Errorf("Expected 6th request to be blocked by rate limiter")
	}
}

func TestMemoryLimiter_Concurrency(t *testing.T) {
	limiter := NewMemoryLimiter()
	policy := &core.RateLimitPolicy{
		ID:        "conc_rl",
		Limit:     50,
		WindowSec: 10,
		Burst:     0,
	}

	var wg sync.WaitGroup
	var allowedCount int64
	var blockedCount int64
	var mu sync.Mutex

	totalRequests := 100
	ctx := context.Background()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := limiter.Allow(ctx, "conc_key", policy)
			mu.Lock()
			if res.Allowed {
				allowedCount++
			} else {
				blockedCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	if allowedCount != 50 {
		t.Errorf("Expected exactly 50 allowed requests, got %d", allowedCount)
	}
	if blockedCount != 50 {
		t.Errorf("Expected exactly 50 blocked requests, got %d", blockedCount)
	}
}
