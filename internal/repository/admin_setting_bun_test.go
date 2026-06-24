// admin_setting_bun_test.go — Repository integration tests for the Bun-backed admin setting adapter.
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminSettingBunRepository_UpsertAndGet(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewAdminSettingBunRepository(db)

	created, err := repo.UpsertByKeyWithAudit(ctx, "whatsapp", map[string]any{"enabled": true}, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, "whatsapp", created.Key)
	assert.Equal(t, "admin@example.com", created.ChangedBy)

	found, err := repo.GetByKey(ctx, "whatsapp")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, true, found.Value["enabled"])

	updated, err := repo.UpsertByKeyWithAudit(ctx, "whatsapp", map[string]any{"enabled": false}, "other@example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "other@example.com", updated.ChangedBy)
	assert.Equal(t, false, updated.Value["enabled"])
}

func TestAdminSettingBunRepository_GetByKeyNotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewAdminSettingBunRepository(db)

	_, err := repo.GetByKey(ctx, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
