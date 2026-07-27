// tracking_token.go — Issues and verifies HMAC-signed, time-limited order tracking tokens.
package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
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

type TrackingAccess struct {
	Token     string
	ExpiresAt time.Time
	URL       string
}

type TrackingTokenService struct {
	secret      []byte
	ttl         time.Duration
	ttlResolver func(context.Context) time.Duration
	store       TrackingTokenStore
}

func NewTrackingTokenService(secret string, ttl time.Duration, store TrackingTokenStore) *TrackingTokenService {
	if ttl <= 0 {
		ttl = defaultTrackingTokenTTL
	}

	return &TrackingTokenService{secret: []byte(secret), ttl: ttl, store: store}
}

func (s *TrackingTokenService) SetTTLResolver(resolver func(context.Context) time.Duration) {
	s.ttlResolver = resolver
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
	// HMAC avoids storing raw tokens in the DB and guards against length-extension attacks.
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *TrackingTokenService) resolveTTL(ctx context.Context) time.Duration {
	// Resolution order: dynamic resolver → configured TTL → hardcoded 30-day default.
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

// MaskTrackingToken returns only the first 8 chars of a token for safe logging.
func MaskTrackingToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:8]
}
