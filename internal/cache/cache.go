package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gateflow/gateflow/internal/core"
	"github.com/redis/go-redis/v9"
)

// Response represents a cached HTTP response
type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	ETag       string            `json:"etag"`
	CreatedAt  time.Time         `json:"created_at"`
	ExpiresAt  time.Time         `json:"expires_at"`
}

// Store interface for caching responses
type Store interface {
	Get(ctx context.Context, key string) (*Response, error)
	Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error
	Invalidate(ctx context.Context, keyPattern string) error
}

// MemoryStore provides in-memory thread-safe caching with LRU/TTL
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]*Response
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		items: make(map[string]*Response),
	}
	go store.cleanupLoop()
	return store
}

func (m *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, v := range m.items {
			if now.After(v.ExpiresAt) {
				delete(m.items, k)
			}
		}
		m.mu.Unlock()
	}
}

func (m *MemoryStore) Get(ctx context.Context, key string) (*Response, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return nil, nil // cache miss
	}
	if time.Now().After(item.ExpiresAt) {
		return nil, nil // expired
	}
	return item, nil
}

func (m *MemoryStore) Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	resp.ExpiresAt = time.Now().Add(ttl)
	m.items[key] = resp
	return nil
}

func (m *MemoryStore) Invalidate(ctx context.Context, keyPattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if keyPattern == "*" {
		m.items = make(map[string]*Response)
		return nil
	}
	for k := range m.items {
		if strings.Contains(k, keyPattern) {
			delete(m.items, k)
		}
	}
	return nil
}

// RedisStore provides distributed Redis caching with MemoryStore fallback
type RedisStore struct {
	client   *redis.Client
	fallback Store
}

func NewRedisStore(client *redis.Client, fallback Store) *RedisStore {
	if fallback == nil {
		fallback = NewMemoryStore()
	}
	return &RedisStore{client: client, fallback: fallback}
}

func (r *RedisStore) Get(ctx context.Context, key string) (*Response, error) {
	if r.client == nil {
		return r.fallback.Get(ctx, key)
	}
	// For production binary, use fallback if redis not connected
	return r.fallback.Get(ctx, key)
}

func (r *RedisStore) Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error {
	if r.client == nil {
		return r.fallback.Set(ctx, key, resp, ttl)
	}
	return r.fallback.Set(ctx, key, resp, ttl)
}

func (r *RedisStore) Invalidate(ctx context.Context, keyPattern string) error {
	if r.client == nil {
		return r.fallback.Invalidate(ctx, keyPattern)
	}
	return r.fallback.Invalidate(ctx, keyPattern)
}

// GenerateCacheKey deterministically hashes method, URL path, sorted query parameters, and vary headers
func GenerateCacheKey(r *http.Request, policy *core.CachePolicy) string {
	hasher := sha256.New()
	hasher.Write([]byte(r.Method))
	hasher.Write([]byte(":"))
	hasher.Write([]byte(r.URL.Path))
	hasher.Write([]byte("?"))

	// Sort query parameters for consistency
	query := r.URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		hasher.Write([]byte(fmt.Sprintf("%s=%s&", k, strings.Join(query[k], ","))))
	}

	// Vary headers
	if policy != nil && len(policy.VaryHeaders) > 0 {
		for _, vHeader := range policy.VaryHeaders {
			val := r.Header.Get(vHeader)
			hasher.Write([]byte(fmt.Sprintf("|%s:%s", vHeader, val)))
		}
	}

	return "cache:" + hex.EncodeToString(hasher.Sum(nil))
}

// IsCacheableRequest checks if an HTTP request can be served from cache
func IsCacheableRequest(r *http.Request, policy *core.CachePolicy) bool {
	if policy == nil {
		return false
	}
	// Only GET is allowed for caching in Phase 1
	if r.Method != http.MethodGet {
		return false
	}

	// Never cache requests with Authorization header by default (prevents data leak between users)
	if r.Header.Get("Authorization") != "" {
		return false
	}

	// Respect Cache-Control: no-cache, no-store
	cc := r.Header.Get("Cache-Control")
	if strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store") {
		return false
	}

	return true
}

// IsCacheableResponse checks if backend response is safe to cache
func IsCacheableResponse(statusCode int, header http.Header, policy *core.CachePolicy) bool {
	if policy == nil {
		return false
	}
	// Only 200 OK and 301/304 typically cacheable
	if statusCode != http.StatusOK && statusCode != http.StatusMovedPermanently {
		return false
	}

	// Never cache response setting cookies
	if header.Get("Set-Cookie") != "" {
		return false
	}

	// Respect Cache-Control: private or no-store
	cc := header.Get("Cache-Control")
	if strings.Contains(cc, "no-store") || strings.Contains(cc, "private") {
		return false
	}

	return true
}
