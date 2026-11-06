package auth

import (
	"testing"
	"time"

	"github.com/gateflow/gateflow/internal/core"
)

func TestAPIKey_GenerationAndValidation(t *testing.T) {
	rawKey, entity, err := GenerateAPIKey("test-owner", "Test service key", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if rawKey == "" || entity.KeyPrefix == "" || entity.KeyHash == "" {
		t.Fatalf("Malformed generated key")
	}

	validator := NewValidator()

	// 1. Valid key with required scopes
	validated, err := validator.Validate(rawKey, entity, []string{"read"})
	if err != nil || validated == nil {
		t.Errorf("Validation failed for valid key: %v", err)
	}

	// 2. Missing required scope
	_, err = validator.Validate(rawKey, entity, []string{"admin"})
	if err == nil {
		t.Errorf("Expected error for missing admin scope, got nil")
	}

	// 3. Revoked key
	entity.Active = false
	_, err = validator.Validate(rawKey, entity, []string{"read"})
	if err == nil || err.Error() != "API key is revoked" {
		t.Errorf("Expected revoked error, got: %v", err)
	}
	entity.Active = true

	// 4. Expired key
	expiredTime := time.Now().Add(-1 * time.Hour)
	expiredKey := &core.APIKey{
		ID:        "exp_1",
		KeyHash:   entity.KeyHash,
		Active:    true,
		ExpiresAt: &expiredTime,
	}
	_, err = validator.Validate(rawKey, expiredKey, nil)
	if err == nil || err.Error() != "API key has expired" {
		t.Errorf("Expected expired error, got: %v", err)
	}

	// 5. Wrong key
	_, err = validator.Validate("wrong_key_1234567890", entity, nil)
	if err == nil {
		t.Errorf("Expected error for wrong key, got nil")
	}
}
