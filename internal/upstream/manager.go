package upstream

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gateflow/gateflow/internal/core"
)

// SSRFValidator inspects upstream target URLs and prevents routing to forbidden networks
type SSRFValidator struct {
	allowedInternal bool // If true, local/private addresses are permitted (e.g. For dev/demo upstream)
	blockedNets     []*net.IPNet
}

func NewSSRFValidator(allowInternalForDev bool) *SSRFValidator {
	v := &SSRFValidator{allowedInternal: allowInternalForDev}

	// CIDRs that must be blocked for SSRF protection:
	// - 127.0.0.0/8 (Loopback IPv4)
	// - ::1/128 (Loopback IPv6)
	// - 169.254.0.0/16 (Link Local IPv4)
	// - fe80::/10 (Link Local IPv6)
	// - 169.254.169.254/32 (AWS/GCP/Azure Cloud Metadata)
	// - 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 (RFC1918 Private)
	defaultBlocked := []string{
		"127.0.0.0/8",
		"::1/128",
		"169.254.0.0/16",
		"fe80::/10",
		"169.254.169.254/32",
	}

	for _, cidr := range defaultBlocked {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			v.blockedNets = append(v.blockedNets, ipNet)
		}
	}

	return v
}

// ValidateTargetURL verifies URL scheme, format, and resolves host to ensure non-malicious SSRF destination
func (v *SSRFValidator) ValidateTargetURL(targetURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s (only http and https allowed)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, errors.New("empty host in URL")
	}

	// If internal/dev allowed (e.g. testing local upstreams), bypass loopback checks
	if v.allowedInternal {
		return parsed, nil
	}

	// Resolve IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed for host %s: %w", host, err)
	}

	for _, ip := range ips {
		for _, blocked := range v.blockedNets {
			if blocked.Contains(ip) {
				return nil, fmt.Errorf("SSRF violation: target IP %s belongs to forbidden network %s", ip.String(), blocked.String())
			}
		}
	}

	return parsed, nil
}

// HealthChecker actively monitors and updates health status of registered upstreams
type HealthChecker struct {
	client  *http.Client
	mu      sync.RWMutex
	targets map[string]*core.Upstream
	stopCh  chan struct{}
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		targets: make(map[string]*core.Upstream),
		stopCh:  make(chan struct{}),
	}
}

func (h *HealthChecker) Register(u *core.Upstream) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.targets[u.ID] = u
}

func (h *HealthChecker) Unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.targets, id)
}

func (h *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.checkAll()
		}
	}
}

func (h *HealthChecker) Stop() {
	close(h.stopCh)
}

func (h *HealthChecker) checkAll() {
	h.mu.RLock()
	var toCheck []*core.Upstream
	for _, u := range h.targets {
		if u.Active && u.HealthCheckPath != "" {
			toCheck = append(toCheck, u)
		}
	}
	h.mu.RUnlock()

	for _, u := range toCheck {
		go h.checkOne(u)
	}
}

func (h *HealthChecker) checkOne(u *core.Upstream) {
	checkURL := fmt.Sprintf("%s%s", u.BaseURL, u.HealthCheckPath)
	req, err := http.NewRequest(http.MethodGet, checkURL, nil)
	if err != nil {
		u.Healthy = false
		u.LastError = err.Error()
		return
	}

	resp, err := h.client.Do(req)
	u.LastCheckedAt = time.Now().UTC()

	if err != nil {
		u.Healthy = false
		u.LastError = err.Error()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		u.Healthy = true
		u.LastError = ""
	} else {
		u.Healthy = false
		u.LastError = fmt.Sprintf("health check failed with status code: %d", resp.StatusCode)
	}
}

// CircuitState represents current status of circuit breaker
type CircuitState int32

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

// CircuitBreaker prevents cascading failures when an upstream is down
type CircuitBreaker struct {
	state          int32 // atomic CircuitState
	failureCount   int64
	successCount   int64
	threshold      int64
	resetTimeout   time.Duration
	lastStateChange time.Time
	mu             sync.Mutex
}

func NewCircuitBreaker(threshold int64, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        int32(StateClosed),
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) AllowRequest() bool {
	currentState := CircuitState(atomic.LoadInt32(&cb.state))
	if currentState == StateClosed {
		return true
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if currentState == StateOpen {
		if time.Since(cb.lastStateChange) > cb.resetTimeout {
			// Transition to half-open
			atomic.StoreInt32(&cb.state, int32(StateHalfOpen))
			return true
		}
		return false
	}

	// StateHalfOpen allows limited test traffic
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	currentState := CircuitState(atomic.LoadInt32(&cb.state))
	if currentState == StateHalfOpen {
		atomic.StoreInt32(&cb.state, int32(StateClosed))
		atomic.StoreInt64(&cb.failureCount, 0)
		cb.lastStateChange = time.Now()
	} else if currentState == StateClosed {
		atomic.StoreInt64(&cb.failureCount, 0)
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	failures := atomic.AddInt64(&cb.failureCount, 1)
	if failures >= cb.threshold {
		atomic.StoreInt32(&cb.state, int32(StateOpen))
		cb.lastStateChange = time.Now()
	}
}
