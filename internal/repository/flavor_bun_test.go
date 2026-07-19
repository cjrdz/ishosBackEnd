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

	// Use a single connection so SET search_path applies to every query and
	// transaction in this test.
	rawDB.SetMaxOpenConns(1)
	rawDB.SetMaxIdleConns(1)

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
		DisplayOrder: intPtr(1),
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

	created, err := repo.Create(ctx, CreateFlavorInput{Name: "Vanilla", DisplayOrder: intPtr(2), IsActive: true})
	require.NoError(t, err)

	name := "Vanilla Bean"
	updated, err := repo.Update(ctx, created.ID, UpdateFlavorInput{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "Vanilla Bean", updated.Name)
	assert.Equal(t, 2, updated.DisplayOrder)
}

func TestFlavorBunRepository_AutoDisplayOrder(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewFlavorBunRepository(db)

	first, err := repo.Create(ctx, CreateFlavorInput{Name: "Vanilla", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, 0, first.DisplayOrder)

	second, err := repo.Create(ctx, CreateFlavorInput{Name: "Chocolate", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, 1, second.DisplayOrder)
}

func TestFlavorBunRepository_DeleteCompactsOrder(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewFlavorBunRepository(db)

	first, err := repo.Create(ctx, CreateFlavorInput{Name: "Vanilla", DisplayOrder: intPtr(0), IsActive: true})
	require.NoError(t, err)
	second, err := repo.Create(ctx, CreateFlavorInput{Name: "Chocolate", DisplayOrder: intPtr(1), IsActive: true})
	require.NoError(t, err)
	third, err := repo.Create(ctx, CreateFlavorInput{Name: "Strawberry", DisplayOrder: intPtr(2), IsActive: true})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, second.ID))

	updatedThird, err := repo.FindByID(ctx, third.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, updatedThird.DisplayOrder)

	updatedFirst, err := repo.FindByID(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, updatedFirst.DisplayOrder)
}
