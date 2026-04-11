package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	stdsql "database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidTrackingToken = errors.New("invalid tracking token")
	ErrExpiredTrackingToken = errors.New("expired tracking token")
)

const defaultTrackingTokenTTL = 30 * 24 * time.Hour

type trackingTokenStore interface {
	Store(ctx context.Context, orderNumber, tokenHash string, expiresAt time.Time) error
	Lookup(ctx context.Context, orderNumber, tokenHash string) (*trackingTokenRecord, error)
	RevokeAllByOrder(ctx context.Context, orderNumber string) error
}

type trackingTokenRecord struct {
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type sqlTrackingTokenStore struct {
	db *stdsql.DB
}

type TrackingAccess struct {
	Token     string
	ExpiresAt time.Time
	URL       string
}

type TrackingTokenService struct {
	secret      []byte
	ttl         time.Duration
	ttlResolver func(context.Context) time.Duration
	store       trackingTokenStore
}

func NewTrackingTokenService(secret string, ttl time.Duration, store trackingTokenStore) *TrackingTokenService {
	if ttl <= 0 {
		ttl = defaultTrackingTokenTTL
	}

	return &TrackingTokenService{secret: []byte(secret), ttl: ttl, store: store}
}

func (s *TrackingTokenService) SetTTLResolver(resolver func(context.Context) time.Duration) {
	s.ttlResolver = resolver
}

func EnsureTrackingTokenSchema(ctx context.Context, db *stdsql.DB) error {
	if db == nil {
		return nil
	}

	const query = `
CREATE TABLE IF NOT EXISTS order_tracking_tokens (
	token_hash TEXT PRIMARY KEY,
	order_number TEXT NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	revoked_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_order_tracking_tokens_order_number ON order_tracking_tokens (order_number);
CREATE INDEX IF NOT EXISTS idx_order_tracking_tokens_expires_at ON order_tracking_tokens (expires_at);
CREATE INDEX IF NOT EXISTS idx_order_tracking_tokens_revoked_at ON order_tracking_tokens (revoked_at);`

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure order_tracking_tokens schema: %w", err)
	}

	return nil
}

func NewSQLTrackingTokenStore(db *stdsql.DB) trackingTokenStore {
	if db == nil {
		return nil
	}
	return &sqlTrackingTokenStore{db: db}
}

func (s *TrackingTokenService) Issue(ctx context.Context, orderNumber string, now time.Time) (string, time.Time, error) {
	if len(s.secret) == 0 || s.store == nil {
		return "", time.Time{}, ErrInvalidTrackingToken
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate tracking token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := now.UTC().Add(s.resolveTTL(ctx))
	if err := s.store.Store(ctx, strings.TrimSpace(orderNumber), s.hashToken(token), expiresAt); err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

func (s *TrackingTokenService) Verify(ctx context.Context, orderNumber, token string, now time.Time) error {
	if len(s.secret) == 0 || s.store == nil {
		return ErrInvalidTrackingToken
	}

	token = strings.TrimSpace(token)
	orderNumber = strings.TrimSpace(orderNumber)
	if token == "" || orderNumber == "" {
		return ErrInvalidTrackingToken
	}

	record, err := s.store.Lookup(ctx, orderNumber, s.hashToken(token))
	if err != nil {
		return err
	}
	if record == nil || record.RevokedAt != nil {
		return ErrInvalidTrackingToken
	}
	if now.UTC().After(record.ExpiresAt.UTC()) {
		return ErrExpiredTrackingToken
	}

	return nil
}

func (s *TrackingTokenService) RevokeAllByOrder(ctx context.Context, orderNumber string) error {
	if s.store == nil {
		return nil
	}
	return s.store.RevokeAllByOrder(ctx, strings.TrimSpace(orderNumber))
}

func (s *TrackingTokenService) BuildTrackingURL(baseURL, orderNumber, token string) string {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	trimmedOrderNumber := strings.TrimSpace(orderNumber)
	trimmedToken := strings.TrimSpace(token)
	if trimmedBaseURL == "" || trimmedOrderNumber == "" || trimmedToken == "" {
		return ""
	}

	return fmt.Sprintf(
		"%s/order/tracking?order=%s&token=%s",
		trimmedBaseURL,
		url.QueryEscape(trimmedOrderNumber),
		url.QueryEscape(trimmedToken),
	)
}

func (s *TrackingTokenService) hashToken(token string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *TrackingTokenService) resolveTTL(ctx context.Context) time.Duration {
	if s.ttlResolver != nil {
		if resolved := s.ttlResolver(ctx); resolved > 0 {
			return resolved
		}
	}

	if s.ttl > 0 {
		return s.ttl
	}

	return defaultTrackingTokenTTL
}

func (s *sqlTrackingTokenStore) Store(ctx context.Context, orderNumber, tokenHash string, expiresAt time.Time) error {
	const cleanupQuery = `
DELETE FROM order_tracking_tokens WHERE expires_at <= NOW()`
	const insertQuery = `
INSERT INTO order_tracking_tokens (token_hash, order_number, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (token_hash) DO UPDATE
SET order_number = EXCLUDED.order_number,
	expires_at = EXCLUDED.expires_at,
	issued_at = NOW(),
	revoked_at = NULL;`

	if _, err := s.db.ExecContext(ctx, cleanupQuery); err != nil {
		return fmt.Errorf("cleanup expired tracking tokens: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, insertQuery, tokenHash, orderNumber, expiresAt.UTC()); err != nil {
		return fmt.Errorf("store tracking token: %w", err)
	}
	return nil
}

func (s *sqlTrackingTokenStore) Lookup(ctx context.Context, orderNumber, tokenHash string) (*trackingTokenRecord, error) {
	const query = `
SELECT expires_at, revoked_at
FROM order_tracking_tokens
WHERE order_number = $1 AND token_hash = $2
LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, orderNumber, tokenHash)
	var record trackingTokenRecord
	var revokedAt stdsql.NullTime
	if err := row.Scan(&record.ExpiresAt, &revokedAt); err != nil {
		if errors.Is(err, stdsql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup tracking token: %w", err)
	}
	if revokedAt.Valid {
		record.RevokedAt = &revokedAt.Time
	}
	return &record, nil
}

func (s *sqlTrackingTokenStore) RevokeAllByOrder(ctx context.Context, orderNumber string) error {
	const query = `
UPDATE order_tracking_tokens
SET revoked_at = NOW()
WHERE order_number = $1 AND revoked_at IS NULL`

	if _, err := s.db.ExecContext(ctx, query, orderNumber); err != nil {
		return fmt.Errorf("revoke tracking tokens: %w", err)
	}
	return nil
}
