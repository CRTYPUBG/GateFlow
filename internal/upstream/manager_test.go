package upstream

import (
	"testing"
)

func TestSSRFValidator_Blocking(t *testing.T) {
	// Disallow internal/private/loopback addresses
	validator := NewSSRFValidator(false)

	// 1. Loopback addresses must be blocked
	_, err := validator.ValidateTargetURL("http://127.0.0.1:8080/admin")
	if err == nil {
		t.Errorf("Expected loopback 127.0.0.1 to be blocked by SSRF validator")
	}

	// 2. Cloud metadata must be blocked
	_, err = validator.ValidateTargetURL("http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Errorf("Expected cloud metadata 169.254.169.254 to be blocked by SSRF validator")
	}

	// 3. Invalid scheme
	_, err = validator.ValidateTargetURL("ftp://malicious.com/test")
	if err == nil {
		t.Errorf("Expected non-http/https scheme to be rejected")
	}
}
