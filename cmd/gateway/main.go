package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gateflow/gateflow/internal/analytics"
	"github.com/gateflow/gateflow/internal/cache"
	"github.com/gateflow/gateflow/internal/config"
	"github.com/gateflow/gateflow/internal/controlplane"
	"github.com/gateflow/gateflow/internal/core"
	"github.com/gateflow/gateflow/internal/monitoring"
	"github.com/gateflow/gateflow/internal/proxy"
	"github.com/gateflow/gateflow/internal/ratelimit"
	"github.com/gateflow/gateflow/internal/storage"
	"github.com/gateflow/gateflow/internal/upstream"
)

func main() {
	cfg := config.LoadConfig()
	log.Printf("[GateFlow] Initializing GateFlow API Gateway (Data Plane & Control Plane)...")

	// Storage setup (Memory storage with initial seed for instant zero-dependency execution)
	store := storage.NewMemoryStorage()
	analyticsCol := analytics.NewCollector(10000)
	metricsCol := monitoring.DefaultCollector
	limiter := ratelimit.NewMemoryLimiter()
	cacheStore := cache.NewMemoryStore()

	// Seed demo upstreams and routes so gateway is functional immediately
	seedDemoData(store)

	engine, err := proxy.NewEngine(proxy.Config{
		Storage:         store,
		TrustedProxies:  cfg.TrustedProxies,
		TrustCloudflare: cfg.TrustCloudflare,
		Limiter:         limiter,
		CacheStore:      cacheStore,
		Analytics:       analyticsCol,
		Metrics:         metricsCol,
		AllowLocalDev:   cfg.AllowLocalDev,
	})
	if err != nil {
		log.Fatalf("[GateFlow] Failed to create proxy engine: %v", err)
	}

	// Reload routes from storage
	reloadRoutes := func() error {
		routes, err := store.ListRoutes(context.Background())
		if err != nil {
			return err
		}
		return engine.UpdateRoutes(routes)
	}
	_ = reloadRoutes()

	// Health checker for upstreams
	healthChecker := upstream.NewHealthChecker()
	upstreams, _ := store.ListUpstreams(context.Background())
	for _, u := range upstreams {
		healthChecker.Register(u)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go healthChecker.Start(ctx)

	// Gateway Data Plane HTTP Server
	gatewayMux := http.NewServeMux()

	// Operational endpoints
	gatewayMux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"UP","time":"` + time.Now().UTC().Format(time.RFC3339) + `"}` + "\n"))
	})
	gatewayMux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"READY"}` + "\n"))
	})
	gatewayMux.Handle("/metrics", metricsCol)

	// Gateway Traffic Handler
	gatewayMux.Handle("/", engine)

	// Control Plane Server
	cpServer := controlplane.NewServer(store, analyticsCol, cfg.AdminKey, reloadRoutes)
	controlMux := http.NewServeMux()
	cpServer.RegisterRoutes(controlMux)

	// Mount Control Plane /api/v1/ directly onto Gateway for single-port PaaS hosts (Render/Koyeb/Fly/Railway)
	gatewayMux.Handle("/api/v1/", corsWrapper(controlMux))

	// CORS for Control Plane frontend
	corsHandler := corsWrapper(controlMux)

	// Start Gateway Server
	gatewayServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      gatewayMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start Dedicated Admin Control Plane Server if running on different ports
	var adminServer *http.Server
	if cfg.AdminPort != "" && cfg.AdminPort != cfg.HTTPPort {
		adminServer = &http.Server{
			Addr:         ":" + cfg.AdminPort,
			Handler:      corsHandler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		go func() {
			log.Printf("[GateFlow] Dedicated Control Plane listening on http://0.0.0.0:%s (Admin Key: %s)", cfg.AdminPort, cfg.AdminKey)
			if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("[GateFlow] Admin server stopped: %v", err)
			}
		}()
	}

	go func() {
		log.Printf("[GateFlow] Traffic Engine & Unified API listening on http://0.0.0.0:%s", cfg.HTTPPort)
		if err := gatewayServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[GateFlow] Gateway server failed: %v", err)
		}
	}()

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[GateFlow] Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	_ = gatewayServer.Shutdown(shutdownCtx)
	if adminServer != nil {
		_ = adminServer.Shutdown(shutdownCtx)
	}
	healthChecker.Stop()
	log.Println("[GateFlow] Stopped successfully.")
}

func corsWrapper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func seedDemoData(store storage.Storage) {
	ctx := context.Background()

	// 1. Seed demo upstream (e.g. Httpbin or local echo)
	demoUpstream := &core.Upstream{
		ID:                 "ups_demo",
		Name:               "Demo HttpBin Upstream",
		BaseURL:            "https://httpbin.org",
		Active:             true,
		Timeout:            5 * time.Second,
		HealthCheckPath:    "/status/200",
		HealthInterval:     10 * time.Second,
		HealthTimeout:      3 * time.Second,
		HealthyThreshold:   2,
		UnhealthyThreshold: 3,
		Algorithm:          core.AlgorithmRoundRobin,
		Weight:             1,
		AllowedProtocols:   []string{"http", "https"},
		Healthy:            true,
	}
	_ = store.SaveUpstream(ctx, demoUpstream)

	// 2. Seed Rate Limit Policy (e.g. 5 requests per 10 seconds)
	demoRateLimit := &core.RateLimitPolicy{
		ID:        "rl_demo",
		Name:      "Demo Throttling (5 req / 10s)",
		Limit:     5,
		WindowSec: 10,
		Burst:     2,
		Dimension: "ip",
	}
	_ = store.SaveRateLimit(ctx, demoRateLimit)

	// 3. Seed Cache Policy (30s TTL)
	demoCache := &core.CachePolicy{
		ID:           "cp_demo",
		Name:         "Standard GET Cache 30s",
		TTLSeconds:   30,
		MaxSizeBytes: 1048576,
		CacheMethods: []string{"GET"},
		HonorETag:    true,
	}
	_ = store.SaveCachePolicy(ctx, demoCache)

	// 4. Seed IP Policy (Allow all)
	demoIP := &core.IPPolicy{
		ID:          "ip_demo",
		Name:        "Default Allow All",
		DefaultRule: core.IPRuleAllow,
	}
	_ = store.SaveIPPolicy(ctx, demoIP)

	// 5. Seed Routes
	// Route 1: Public route to httpbin with rate limit and cache
	_ = store.SaveRoute(ctx, &core.Route{
		ID:            "rt_public_anything",
		Name:          "Public Anything Proxy",
		PathPattern:   "/anything/*",
		Methods:       []string{"GET", "POST", "PUT"},
		UpstreamID:    "ups_demo",
		RateLimitID:   "rl_demo",
		CachePolicyID: "cp_demo",
		IPPolicyID:    "ip_demo",
		Active:        true,
		Priority:      10,
	})

	// Route 2: Protected route requiring API key
	_ = store.SaveRoute(ctx, &core.Route{
		ID:             "rt_protected_headers",
		Name:           "Protected Headers Proxy",
		PathPattern:    "/headers",
		Methods:        []string{"GET"},
		UpstreamID:     "ups_demo",
		AuthRequired:   true,
		RequiredScopes: []string{"read"},
		Active:         true,
		Priority:       20,
	})
}
