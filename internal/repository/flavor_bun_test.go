// flavor_bun_test.go — Repository integration tests for the Bun-backed flavor adapter.
package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/cjrdz/ishosBackEnd/internal/database"
)

func newTestBunDB(t *testing.T) *bun.DB {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/ishosFactory?sslmode=disable"
	}

	rawDB, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rawDB.Close() })

	if err := rawDB.Ping(); err != nil {
		t.Skipf("postgres not reachable (%v); skipping repository integration test", err)
	}

	ctx := context.Background()
	schemaName := "flavor_test_" + strconvItoa(int(timeNow().UnixNano()))
	_, err = rawDB.ExecContext(ctx, "CREATE SCHEMA "+schemaName)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, "SET search_path TO "+schemaName)
	require.NoError(t, err)

	db := bun.NewDB(rawDB, pgdialect.New())
	require.NoError(t, database.EnsureBunSchema(ctx, db))
	return db
}

func TestFlavorBunRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewFlavorBunRepository(db)

	created, err := repo.Create(ctx, CreateFlavorInput{
		Name:         "Chocolate",
		DisplayOrder: 1,
		IsSeasonal:   false,
		IsActive:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Chocolate", created.Name)
	assert.True(t, created.IsActive)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Chocolate", found.Name)

	all, err := repo.FindAll(ctx, false)
	require.NoError(t, err)
	require.Len(t, all, 1)
}

func TestFlavorBunRepository_Update(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewFlavorBunRepository(db)

	created, err := repo.Create(ctx, CreateFlavorInput{Name: "Vanilla", DisplayOrder: 2, IsActive: true})
	require.NoError(t, err)

	name := "Vanilla Bean"
	updated, err := repo.Update(ctx, created.ID, UpdateFlavorInput{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "Vanilla Bean", updated.Name)
	assert.Equal(t, 2, updated.DisplayOrder)
}

func TestFlavorBunRepository_Delete(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewFlavorBunRepository(db)

	created, err := repo.Create(ctx, CreateFlavorInput{Name: "Strawberry", DisplayOrder: 3, IsActive: true})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, created.ID))

	_, err = repo.FindByID(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
