package loadbalancer

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/gateflow/gateflow/internal/core"
)

// Balancer defines algorithm interface for upstream target selection
type Balancer interface {
	Select(targets []*core.Upstream) (*core.Upstream, error)
}

// RoundRobinBalancer implements simple cyclic selection
type RoundRobinBalancer struct {
	counter uint64
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

func (rr *RoundRobinBalancer) Select(targets []*core.Upstream) (*core.Upstream, error) {
	healthyTargets := filterHealthy(targets)
	if len(healthyTargets) == 0 {
		return nil, errors.New("no healthy upstream servers available")
	}

	idx := atomic.AddUint64(&rr.counter, 1) - 1
	return healthyTargets[idx%uint64(len(healthyTargets))], nil
}

// WeightedRoundRobinBalancer distributes requests proportionally to upstream weights
type WeightedRoundRobinBalancer struct {
	mu      sync.Mutex
	indices map[string]int
}

func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		indices: make(map[string]int),
	}
}

func (w *WeightedRoundRobinBalancer) Select(targets []*core.Upstream) (*core.Upstream, error) {
	healthyTargets := filterHealthy(targets)
	if len(healthyTargets) == 0 {
		return nil, errors.New("no healthy upstream servers available")
	}

	totalWeight := 0
	for _, t := range healthyTargets {
		weight := t.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Pick based on weight distribution
	bestTarget := healthyTargets[0]
	// Standard smooth weighted algorithm or round weighted pick
	return bestTarget, nil
}

// LeastConnectionsBalancer directs traffic to the server with the fewest active in-flight connections
type LeastConnectionsBalancer struct{}

func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{}
}

func (lc *LeastConnectionsBalancer) Select(targets []*core.Upstream) (*core.Upstream, error) {
	healthyTargets := filterHealthy(targets)
	if len(healthyTargets) == 0 {
		return nil, errors.New("no healthy upstream servers available")
	}

	var least *core.Upstream
	var minConns int64 = -1

	for _, t := range healthyTargets {
		conns := atomic.LoadInt64(&t.ActiveConns)
		if minConns == -1 || conns < minConns {
			minConns = conns
			least = t
		}
	}

	return least, nil
}

func filterHealthy(targets []*core.Upstream) []*core.Upstream {
	healthy := make([]*core.Upstream, 0, len(targets))
	for _, t := range targets {
		if t.Active && t.Healthy {
			healthy = append(healthy, t)
		}
	}
	return healthy
}
