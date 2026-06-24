// tracking_token_bun.go — Bun-backed store for order tracking tokens.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// TrackingTokenBunStore implements the services.TrackingTokenStore contract using Bun.
type TrackingTokenBunStore struct {
	db bun.IDB
}

// NewTrackingTokenBunStore creates a new Bun-backed tracking token store.
func NewTrackingTokenBunStore(db bun.IDB) *TrackingTokenBunStore {
	return &TrackingTokenBunStore{db: db}
}

// Store saves a tracking token in the database.
// Uses upsert for duplicate handling.
func (s *TrackingTokenBunStore) Store(ctx context.Context, orderNumber, tokenHash string, expiresAt time.Time) error {
	if tokenHash == "" || orderNumber == "" {
		return nil
	}

	var existing models.OrderTrackingToken
	err := s.db.NewSelect().Model(&existing).Where("token_hash = ?", tokenHash).Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query existing tracking token (bun): %w", err)
	}

	if err == nil {
		existing.OrderNumber = orderNumber
		existing.ExpiresAt = expiresAt.UTC()
		existing.RevokedAt = nil
		if _, err := s.db.NewUpdate().Model(&existing).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update tracking token (bun): %w", err)
		}
	} else {
		row := models.OrderTrackingToken{
			TokenHash:   tokenHash,
			OrderNumber: orderNumber,
			ExpiresAt:   expiresAt.UTC(),
			IssuedAt:    time.Now().UTC(),
		}
		if _, err := s.db.NewInsert().Model(&row).Exec(ctx); err != nil {
			return fmt.Errorf("create tracking token (bun): %w", err)
		}
	}

	return nil
}

// Lookup retrieves a tracking token by order number and hash.
func (s *TrackingTokenBunStore) Lookup(ctx context.Context, orderNumber, tokenHash string) (*dto.TrackingTokenRecord, error) {
	if tokenHash == "" || orderNumber == "" {
		return nil, nil
	}

	var token models.OrderTrackingToken
	if err := s.db.NewSelect().Model(&token).Where("order_number = ? AND token_hash = ?", orderNumber, tokenHash).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup tracking token (bun): %w", err)
	}

	record := &dto.TrackingTokenRecord{
		ExpiresAt: token.ExpiresAt,
	}
	if token.RevokedAt != nil && !token.RevokedAt.IsZero() {
		record.RevokedAt = token.RevokedAt
	}
	return record, nil
}

// RevokeAllByOrder marks all tracking tokens for an order as revoked.
func (s *TrackingTokenBunStore) RevokeAllByOrder(ctx context.Context, orderNumber string) error {
	if orderNumber == "" {
		return nil
	}

	now := time.Now().UTC()
	if _, err := s.db.NewUpdate().Model((*models.OrderTrackingToken)(nil)).Set("revoked_at = ?", now).Where("order_number = ? AND revoked_at IS NULL", orderNumber).Exec(ctx); err != nil {
		return fmt.Errorf("revoke tracking tokens (bun): %w", err)
	}
	return nil
}

// InitTrackingTokenStore verifies the tracking token store can be used.
// Call this during server initialization for symmetry with the previous setup.
func InitTrackingTokenStore(db bun.IDB) error {
	if db == nil {
		return errors.New("db cannot be nil")
	}

	log.Info().Msg("Tracking token store initialized with Bun")
	return nil
}

// CleanupExpired removes tracking tokens whose ExpiresAt has passed.
func (s *TrackingTokenBunStore) CleanupExpired(ctx context.Context) error {
	if _, err := s.db.NewDelete().Model((*models.OrderTrackingToken)(nil)).Where("expires_at <= ?", time.Now().UTC()).Exec(ctx); err != nil {
		return fmt.Errorf("cleanup expired tracking tokens (bun): %w", err)
	}
	return nil
}
