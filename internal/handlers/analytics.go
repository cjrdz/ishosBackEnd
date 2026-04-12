package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/cjrdz/ishosBackEnd/ent"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	entOrderItem "github.com/cjrdz/ishosBackEnd/ent/orderitem"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type AnalyticsHandler struct {
	db *ent.Client
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

func NewAnalyticsHandler(db *ent.Client) *AnalyticsHandler {
	return &AnalyticsHandler{db: db}
}

func (h *AnalyticsHandler) Overview(w http.ResponseWriter, r *http.Request) {
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

	response.JSON(w, http.StatusOK, analyticsOverviewResponse{
		Period:          period,
		TotalOrders:     len(orders),
		TotalRevenue:    totalRevenue,
		AvgOrderValue:   avgOrderValue,
		StatusBreakdown: statusBreakdown,
	})
}

func (h *AnalyticsHandler) OrdersOverTime(w http.ResponseWriter, r *http.Request) {
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

	response.JSON(w, http.StatusOK, result)
}

func (h *AnalyticsHandler) TopProducts(w http.ResponseWriter, r *http.Request) {
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

	response.JSON(w, http.StatusOK, result)
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
