// tracking_token_store.go — Ent-backed store for order tracking tokens.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/ordertrackingtoken"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/rs/zerolog/log"
)

// TrackingTokenStore implements the services.TrackingTokenStore contract using Ent.
type TrackingTokenStore struct {
	client *ent.Client
}

// NewTrackingTokenStore creates a new Ent-backed tracking token store.
func NewTrackingTokenStore(client *ent.Client) *TrackingTokenStore {
	return &TrackingTokenStore{client: client}
}

// Store saves a tracking token in the database.
// Uses upsert for duplicate handling.
func (s *TrackingTokenStore) Store(ctx context.Context, orderNumber, tokenHash string, expiresAt time.Time) error {
	if tokenHash == "" || orderNumber == "" {
		return nil
	}

	// Check if token already exists
	existing, err := s.client.OrderTrackingToken.
		Query().
		Where(ordertrackingtoken.TokenHashEQ(tokenHash)).
		First(ctx)

	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("query existing tracking token (ent): %w", err)
	}

	// Upsert: update if exists, create if new
	if existing != nil {
		_, err = s.client.OrderTrackingToken.
			UpdateOne(existing).
			SetOrderNumber(orderNumber).
			SetExpiresAt(expiresAt.UTC()).
			ClearRevokedAt().
			Save(ctx)
		if err != nil {
			return fmt.Errorf("update tracking token (ent): %w", err)
		}
	} else {
		_, err = s.client.OrderTrackingToken.
			Create().
			SetTokenHash(tokenHash).
			SetOrderNumber(orderNumber).
			SetExpiresAt(expiresAt.UTC()).
			SetIssuedAt(time.Now().UTC()).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create tracking token (ent): %w", err)
		}
	}

	return nil
}

// Lookup retrieves a tracking token by order number and hash.
func (s *TrackingTokenStore) Lookup(ctx context.Context, orderNumber, tokenHash string) (*dto.TrackingTokenRecord, error) {
	if tokenHash == "" || orderNumber == "" {
		return nil, nil
	}

	token, err := s.client.OrderTrackingToken.
		Query().
		Where(
			ordertrackingtoken.OrderNumberEQ(orderNumber),
			ordertrackingtoken.TokenHashEQ(tokenHash),
		).
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup tracking token (ent): %w", err)
	}

	record := &dto.TrackingTokenRecord{
		ExpiresAt: token.ExpiresAt,
	}

	if !token.RevokedAt.IsZero() {
		record.RevokedAt = &token.RevokedAt
	}

	return record, nil
}

// RevokeAllByOrder marks all tracking tokens for an order as revoked.
func (s *TrackingTokenStore) RevokeAllByOrder(ctx context.Context, orderNumber string) error {
	if orderNumber == "" {
		return nil
	}

	now := time.Now().UTC()
	err := s.client.OrderTrackingToken.
		Update().
		Where(
			ordertrackingtoken.OrderNumberEQ(orderNumber),
			ordertrackingtoken.RevokedAtIsNil(),
		).
		SetRevokedAt(now).
		Exec(ctx)

	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("revoke tracking tokens (ent): %w", err)
	}

	return nil
}

// InitTrackingTokenStore verifies the tracking token store can be used.
// Call this during server initialization for symmetry with the previous Ent setup.
func InitTrackingTokenStore(client *ent.Client) error {
	if client == nil {
		return errors.New("client cannot be nil")
	}

	log.Info().Msg("Tracking token store initialized with Ent ORM")
	return nil
}

// CleanupExpired removes tracking tokens whose ExpiresAt has passed.
func (s *TrackingTokenStore) CleanupExpired(ctx context.Context) error {
	_, err := s.client.OrderTrackingToken.
		Delete().
		Where(ordertrackingtoken.ExpiresAtLTE(time.Now().UTC())).
		Exec(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("cleanup expired tracking tokens (ent): %w", err)
	}
	return nil
}
