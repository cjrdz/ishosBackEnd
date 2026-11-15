// analytics.go — Read-only analytics endpoints (overview, orders over time, top products).
// Results are cached for 5 minutes to avoid repeated full-table scans under dashboard load.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	cachepkg "github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AnalyticsHandler struct {
	orderRepo   services.OrderRepository
	loc         *time.Location // store time zone for date ranges
	cache       *cachepkg.TTLCache
	redis       *redis.Client
	redisPrefix string
}

func NewAnalyticsHandler(orderRepo services.OrderRepository, cfg *config.Config) *AnalyticsHandler {
	h := &AnalyticsHandler{orderRepo: orderRepo, cache: cachepkg.New(5 * time.Minute), loc: time.UTC}
	if cfg != nil {
		h.loc = cfg.StoreLocation()
	}

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
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	cacheKey := "analytics:overview:" + loc.String() + ":" + r.URL.RawQuery
	var cached dto.AnalyticsOverview
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

	result, err := h.orderRepo.AnalyticsOverview(r.Context(), loc, start, time.Now())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute analytics overview")
		return
	}
	result.Period = period

	h.cacheSet(r.Context(), cacheKey, result)
	response.JSON(w, http.StatusOK, result)
}

func (h *AnalyticsHandler) OrdersOverTime(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	cacheKey := "analytics:orders-over-time:" + loc.String() + ":" + r.URL.RawQuery
	var cached []dto.AnalyticsTimelinePoint
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

	start, end, err := capDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"), 365, h.loc)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	points, err := h.orderRepo.AnalyticsOrdersOverTime(r.Context(), loc, start, end, groupBy)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute orders over time")
		return
	}

	buckets := make(map[string]dto.AnalyticsTimelinePoint, len(points))
	for _, p := range points {
		buckets[p.Date] = p
	}

	seriesKeys := buildSeriesKeys(start, end, groupBy)
	result := make([]dto.AnalyticsTimelinePoint, 0, len(seriesKeys))
	for _, key := range seriesKeys {
		if p, ok := buckets[key]; ok {
			result = append(result, p)
			continue
		}
		result = append(result, dto.AnalyticsTimelinePoint{Date: key, Count: 0, Revenue: 0})
	}

	h.cacheSet(r.Context(), cacheKey, result)
	response.JSON(w, http.StatusOK, result)
}

func (h *AnalyticsHandler) TopProducts(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	cacheKey := "analytics:top-products:" + loc.String() + ":" + r.URL.RawQuery
	var cached []dto.AnalyticsTopProduct
	if h.cacheGet(r.Context(), cacheKey, &cached) {
		response.JSON(w, http.StatusOK, cached)
		return
	}

	start, end, limit, err := analyticsTopProductsParams(r, h.loc)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.orderRepo.AnalyticsTopProducts(r.Context(), loc, start, end, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute top products")
		return
	}

	h.cacheSet(r.Context(), cacheKey, result)
	response.JSON(w, http.StatusOK, result)
}

// Dashboard returns overview, orders-over-time, and top-products in a single
// response. All three sections are computed from the same start/end date range
// so the overview period always matches the timeline and top-products window.
func (h *AnalyticsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	cacheKey := "analytics:dashboard:" + loc.String() + ":" + r.URL.RawQuery
	var cached dto.AnalyticsDashboard
	if h.cacheGet(r.Context(), cacheKey, &cached) {
		response.JSON(w, http.StatusOK, cached)
		return
	}

	start, end, err := capDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"), 365, h.loc)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
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

	_, _, limit, err := analyticsTopProductsParams(r, h.loc)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	overview, err := h.orderRepo.AnalyticsOverview(r.Context(), loc, start, end)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute analytics overview")
		return
	}
	overview.Period = "custom"

	rawPoints, err := h.orderRepo.AnalyticsOrdersOverTime(r.Context(), loc, start, end, groupBy)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute orders over time")
		return
	}
	buckets := make(map[string]dto.AnalyticsTimelinePoint, len(rawPoints))
	for _, p := range rawPoints {
		buckets[p.Date] = p
	}
	seriesKeys := buildSeriesKeys(start, end, groupBy)
	ordersOverTime := make([]dto.AnalyticsTimelinePoint, 0, len(seriesKeys))
	for _, key := range seriesKeys {
		if p, ok := buckets[key]; ok {
			ordersOverTime = append(ordersOverTime, p)
			continue
		}
		ordersOverTime = append(ordersOverTime, dto.AnalyticsTimelinePoint{Date: key, Count: 0, Revenue: 0})
	}

	topProducts, err := h.orderRepo.AnalyticsTopProducts(r.Context(), loc, start, end, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to compute top products")
		return
	}

	result := dto.AnalyticsDashboard{
		Overview:       overview,
		OrdersOverTime: ordersOverTime,
		TopProducts:    topProducts,
	}

	h.cacheSet(r.Context(), cacheKey, result)
	response.JSON(w, http.StatusOK, result)
}

func analyticsTopProductsParams(r *http.Request, loc *time.Location) (time.Time, time.Time, int, error) {
	start, end, err := capDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"), 365, loc)
	if err != nil {
		return time.Time{}, time.Time{}, 0, err
	}

	limit := 10
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("limit must be a positive integer")
		}
		if parsedLimit > 50 {
			parsedLimit = 50
		}
		limit = parsedLimit
	}

	return start, end, limit, nil
}

func (h *AnalyticsHandler) cacheGet(ctx context.Context, key string, target any) bool {
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

func (h *AnalyticsHandler) cacheSet(ctx context.Context, key string, value any) {
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

// parseDateRange parses YYYY-MM-DD bounds as calendar days in loc (the store
// time zone): start is 00:00 and end is 23:59:59.999999999 local time.
func parseDateRange(startRaw, endRaw string, loc *time.Location) (time.Time, time.Time, error) {
	now := time.Now().In(loc)
	if endRaw == "" {
		endRaw = now.Format("2006-01-02")
	}
	if startRaw == "" {
		startRaw = now.AddDate(0, -1, 0).Format("2006-01-02")
	}

	startDate, err := time.ParseInLocation("2006-01-02", startRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start must use YYYY-MM-DD format")
	}
	endDate, err := time.ParseInLocation("2006-01-02", endRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end must use YYYY-MM-DD format")
	}

	start := startDate
	// AddDate, not +24h, so DST-observing zones still end at local midnight.
	end := endDate.AddDate(0, 0, 1).Add(-time.Nanosecond)
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be greater than or equal to start")
	}

	return start, end, nil
}

// capDateRange is parseDateRange plus a maximum inclusive window in days.
func capDateRange(startRaw, endRaw string, maxDays int, loc *time.Location) (time.Time, time.Time, error) {
	start, end, err := parseDateRange(startRaw, endRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	// Count calendar days; rounding absorbs a 23/25h day in DST zones.
	inclusiveDays := int(math.Round(end.Sub(start).Hours() / 24))
	if inclusiveDays > maxDays {
		return time.Time{}, time.Time{}, fmt.Errorf("date range cannot exceed %d days", maxDays)
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
