// flavor_bun_test.go — Repository integration tests for the Bun-backed flavor adapter.
package repository

import (
	"context"
	"database/sql"
	"net/url"
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

	// Open a temporary connection just to create the isolated schema.
	setupDB, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)
	if err := setupDB.Ping(); err != nil {
		t.Skipf("postgres not reachable (%v); skipping repository integration test", err)
	}

	ctx := context.Background()
	schemaName := "test_" + strconvItoa(int(timeNow().UnixNano()))
	_, err = setupDB.ExecContext(ctx, "CREATE SCHEMA "+schemaName)
	require.NoError(t, err)
	require.NoError(t, setupDB.Close())

	// Drop the schema when the test ends so repeated runs do not accumulate
	// test_* schemas in the database. Registered before the pool cleanup below,
	// so it runs after that pool is closed (t.Cleanup is LIFO).
	t.Cleanup(func() {
		dropDB, err := sql.Open("pgx", databaseURL)
		if err != nil {
			return
		}
		defer func() { _ = dropDB.Close() }()
		_, _ = dropDB.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	})

	// Re-open the pool with the test schema baked into the connection string so
	// every connection uses it automatically.
	databaseURLWithSchema, err := withSearchPath(databaseURL, schemaName)
	require.NoError(t, err)

	rawDB, err := sql.Open("pgx", databaseURLWithSchema)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rawDB.Close() })

	if err := rawDB.Ping(); err != nil {
		t.Skipf("postgres not reachable (%v); skipping repository integration test", err)
	}

	db := bun.NewDB(rawDB, pgdialect.New())
	require.NoError(t, database.EnsureBunSchema(ctx, db))
	return db
}

// withSearchPath returns a PostgreSQL connection string with search_path set to
// the given schema. It supports URI-style connection strings.
func withSearchPath(databaseURL, schema string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
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
