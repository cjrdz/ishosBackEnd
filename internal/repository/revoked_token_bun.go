// revoked_token_bun.go — Bun-backed token revocation store.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// RevokedTokenBunStore is a Bun-backed token revocation store.
type RevokedTokenBunStore struct {
	db bun.IDB
}

// NewRevokedTokenBunStore creates a new Bun-backed revoked token store.
func NewRevokedTokenBunStore(db bun.IDB) *RevokedTokenBunStore {
	return &RevokedTokenBunStore{db: db}
}

// RevokeToken marks a token hash as revoked in the database.
func (s *RevokedTokenBunStore) RevokeToken(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	if s.db == nil {
		return nil
	}
	if tokenHash == "" {
		return nil
	}

	var existing models.RevokedToken
	err := s.db.NewSelect().Model(&existing).Where("token_hash = ?", tokenHash).Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query existing token (bun): %w", err)
	}

	if err == nil {
		existing.ExpiresAt = expiresAt.UTC()
		if _, err := s.db.NewUpdate().Model(&existing).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update revoked token (bun): %w", err)
		}
	} else {
		row := models.RevokedToken{
			TokenHash: tokenHash,
			ExpiresAt: expiresAt.UTC(),
			RevokedAt: time.Now().UTC(),
		}
		if _, err := s.db.NewInsert().Model(&row).Exec(ctx); err != nil {
			return fmt.Errorf("create revoked token (bun): %w", err)
		}
	}

	return nil
}

// IsTokenRevoked checks if a token hash is revoked.
func (s *RevokedTokenBunStore) IsTokenRevoked(ctx context.Context, tokenHash string, now time.Time) (bool, error) {
	if s.db == nil {
		return false, nil
	}
	if tokenHash == "" {
		return false, nil
	}

	exists, err := s.db.NewSelect().Model((*models.RevokedToken)(nil)).Where("token_hash = ? AND expires_at > ?", tokenHash, now.UTC()).Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check token revocation (bun): %w", err)
	}

	return exists, nil
}

// CleanupExpired deletes revoked token rows whose expires_at has passed.
func (s *RevokedTokenBunStore) CleanupExpired(ctx context.Context, now time.Time) (int64, error) {
	if s.db == nil {
		return 0, nil
	}

	res, err := s.db.NewDelete().Model((*models.RevokedToken)(nil)).Where("expires_at <= ?", now.UTC()).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired revoked tokens (bun): %w", err)
	}

	return res.RowsAffected()
}
