package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gateflow/gateflow/internal/core"
)

// Result describes the rate limit evaluation
type Result struct {
	Allowed   bool
	Remaining int64
	Reset     time.Duration
	Limit     int64
}

// Limiter interface supported by both Redis and local in-memory fallback
type Limiter interface {
	Allow(ctx context.Context, key string, policy *core.RateLimitPolicy) (*Result, error)
}

// MemoryLimiter is a thread-safe token bucket rate limiter for single-instance / fallback
type MemoryLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens     float64
	lastUpdate time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	limiter := &MemoryLimiter{
		buckets: make(map[string]*bucket),
	}
	// Background cleanup of stale buckets every 5 minutes
	go limiter.cleanupLoop()
	return limiter
}

func (m *MemoryLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, b := range m.buckets {
			if now.Sub(b.lastUpdate) > 10*time.Minute {
				delete(m.buckets, k)
			}
		}
		m.mu.Unlock()
	}
}

func (m *MemoryLimiter) Allow(ctx context.Context, key string, policy *core.RateLimitPolicy) (*Result, error) {
	if policy == nil || policy.Limit <= 0 || policy.WindowSec <= 0 {
		return &Result{Allowed: true, Remaining: 999999, Reset: 0, Limit: 999999}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	maxCapacity := float64(policy.Limit + policy.Burst)
	refillRate := float64(policy.Limit) / float64(policy.WindowSec) // tokens per second

	b, exists := m.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     maxCapacity - 1, // deduct 1 for the current request
			lastUpdate: now,
		}
		m.buckets[key] = b
		return &Result{
			Allowed:   true,
			Remaining: int64(b.tokens),
			Reset:     time.Duration(policy.WindowSec) * time.Second,
			Limit:     policy.Limit,
		}, nil
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.tokens += elapsed * refillRate
	if b.tokens > maxCapacity {
		b.tokens = maxCapacity
	}
	b.lastUpdate = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return &Result{
			Allowed:   true,
			Remaining: int64(b.tokens),
			Reset:     time.Duration(float64(policy.Limit-int64(b.tokens))/refillRate) * time.Second,
			Limit:     policy.Limit,
		}, nil
	}

	// Rate limited
	resetSeconds := (1.0 - b.tokens) / refillRate
	if resetSeconds < 1.0 {
		resetSeconds = 1.0
	}

	return &Result{
		Allowed:   false,
		Remaining: 0,
		Reset:     time.Duration(resetSeconds) * time.Second,
		Limit:     policy.Limit,
	}, nil
}

// BuildKey constructs a partitioned, collision-free rate limit key
func BuildKey(policyID, dimension, clientIdentifier string) string {
	return fmt.Sprintf("rl:%s:%s:%s", policyID, dimension, clientIdentifier)
}
