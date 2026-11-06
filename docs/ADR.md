# GateFlow Mimari Kararları & Uygulama Planı (ADR-001)

## 1. Mimari Kararlar (ADR)
- **Data Plane:** Go `net/http` ve `httputil.ReverseProxy` (Director/Rewrite tabanlı). Katmanlı middleware mimarisi (Client IP & Protocol check -> IP ACL -> Route Match -> API Key Auth -> Rate Limiter -> Cache -> Load Balancer / Circuit Breaker -> Upstream Proxy).
- **Control Plane:** REST API (`/api/v1/...`), JWT/Admin Auth, Route, Upstream, Key, RateLimit, Cache, IP Rule, Analytics ve Audit CRUD.
- **Storage Layer:** PostgreSQL (`pgx` driver) kalıcı yapılandırma ve audit logları için. Ayrıca local development/single-instance modunda sıfır bağımlılık veya fallback için in-memory / SQLite / mockable storage interface mimarisi.
- **Distributed State & Fallback:** Redis (`go-redis/v9`) distributed token bucket/sliding window rate limiting ve response caching için. Redis ulaşılamadığında fail-safe in-memory cache/rate-limiter fallback mekanizması (Single Instance Graceful Fallback).
- **Security & SSRF Koruması:** Upstream URL'lerinde SSRF koruması: loopback (127.0.0.0/8, ::1), RFC1918 private IP'ler, link-local (169.254.0.0/16) ve Cloud metadata adresleri (169.254.169.254) varsayılan olarak engellenir; güvenli allowlist olmadan yönlendirme yapılmaz.
- **Trusted Proxy & Cloudflare IP Koruma:** `RemoteAddr` doğrulamasından geçmeden `CF-Connecting-IP` veya `X-Forwarded-For` kabul edilmez. Spoofing testleri ile doğrulanır.
- **Frontend:** React 19 + TypeScript + Vite + Tailwind/Modern Dark UI Dashboard.

## 2. Fazlar
- **Faz 1:** Dizin yapısı, Go modülü, Core tipler ve config.
- **Faz 2:** Data Plane: Reverse Proxy, Route matcher, Upstream manager, SSRF guard.
- **Faz 3:** Security: API Key (hash/prefix/scopes), IP ACL (CIDR/IPv4/IPv6), Trusted Proxy, Rate Limiter (Token bucket + Redis + Memory fallback).
- **Faz 4:** Traffic Control: Load Balancers (Round Robin, Weighted RR, Least Connections), Health Checker, Circuit Breaker, Caching (GET, TTL, ETag, Invalidation).
- **Faz 5:** Control Plane & Storage: Storage interface, PostgreSQL schema & migrations, Control Plane REST handlers, Audit logs.
- **Faz 6:** Observability: Metrics (Prometheus), Access logs (JSON, masked), Analytics Aggregator.
- **Faz 7:** Frontend: React + TypeScript + Vite Dashboard (Tüm ekranlar ve backend bağlantısı).
- **Faz 8:** Tests (Unit, Integration, Security, Race), Docker Compose, CI workflows, Dokümantasyon ve Doğrulama.
