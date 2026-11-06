package accesscontrol

import (
	"net"
	"net/http"
	"testing"

	"github.com/gateflow/gateflow/internal/core"
)

func TestTrustedProxy_SpoofingProtection(t *testing.T) {
	// Only 10.0.0.1 is trusted proxy
	pm, err := NewTrustedProxyManager([]string{"10.0.0.1"}, true)
	if err != nil {
		t.Fatalf("NewTrustedProxyManager failed: %v", err)
	}

	// 1. Direct connection from untrusted IP trying to spoof X-Forwarded-For and CF-Connecting-IP
	req, _ := http.NewRequest("GET", "http://gateway/test", nil)
	req.RemoteAddr = "203.0.113.195:12345" // Untrusted client
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	req.Header.Set("CF-Connecting-IP", "1.1.1.1")

	clientIP, err := pm.GetClientIP(req)
	if err != nil {
		t.Fatalf("GetClientIP failed: %v", err)
	}

	// Must reject spoofed headers and strictly use RemoteAddr
	if clientIP.String() != "203.0.113.195" {
		t.Errorf("Expected RemoteAddr 203.0.113.195, got %s (header spoofing succeeded!)", clientIP.String())
	}

	// 2. Connection from trusted proxy forwarding client IP
	reqTrusted, _ := http.NewRequest("GET", "http://gateway/test", nil)
	reqTrusted.RemoteAddr = "10.0.0.1:4567" // Trusted proxy
	reqTrusted.Header.Set("X-Forwarded-For", "198.51.100.42, 10.0.0.1")

	clientIP2, err := pm.GetClientIP(reqTrusted)
	if err != nil {
		t.Fatalf("GetClientIP failed: %v", err)
	}
	if clientIP2.String() != "198.51.100.42" {
		t.Errorf("Expected forwarded client 198.51.100.42, got %s", clientIP2.String())
	}
}

func TestIPChecker_CIDRMatching(t *testing.T) {
	checker := NewIPChecker()

	policy := &core.IPPolicy{
		ID:          "policy_1",
		DefaultRule: core.IPRuleDeny,
		CIDRs: []core.CIDRRule{
			{CIDR: "192.168.1.0/24", Action: core.IPRuleAllow},
			{CIDR: "10.10.10.5", Action: core.IPRuleAllow},
		},
	}

	// Allowed CIDR
	allowed, _ := checker.Check(policy, net.ParseIP("192.168.1.55"))
	if !allowed {
		t.Errorf("Expected 192.168.1.55 to be allowed")
	}

	// Allowed Single IP
	allowed, _ = checker.Check(policy, net.ParseIP("10.10.10.5"))
	if !allowed {
		t.Errorf("Expected 10.10.10.5 to be allowed")
	}

	// Denied IP (fallback to DefaultRule)
	allowed, _ = checker.Check(policy, net.ParseIP("172.16.0.1"))
	if allowed {
		t.Errorf("Expected 172.16.0.1 to be denied")
	}
}
