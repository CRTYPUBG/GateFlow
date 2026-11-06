package cache

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gateflow/gateflow/internal/core"
)

func TestCache_KeyNormalizationAndSecurity(t *testing.T) {
	policy := &core.CachePolicy{
		ID:           "test_cp",
		TTLSeconds:   60,
		MaxSizeBytes: 1048576,
		CacheMethods: []string{"GET"},
	}

	// 1. Query parameter ordering normalization
	req1, _ := http.NewRequest("GET", "http://example.com/api?b=2&a=1", nil)
	req2, _ := http.NewRequest("GET", "http://example.com/api?a=1&b=2", nil)

	key1 := GenerateCacheKey(req1, policy)
	key2 := GenerateCacheKey(req2, policy)

	if key1 != key2 {
		t.Errorf("Cache key must be identical regardless of query parameter order: %s vs %s", key1, key2)
	}

	// 2. Security: Never cache request with Authorization header
	reqAuth, _ := http.NewRequest("GET", "http://example.com/profile", nil)
	reqAuth.Header.Set("Authorization", "Bearer secret-token")
	if IsCacheableRequest(reqAuth, policy) {
		t.Errorf("Requests with Authorization header must never be cached")
	}

	// 3. Security: Never cache response setting Set-Cookie
	headerWithCookie := http.Header{}
	headerWithCookie.Set("Set-Cookie", "session=123")
	if IsCacheableResponse(http.StatusOK, headerWithCookie, policy) {
		t.Errorf("Responses with Set-Cookie must never be cached")
	}

	// 4. Memory store get and set
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Set(ctx, key1, &Response{
		StatusCode: 200,
		Body:       []byte("cached-payload"),
	}, 10*time.Second)

	res, err := store.Get(ctx, key1)
	if err != nil || res == nil || string(res.Body) != "cached-payload" {
		t.Fatalf("Cache lookup failed: %v", err)
	}
}
