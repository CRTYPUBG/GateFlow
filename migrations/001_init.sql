-- GateFlow PostgreSQL Database Schema & Initial Migration
-- Version: 001_init.sql

CREATE TABLE IF NOT EXISTS admin_users (
    id VARCHAR(64) PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'admin',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS upstreams (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    base_url TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    timeout_ms BIGINT NOT NULL DEFAULT 5000,
    health_check_path VARCHAR(255) DEFAULT '/health',
    health_interval_sec INT NOT NULL DEFAULT 10,
    health_timeout_sec INT NOT NULL DEFAULT 3,
    healthy_threshold INT NOT NULL DEFAULT 2,
    unhealthy_threshold INT NOT NULL DEFAULT 3,
    algorithm VARCHAR(50) NOT NULL DEFAULT 'round_robin',
    weight INT NOT NULL DEFAULT 1,
    allowed_protocols TEXT[] DEFAULT ARRAY['http', 'https'],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rate_limit_policies (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    request_limit BIGINT NOT NULL,
    window_sec BIGINT NOT NULL,
    burst BIGINT NOT NULL DEFAULT 0,
    dimension VARCHAR(50) NOT NULL DEFAULT 'ip',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cache_policies (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    ttl_seconds BIGINT NOT NULL DEFAULT 60,
    max_size_bytes BIGINT NOT NULL DEFAULT 1048576,
    vary_headers TEXT[] DEFAULT ARRAY[]::TEXT[],
    cache_methods TEXT[] DEFAULT ARRAY['GET']::TEXT[],
    honor_etag BOOLEAN NOT NULL DEFAULT true,
    respect_no_store BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ip_policies (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    default_rule VARCHAR(10) NOT NULL DEFAULT 'allow',
    cidrs JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS routes (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    path_pattern TEXT NOT NULL,
    methods TEXT[] NOT NULL DEFAULT ARRAY['GET']::TEXT[],
    upstream_id VARCHAR(64) REFERENCES upstreams(id) ON DELETE RESTRICT,
    path_rewrite TEXT,
    strip_prefix TEXT,
    auth_required BOOLEAN NOT NULL DEFAULT false,
    required_scopes TEXT[] DEFAULT ARRAY[]::TEXT[],
    rate_limit_id VARCHAR(64) REFERENCES rate_limit_policies(id) ON DELETE SET NULL,
    cache_policy_id VARCHAR(64) REFERENCES cache_policies(id) ON DELETE SET NULL,
    ip_policy_id VARCHAR(64) REFERENCES ip_policies(id) ON DELETE SET NULL,
    timeout_ms BIGINT DEFAULT 10000,
    max_retries INT DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT true,
    priority INT NOT NULL DEFAULT 10,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(64) PRIMARY KEY,
    key_prefix VARCHAR(32) NOT NULL,
    key_hash VARCHAR(128) NOT NULL UNIQUE,
    owner VARCHAR(255) NOT NULL,
    description TEXT,
    scopes TEXT[] DEFAULT ARRAY[]::TEXT[],
    active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);

CREATE TABLE IF NOT EXISTS audit_logs (
    id VARCHAR(64) PRIMARY KEY,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    admin_user VARCHAR(100) NOT NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(64) NOT NULL,
    details TEXT,
    ip_address VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);

CREATE TABLE IF NOT EXISTS request_logs (
    id VARCHAR(64) PRIMARY KEY,
    request_id VARCHAR(64) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    route_id VARCHAR(64),
    upstream_id VARCHAR(64),
    method VARCHAR(16) NOT NULL,
    path TEXT NOT NULL,
    status_code INT NOT NULL,
    duration_ms DOUBLE PRECISION NOT NULL,
    client_ip VARCHAR(64) NOT NULL,
    api_key_id VARCHAR(64),
    cache_status VARCHAR(16),
    rate_limited BOOLEAN DEFAULT false,
    error_msg TEXT
);

CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_status ON request_logs(status_code);
