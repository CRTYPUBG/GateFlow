package ratelimit

import (
	"context"
	"time"

	"github.com/gateflow/gateflow/internal/core"
	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements sliding window / token bucket using Redis Lua scripts
type RedisLimiter struct {
	client  *redis.Client
	fallback Limiter
}

func NewRedisLimiter(client *redis.Client, fallback Limiter) *RedisLimiter {
	if fallback == nil {
		fallback = NewMemoryLimiter()
	}
	return &RedisLimiter{
		client:   client,
		fallback: fallback,
	}
}

// Token bucket atomic Lua script
const tokenBucketScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local max_capacity = limit + burst
local refill_rate = limit / window

local data = redis.call("HMGET", key, "tokens", "last_update")
local tokens = tonumber(data[1])
local last_update = tonumber(data[2])

if not tokens then
    tokens = max_capacity - 1
    last_update = now
    redis.call("HMSET", key, "tokens", tokens, "last_update", now)
    redis.call("EXPIRE", key, window * 2)
    return {1, math.floor(tokens), window}
end

local elapsed = now - last_update
if elapsed < 0 then
    elapsed = 0
end

tokens = math.min(max_capacity, tokens + (elapsed * refill_rate))
last_update = now

if tokens >= 1 then
    tokens = tokens - 1
    redis.call("HMSET", key, "tokens", tokens, "last_update", now)
    redis.call("EXPIRE", key, window * 2)
    local reset = math.ceil((max_capacity - tokens) / refill_rate)
    return {1, math.floor(tokens), reset}
else
    redis.call("HMSET", key, "tokens", tokens, "last_update", now)
    local reset = math.ceil((1 - tokens) / refill_rate)
    return {0, 0, reset}
end
`

func (r *RedisLimiter) Allow(ctx context.Context, key string, policy *core.RateLimitPolicy) (*Result, error) {
	if r.client == nil {
		return r.fallback.Allow(ctx, key, policy)
	}

	if policy == nil || policy.Limit <= 0 || policy.WindowSec <= 0 {
		return &Result{Allowed: true, Remaining: 999999, Reset: 0, Limit: 999999}, nil
	}

	now := time.Now().Unix()
	res, err := r.client.Eval(ctx, tokenBucketScript, []string{key}, policy.Limit, policy.WindowSec, policy.Burst, now).Result()
	if err != nil {
		// Log redis error and gracefully fallback to local in-memory rate limiter!
		return r.fallback.Allow(ctx, key, policy)
	}

	vals, ok := res.([]interface{})
	if !ok || len(vals) < 3 {
		return r.fallback.Allow(ctx, key, policy)
	}

	allowed := vals[0].(int64) == 1
	remaining := vals[1].(int64)
	resetSeconds := vals[2].(int64)

	return &Result{
		Allowed:   allowed,
		Remaining: remaining,
		Reset:     time.Duration(resetSeconds) * time.Second,
		Limit:     policy.Limit,
	}, nil
}
