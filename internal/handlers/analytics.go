// analytics.go — Read-only analytics endpoints (overview, orders over time, top products).
// Results are cached for 5 minutes to avoid repeated full-table scans under dashboard load.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cjrdz/ishosBackEnd/ent"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	entOrderItem "github.com/cjrdz/ishosBackEnd/ent/orderitem"
	cachepkg "github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type AnalyticsHandler struct {
	db          *ent.Client
	cache       *cachepkg.TTLCache
	redis       *redis.Client
	redisPrefix string
}

type analyticsOverviewResponse struct {
	Period          string         `json:"period"`
	TotalOrders     int            `json:"total_orders"`
	TotalRevenue    float64        `json:"total_revenue"`
	AvgOrderValue   float64        `json:"avg_order_value"`
	StatusBreakdown map[string]int `json:"status_breakdown"`
}

type analyticsTimelinePointResponse struct {
	Date    string  `json:"date"`
	Count   int     `json:"count"`
	Revenue float64 `json:"revenue"`
}

type analyticsTopProductResponse struct {
	Name       string  `json:"name"`
	TotalSold  int     `json:"total_sold"`
	TotalValue float64 `json:"total_value"`
}

func NewAnalyticsHandler(db *ent.Client, cfg *config.Config) *AnalyticsHandler {
	h := &AnalyticsHandler{db: db, cache: cachepkg.New(5 * time.Minute)}

	if cfg == nil || strings.TrimSpace(cfg.RedisURL) == "" {
		return h
	}

	options, err := redis.ParseURL(strings.TrimSpace(cfg.RedisURL))
	if err != nil {
		log.Warn().Err(err).Msg("analytics cache: invalid REDIS_URL, falling back to in-memory cache")
		return h
	}

	h.redis = redis.NewClient(options)
	h.redisPrefix = strings.TrimSpace(cfg.RedisPrefix)
	if h.redisPrefix == "" {
		h.redisPrefix = "ishos"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if pingErr := h.redis.Ping(ctx).Err(); pingErr != nil {
		log.Warn().Err(pingErr).Msg("analytics cache: redis unavailable, using in-memory cache")
		_ = h.redis.Close()
		h.redis = nil
	}

	return h
}

func (h *AnalyticsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	// cache key includes the full query string so period variants are cached independently.
	cacheKey := "analytics:overview:" + r.URL.RawQuery
	var cached analyticsOverviewResponse
	if h.cacheGet(r.Context(), cacheKey, &cached) {
		response.JSON(w, http.StatusOK, cached)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}

	start, err := periodStart(period)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	orders, err := h.db.Order.Query().
		Where(
			entOrder.CreatedAtGTE(start),
			entOrder.CreatedAtLTE(time.Now()),
		).
		All(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute analytics overview")
		return
	}

	statusBreakdown := make(map[string]int)
	totalRevenue := 0.0
	nonCancelledCount := 0

	for _, order := range orders {
		status := string(order.Status)
		statusBreakdown[status]++

		if order.Status != entOrder.StatusCancelada {
			totalRevenue += order.TotalAmount
			nonCancelledCount++
		}
	}

	avgOrderValue := 0.0
	if nonCancelledCount > 0 {
		avgOrderValue = totalRevenue / float64(nonCancelledCount)
	}

	result := analyticsOverviewResponse{
		Period:          period,
		TotalOrders:     len(orders),
		TotalRevenue:    totalRevenue,
		AvgOrderValue:   avgOrderValue,
		StatusBreakdown: statusBreakdown,
	}
	h.cacheSet(r.Context(), cacheKey, result)
	response.JSON(w, http.StatusOK, result)
}

func (h *AnalyticsHandler) OrdersOverTime(w http.ResponseWriter, r *http.Request) {
	cacheKey := "analytics:orders-over-time:" + r.URL.RawQuery
	var cached []analyticsTimelinePointResponse
	if h.cacheGet(r.Context(), cacheKey, &cached) {
		response.JSON(w, http.StatusOK, cached)
		return
	}

	groupBy := r.URL.Query().Get("groupBy")
	if groupBy == "" {
		groupBy = "day"
	}
	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		response.Error(w, http.StatusBadRequest, "groupBy must be one of: day, week, month")
		return
	}

	start, end, err := parseDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	orders, err := h.db.Order.Query().
		Where(
			entOrder.CreatedAtGTE(start),
			entOrder.CreatedAtLTE(end),
		).
		All(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute orders over time")
		return
	}

	type bucket struct {
		count   int
		revenue float64
	}
	buckets := make(map[string]bucket)

	for _, order := range orders {
		key := bucketKey(order.CreatedAt, groupBy)
		current := buckets[key]
		current.count++
		if order.Status != entOrder.StatusCancelada {
			current.revenue += order.TotalAmount
		}
		buckets[key] = current
	}

	seriesKeys := buildSeriesKeys(start, end, groupBy)
	result := make([]analyticsTimelinePointResponse, 0, len(seriesKeys))
	for _, key := range seriesKeys {
		b := buckets[key]
		result = append(result, analyticsTimelinePointResponse{
			Date:    key,
			Count:   b.count,
			Revenue: b.revenue,
		})
	}

	h.cacheSet(r.Context(), cacheKey, result)
	response.JSON(w, http.StatusOK, result)
}

func (h *AnalyticsHandler) TopProducts(w http.ResponseWriter, r *http.Request) {
	cacheKey := "analytics:top-products:" + r.URL.RawQuery
	var cached []analyticsTopProductResponse
	if h.cacheGet(r.Context(), cacheKey, &cached) {
		response.JSON(w, http.StatusOK, cached)
		return
	}

	start, end, err := parseDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	limit := 10
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			response.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsedLimit > 50 {
			// hard cap: top-products is a full-scan aggregation, capped to keep response size bounded.
			parsedLimit = 50
		}
		limit = parsedLimit
	}

	items, err := h.db.OrderItem.Query().
		Where(
			entOrderItem.HasOrderWith(
				entOrder.CreatedAtGTE(start),
				entOrder.CreatedAtLTE(end),
				entOrder.StatusNEQ(entOrder.StatusCancelada),
			),
		).
		All(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute top products")
		return
	}

	type aggregate struct {
		totalSold  int
		totalValue float64
	}
	byProduct := make(map[string]aggregate)

	for _, item := range items {
		current := byProduct[item.ProductName]
		current.totalSold += item.Quantity
		current.totalValue += item.Subtotal
		byProduct[item.ProductName] = current
	}

	result := make([]analyticsTopProductResponse, 0, len(byProduct))
	for name, data := range byProduct {
		result = append(result, analyticsTopProductResponse{
			Name:       name,
			TotalSold:  data.totalSold,
			TotalValue: data.totalValue,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalSold == result[j].TotalSold {
			return result[i].TotalValue > result[j].TotalValue
		}
		return result[i].TotalSold > result[j].TotalSold
	})

	if len(result) > limit {
		result = result[:limit]
	}

	h.cacheSet(r.Context(), cacheKey, result)
	response.JSON(w, http.StatusOK, result)
}

func (h *AnalyticsHandler) cacheGet(ctx context.Context, key string, target interface{}) bool {
	if h.redis != nil {
		redisKey := h.redisPrefix + ":" + key
		redisCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if payload, err := h.redis.Get(redisCtx, redisKey).Bytes(); err == nil {
			if unmarshalErr := json.Unmarshal(payload, target); unmarshalErr == nil {
				return true
			}
		}
	}

	v, ok := h.cache.Get(key)
	if !ok {
		return false
	}

	payload, err := json.Marshal(v)
	if err != nil {
		return false
	}

	return json.Unmarshal(payload, target) == nil
}

func (h *AnalyticsHandler) cacheSet(ctx context.Context, key string, value interface{}) {
	h.cache.Set(key, value)
	if h.redis == nil {
		return
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return
	}

	redisKey := h.redisPrefix + ":" + key
	redisCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = h.redis.Set(redisCtx, redisKey, payload, 5*time.Minute).Err()
}

func parseDateRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	now := time.Now()
	if endRaw == "" {
		endRaw = now.Format("2006-01-02")
	}
	if startRaw == "" {
		startRaw = now.AddDate(0, -1, 0).Format("2006-01-02")
	}

	startDate, err := time.Parse("2006-01-02", startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start must use YYYY-MM-DD format")
	}
	endDate, err := time.Parse("2006-01-02", endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end must use YYYY-MM-DD format")
	}

	start := startDate
	end := endDate.Add(24*time.Hour - time.Nanosecond)
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be greater than or equal to start")
	}

	return start, end, nil
}

func periodStart(period string) (time.Time, error) {
	now := time.Now()
	switch period {
	case "week":
		return now.AddDate(0, 0, -7), nil
	case "month":
		return now.AddDate(0, -1, 0), nil
	case "year":
		return now.AddDate(-1, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("period must be one of: week, month, year")
	}
}

func bucketKey(t time.Time, groupBy string) string {
	switch groupBy {
	case "week":
		year, week := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	case "month":
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

func buildSeriesKeys(start, end time.Time, groupBy string) []string {
	keys := make([]string, 0)

	switch groupBy {
	case "month":
		cursor := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
		last := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())
		for !cursor.After(last) {
			keys = append(keys, cursor.Format("2006-01"))
			cursor = cursor.AddDate(0, 1, 0)
		}
	case "week":
		// Advance day-by-day and deduplicate ISO week keys; a simple week-count
		// arithmetic would misalign when start/end straddle a year boundary.
		cursor := start
		seen := map[string]struct{}{}
		for !cursor.After(end) {
			key := bucketKey(cursor, "week")
			if _, exists := seen[key]; !exists {
				keys = append(keys, key)
				seen[key] = struct{}{}
			}
			cursor = cursor.AddDate(0, 0, 1)
		}
	default:
		cursor := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		last := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
		for !cursor.After(last) {
			keys = append(keys, cursor.Format("2006-01-02"))
			cursor = cursor.AddDate(0, 0, 1)
		}
	}

	return keys
}
