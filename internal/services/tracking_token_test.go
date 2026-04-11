package services

import (
	"context"
	"testing"
	"time"
)

type memoryTrackingTokenStore struct {
	records map[string]trackingTokenRecord
	orders  map[string][]string
}

func newMemoryTrackingTokenStore() *memoryTrackingTokenStore {
	return &memoryTrackingTokenStore{
		records: make(map[string]trackingTokenRecord),
		orders:  make(map[string][]string),
	}
}

func (s *memoryTrackingTokenStore) Store(_ context.Context, orderNumber, tokenHash string, expiresAt time.Time) error {
	s.records[tokenHash] = trackingTokenRecord{ExpiresAt: expiresAt}
	s.orders[orderNumber] = append(s.orders[orderNumber], tokenHash)
	return nil
}

func (s *memoryTrackingTokenStore) Lookup(_ context.Context, orderNumber, tokenHash string) (*trackingTokenRecord, error) {
	for _, known := range s.orders[orderNumber] {
		if known == tokenHash {
			record := s.records[tokenHash]
			return &record, nil
		}
	}
	return nil, nil
}

func (s *memoryTrackingTokenStore) RevokeAllByOrder(_ context.Context, orderNumber string) error {
	now := time.Now().UTC()
	for _, tokenHash := range s.orders[orderNumber] {
		record := s.records[tokenHash]
		record.RevokedAt = &now
		s.records[tokenHash] = record
	}
	return nil
}

func TestTrackingTokenIssueAndVerify(t *testing.T) {
	t.Parallel()

	store := newMemoryTrackingTokenStore()
	svc := NewTrackingTokenService("test-secret", 30*24*time.Hour, store)
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)

	token, expiresAt, err := svc.Issue(context.Background(), "ORD-20260410-7544", now)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token == "" {
		t.Fatal("Issue() returned empty token")
	}
	if !expiresAt.After(now) {
		t.Fatal("Issue() returned non-future expiry")
	}

	if err := svc.Verify(context.Background(), "ORD-20260410-7544", token, now.Add(time.Hour)); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestTrackingTokenVerifyRejectsWrongOrder(t *testing.T) {
	t.Parallel()

	store := newMemoryTrackingTokenStore()
	svc := NewTrackingTokenService("test-secret", 30*24*time.Hour, store)
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)

	token, _, err := svc.Issue(context.Background(), "ORD-20260410-7544", now)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	err = svc.Verify(context.Background(), "ORD-20260410-9999", token, now.Add(time.Hour))
	if err != ErrInvalidTrackingToken {
		t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidTrackingToken)
	}
}

func TestTrackingTokenVerifyRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	store := newMemoryTrackingTokenStore()
	svc := NewTrackingTokenService("test-secret", 30*24*time.Hour, store)
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)

	token, expiresAt, err := svc.Issue(context.Background(), "ORD-20260410-7544", now)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	err = svc.Verify(context.Background(), "ORD-20260410-7544", token, expiresAt.Add(time.Second))
	if err != ErrExpiredTrackingToken {
		t.Fatalf("Verify() error = %v, want %v", err, ErrExpiredTrackingToken)
	}
}

func TestTrackingTokenRevokeAllByOrder(t *testing.T) {
	t.Parallel()

	store := newMemoryTrackingTokenStore()
	svc := NewTrackingTokenService("test-secret", 30*24*time.Hour, store)
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)

	token, _, err := svc.Issue(context.Background(), "ORD-20260410-7544", now)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if err := svc.RevokeAllByOrder(context.Background(), "ORD-20260410-7544"); err != nil {
		t.Fatalf("RevokeAllByOrder() error = %v", err)
	}

	err = svc.Verify(context.Background(), "ORD-20260410-7544", token, now.Add(time.Hour))
	if err != ErrInvalidTrackingToken {
		t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidTrackingToken)
	}
}
