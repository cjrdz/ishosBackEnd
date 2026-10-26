// order_analytics_tz_test.go — Analytics must bucket by the store's calendar
// day and count only approved orders as revenue. Requires PostgreSQL.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

func TestAnalytics_StoreTimeZoneAndApprovedRevenue(t *testing.T) {
	ctx := context.Background()
	db := newTestBunDB(t)
	repo := NewOrderBunRepository(db).WithStoreTimeZone("America/El_Salvador")

	// 23:30 on 2026-10-25 in El Salvador is 05:30 UTC on 2026-10-26.
	lateEvening := time.Date(2026, 10, 26, 5, 30, 0, 0, time.UTC)
	approved := createRegressionOrder(t, ctx, repo, "ORD-TZ-0001", string(models.OrderStatusEntregada), nil)
	pending := createRegressionOrder(t, ctx, repo, "ORD-TZ-0002", string(models.OrderStatusPendienteRevision), nil)
	for _, id := range []any{approved.ID, pending.ID} {
		_, err := db.ExecContext(ctx, "UPDATE orders SET created_at = ? WHERE id = ?", lateEvening, id)
		require.NoError(t, err)
	}

	loc, err := time.LoadLocation("America/El_Salvador")
	require.NoError(t, err)
	start := time.Date(2026, 10, 25, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1).Add(-time.Nanosecond)

	timeline, err := repo.AnalyticsOrdersOverTime(ctx, start, end, "day")
	require.NoError(t, err)
	require.Len(t, timeline, 1)
	assert.Equal(t, "2026-10-25", timeline[0].Date, "late-evening orders belong to the store's local day")
	assert.Equal(t, 2, timeline[0].Count)
	assert.InDelta(t, 10.0, timeline[0].Revenue, 0.001, "only the approved order counts as revenue")

	overview, err := repo.AnalyticsOverview(ctx, start, end)
	require.NoError(t, err)
	assert.Equal(t, 2, overview.TotalOrders)
	assert.InDelta(t, 10.0, overview.TotalRevenue, 0.001, "pending-review orders are not revenue")

	// Without a store time zone the same orders land on the UTC day.
	utcTimeline, err := NewOrderBunRepository(db).AnalyticsOrdersOverTime(ctx, start, end, "day")
	require.NoError(t, err)
	require.Len(t, utcTimeline, 1)
	assert.Equal(t, "2026-10-26", utcTimeline[0].Date)
}
