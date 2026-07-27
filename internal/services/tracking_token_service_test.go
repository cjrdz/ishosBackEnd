package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type trackingTokenStoreStub struct {
	storeFn            func(ctx context.Context, orderNumber, tokenHash string, expiresAt time.Time) error
	lookupFn           func(ctx context.Context, orderNumber, tokenHash string) (*dto.TrackingTokenRecord, error)
	revokeAllByOrderFn func(ctx context.Context, orderNumber string) error
}

func (s *trackingTokenStoreStub) Store(ctx context.Context, orderNumber, tokenHash string, expiresAt time.Time) error {
	if s.storeFn != nil {
		return s.storeFn(ctx, orderNumber, tokenHash, expiresAt)
	}
	return nil
}

func (s *trackingTokenStoreStub) Lookup(ctx context.Context, orderNumber, tokenHash string) (*dto.TrackingTokenRecord, error) {
	if s.lookupFn != nil {
		return s.lookupFn(ctx, orderNumber, tokenHash)
	}
	return nil, nil
}

func (s *trackingTokenStoreStub) RevokeAllByOrder(ctx context.Context, orderNumber string) error {
	if s.revokeAllByOrderFn != nil {
		return s.revokeAllByOrderFn(ctx, orderNumber)
	}
	return nil
}

func TestTrackingTokenService_Issue_StoresHash(t *testing.T) {
	stored := false
	store := &trackingTokenStoreStub{
		storeFn: func(_ context.Context, orderNumber, tokenHash string, expiresAt time.Time) error {
			stored = true
			if orderNumber != "ORD-123" {
				t.Fatalf("unexpected order number: %s", orderNumber)
			}
			if tokenHash == "" {
				t.Fatal("expected non-empty token hash")
			}
			return nil
		},
	}
	svc := NewTrackingTokenService("secret", 0, store)

	token, expiresAt, err := svc.Issue(context.Background(), "ORD-123", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiresAt.IsZero() {
		t.Fatal("expected expiresAt to be set")
	}
	if !stored {
		t.Fatal("expected store to be called")
	}
}

func TestTrackingTokenService_Verify_Missing(t *testing.T) {
	store := &trackingTokenStoreStub{
		lookupFn: func(_ context.Context, _, _ string) (*dto.TrackingTokenRecord, error) {
			return nil, nil
		},
	}
	svc := NewTrackingTokenService("secret", time.Hour, store)

	err := svc.Verify(context.Background(), "ORD-123", "token", time.Now())
	if !errors.Is(err, ErrInvalidTrackingToken) {
		t.Fatalf("expected ErrInvalidTrackingToken, got %v", err)
	}
}

func TestTrackingTokenService_Verify_Expired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	store := &trackingTokenStoreStub{
		lookupFn: func(_ context.Context, _, _ string) (*dto.TrackingTokenRecord, error) {
			return &dto.TrackingTokenRecord{ExpiresAt: past}, nil
		},
	}
	svc := NewTrackingTokenService("secret", time.Hour, store)

	err := svc.Verify(context.Background(), "ORD-123", "token", time.Now())
	if !errors.Is(err, ErrExpiredTrackingToken) {
		t.Fatalf("expected ErrExpiredTrackingToken, got %v", err)
	}
}

func TestTrackingTokenService_Verify_Revoked(t *testing.T) {
	revokedAt := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Hour)
	store := &trackingTokenStoreStub{
		lookupFn: func(_ context.Context, _, _ string) (*dto.TrackingTokenRecord, error) {
			return &dto.TrackingTokenRecord{ExpiresAt: future, RevokedAt: &revokedAt}, nil
		},
	}
	svc := NewTrackingTokenService("secret", time.Hour, store)

	err := svc.Verify(context.Background(), "ORD-123", "token", time.Now())
	if !errors.Is(err, ErrInvalidTrackingToken) {
		t.Fatalf("expected ErrInvalidTrackingToken, got %v", err)
	}
}

func TestTrackingTokenService_Verify_Valid(t *testing.T) {
	future := time.Now().Add(time.Hour)
	store := &trackingTokenStoreStub{
		lookupFn: func(_ context.Context, _, _ string) (*dto.TrackingTokenRecord, error) {
			return &dto.TrackingTokenRecord{ExpiresAt: future}, nil
		},
	}
	svc := NewTrackingTokenService("secret", time.Hour, store)

	err := svc.Verify(context.Background(), "ORD-123", "token", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTrackingTokenService_BuildTrackingURL(t *testing.T) {
	svc := NewTrackingTokenService("secret", time.Hour, nil)

	url := svc.BuildTrackingURL("https://store.example.com", "ORD-123", "abc/def")
	expected := "https://store.example.com/order/tracking?order=ORD-123&token=abc%2Fdef"
	if url != expected {
		t.Fatalf("url = %q, want %q", url, expected)
	}
}
