// test_helpers_test.go — Shared test helpers for repository tests.
package repository

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func intPtr(v int) *int {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func strconvItoa(v int) string {
	return strconv.Itoa(v)
}

func timeNow() time.Time {
	return time.Now()
}

// testLocationBySlug returns the ID of a seeded location (sucursal-1…3).
func testLocationBySlug(t *testing.T, db bun.IDB, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := db.NewSelect().Model((*models.Location)(nil)).Column("id").Where("slug = ?", slug).Scan(context.Background(), &id); err != nil {
		t.Fatalf("find location %s: %v", slug, err)
	}
	return id
}

// testLocationID returns Sucursal 1, the location fixtures belong to.
func testLocationID(t *testing.T, db bun.IDB) uuid.UUID {
	t.Helper()
	return testLocationBySlug(t, db, "sucursal-1")
}
