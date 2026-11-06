package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPPort        string
	AdminPort       string
	AdminKey        string
	DatabaseURL     string
	RedisURL        string
	TrustedProxies  []string
	TrustCloudflare bool
	AllowLocalDev   bool
}

func LoadConfig() *Config {
	httpPort := getEnv("PORT", getEnv("GATEFLOW_PORT", "8080"))
	adminPort := getEnv("GATEFLOW_ADMIN_PORT", "8081")
	adminKey := getEnv("GATEFLOW_ADMIN_KEY", "admin-secret-gateflow")
	databaseURL := getEnv("DATABASE_URL", "")
	redisURL := getEnv("REDIS_URL", "")
	trustCF := getEnv("GATEFLOW_TRUST_CLOUDFLARE", "true") == "true"
	allowLocal := getEnv("GATEFLOW_ALLOW_LOCAL_DEV", "true") == "true"

	proxiesStr := getEnv("GATEFLOW_TRUSTED_PROXIES", "127.0.0.1,::1")
	var proxies []string
	if proxiesStr != "" {
		for _, p := range strings.Split(proxiesStr, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				proxies = append(proxies, trimmed)
			}
		}
	}

	return &Config{
		HTTPPort:        httpPort,
		AdminPort:       adminPort,
		AdminKey:        adminKey,
		DatabaseURL:     databaseURL,
		RedisURL:        redisURL,
		TrustedProxies:  proxies,
		TrustCloudflare: trustCF,
		AllowLocalDev:   allowLocal,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
