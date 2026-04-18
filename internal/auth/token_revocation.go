// token_revocation.go — Tracks explicitly revoked JWTs using a pluggable backend.
// Defaults to an in-memory store; call InitTokenRevocationStore to switch to Postgres.
// RevokeToken and IsTokenRevoked are the only public entry points.
package auth

import (
	"context"
	"crypto/sha256"
	stdsql "database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

type tokenRevocationBackend interface {
	RevokeToken(ctx context.Context, token string, expiresAt time.Time) error
	IsTokenRevoked(ctx context.Context, token string, now time.Time) (bool, error)
}

// tokenRevocationHolder wraps the backend so atomic.Value always stores the same
// concrete type regardless of which backend implementation is active.
type tokenRevocationHolder struct {
	backend tokenRevocationBackend
}

var revokedTokens atomic.Value

func init() {
	setTokenRevocationBackend(newMemoryTokenRevocationStore())
}

type memoryTokenRevocationStore struct {
	mu        sync.Mutex
	expiresAt map[string]time.Time
}

type postgresTokenRevocationStore struct {
	db *stdsql.DB
}

func newMemoryTokenRevocationStore() *memoryTokenRevocationStore {
	return &memoryTokenRevocationStore{expiresAt: make(map[string]time.Time)}
}

func InitTokenRevocationStore(db *stdsql.DB) error {
	if db == nil {
		return nil
	}

	store := &postgresTokenRevocationStore{db: db}
	if err := store.ensureSchema(context.Background()); err != nil {
		return err
	}
	setTokenRevocationBackend(store)
	return nil
}

func setTokenRevocationBackend(backend tokenRevocationBackend) {
	revokedTokens.Store(tokenRevocationHolder{backend: backend})
}

// RevokeToken marks a token as invalid until its expiry (or a safe fallback TTL).
func RevokeToken(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}

	expiresAt := tokenExpiry(token)
	// token is already expired but may still be in-flight; keep it blocked for 24 h to cover clock skew.
	if expiresAt.Before(time.Now()) {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	store := revokedTokens.Load().(tokenRevocationHolder).backend
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := store.RevokeToken(ctx, token, expiresAt); err != nil {
		log.Error().Err(err).Msg("Auth token revocation: failed to persist token revocation")
	}
}

// IsTokenRevoked returns true when the token was explicitly revoked.
func IsTokenRevoked(token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}

	now := time.Now()
	store := revokedTokens.Load().(tokenRevocationHolder).backend
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	revoked, err := store.IsTokenRevoked(ctx, token, now)
	if err != nil {
		log.Error().Err(err).Msg("Auth token revocation: failed to check token status")
		return false
	}
	return revoked
}

func (s *memoryTokenRevocationStore) RevokeToken(_ context.Context, token string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.expiresAt[tokenHash(token)] = expiresAt
	return nil
}

func (s *memoryTokenRevocationStore) IsTokenRevoked(_ context.Context, token string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	expiresAt, ok := s.expiresAt[tokenHash(token)]
	return ok && expiresAt.After(now), nil
}

// cleanupLocked removes expired entries; caller must hold mu.
func (s *memoryTokenRevocationStore) cleanupLocked(now time.Time) {
	for token, expiresAt := range s.expiresAt {
		if !expiresAt.After(now) {
			delete(s.expiresAt, token)
		}
	}
}

func (s *postgresTokenRevocationStore) ensureSchema(ctx context.Context) error {
	const query = `
CREATE TABLE IF NOT EXISTS auth_revoked_tokens (
	token_hash TEXT PRIMARY KEY,
	expires_at TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_auth_revoked_tokens_expires_at ON auth_revoked_tokens (expires_at);`

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure auth_revoked_tokens schema: %w", err)
	}
	return nil
}

func (s *postgresTokenRevocationStore) RevokeToken(ctx context.Context, token string, expiresAt time.Time) error {
	hash := tokenHash(token)
	if hash == "" {
		return nil
	}

	const cleanupQuery = `
DELETE FROM auth_revoked_tokens WHERE expires_at <= NOW()`
	const insertQuery = `
INSERT INTO auth_revoked_tokens (token_hash, expires_at)
VALUES ($1, $2)
ON CONFLICT (token_hash) DO UPDATE SET expires_at = EXCLUDED.expires_at, revoked_at = NOW();`

	// purge expired rows first so the table doesn't grow unboundedly before the insert.
	if _, err := s.db.ExecContext(ctx, cleanupQuery); err != nil {
		return fmt.Errorf("cleanup auth token revocations: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, insertQuery, hash, expiresAt.UTC()); err != nil {
		return fmt.Errorf("revoke auth token: %w", err)
	}
	return nil
}

func (s *postgresTokenRevocationStore) IsTokenRevoked(ctx context.Context, token string, now time.Time) (bool, error) {
	hash := tokenHash(token)
	if hash == "" {
		return false, nil
	}

	const query = `
SELECT EXISTS (
	SELECT 1 FROM auth_revoked_tokens
	WHERE token_hash = $1 AND expires_at > $2
)`

	var revoked bool
	if err := s.db.QueryRowContext(ctx, query, hash, now.UTC()).Scan(&revoked); err != nil {
		return false, fmt.Errorf("check auth token revocation: %w", err)
	}
	return revoked, nil
}

func tokenHash(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

// tokenExpiry extracts the exp claim from a JWT without verifying its signature.
// Falls back to 24 h when the token is malformed or has no parseable expiry.
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
