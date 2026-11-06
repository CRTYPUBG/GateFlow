package core

import (
	"time"
)

// Algorithm specifies load balancing algorithms
type Algorithm string

const (
	AlgorithmRoundRobin         Algorithm = "round_robin"
	AlgorithmWeightedRoundRobin Algorithm = "weighted_round_robin"
	AlgorithmLeastConnections   Algorithm = "least_connections"
)

// Upstream defines a backend service
type Upstream struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	BaseURL            string        `json:"base_url"`
	Active             bool          `json:"active"`
	Timeout            time.Duration `json:"timeout"`
	HealthCheckPath    string        `json:"health_check_path"`
	HealthInterval     time.Duration `json:"health_interval"`
	HealthTimeout      time.Duration `json:"health_timeout"`
	HealthyThreshold   int           `json:"healthy_threshold"`
	UnhealthyThreshold int           `json:"unhealthy_threshold"`
	Algorithm          Algorithm     `json:"algorithm"`
	Weight             int           `json:"weight"`
	AllowedProtocols   []string      `json:"allowed_protocols"` // e.g. ["http", "https"]
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`

	// Runtime health status
	Healthy       bool      `json:"healthy"`
	ActiveConns   int64     `json:"active_conns"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	LastError     string    `json:"last_error,omitempty"`
}

// Route defines routing rules and policies for traffic matching
type Route struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	PathPattern    string        `json:"path_pattern"` // e.g. "/api/v1/users", "/api/v1/users/*"
	Methods        []string      `json:"methods"`      // e.g. ["GET", "POST"]
	UpstreamID     string        `json:"upstream_id"`
	PathRewrite    string        `json:"path_rewrite,omitempty"` // prefix stripping or regex replacement
	StripPrefix    string        `json:"strip_prefix,omitempty"`
	AuthRequired   bool          `json:"auth_required"`
	RequiredScopes []string      `json:"required_scopes,omitempty"`
	RateLimitID    string        `json:"rate_limit_id,omitempty"`
	CachePolicyID  string        `json:"cache_policy_id,omitempty"`
	IPPolicyID     string        `json:"ip_policy_id,omitempty"`
	Timeout        time.Duration `json:"timeout"`
	MaxRetries     int           `json:"max_retries"`
	Active         bool          `json:"active"`
	Priority       int           `json:"priority"` // Higher number = higher precedence
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// APIKey represents authenticated client identity and permissions
type APIKey struct {
	ID          string     `json:"id"`
	KeyPrefix   string     `json:"key_prefix"` // e.g. "gf_live_abc12"
	KeyHash     string     `json:"-"`          // SHA-256 hash
	Owner       string     `json:"owner"`
	Description string     `json:"description"`
	Scopes      []string   `json:"scopes"`
	Active      bool       `json:"active"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

// RateLimitPolicy defines traffic throttling rules
type RateLimitPolicy struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Limit     int64     `json:"limit"`      // Max requests allowed
	WindowSec int64     `json:"window_sec"` // In seconds (e.g. 60)
	Burst     int64     `json:"burst"`      // Allowed burst above limit
	Dimension string    `json:"dimension"`  // "ip", "api_key", "route", "global"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CachePolicy defines caching behavior for routes
type CachePolicy struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	TTLSeconds     int64     `json:"ttl_seconds"`
	MaxSizeBytes   int64     `json:"max_size_bytes"`
	VaryHeaders    []string  `json:"vary_headers,omitempty"`
	CacheMethods   []string  `json:"cache_methods"` // typically ["GET"]
	HonorETag      bool      `json:"honor_etag"`
	RespectNoStore bool      `json:"respect_no_store"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// IPRuleType defines whether an IP rule allows or denies access
type IPRuleType string

const (
	IPRuleAllow IPRuleType = "allow"
	IPRuleDeny  IPRuleType = "deny"
)

// IPPolicy defines IP ACLs
type IPPolicy struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	DefaultRule IPRuleType `json:"default_rule"` // "allow" or "deny"
	CIDRs       []CIDRRule `json:"cidrs"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CIDRRule maps a network CIDR to an action
type CIDRRule struct {
	CIDR        string     `json:"cidr"`
	Action      IPRuleType `json:"action"`
	Description string     `json:"description,omitempty"`
}

// RequestLog represents an analyzed HTTP transaction for metrics and analytics
type RequestLog struct {
	ID          string        `json:"id"`
	RequestID   string        `json:"request_id"`
	Timestamp   time.Time     `json:"timestamp"`
	RouteID     string        `json:"route_id"`
	UpstreamID  string        `json:"upstream_id"`
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	StatusCode  int           `json:"status_code"`
	Duration    time.Duration `json:"duration"`
	ClientIP    string        `json:"client_ip"` // Masked if privacy configured
	APIKeyID    string        `json:"api_key_id,omitempty"`
	CacheStatus string        `json:"cache_status"` // "HIT", "MISS", "BYPASS"
	RateLimited bool          `json:"rate_limited"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
}

// AuditLog tracks administrative actions on the control plane
type AuditLog struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	AdminUser  string    `json:"admin_user"`
	Action     string    `json:"action"`      // e.g. "CREATE_ROUTE", "REVOKE_KEY"
	EntityType string    `json:"entity_type"` // e.g. "route", "api_key"
	EntityID   string    `json:"entity_id"`
	Details    string    `json:"details,omitempty"`
	IPAddress  string    `json:"ip_address"`
}
