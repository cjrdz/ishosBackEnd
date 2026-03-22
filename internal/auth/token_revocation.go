package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

var revokedTokens = &tokenRevocationStore{expiresAt: make(map[string]time.Time)}

type tokenRevocationStore struct {
	mu        sync.Mutex
	expiresAt map[string]time.Time
}

// RevokeToken marks a token as invalid until its expiry (or a safe fallback TTL).
func RevokeToken(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}

	expiresAt := tokenExpiry(token)
	if expiresAt.Before(time.Now()) {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	revokedTokens.mu.Lock()
	defer revokedTokens.mu.Unlock()
	revokedTokens.cleanupLocked(time.Now())
	revokedTokens.expiresAt[token] = expiresAt
}

// IsTokenRevoked returns true when the token was explicitly revoked.
func IsTokenRevoked(token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}

	now := time.Now()
	revokedTokens.mu.Lock()
	defer revokedTokens.mu.Unlock()
	revokedTokens.cleanupLocked(now)

	expiresAt, ok := revokedTokens.expiresAt[token]
	return ok && expiresAt.After(now)
}

func (s *tokenRevocationStore) cleanupLocked(now time.Time) {
	for token, expiresAt := range s.expiresAt {
		if !expiresAt.After(now) {
			delete(s.expiresAt, token)
		}
	}
}

func tokenExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Now().Add(24 * time.Hour)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Now().Add(24 * time.Hour)
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil || claims.Exp <= 0 {
		return time.Now().Add(24 * time.Hour)
	}

	return time.Unix(claims.Exp, 0)
}
