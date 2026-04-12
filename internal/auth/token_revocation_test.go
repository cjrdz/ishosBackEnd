package auth

import (
	"context"
	"testing"
	"time"
)

type fakeTokenRevocationBackend struct{}

func (fakeTokenRevocationBackend) RevokeToken(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (fakeTokenRevocationBackend) IsTokenRevoked(_ context.Context, _ string, _ time.Time) (bool, error) {
	return false, nil
}

func TestSetTokenRevocationBackendAllowsDifferentImplementations(t *testing.T) {
	setTokenRevocationBackend(newMemoryTokenRevocationStore())

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("setTokenRevocationBackend panicked when swapping implementations: %v", recovered)
		}
	}()

	setTokenRevocationBackend(fakeTokenRevocationBackend{})

	loaded := revokedTokens.Load().(tokenRevocationHolder)
	if loaded.backend == nil {
		t.Fatal("expected backend to be set")
	}
}
