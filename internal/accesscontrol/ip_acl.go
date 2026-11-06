package accesscontrol

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gateflow/gateflow/internal/core"
)

// TrustedProxyManager parses and resolves client IP based on trusted proxy configuration
type TrustedProxyManager struct {
	trustedNetworks []*net.IPNet
	trustCloudflare bool
	cloudflareIPs   []*net.IPNet
}

// Default Cloudflare published IP ranges (IPv4 and IPv6)
var defaultCloudflareCIDRs = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

// NewTrustedProxyManager creates a new proxy trust evaluator
func NewTrustedProxyManager(trustedCIDRs []string, trustCloudflare bool) (*TrustedProxyManager, error) {
	mgr := &TrustedProxyManager{
		trustCloudflare: trustCloudflare,
	}

	for _, cidr := range trustedCIDRs {
		_, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			// Might be a single IP
			ip := net.ParseIP(strings.TrimSpace(cidr))
			if ip == nil {
				return nil, fmt.Errorf("invalid trusted CIDR/IP: %s", cidr)
			}
			mask := net.CIDRMask(32, 32)
			if ip.To4() == nil {
				mask = net.CIDRMask(128, 128)
			}
			ipNet = &net.IPNet{IP: ip, Mask: mask}
		}
		mgr.trustedNetworks = append(mgr.trustedNetworks, ipNet)
	}

	if trustCloudflare {
		for _, cidr := range defaultCloudflareCIDRs {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil {
				mgr.cloudflareIPs = append(mgr.cloudflareIPs, ipNet)
			}
		}
	}

	return mgr, nil
}

// IsTrustedProxy checks if an IP is in the trusted network list
func (m *TrustedProxyManager) IsTrustedProxy(ip net.IP) bool {
	for _, network := range m.trustedNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	if m.trustCloudflare {
		for _, network := range m.cloudflareIPs {
			if network.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// GetClientIP securely resolves client IP preventing header spoofing
func (m *TrustedProxyManager) GetClientIP(r *http.Request) (net.IP, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	remoteIP := net.ParseIP(strings.TrimSpace(host))
	if remoteIP == nil {
		return nil, errors.New("unable to parse RemoteAddr IP")
	}

	// If remote address is not a trusted proxy, RemoteAddr is the only trustworthy source
	if !m.IsTrustedProxy(remoteIP) {
		return remoteIP, nil
	}

	// RemoteAddr is trusted! Now inspect Cloudflare header if enabled
	if m.trustCloudflare {
		cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
		if cfIP != "" {
			parsedCF := net.ParseIP(cfIP)
			if parsedCF != nil {
				return parsedCF, nil
			}
		}
	}

	// Inspect X-Forwarded-For if from trusted proxy
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		// Right-to-left evaluation or leftmost client IP
		// The first IP is the original client
		clientCandidate := strings.TrimSpace(parts[0])
		parsed := net.ParseIP(clientCandidate)
		if parsed != nil {
			return parsed, nil
		}
	}

	return remoteIP, nil
}

// IPChecker validates whether an IP is allowed under an IPPolicy
type IPChecker struct{}

// NewIPChecker creates a new IPChecker
func NewIPChecker() *IPChecker {
	return &IPChecker{}
}

// Check evaluates whether the given IP is permitted by policy
func (c *IPChecker) Check(policy *core.IPPolicy, clientIP net.IP) (bool, error) {
	if policy == nil {
		return true, nil // No policy means allowed
	}

	// Evaluate explicit CIDR rules in order
	for _, rule := range policy.CIDRs {
		_, ipNet, err := net.ParseCIDR(strings.TrimSpace(rule.CIDR))
		if err != nil {
			// Single IP check
			single := net.ParseIP(strings.TrimSpace(rule.CIDR))
			if single != nil && single.Equal(clientIP) {
				return rule.Action == core.IPRuleAllow, nil
			}
			continue
		}

		if ipNet.Contains(clientIP) {
			return rule.Action == core.IPRuleAllow, nil
		}
	}

	// Fallback to default rule
	return policy.DefaultRule == core.IPRuleAllow, nil
}
