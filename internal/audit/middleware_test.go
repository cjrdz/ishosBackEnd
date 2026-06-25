// middleware_test.go — Tests for audit logging middleware.
package audit

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	orderDomain "github.com/cjrdz/ishosBackEnd/internal/domain/order"
	"github.com/cjrdz/ishosBackEnd/internal/models"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

func newTestAuditBunDB(t *testing.T) *bun.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name())
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	_, err = bunDB.NewCreateTable().Model((*models.AuditLog)(nil)).Exec(context.Background())
	require.NoError(t, err)
	return bunDB
}

func TestMiddlewareOrderCreated(t *testing.T) {
	ctx := context.Background()
	db := newTestAuditBunDB(t)

	repo := repository.NewAuditLogBunRepository(db)
	svc := NewService(repo)
	middleware := NewLogMiddleware(svc)

	orderID := uuid.New()

	event := orderDomain.NewOrderCreated(
		orderID,
		"ORD-001",
		"John Doe",
		"555-1234",
		"john@example.com",
		"card",
		"delivery",
		99.99,
		2,
		"https://tracking.local/ORD-001",
	)

	err := middleware.OnDomainEvent(ctx, event)
	require.NoError(t, err)

	// Verify audit entry was created
	var entries []models.AuditLog
	require.NoError(t, db.NewSelect().Model(&entries).Scan(ctx))
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, "order.created", entry.Action)
	assert.Contains(t, entry.Resource, orderID.String())
	assert.Equal(t, "ORD-001", entry.Changes["order_number"])
	assert.Equal(t, float64(2), entry.Changes["items"])
}

func TestMiddlewareOrderApproved(t *testing.T) {
	ctx := context.Background()
	db := newTestAuditBunDB(t)

	repo := repository.NewAuditLogBunRepository(db)
	svc := NewService(repo)
	middleware := NewLogMiddleware(svc)

	orderID := uuid.New()

	event := &orderDomain.OrderApproved{
		BaseDomainEvent: orderDomain.BaseDomainEvent{
			AggregateIDVal: orderID,
		},
		OrderNumber: "ORD-002",
		ReviewerID:  "emp-123",
	}

	err := middleware.OnDomainEvent(ctx, event)
	require.NoError(t, err)

	// Verify audit entry was created
	var entries []models.AuditLog
	require.NoError(t, db.NewSelect().Model(&entries).Scan(ctx))
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, "order.approved", entry.Action)
	assert.Equal(t, "emp-123", entry.Changes["reviewer_id"])
}

func TestMiddlewareNilEvent(t *testing.T) {
	ctx := context.Background()
	db := newTestAuditBunDB(t)

	repo := repository.NewAuditLogBunRepository(db)
	svc := NewService(repo)
	middleware := NewLogMiddleware(svc)

	// Should handle nil event gracefully
	err := middleware.OnDomainEvent(ctx, nil)
	assert.NoError(t, err)

	// Verify no entries were created
	count, err := db.NewSelect().Model((*models.AuditLog)(nil)).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestMiddlewareNilService(t *testing.T) {
	ctx := context.Background()
	middleware := NewLogMiddleware(nil)

	orderID := uuid.New()

	event := orderDomain.NewOrderCreated(
		orderID,
		"ORD-006",
		"Jane Doe",
		"555-5678",
		"jane@example.com",
		"card",
		"delivery",
		50.00,
		1,
		"https://tracking.local/ORD-006",
	)

	// Should handle nil service gracefully
	err := middleware.OnDomainEvent(ctx, event)
	assert.NoError(t, err)
}

func TestMiddlewareMultipleEvents(t *testing.T) {
	ctx := context.Background()
	db := newTestAuditBunDB(t)

	repo := repository.NewAuditLogBunRepository(db)
	svc := NewService(repo)
	middleware := NewLogMiddleware(svc)

	orderID := uuid.New()

	// Create sequence: Created → Approved → Ready
	events := []orderDomain.DomainEvent{
		orderDomain.NewOrderCreated(
			orderID,
			"ORD-007",
			"Bob Smith",
			"555-9999",
			"bob@example.com",
			"cash",
			"pickup",
			75.00,
			3,
			"https://tracking.local/ORD-007",
		),
		&orderDomain.OrderApproved{
			BaseDomainEvent: orderDomain.BaseDomainEvent{
				AggregateIDVal: orderID,
			},
			OrderNumber: "ORD-007",
			ReviewerID:  "emp-456",
		},
		&orderDomain.OrderReady{
			BaseDomainEvent: orderDomain.BaseDomainEvent{
				AggregateIDVal: orderID,
			},
			OrderNumber: "ORD-007",
		},
	}

	for _, event := range events {
		err := middleware.OnDomainEvent(ctx, event)
		require.NoError(t, err)
	}

	// Verify all 3 entries were created
	var entries []models.AuditLog
	require.NoError(t, db.NewSelect().Model(&entries).Order("created_at ASC").Scan(ctx))
	assert.Len(t, entries, 3)

	// Verify order of actions
	assert.Equal(t, "order.created", entries[0].Action)
	assert.Equal(t, "order.approved", entries[1].Action)
	assert.Equal(t, "order.ready", entries[2].Action)
}
