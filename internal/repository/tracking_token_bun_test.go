// tracking_token_bun_test.go — Repository integration tests for the Bun-backed tracking token store.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackingTokenBunStore_StoreAndLookup(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	store := NewTrackingTokenBunStore(db)

	expires := time.Now().Add(time.Hour)
	require.NoError(t, store.Store(ctx, "ORD-001", "hash1", expires))

	record, err := store.Lookup(ctx, "ORD-001", "hash1")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.WithinDuration(t, expires.UTC(), record.ExpiresAt, time.Second)

	missing, err := store.Lookup(ctx, "ORD-001", "hash2")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestTrackingTokenBunStore_RevokeAllByOrder(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	store := NewTrackingTokenBunStore(db)

	require.NoError(t, store.Store(ctx, "ORD-002", "hash1", time.Now().Add(time.Hour)))
	require.NoError(t, store.Store(ctx, "ORD-002", "hash2", time.Now().Add(time.Hour)))
	require.NoError(t, store.Store(ctx, "ORD-003", "hash3", time.Now().Add(time.Hour)))

	require.NoError(t, store.RevokeAllByOrder(ctx, "ORD-002"))

	record1, err := store.Lookup(ctx, "ORD-002", "hash1")
	require.NoError(t, err)
	require.NotNil(t, record1.RevokedAt)

	record3, err := store.Lookup(ctx, "ORD-003", "hash3")
	require.NoError(t, err)
	assert.Nil(t, record3.RevokedAt)
}

func TestTrackingTokenBunStore_CleanupExpired(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	store := NewTrackingTokenBunStore(db)

	require.NoError(t, store.Store(ctx, "ORD-004", "hash1", time.Now().Add(-time.Hour)))
	require.NoError(t, store.Store(ctx, "ORD-005", "hash2", time.Now().Add(time.Hour)))

	require.NoError(t, store.CleanupExpired(ctx))

	expired, err := store.Lookup(ctx, "ORD-004", "hash1")
	require.NoError(t, err)
	assert.Nil(t, expired)

	valid, err := store.Lookup(ctx, "ORD-005", "hash2")
	require.NoError(t, err)
	assert.NotNil(t, valid)
}
