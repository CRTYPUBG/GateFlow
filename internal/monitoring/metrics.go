package monitoring

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

// MetricsCollector tracks real-time traffic statistics safely
type MetricsCollector struct {
	TotalRequests      uint64
	Requests2xx        uint64
	Requests3xx        uint64
	Requests4xx        uint64
	Requests5xx        uint64
	RateLimitBlocks    uint64
	CacheHits          uint64
	CacheMisses        uint64
	ActiveConnections  int64
	mu                 sync.RWMutex
}

var DefaultCollector = &MetricsCollector{}

func (m *MetricsCollector) RecordRequest(status int, cacheHit bool, rateLimited bool) {
	atomic.AddUint64(&m.TotalRequests, 1)

	if rateLimited {
		atomic.AddUint64(&m.RateLimitBlocks, 1)
	}
	if cacheHit {
		atomic.AddUint64(&m.CacheHits, 1)
	} else {
		atomic.AddUint64(&m.CacheMisses, 1)
	}

	switch {
	case status >= 200 && status < 300:
		atomic.AddUint64(&m.Requests2xx, 1)
	case status >= 300 && status < 400:
		atomic.AddUint64(&m.Requests3xx, 1)
	case status >= 400 && status < 500:
		atomic.AddUint64(&m.Requests4xx, 1)
	case status >= 500:
		atomic.AddUint64(&m.Requests5xx, 1)
	}
}

// ServeMetrics outputs standard Prometheus text format metrics
func (m *MetricsCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP gateflow_requests_total Total number of HTTP requests processed.\n")
	fmt.Fprintf(w, "# TYPE gateflow_requests_total counter\n")
	fmt.Fprintf(w, "gateflow_requests_total %d\n\n", atomic.LoadUint64(&m.TotalRequests))

	fmt.Fprintf(w, "# HELP gateflow_requests_status Total requests partitioned by status class.\n")
	fmt.Fprintf(w, "# TYPE gateflow_requests_status counter\n")
	fmt.Fprintf(w, "gateflow_requests_status{class=\"2xx\"} %d\n", atomic.LoadUint64(&m.Requests2xx))
	fmt.Fprintf(w, "gateflow_requests_status{class=\"3xx\"} %d\n", atomic.LoadUint64(&m.Requests3xx))
	fmt.Fprintf(w, "gateflow_requests_status{class=\"4xx\"} %d\n", atomic.LoadUint64(&m.Requests4xx))
	fmt.Fprintf(w, "gateflow_requests_status{class=\"5xx\"} %d\n\n", atomic.LoadUint64(&m.Requests5xx))

	fmt.Fprintf(w, "# HELP gateflow_ratelimit_blocks_total Total requests blocked by rate limiting.\n")
	fmt.Fprintf(w, "# TYPE gateflow_ratelimit_blocks_total counter\n")
	fmt.Fprintf(w, "gateflow_ratelimit_blocks_total %d\n\n", atomic.LoadUint64(&m.RateLimitBlocks))

	fmt.Fprintf(w, "# HELP gateflow_cache_events_total Total cache hits and misses.\n")
	fmt.Fprintf(w, "# TYPE gateflow_cache_events_total counter\n")
	fmt.Fprintf(w, "gateflow_cache_events_total{result=\"hit\"} %d\n", atomic.LoadUint64(&m.CacheHits))
	fmt.Fprintf(w, "gateflow_cache_events_total{result=\"miss\"} %d\n", atomic.LoadUint64(&m.CacheMisses))
}
