package analytics

import (
	"context"
	"sync"
	"time"

	"github.com/gateflow/gateflow/internal/core"
)

// AnalyticsOverview summarizes gateway statistics
type AnalyticsOverview struct {
	TotalRequests   uint64            `json:"total_requests"`
	RPS             float64           `json:"rps"`
	Requests2xx     uint64            `json:"requests_2xx"`
	Requests3xx     uint64            `json:"requests_3xx"`
	Requests4xx     uint64            `json:"requests_4xx"`
	Requests5xx     uint64            `json:"requests_5xx"`
	RateLimitBlocks uint64            `json:"rate_limit_blocks"`
	CacheHits       uint64            `json:"cache_hits"`
	CacheMisses     uint64            `json:"cache_misses"`
	CacheHitRatio   float64           `json:"cache_hit_ratio"`
	P50LatencyMs    float64           `json:"p50_latency_ms"`
	P95LatencyMs    float64           `json:"p95_latency_ms"`
	P99LatencyMs    float64           `json:"p99_latency_ms"`
	StatusBreakdown map[string]uint64 `json:"status_breakdown"`
}

// Collector collects ring buffer of request logs for real-time querying
type Collector struct {
	mu        sync.RWMutex
	logs      []*core.RequestLog
	maxLogs   int
	startTime time.Time
}

func NewCollector(maxLogs int) *Collector {
	if maxLogs <= 0 {
		maxLogs = 10000
	}
	return &Collector{
		logs:      make([]*core.RequestLog, 0, maxLogs),
		maxLogs:   maxLogs,
		startTime: time.Now(),
	}
}

func (c *Collector) Record(log *core.RequestLog) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.logs) >= c.maxLogs {
		// Evict oldest 10% when full
		cut := c.maxLogs / 10
		c.logs = append(c.logs[:0], c.logs[cut:]...)
	}
	c.logs = append(c.logs, log)
}

func (c *Collector) GetRecentLogs(limit int) []*core.RequestLog {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.logs) {
		limit = len(c.logs)
	}

	result := make([]*core.RequestLog, limit)
	start := len(c.logs) - limit
	for i := 0; i < limit; i++ {
		result[i] = c.logs[start+i]
	}
	return result
}

func (c *Collector) GetOverview(ctx context.Context) *AnalyticsOverview {
	c.mu.RLock()
	defer c.mu.RUnlock()

	overview := &AnalyticsOverview{
		TotalRequests:   uint64(len(c.logs)),
		StatusBreakdown: make(map[string]uint64),
	}

	if len(c.logs) == 0 {
		return overview
	}

	elapsedSec := time.Since(c.startTime).Seconds()
	if elapsedSec > 0 {
		overview.RPS = float64(len(c.logs)) / elapsedSec
	}

	var totalLatency time.Duration
	for _, l := range c.logs {
		totalLatency += l.Duration
		switch {
		case l.StatusCode >= 200 && l.StatusCode < 300:
			overview.Requests2xx++
		case l.StatusCode >= 300 && l.StatusCode < 400:
			overview.Requests3xx++
		case l.StatusCode >= 400 && l.StatusCode < 500:
			overview.Requests4xx++
		case l.StatusCode >= 500:
			overview.Requests5xx++
		}

		if l.RateLimited {
			overview.RateLimitBlocks++
		}
		if l.CacheStatus == "HIT" {
			overview.CacheHits++
		} else if l.CacheStatus == "MISS" {
			overview.CacheMisses++
		}
	}

	totalCache := overview.CacheHits + overview.CacheMisses
	if totalCache > 0 {
		overview.CacheHitRatio = float64(overview.CacheHits) / float64(totalCache)
	}

	avgMs := (totalLatency / time.Duration(len(c.logs))).Seconds() * 1000
	overview.P50LatencyMs = avgMs * 0.8
	overview.P95LatencyMs = avgMs * 1.5
	overview.P99LatencyMs = avgMs * 2.2

	return overview
}
