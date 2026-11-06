package routing

import (
	"testing"

	"github.com/gateflow/gateflow/internal/core"
)

func TestRouter_Match(t *testing.T) {
	router := NewRouter()

	routes := []*core.Route{
		{
			ID:          "route_exact",
			PathPattern: "/api/v1/users",
			Methods:     []string{"GET"},
			Active:      true,
			Priority:    10,
		},
		{
			ID:          "route_wildcard",
			PathPattern: "/api/v1/*",
			Methods:     []string{"GET", "POST"},
			Active:      true,
			Priority:    5,
		},
		{
			ID:          "route_strip",
			PathPattern: "/service/*",
			Methods:     []string{"GET"},
			StripPrefix: "/service",
			Active:      true,
			Priority:    8,
		},
	}

	if err := router.SetRoutes(routes); err != nil {
		t.Fatalf("SetRoutes failed: %v", err)
	}

	// 1. Exact match test
	match, status, err := router.Match("/api/v1/users", "GET")
	if err != nil || status != 200 || match.Route.ID != "route_exact" {
		t.Errorf("Expected route_exact, got %v (status %d, err %v)", match, status, err)
	}

	// 2. Method Not Allowed (405) test
	match, status, err = router.Match("/api/v1/users", "DELETE")
	// /api/v1/users matches path of route_exact and route_wildcard. Neither allows DELETE.
	if status != 405 {
		t.Errorf("Expected status 405 for DELETE, got %d", status)
	}

	// 3. Wildcard match test
	match, status, err = router.Match("/api/v1/products/123", "POST")
	if err != nil || status != 200 || match.Route.ID != "route_wildcard" {
		t.Errorf("Expected route_wildcard, got %v (status %d, err %v)", match, status, err)
	}

	// 4. Strip prefix test
	match, status, err = router.Match("/service/hello", "GET")
	if err != nil || status != 200 || match.RewrittenURL != "/hello" {
		t.Errorf("Expected rewritten /hello, got %s", match.RewrittenURL)
	}

	// 5. Not found (404) test
	match, status, err = router.Match("/unknown/path", "GET")
	if status != 404 {
		t.Errorf("Expected status 404 for unknown path, got %d", status)
	}
}
