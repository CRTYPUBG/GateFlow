package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gateflow/gateflow/internal/core"
)

// Storage defines the persistence interface for GateFlow control plane
type Storage interface {
	// Upstreams
	ListUpstreams(ctx context.Context) ([]*core.Upstream, error)
	GetUpstream(ctx context.Context, id string) (*core.Upstream, error)
	SaveUpstream(ctx context.Context, u *core.Upstream) error
	DeleteUpstream(ctx context.Context, id string) error

	// Routes
	ListRoutes(ctx context.Context) ([]*core.Route, error)
	GetRoute(ctx context.Context, id string) (*core.Route, error)
	SaveRoute(ctx context.Context, r *core.Route) error
	DeleteRoute(ctx context.Context, id string) error

	// API Keys
	ListAPIKeys(ctx context.Context) ([]*core.APIKey, error)
	GetAPIKey(ctx context.Context, id string) (*core.APIKey, error)
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*core.APIKey, error)
	SaveAPIKey(ctx context.Context, k *core.APIKey) error
	DeleteAPIKey(ctx context.Context, id string) error

	// Rate Limits
	ListRateLimits(ctx context.Context) ([]*core.RateLimitPolicy, error)
	GetRateLimit(ctx context.Context, id string) (*core.RateLimitPolicy, error)
	SaveRateLimit(ctx context.Context, p *core.RateLimitPolicy) error
	DeleteRateLimit(ctx context.Context, id string) error

	// IP Policies
	ListIPPolicies(ctx context.Context) ([]*core.IPPolicy, error)
	GetIPPolicy(ctx context.Context, id string) (*core.IPPolicy, error)
	SaveIPPolicy(ctx context.Context, p *core.IPPolicy) error
	DeleteIPPolicy(ctx context.Context, id string) error

	// Cache Policies
	ListCachePolicies(ctx context.Context) ([]*core.CachePolicy, error)
	GetCachePolicy(ctx context.Context, id string) (*core.CachePolicy, error)
	SaveCachePolicy(ctx context.Context, p *core.CachePolicy) error
	DeleteCachePolicy(ctx context.Context, id string) error

	// Audit Logs
	SaveAuditLog(ctx context.Context, log *core.AuditLog) error
	ListAuditLogs(ctx context.Context, limit int) ([]*core.AuditLog, error)
}

// MemoryStorage is a thread-safe in-memory storage implementation for standalone execution and tests
type MemoryStorage struct {
	mu           sync.RWMutex
	upstreams    map[string]*core.Upstream
	routes       map[string]*core.Route
	apiKeys      map[string]*core.APIKey
	rateLimits   map[string]*core.RateLimitPolicy
	ipPolicies   map[string]*core.IPPolicy
	cachePolicies map[string]*core.CachePolicy
	auditLogs    []*core.AuditLog
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		upstreams:     make(map[string]*core.Upstream),
		routes:        make(map[string]*core.Route),
		apiKeys:       make(map[string]*core.APIKey),
		rateLimits:    make(map[string]*core.RateLimitPolicy),
		ipPolicies:    make(map[string]*core.IPPolicy),
		cachePolicies: make(map[string]*core.CachePolicy),
		auditLogs:     make([]*core.AuditLog, 0),
	}
}

func (m *MemoryStorage) ListUpstreams(ctx context.Context) ([]*core.Upstream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*core.Upstream, 0, len(m.upstreams))
	for _, u := range m.upstreams {
		res = append(res, u)
	}
	return res, nil
}

func (m *MemoryStorage) GetUpstream(ctx context.Context, id string) (*core.Upstream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.upstreams[id]
	if !ok {
		return nil, errors.New("upstream not found")
	}
	return u, nil
}

func (m *MemoryStorage) SaveUpstream(ctx context.Context, u *core.Upstream) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	u.UpdatedAt = time.Now().UTC()
	m.upstreams[u.ID] = u
	return nil
}

func (m *MemoryStorage) DeleteUpstream(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.upstreams, id)
	return nil
}

func (m *MemoryStorage) ListRoutes(ctx context.Context) ([]*core.Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*core.Route, 0, len(m.routes))
	for _, r := range m.routes {
		res = append(res, r)
	}
	return res, nil
}

func (m *MemoryStorage) GetRoute(ctx context.Context, id string) (*core.Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.routes[id]
	if !ok {
		return nil, errors.New("route not found")
	}
	return r, nil
}

func (m *MemoryStorage) SaveRoute(ctx context.Context, r *core.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	r.UpdatedAt = time.Now().UTC()
	m.routes[r.ID] = r
	return nil
}

func (m *MemoryStorage) DeleteRoute(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.routes, id)
	return nil
}

func (m *MemoryStorage) ListAPIKeys(ctx context.Context) ([]*core.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*core.APIKey, 0, len(m.apiKeys))
	for _, k := range m.apiKeys {
		res = append(res, k)
	}
	return res, nil
}

func (m *MemoryStorage) GetAPIKey(ctx context.Context, id string) (*core.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k, ok := m.apiKeys[id]
	if !ok {
		return nil, errors.New("api key not found")
	}
	return k, nil
}

func (m *MemoryStorage) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*core.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, k := range m.apiKeys {
		if k.KeyPrefix == prefix {
			return k, nil
		}
	}
	return nil, errors.New("api key not found")
}

func (m *MemoryStorage) SaveAPIKey(ctx context.Context, k *core.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now().UTC()
	}
	m.apiKeys[k.ID] = k
	return nil
}

func (m *MemoryStorage) DeleteAPIKey(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.apiKeys, id)
	return nil
}

func (m *MemoryStorage) ListRateLimits(ctx context.Context) ([]*core.RateLimitPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*core.RateLimitPolicy, 0, len(m.rateLimits))
	for _, p := range m.rateLimits {
		res = append(res, p)
	}
	return res, nil
}

func (m *MemoryStorage) GetRateLimit(ctx context.Context, id string) (*core.RateLimitPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.rateLimits[id]
	if !ok {
		return nil, errors.New("rate limit not found")
	}
	return p, nil
}

func (m *MemoryStorage) SaveRateLimit(ctx context.Context, p *core.RateLimitPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimits[p.ID] = p
	return nil
}

func (m *MemoryStorage) DeleteRateLimit(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rateLimits, id)
	return nil
}

func (m *MemoryStorage) ListIPPolicies(ctx context.Context) ([]*core.IPPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*core.IPPolicy, 0, len(m.ipPolicies))
	for _, p := range m.ipPolicies {
		res = append(res, p)
	}
	return res, nil
}

func (m *MemoryStorage) GetIPPolicy(ctx context.Context, id string) (*core.IPPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.ipPolicies[id]
	if !ok {
		return nil, errors.New("ip policy not found")
	}
	return p, nil
}

func (m *MemoryStorage) SaveIPPolicy(ctx context.Context, p *core.IPPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ipPolicies[p.ID] = p
	return nil
}

func (m *MemoryStorage) DeleteIPPolicy(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.ipPolicies, id)
	return nil
}

func (m *MemoryStorage) ListCachePolicies(ctx context.Context) ([]*core.CachePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*core.CachePolicy, 0, len(m.cachePolicies))
	for _, p := range m.cachePolicies {
		res = append(res, p)
	}
	return res, nil
}

func (m *MemoryStorage) GetCachePolicy(ctx context.Context, id string) (*core.CachePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.cachePolicies[id]
	if !ok {
		return nil, errors.New("cache policy not found")
	}
	return p, nil
}

func (m *MemoryStorage) SaveCachePolicy(ctx context.Context, p *core.CachePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cachePolicies[p.ID] = p
	return nil
}

func (m *MemoryStorage) DeleteCachePolicy(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cachePolicies, id)
	return nil
}

func (m *MemoryStorage) SaveAuditLog(ctx context.Context, log *core.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditLogs = append(m.auditLogs, log)
	return nil
}

func (m *MemoryStorage) ListAuditLogs(ctx context.Context, limit int) ([]*core.AuditLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.auditLogs) {
		limit = len(m.auditLogs)
	}
	res := make([]*core.AuditLog, limit)
	start := len(m.auditLogs) - limit
	for i := 0; i < limit; i++ {
		res[i] = m.auditLogs[start+i]
	}
	return res, nil
}
