// auditlog_bun_test.go — Repository integration tests for the Bun-backed audit log adapter.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogBunRepository_LogAndQuery(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewAuditLogBunRepository(db)

	require.NoError(t, repo.Log(ctx, "order.created", "user-1", "order:123", map[string]any{"total": 10.0}, "127.0.0.1"))

	byResource, err := repo.GetByResource(ctx, "order:123", 10, 0)
	require.NoError(t, err)
	require.Len(t, byResource, 1)
	assert.Equal(t, "order.created", byResource[0].Action)

	byActor, err := repo.GetByActor(ctx, "user-1", 10, 0)
	require.NoError(t, err)
	require.Len(t, byActor, 1)

	byAction, err := repo.GetByAction(ctx, "order.created", 10, 0)
	require.NoError(t, err)
	require.Len(t, byAction, 1)
}

func TestAuditLogBunRepository_CleanupOlderThan(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewAuditLogBunRepository(db)

	require.NoError(t, repo.Log(ctx, "settings.updated", "user-2", "settings:x", nil, ""))

	deleted, err := repo.CleanupOlderThan(ctx, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)

	deleted, err = repo.CleanupOlderThan(ctx, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
}
