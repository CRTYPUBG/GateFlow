package controlplane

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gateflow/gateflow/internal/analytics"
	"github.com/gateflow/gateflow/internal/auth"
	"github.com/gateflow/gateflow/internal/core"
	"github.com/gateflow/gateflow/internal/storage"
)

// Server handles administrative REST endpoints
type Server struct {
	storage   storage.Storage
	analytics *analytics.Collector
	adminKey  string
	onUpdate  func() error // Hook to reload gateway routes/state
}

func NewServer(store storage.Storage, analytics *analytics.Collector, adminKey string, onUpdate func() error) *Server {
	return &Server{
		storage:   store,
		analytics: analytics,
		adminKey:  adminKey,
		onUpdate:  onUpdate,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/upstreams", s.authMiddleware(s.handleUpstreams))
	mux.HandleFunc("/api/v1/upstreams/", s.authMiddleware(s.handleUpstreamByID))

	mux.HandleFunc("/api/v1/routes", s.authMiddleware(s.handleRoutes))
	mux.HandleFunc("/api/v1/routes/", s.authMiddleware(s.handleRouteByID))

	mux.HandleFunc("/api/v1/keys", s.authMiddleware(s.handleAPIKeys))
	mux.HandleFunc("/api/v1/keys/", s.authMiddleware(s.handleAPIKeyByID))

	mux.HandleFunc("/api/v1/rate-limits", s.authMiddleware(s.handleRateLimits))
	mux.HandleFunc("/api/v1/rate-limits/", s.authMiddleware(s.handleRateLimitByID))

	mux.HandleFunc("/api/v1/ip-rules", s.authMiddleware(s.handleIPPolicies))
	mux.HandleFunc("/api/v1/ip-rules/", s.authMiddleware(s.handleIPPolicyByID))

	mux.HandleFunc("/api/v1/cache-policies", s.authMiddleware(s.handleCachePolicies))
	mux.HandleFunc("/api/v1/cache-policies/", s.authMiddleware(s.handleCachePolicyByID))

	mux.HandleFunc("/api/v1/analytics/overview", s.authMiddleware(s.handleAnalyticsOverview))
	mux.HandleFunc("/api/v1/analytics/logs", s.authMiddleware(s.handleAnalyticsLogs))
	mux.HandleFunc("/api/v1/audit-logs", s.authMiddleware(s.handleAuditLogs))
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Admin auth via X-Admin-Key or Authorization: Bearer <adminKey>
		authHeader := r.Header.Get("X-Admin-Key")
		if authHeader == "" {
			parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				authHeader = parts[1]
			}
		}

		if s.adminKey != "" && authHeader != s.adminKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized admin access"})
			return
		}

		next(w, r)
	}
}

func (s *Server) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.ListUpstreams(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var u core.Upstream
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if u.ID == "" {
			u.ID = fmt.Sprintf("ups_%d", time.Now().UnixNano())
		}
		u.CreatedAt = time.Now().UTC()
		u.UpdatedAt = time.Now().UTC()
		u.Healthy = true
		if err := s.storage.SaveUpstream(r.Context(), &u); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.storage.SaveAuditLog(r.Context(), &core.AuditLog{
			ID:         fmt.Sprintf("aud_%d", time.Now().UnixNano()),
			Timestamp:  time.Now().UTC(),
			AdminUser:  "admin",
			Action:     "CREATE_UPSTREAM",
			EntityType: "upstream",
			EntityID:   u.ID,
			IPAddress:  r.RemoteAddr,
		})
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(u)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUpstreamByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/upstreams/")
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		u, err := s.storage.GetUpstream(r.Context(), id)
		if err != nil {
			http.Error(w, "Upstream not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(u)
	case http.MethodDelete:
		_ = s.storage.DeleteUpstream(r.Context(), id)
		_ = s.storage.SaveAuditLog(r.Context(), &core.AuditLog{
			ID:         fmt.Sprintf("aud_%d", time.Now().UnixNano()),
			Timestamp:  time.Now().UTC(),
			AdminUser:  "admin",
			Action:     "DELETE_UPSTREAM",
			EntityType: "upstream",
			EntityID:   id,
			IPAddress:  r.RemoteAddr,
		})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.ListRoutes(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var rt core.Route
		if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if rt.ID == "" {
			rt.ID = fmt.Sprintf("rt_%d", time.Now().UnixNano())
		}
		if len(rt.Methods) == 0 {
			rt.Methods = []string{"GET"}
		}
		rt.CreatedAt = time.Now().UTC()
		rt.UpdatedAt = time.Now().UTC()
		if err := s.storage.SaveRoute(r.Context(), &rt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if s.onUpdate != nil {
			_ = s.onUpdate()
		}
		_ = s.storage.SaveAuditLog(r.Context(), &core.AuditLog{
			ID:         fmt.Sprintf("aud_%d", time.Now().UnixNano()),
			Timestamp:  time.Now().UTC(),
			AdminUser:  "admin",
			Action:     "CREATE_ROUTE",
			EntityType: "route",
			EntityID:   rt.ID,
			IPAddress:  r.RemoteAddr,
		})
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rt)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRouteByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/routes/")
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		rt, err := s.storage.GetRoute(r.Context(), id)
		if err != nil {
			http.Error(w, "Route not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(rt)
	case http.MethodDelete:
		_ = s.storage.DeleteRoute(r.Context(), id)
		if s.onUpdate != nil {
			_ = s.onUpdate()
		}
		_ = s.storage.SaveAuditLog(r.Context(), &core.AuditLog{
			ID:         fmt.Sprintf("aud_%d", time.Now().UnixNano()),
			Timestamp:  time.Now().UTC(),
			AdminUser:  "admin",
			Action:     "DELETE_ROUTE",
			EntityType: "route",
			EntityID:   id,
			IPAddress:  r.RemoteAddr,
		})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type CreateAPIKeyRequest struct {
	Owner       string   `json:"owner"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
}

type CreateAPIKeyResponse struct {
	RawKey string       `json:"raw_key"`
	APIKey *core.APIKey `json:"api_key"`
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.ListAPIKeys(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var req CreateAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		rawKey, apiKey, err := auth.GenerateAPIKey(req.Owner, req.Description, req.Scopes, nil)
		if err != nil {
			http.Error(w, "Failed to generate key", http.StatusInternalServerError)
			return
		}
		if err := s.storage.SaveAPIKey(r.Context(), apiKey); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.storage.SaveAuditLog(r.Context(), &core.AuditLog{
			ID:         fmt.Sprintf("aud_%d", time.Now().UnixNano()),
			Timestamp:  time.Now().UTC(),
			AdminUser:  "admin",
			Action:     "CREATE_API_KEY",
			EntityType: "api_key",
			EntityID:   apiKey.ID,
			IPAddress:  r.RemoteAddr,
		})
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateAPIKeyResponse{RawKey: rawKey, APIKey: apiKey})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/")
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodDelete:
		// Revoke key
		key, err := s.storage.GetAPIKey(r.Context(), id)
		if err == nil && key != nil {
			key.Active = false
			_ = s.storage.SaveAPIKey(r.Context(), key)
		}
		_ = s.storage.SaveAuditLog(r.Context(), &core.AuditLog{
			ID:         fmt.Sprintf("aud_%d", time.Now().UnixNano()),
			Timestamp:  time.Now().UTC(),
			AdminUser:  "admin",
			Action:     "REVOKE_API_KEY",
			EntityType: "api_key",
			EntityID:   id,
			IPAddress:  r.RemoteAddr,
		})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRateLimits(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		list, _ := s.storage.ListRateLimits(r.Context())
		json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var p core.RateLimitPolicy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("rl_%d", time.Now().UnixNano())
		}
		_ = s.storage.SaveRateLimit(r.Context(), &p)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRateLimitByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/rate-limits/")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodDelete {
		_ = s.storage.DeleteRateLimit(r.Context(), id)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleIPPolicies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		list, _ := s.storage.ListIPPolicies(r.Context())
		json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var p core.IPPolicy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("ip_%d", time.Now().UnixNano())
		}
		_ = s.storage.SaveIPPolicy(r.Context(), &p)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleIPPolicyByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/ip-rules/")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodDelete {
		_ = s.storage.DeleteIPPolicy(r.Context(), id)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleCachePolicies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		list, _ := s.storage.ListCachePolicies(r.Context())
		json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var p core.CachePolicy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("cp_%d", time.Now().UnixNano())
		}
		_ = s.storage.SaveCachePolicy(r.Context(), &p)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCachePolicyByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/cache-policies/")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodDelete {
		_ = s.storage.DeleteCachePolicy(r.Context(), id)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	overview := s.analytics.GetOverview(r.Context())
	json.NewEncoder(w).Encode(overview)
}

func (s *Server) handleAnalyticsLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logs := s.analytics.GetRecentLogs(50)
	json.NewEncoder(w).Encode(logs)
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logs, _ := s.storage.ListAuditLogs(r.Context(), 50)
	json.NewEncoder(w).Encode(logs)
}
