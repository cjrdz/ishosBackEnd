// token_revocation.go — Token revocation wrapper and interface.
// Backend implementations (e.g. Bun-backed) satisfy TokenRevocationBackend.
// RevokeToken and IsTokenRevoked are the only public entry points.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// TokenRevocationBackend defines the interface for token revocation stores.
// Implementations operate on token hashes, not raw tokens.
type TokenRevocationBackend interface {
	RevokeToken(ctx context.Context, tokenHash string, expiresAt time.Time) error
	IsTokenRevoked(ctx context.Context, tokenHash string, now time.Time) (bool, error)
}

// tokenRevocationHolder wraps the backend so atomic.Value always stores the same concrete type.
type tokenRevocationHolder struct {
	backend TokenRevocationBackend
}

var revokedTokens atomic.Value

func init() {
	revokedTokens.Store(tokenRevocationHolder{backend: nil})
}

// revocationCheckCache caches IsTokenRevoked results to avoid hitting PostgreSQL
// on every authenticated request. Tokens are rarely revoked, so negative results
// dominate. The cache is invalidated immediately when RevokeToken is called.
type revocationCheckCache struct {
	mu      sync.RWMutex
	entries map[string]revocationCacheEntry
}

type revocationCacheEntry struct {
	revoked  bool
	cachedAt time.Time
}

var revocationCache = &revocationCheckCache{
	entries: make(map[string]revocationCacheEntry),
}

const revocationCacheTTL = 10 * time.Second

func (c *revocationCheckCache) get(hash string) (revoked bool, ok bool) {
	c.mu.RLock()
	entry, exists := c.entries[hash]
	c.mu.RUnlock()
	if !exists || time.Since(entry.cachedAt) > revocationCacheTTL {
		return false, false
	}
	return entry.revoked, true
}

func (c *revocationCheckCache) set(hash string, revoked bool) {
	c.mu.Lock()
	c.entries[hash] = revocationCacheEntry{revoked: revoked, cachedAt: time.Now()}
	c.mu.Unlock()
}

// SetTokenRevocationBackend sets the active token revocation backend.
// Thread-safe; used during server wiring to inject the production store.
func SetTokenRevocationBackend(backend TokenRevocationBackend) {
	revokedTokens.Store(tokenRevocationHolder{backend: backend})
}

// RevokeToken marks a token as revoked.
// Called during logout; safe to call concurrently.
func RevokeToken(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}

	hash := tokenHash(token)

	// Immediately update the in-memory cache so subsequent checks fail fast.
	revocationCache.set(hash, true)

	expiresAt := tokenExpiry(token)
	// token is already expired but may still be in-flight; keep it blocked for 24h to cover clock skew
	if expiresAt.Before(time.Now()) {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	holder := revokedTokens.Load().(tokenRevocationHolder)
	if holder.backend == nil {
		log.Error().Msg("Auth token revocation: no backend configured")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := holder.backend.RevokeToken(ctx, hash, expiresAt); err != nil {
		log.Error().Err(err).Msg("Auth token revocation: failed to persist token revocation")
	}
}

// IsTokenRevoked returns true when the token was explicitly revoked.
// Called during token validation; fast path returns false for empty tokens.
// Uses an in-memory cache to avoid a PostgreSQL round-trip on every request.
// Fail-closed: returns true on store errors.
func IsTokenRevoked(token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}

	hash := tokenHash(token)
	if hash == "" {
		return false
	}

	// Check in-memory cache first.
	if revoked, ok := revocationCache.get(hash); ok {
		return revoked
	}

	now := time.Now()
	holder := revokedTokens.Load().(tokenRevocationHolder)
	if holder.backend == nil {
		log.Error().Msg("Auth token revocation: no backend configured, treating as revoked")
		return true // fail-closed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	revoked, err := holder.backend.IsTokenRevoked(ctx, hash, now)
	if err != nil {
		log.Error().Err(err).Msg("Auth token revocation: failed to check token status, treating as revoked")
		return true // fail-closed
	}

	// Cache the result so the next request avoids the DB round-trip.
	revocationCache.set(hash, revoked)
	return revoked
}

// tokenHash returns a SHA256 hex digest of the token.
// Used to store only hashes (not tokens) in the database.
func tokenHash(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

// tokenExpiry extracts the exp claim from a JWT without verifying the signature.
// Used to determine how long to keep a revoked token in the store.
// Returns (now + 24h) if parsing fails (safe default).
func tokenExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Now().Add(24 * time.Hour)
	}

	// Decode payload (second part)
	payload := parts[1]
	// Add padding if needed
	if pad := len(payload) % 4; pad != 0 {
		payload += strings.Repeat("=", 4-pad)
	}

	data, err := decodeBase64(payload)
	if err != nil {
		return time.Now().Add(24 * time.Hour)
	}

	// Parse JSON to extract "exp" field
	var claims struct {
		Exp int64 `json:"exp"`
	}

	// Use simple JSON parsing to avoid dependency on jwt library
	// This is safe since we're just extracting one field
	if err := parseJSONInt(data, "exp", &claims.Exp); err != nil {
		return time.Now().Add(24 * time.Hour)
	}

	if claims.Exp == 0 {
		return time.Now().Add(24 * time.Hour)
	}

	return time.Unix(claims.Exp, 0)
}

// decodeBase64 decodes a base64 string, handling standard and URL-safe variants.
func decodeBase64(s string) ([]byte, error) {
	// Try standard base64 first
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	// Fallback to URL-safe encoding
	return base64.URLEncoding.DecodeString(s)
}

// parseJSONInt extracts an integer value from JSON data.
// This is a simplified parser for extracting just the "exp" claim.
func parseJSONInt(data []byte, key string, dest *int64) error {
	// Simple string search for now; could use encoding/json for robustness
	s := string(data)
	prefix := `"` + key + `":`
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return nil
	}

	start := idx + len(prefix)
	end := start

	// Find end of number (comma or closing brace)
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}

	if start >= len(s) {
		return nil
	}

	numStr := s[start:end]
	n := int64(0)
	for _, ch := range numStr {
		n = n*10 + int64(ch-'0')
	}

	*dest = n
	return nil
}
