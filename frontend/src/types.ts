export interface Upstream {
  id: string;
  name: string;
  base_url: string;
  active: boolean;
  healthy: boolean;
  algorithm: string;
  weight: number;
}

export interface Route {
  id: string;
  name: string;
  path_pattern: string;
  methods: string[];
  upstream_id: string;
  auth_required: boolean;
  rate_limit_id?: string;
  cache_policy_id?: string;
  ip_policy_id?: string;
  active: boolean;
  priority: number;
}

export interface APIKey {
  id: string;
  key_prefix: string;
  owner: string;
  description: string;
  scopes: string[];
  active: boolean;
  created_at: string;
}

export interface RateLimitPolicy {
  id: string;
  name: string;
  limit: number;
  window_sec: number;
  burst: number;
  dimension: string;
}

export interface IPPolicy {
  id: string;
  name: string;
  default_rule: string;
}

export interface CachePolicy {
  id: string;
  name: string;
  ttl_seconds: number;
  cache_methods: string[];
}

export interface AnalyticsOverview {
  total_requests: number;
  rps: number;
  requests_2xx: number;
  requests_3xx: number;
  requests_4xx: number;
  requests_5xx: number;
  rate_limit_blocks: number;
  cache_hits: number;
  cache_misses: number;
  cache_hit_ratio: number;
  p50_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
}

export interface RequestLog {
  id: string;
  request_id: string;
  timestamp: string;
  route_id: string;
  upstream_id: string;
  method: string;
  path: string;
  status_code: number;
  duration: number;
  client_ip: string;
  api_key_id?: string;
  cache_status: string;
  rate_limited: boolean;
  error_msg?: string;
}

export interface AuditLog {
  id: string;
  timestamp: string;
  admin_user: string;
  action: string;
  entity_type: string;
  entity_id: string;
  details?: string;
  ip_address: string;
}
