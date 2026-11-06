package routing

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gateflow/gateflow/internal/core"
)

// MatchResult contains the selected route and any extracted route variables
type MatchResult struct {
	Route        *core.Route
	MatchedPath  string
	RewrittenURL string
}

// Router provides deterministic, priority-ordered prefix and wildcard path matching
type Router struct {
	mu     sync.RWMutex
	routes []*core.Route
}

func NewRouter() *Router {
	return &Router{
		routes: make([]*core.Route, 0),
	}
}

// SetRoutes atomically replaces the active route table with validation
func (r *Router) SetRoutes(newRoutes []*core.Route) error {
	// Validate routes for pattern conflicts and syntax
	for i, rt := range newRoutes {
		if rt.PathPattern == "" || !strings.HasPrefix(rt.PathPattern, "/") {
			return fmt.Errorf("invalid path pattern at index %d: must start with /", i)
		}
		if len(rt.Methods) == 0 {
			return fmt.Errorf("route %s must specify at least one HTTP method", rt.ID)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Sort routes deterministically:
	// 1. Higher Priority first
	// 2. Longer path patterns (more specific) before shorter ones
	// 3. Exact matches before wildcards
	sorted := make([]*core.Route, len(newRoutes))
	copy(sorted, newRoutes)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if shouldSwap(sorted[i], sorted[j]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	r.routes = sorted
	return nil
}

func shouldSwap(a, b *core.Route) bool {
	if a.Priority != b.Priority {
		return a.Priority < b.Priority // want higher priority first
	}
	// Prefer non-wildcard
	aWild := strings.Contains(a.PathPattern, "*")
	bWild := strings.Contains(b.PathPattern, "*")
	if aWild != bWild {
		return aWild // if a is wild and b is not, swap so b comes first
	}
	return len(a.PathPattern) < len(b.PathPattern) // want longer paths first
}

// Match evaluates an incoming request path and method against the route table
// Returns (MatchResult, Status code). If no route matches -> 404. If path matches but method doesn't -> 405.
func (r *Router) Match(reqPath, method string) (*MatchResult, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var pathMatchedRoute *core.Route

	for _, route := range r.routes {
		if !route.Active {
			continue
		}

		if matchPathPattern(route.PathPattern, reqPath) {
			pathMatchedRoute = route

			// Check HTTP method
			methodAllowed := false
			for _, m := range route.Methods {
				if strings.EqualFold(m, method) || m == "*" {
					methodAllowed = true
					break
				}
			}

			if methodAllowed {
				rewritten := rewritePath(reqPath, route)
				return &MatchResult{
					Route:        route,
					MatchedPath:  route.PathPattern,
					RewrittenURL: rewritten,
				}, http.StatusOK, nil
			}
		}
	}

	if pathMatchedRoute != nil {
		// Path matched at least one route, but method was not allowed -> HTTP 405 Method Not Allowed
		return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
	}

	// No route matched -> HTTP 404 Not Found
	return nil, http.StatusNotFound, errors.New("route not found")
}

func matchPathPattern(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Wildcard prefix matching: e.g. "/api/v1/*" matches "/api/v1/users"
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(path, prefix) {
			// Ensure matching on path boundary
			rest := strings.TrimPrefix(path, prefix)
			return strings.HasPrefix(rest, "/") || rest == ""
		}
	}

	// Trailing slash equality
	if strings.TrimSuffix(pattern, "/") == strings.TrimSuffix(path, "/") {
		return true
	}

	return false
}

func rewritePath(reqPath string, route *core.Route) string {
	target := reqPath
	if route.StripPrefix != "" && strings.HasPrefix(target, route.StripPrefix) {
		target = strings.TrimPrefix(target, route.StripPrefix)
		if !strings.HasPrefix(target, "/") {
			target = "/" + target
		}
	}
	if route.PathRewrite != "" {
		// Simple prefix rewrite
		target = route.PathRewrite + target
	}
	return target
}
