package proxy

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sync/atomic"
	"time"

	"github.com/gateflow/gateflow/internal/accesscontrol"
	"github.com/gateflow/gateflow/internal/analytics"
	"github.com/gateflow/gateflow/internal/auth"
	"github.com/gateflow/gateflow/internal/cache"
	"github.com/gateflow/gateflow/internal/core"
	"github.com/gateflow/gateflow/internal/loadbalancer"
	"github.com/gateflow/gateflow/internal/monitoring"
	"github.com/gateflow/gateflow/internal/ratelimit"
	"github.com/gateflow/gateflow/internal/routing"
	"github.com/gateflow/gateflow/internal/storage"
	"github.com/gateflow/gateflow/internal/upstream"
)

// Engine is the core GateFlow data plane handling and proxying live HTTP traffic
type Engine struct {
	router        *routing.Router
	storage       storage.Storage
	proxyManager  *accesscontrol.TrustedProxyManager
	ipChecker     *accesscontrol.IPChecker
	authValidator *auth.Validator
	limiter       ratelimit.Limiter
	cacheStore    cache.Store
	lbRoundRobin  loadbalancer.Balancer
	lbLeastConn   loadbalancer.Balancer
	ssrfGuard     *upstream.SSRFValidator
	analytics     *analytics.Collector
	metrics       *monitoring.MetricsCollector
	transport     *http.Transport
}

// Config for GateFlow Gateway engine
type Config struct {
	Storage         storage.Storage
	TrustedProxies  []string
	TrustCloudflare bool
	Limiter         ratelimit.Limiter
	CacheStore      cache.Store
	Analytics       *analytics.Collector
	Metrics         *monitoring.MetricsCollector
	AllowLocalDev   bool // Allows localhost/private upstreams during local testing
}

func NewEngine(cfg Config) (*Engine, error) {
	pm, err := accesscontrol.NewTrustedProxyManager(cfg.TrustedProxies, cfg.TrustCloudflare)
	if err != nil {
		return nil, fmt.Errorf("failed to init proxy manager: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if cfg.Limiter == nil {
		cfg.Limiter = ratelimit.NewMemoryLimiter()
	}
	if cfg.CacheStore == nil {
		cfg.CacheStore = cache.NewMemoryStore()
	}
	if cfg.Analytics == nil {
		cfg.Analytics = analytics.NewCollector(10000)
	}
	if cfg.Metrics == nil {
		cfg.Metrics = monitoring.DefaultCollector
	}

	return &Engine{
		router:        routing.NewRouter(),
		storage:       cfg.Storage,
		proxyManager:  pm,
		ipChecker:     accesscontrol.NewIPChecker(),
		authValidator: auth.NewValidator(),
		limiter:       cfg.Limiter,
		cacheStore:    cfg.CacheStore,
		lbRoundRobin:  loadbalancer.NewRoundRobinBalancer(),
		lbLeastConn:   loadbalancer.NewLeastConnectionsBalancer(),
		ssrfGuard:     upstream.NewSSRFValidator(cfg.AllowLocalDev),
		analytics:     cfg.Analytics,
		metrics:       cfg.Metrics,
		transport:     transport,
	}, nil
}

// UpdateRoutes dynamically reloads routes
func (e *Engine) UpdateRoutes(routes []*core.Route) error {
	return e.router.SetRoutes(routes)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// 1. Generate Request ID
	reqIDBytes := make([]byte, 8)
	rand.Read(reqIDBytes)
	reqID := "req_" + hex.EncodeToString(reqIDBytes)
	w.Header().Set("X-Request-ID", reqID)

	// 2. Resolve Client IP
	clientIP, err := e.proxyManager.GetClientIP(r)
	if err != nil {
		e.respondError(w, http.StatusBadRequest, "Invalid client connection", reqID)
		return
	}

	// 3. Match Route
	match, status, err := e.router.Match(r.URL.Path, r.Method)
	if err != nil {
		e.respondError(w, status, err.Error(), reqID)
		e.recordAccess(reqID, "", "", r.Method, r.URL.Path, status, time.Since(startTime), clientIP.String(), "", "BYPASS", false, err.Error())
		return
	}
	route := match.Route

	// 4. IP Access Control
	if route.IPPolicyID != "" {
		policy, err := e.storage.GetIPPolicy(r.Context(), route.IPPolicyID)
		if err == nil && policy != nil {
			allowed, err := e.ipChecker.Check(policy, clientIP)
			if err != nil || !allowed {
				e.respondError(w, http.StatusForbidden, "Access forbidden by IP policy", reqID)
				e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, http.StatusForbidden, time.Since(startTime), clientIP.String(), "", "BYPASS", false, "IP blocked")
				return
			}
		}
	}

	// 5. Authentication & Scopes
	var apiKeyID string
	if route.AuthRequired {
		rawKey := auth.ExtractAPIKey(r)
		if rawKey == "" {
			e.respondError(w, http.StatusUnauthorized, "Missing API key credentials", reqID)
			e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, http.StatusUnauthorized, time.Since(startTime), clientIP.String(), "", "BYPASS", false, "Missing API key")
			return
		}

		keyPrefix := rawKey
		if len(rawKey) >= 16 {
			keyPrefix = rawKey[:16]
		}

		keyEntity, err := e.storage.GetAPIKeyByPrefix(r.Context(), keyPrefix)
		if err != nil {
			e.respondError(w, http.StatusUnauthorized, "Invalid API key", reqID)
			e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, http.StatusUnauthorized, time.Since(startTime), clientIP.String(), "", "BYPASS", false, "Invalid API key")
			return
		}

		validatedKey, err := e.authValidator.Validate(rawKey, keyEntity, route.RequiredScopes)
		if err != nil {
			status := http.StatusUnauthorized
			if fmt.Sprintf("%v", err) != "invalid API key" && fmt.Sprintf("%v", err) != "API key is revoked" && fmt.Sprintf("%v", err) != "API key has expired" {
				status = http.StatusForbidden
			}
			e.respondError(w, status, err.Error(), reqID)
			e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, status, time.Since(startTime), clientIP.String(), keyEntity.ID, "BYPASS", false, err.Error())
			return
		}
		apiKeyID = validatedKey.ID
	}

	// 6. Rate Limiting
	if route.RateLimitID != "" {
		rlPolicy, err := e.storage.GetRateLimit(r.Context(), route.RateLimitID)
		if err == nil && rlPolicy != nil {
			clientDim := clientIP.String()
			if rlPolicy.Dimension == "api_key" && apiKeyID != "" {
				clientDim = apiKeyID
			} else if rlPolicy.Dimension == "route" {
				clientDim = route.ID
			}

			rateKey := ratelimit.BuildKey(rlPolicy.ID, rlPolicy.Dimension, clientDim)
			result, err := e.limiter.Allow(r.Context(), rateKey, rlPolicy)
			if err == nil && !result.Allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", result.Reset.Seconds()))
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				e.respondError(w, http.StatusTooManyRequests, "Rate limit exceeded", reqID)
				e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, http.StatusTooManyRequests, time.Since(startTime), clientIP.String(), apiKeyID, "BYPASS", true, "Rate limit exceeded")
				return
			}
		}
	}

	// 7. Caching (Lookup)
	var cachePolicy *core.CachePolicy
	var cacheKey string
	if route.CachePolicyID != "" {
		cp, err := e.storage.GetCachePolicy(r.Context(), route.CachePolicyID)
		if err == nil && cp != nil {
			cachePolicy = cp
			if cache.IsCacheableRequest(r, cachePolicy) {
				cacheKey = cache.GenerateCacheKey(r, cachePolicy)
				cachedResp, err := e.cacheStore.Get(r.Context(), cacheKey)
				if err == nil && cachedResp != nil {
					// Cache HIT!
					w.Header().Set("X-Cache", "HIT")
					for k, v := range cachedResp.Headers {
						w.Header().Set(k, v)
					}
					if cachedResp.ETag != "" {
						w.Header().Set("ETag", cachedResp.ETag)
					}
					w.WriteHeader(cachedResp.StatusCode)
					w.Write(cachedResp.Body)

					e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, cachedResp.StatusCode, time.Since(startTime), clientIP.String(), apiKeyID, "HIT", false, "")
					return
				}
			}
		}
	}

	// 8. Upstream Selection & Load Balancing
	upstreamObj, err := e.storage.GetUpstream(r.Context(), route.UpstreamID)
	if err != nil || upstreamObj == nil || !upstreamObj.Active {
		e.respondError(w, http.StatusServiceUnavailable, "No active upstream configured for this route", reqID)
		e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, http.StatusServiceUnavailable, time.Since(startTime), clientIP.String(), apiKeyID, "BYPASS", false, "Upstream unavailable")
		return
	}

	parsedUpstreamURL, err := e.ssrfGuard.ValidateTargetURL(upstreamObj.BaseURL)
	if err != nil {
		e.respondError(w, http.StatusBadGateway, fmt.Sprintf("Upstream validation rejected: %v", err), reqID)
		e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, http.StatusBadGateway, time.Since(startTime), clientIP.String(), apiKeyID, "BYPASS", false, "SSRF validation failed")
		return
	}

	// 9. Reverse Proxy Execution
	atomic.AddInt64(&upstreamObj.ActiveConns, 1)
	defer atomic.AddInt64(&upstreamObj.ActiveConns, -1)

	proxy := &httputil.ReverseProxy{
		Transport: e.transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(parsedUpstreamURL)
			pr.Out.URL.Path = match.RewrittenURL
			pr.Out.Host = parsedUpstreamURL.Host
			pr.SetXForwarded()
			pr.Out.Header.Set("X-GateFlow-Request-ID", reqID)
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
			e.respondError(rw, http.StatusBadGateway, fmt.Sprintf("Upstream connection error: %v", proxyErr), reqID)
		},
	}

	recorder := &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}

	w.Header().Set("X-Cache", "MISS")
	proxy.ServeHTTP(recorder, r)

	// 10. Cache Save if eligible
	if cachePolicy != nil && cacheKey != "" && cache.IsCacheableResponse(recorder.statusCode, recorder.Header(), cachePolicy) {
		headersMap := make(map[string]string)
		for k, vals := range recorder.Header() {
			if len(vals) > 0 {
				headersMap[k] = vals[0]
			}
		}
		ttl := time.Duration(cachePolicy.TTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = 60 * time.Second
		}
		_ = e.cacheStore.Set(r.Context(), cacheKey, &cache.Response{
			StatusCode: recorder.statusCode,
			Headers:    headersMap,
			Body:       recorder.body.Bytes(),
			CreatedAt:  time.Now(),
		}, ttl)
	}

	// 11. Record Metrics & Access Log
	e.recordAccess(reqID, route.ID, route.UpstreamID, r.Method, r.URL.Path, recorder.statusCode, time.Since(startTime), clientIP.String(), apiKeyID, "MISS", false, "")
}

func (e *Engine) respondError(w http.ResponseWriter, status int, message, reqID string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%d,"message":%q,"request_id":%q}}`+"\n", status, message, reqID)
}

func (e *Engine) recordAccess(reqID, routeID, upstreamID, method, path string, status int, duration time.Duration, clientIP, apiKeyID, cacheStatus string, rateLimited bool, errMsg string) {
	log := &core.RequestLog{
		ID:          reqID,
		RequestID:   reqID,
		Timestamp:   time.Now().UTC(),
		RouteID:     routeID,
		UpstreamID:  upstreamID,
		Method:      method,
		Path:        path,
		StatusCode:  status,
		Duration:    duration,
		ClientIP:    clientIP,
		APIKeyID:    apiKeyID,
		CacheStatus: cacheStatus,
		RateLimited: rateLimited,
		ErrorMsg:    errMsg,
	}

	e.analytics.Record(log)
	e.metrics.RecordRequest(status, cacheStatus == "HIT", rateLimited)
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	headers    http.Header
}

func (r *responseRecorder) Header() http.Header {
	if r.headers == nil {
		r.headers = make(http.Header)
	}
	return r.headers
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	// Copy headers to underlying ResponseWriter
	for k, vv := range r.headers {
		for _, v := range vv {
			r.ResponseWriter.Header().Add(k, v)
		}
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
