// token_revocation_ent.go — Ent-based token revocation store (replaces raw SQL).
// Uses the RevokedToken entity to track revoked JWTs with Ent ORM for SQL safety.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/revokedtoken"
	"github.com/rs/zerolog/log"
)

// entTokenRevocationStore is an Ent-based token revocation store.
type entTokenRevocationStore struct {
	client *ent.Client
}

// NewEntTokenRevocationStore creates a new Ent-based token revocation store.
func NewEntTokenRevocationStore(client *ent.Client) tokenRevocationBackend {
	return &entTokenRevocationStore{client: client}
}

// RevokeToken marks a token as revoked in the database using Ent ORM.
func (s *entTokenRevocationStore) RevokeToken(ctx context.Context, token string, expiresAt time.Time) error {
	if token == "" {
		return nil
	}

	hash := tokenHash(token)
	if hash == "" {
		return nil
	}

	// First, check if token already exists
	existing, err := s.client.RevokedToken.
		Query().
		Where(revokedtoken.TokenHashEQ(hash)).
		First(ctx)

	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("query existing token (ent): %w", err)
	}

	// If exists, update expires_at; otherwise create new
	if existing != nil {
		_, err = s.client.RevokedToken.
			UpdateOne(existing).
			SetExpiresAt(expiresAt.UTC()).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("update revoked token (ent): %w", err)
		}
	} else {
		_, err = s.client.RevokedToken.
			Create().
			SetTokenHash(hash).
			SetExpiresAt(expiresAt.UTC()).
			SetRevokedAt(time.Now().UTC()).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create revoked token (ent): %w", err)
		}
	}

	return nil
}

// IsTokenRevoked checks if a token is revoked using Ent ORM.
func (s *entTokenRevocationStore) IsTokenRevoked(ctx context.Context, token string, now time.Time) (bool, error) {
	if token == "" {
		return false, nil
	}

	hash := tokenHash(token)
	if hash == "" {
		return false, nil
	}

	// Query: SELECT EXISTS(... WHERE token_hash = $1 AND expires_at > $2)
	exists, err := s.client.RevokedToken.
		Query().
		Where(
			revokedtoken.TokenHashEQ(hash),
			revokedtoken.ExpiresAtGT(now.UTC()),
		).
		Exist(ctx)

	if err != nil {
		return false, fmt.Errorf("check token revocation (ent): %w", err)
	}

	return exists, nil
}

// CleanupExpiredTokens removes tokens that have already expired.
// This is a maintenance operation called periodically to keep the table small.
func (s *entTokenRevocationStore) CleanupExpiredTokens(ctx context.Context) error {
	deleted, err := s.client.RevokedToken.
		Delete().
		Where(revokedtoken.ExpiresAtLTE(time.Now().UTC())).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("cleanup expired tokens (ent): %w", err)
	}

	if deleted > 0 {
		log.Debug().Int("deleted", deleted).Msg("Cleaned up expired revoked tokens")
	}

	return nil
}

// InitEntTokenRevocationStore initializes the token revocation store to use Ent ORM.
// This replaces the raw SQL store with a type-safe Ent-based implementation.
func InitEntTokenRevocationStore(client *ent.Client) error {
	if client == nil {
		return nil
	}

	// Create the store and set it as the active backend
	store := NewEntTokenRevocationStore(client)
	setTokenRevocationBackend(store)

	log.Info().Msg("Token revocation store initialized with Ent ORM")
	return nil
}
