package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gateflow/gateflow/internal/core"
)

const (
	KeyPrefixLive = "gf_live_"
	KeyRandomBytes = 24
)

// GenerateAPIKey creates a new cryptographic API key and its hash
// Returns (rawKey, keyEntity, error). The rawKey is shown ONLY once to client!
func GenerateAPIKey(owner, description string, scopes []string, expiresAt *time.Time) (string, *core.APIKey, error) {
	bytes := make([]byte, KeyRandomBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	randomHex := hex.EncodeToString(bytes)
	rawKey := KeyPrefixLive + randomHex
	prefix := rawKey[:len(KeyPrefixLive)+8] // e.g. "gf_live_12345678"

	hasher := sha256.New()
	hasher.Write([]byte(rawKey))
	keyHash := hex.EncodeToString(hasher.Sum(nil))

	keyIDBytes := make([]byte, 8)
	rand.Read(keyIDBytes)
	keyID := "key_" + hex.EncodeToString(keyIDBytes)

	apiKey := &core.APIKey{
		ID:          keyID,
		KeyPrefix:   prefix,
		KeyHash:     keyHash,
		Owner:       owner,
		Description: description,
		Scopes:      scopes,
		Active:      true,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now().UTC(),
	}

	return rawKey, apiKey, nil
}

// HashKey computes the SHA-256 hex string of a raw API key
func HashKey(rawKey string) string {
	hasher := sha256.New()
	hasher.Write([]byte(rawKey))
	return hex.EncodeToString(hasher.Sum(nil))
}

// ExtractAPIKey extracts the key from standard headers
// Checks "X-API-Key" or "Authorization: Bearer <key>"
func ExtractAPIKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return strings.TrimSpace(key)
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

// Validator validates raw keys against known APIKeys in constant time
type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

// Validate checks raw key validity against stored key metadata
func (v *Validator) Validate(rawKey string, storedKey *core.APIKey, requiredScopes []string) (*core.APIKey, error) {
	if storedKey == nil {
		return nil, errors.New("invalid API key")
	}

	if !storedKey.Active {
		return nil, errors.New("API key is revoked")
	}

	if storedKey.ExpiresAt != nil && time.Now().UTC().After(*storedKey.ExpiresAt) {
		return nil, errors.New("API key has expired")
	}

	calculatedHash := HashKey(rawKey)
	if subtle.ConstantTimeCompare([]byte(calculatedHash), []byte(storedKey.KeyHash)) != 1 {
		return nil, errors.New("invalid API key")
	}

	// Validate scopes
	if len(requiredScopes) > 0 {
		keyScopes := make(map[string]bool)
		for _, s := range storedKey.Scopes {
			keyScopes[s] = true
			if s == "*" || s == "admin" {
				// Wildcard permission grants all scopes
				return storedKey, nil
			}
		}

		for _, req := range requiredScopes {
			if !keyScopes[req] {
				return nil, fmt.Errorf("insufficient scope: missing required scope '%s'", req)
			}
		}
	}

	return storedKey, nil
}
