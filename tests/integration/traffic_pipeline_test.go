package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gateflow/gateflow/internal/analytics"
	"github.com/gateflow/gateflow/internal/auth"
	"github.com/gateflow/gateflow/internal/cache"
	"github.com/gateflow/gateflow/internal/core"
	"github.com/gateflow/gateflow/internal/monitoring"
	"github.com/gateflow/gateflow/internal/proxy"
	"github.com/gateflow/gateflow/internal/ratelimit"
	"github.com/gateflow/gateflow/internal/storage"
)

func TestFullTrafficPipeline_Integration(t *testing.T) {
	// 1. Setup real backend upstream HTTP server
	var backendHitCount int64
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&backendHitCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message":"hello from backend","path":"%s","backend_hits":%d}`+"\n", r.URL.Path, atomic.LoadInt64(&backendHitCount))
	}))
	defer upstreamServer.Close()

	// 2. Setup GateFlow Data Plane & Memory Storage
	store := storage.NewMemoryStorage()
	analyticsCol := analytics.NewCollector(1000)
	metricsCol := monitoring.DefaultCollector
	limiter := ratelimit.NewMemoryLimiter()
	cacheStore := cache.NewMemoryStore()

	engine, err := proxy.NewEngine(proxy.Config{
		Storage:         store,
		TrustedProxies:  []string{"127.0.0.1"},
		TrustCloudflare: false,
		Limiter:         limiter,
		CacheStore:      cacheStore,
		Analytics:       analyticsCol,
		Metrics:         metricsCol,
		AllowLocalDev:   true, // allows test server loopback IP
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	gatewayServer := httptest.NewServer(engine)
	defer gatewayServer.Close()

	ctx := context.Background()

	// 3. Register Upstream
	ups := &core.Upstream{
		ID:      "ups_backend",
		Name:    "Test Backend Upstream",
		BaseURL: upstreamServer.URL,
		Active:  true,
		Healthy: true,
		Timeout: 3 * time.Second,
	}
	_ = store.SaveUpstream(ctx, ups)

	// 4. Register Rate Limit Policy (Limit 3 requests per 10s)
	rlPolicy := &core.RateLimitPolicy{
		ID:        "rl_pipeline",
		Limit:     3,
		WindowSec: 10,
		Burst:     0,
		Dimension: "ip",
	}
	_ = store.SaveRateLimit(ctx, rlPolicy)

	// 5. Register Cache Policy (60s TTL)
	cpPolicy := &core.CachePolicy{
		ID:           "cp_pipeline",
		TTLSeconds:   60,
		CacheMethods: []string{"GET"},
	}
	_ = store.SaveCachePolicy(ctx, cpPolicy)

	// 6. Register API Key
	rawKey, apiKeyObj, _ := auth.GenerateAPIKey("admin", "Integration Test Key", []string{"read"}, nil)
	_ = store.SaveAPIKey(ctx, apiKeyObj)

	// 7. Register Route
	route := &core.Route{
		ID:             "rt_test",
		PathPattern:    "/test/service/*",
		Methods:        []string{"GET"},
		UpstreamID:     ups.ID,
		StripPrefix:    "/test/service",
		AuthRequired:   true,
		RequiredScopes: []string{"read"},
		RateLimitID:    rlPolicy.ID,
		CachePolicyID:  cpPolicy.ID,
		Active:         true,
		Priority:       10,
	}
	_ = store.SaveRoute(ctx, route)
	_ = engine.UpdateRoutes([]*core.Route{route})

	client := &http.Client{Timeout: 5 * time.Second}

	// === TEST A: Missing API Key -> Expect 401 Unauthorized ===
	resp, err := client.Get(gatewayServer.URL + "/test/service/hello")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized without API key, got %d", resp.StatusCode)
	}

	// === TEST B: Valid Request -> First call: MISS, hits backend ===
	req, _ := http.NewRequest("GET", gatewayServer.URL+"/test/service/hello", nil)
	req.Header.Set("X-API-Key", rawKey)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, string(body))
	}
	if resp.Header.Get("X-Cache") != "MISS" {
		t.Errorf("Expected X-Cache: MISS on first request, got %s", resp.Header.Get("X-Cache"))
	}
	if atomic.LoadInt64(&backendHitCount) != 1 {
		t.Errorf("Expected backend hit count 1, got %d", atomic.LoadInt64(&backendHitCount))
	}

	// === TEST C: Caching -> Second call: HIT, backend count stays 1 ===
	req2, _ := http.NewRequest("GET", gatewayServer.URL+"/test/service/hello", nil)
	req2.Header.Set("X-API-Key", rawKey)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp2.Header.Get("X-Cache") != "HIT" {
		t.Errorf("Expected X-Cache: HIT on second request, got %s", resp2.Header.Get("X-Cache"))
	}
	if atomic.LoadInt64(&backendHitCount) != 1 {
		t.Errorf("Backend hit count should remain 1 due to cache, got %d", atomic.LoadInt64(&backendHitCount))
	}

	// === TEST D: Rate Limiting -> Exhaust 3 request limit ===
	// Request 3 (allowed)
	req3, _ := http.NewRequest("GET", gatewayServer.URL+"/test/service/hello", nil)
	req3.Header.Set("X-API-Key", rawKey)
	resp3, _ := client.Do(req3)
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected request 3 allowed, got %d", resp3.StatusCode)
	}

	// Request 4 (exceeds limit 3 -> 429 Too Many Requests)
	req4, _ := http.NewRequest("GET", gatewayServer.URL+"/test/service/hello", nil)
	req4.Header.Set("X-API-Key", rawKey)
	resp4, _ := client.Do(req4)
	if resp4.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests on request 4, got %d", resp4.StatusCode)
	}
	if resp4.Header.Get("Retry-After") == "" {
		t.Errorf("Expected Retry-After header on 429 response")
	}

	// === TEST E: API Key Revocation ===
	apiKeyObj.Active = false
	_ = store.SaveAPIKey(ctx, apiKeyObj)

	req5, _ := http.NewRequest("GET", gatewayServer.URL+"/test/service/hello", nil)
	req5.Header.Set("X-API-Key", rawKey)
	resp5, _ := client.Do(req5)
	if resp5.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for revoked key, got %d", resp5.StatusCode)
	}

	// === TEST F: Analytics Overview Verification ===
	overview := analyticsCol.GetOverview(ctx)
	if overview.TotalRequests == 0 {
		t.Errorf("Expected recorded analytics requests, got 0")
	}
	if overview.RateLimitBlocks == 0 {
		t.Errorf("Expected recorded rate limit blocks, got 0")
	}
	if overview.CacheHits == 0 {
		t.Errorf("Expected recorded cache hits, got 0")
	}
}
