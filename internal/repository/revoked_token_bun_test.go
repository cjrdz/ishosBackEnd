// revoked_token_bun_test.go — Repository integration tests for the Bun-backed revoked token store.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevokedTokenBunStore_RevokeAndCheck(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	store := NewRevokedTokenBunStore(db)

	future := time.Now().Add(time.Hour)
	require.NoError(t, store.RevokeToken(ctx, "hash1", future))

	revoked, err := store.IsTokenRevoked(ctx, "hash1", time.Now())
	require.NoError(t, err)
	assert.True(t, revoked)

	notRevoked, err := store.IsTokenRevoked(ctx, "hash2", time.Now())
	require.NoError(t, err)
	assert.False(t, notRevoked)

	past := time.Now().Add(-time.Hour)
	require.NoError(t, store.RevokeToken(ctx, "hash3", past))
	expired, err := store.IsTokenRevoked(ctx, "hash3", time.Now())
	require.NoError(t, err)
	assert.False(t, expired)
}

func TestRevokedTokenBunStore_UpdateExisting(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	store := NewRevokedTokenBunStore(db)

	first := time.Now().Add(time.Hour)
	require.NoError(t, store.RevokeToken(ctx, "hash1", first))

	later := time.Now().Add(2 * time.Hour)
	require.NoError(t, store.RevokeToken(ctx, "hash1", later))

	revoked, err := store.IsTokenRevoked(ctx, "hash1", time.Now().Add(90*time.Minute))
	require.NoError(t, err)
	assert.True(t, revoked)
}
