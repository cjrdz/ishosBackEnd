package dto

// AnalyticsOverview is the aggregate result for the analytics overview endpoint.
type AnalyticsOverview struct {
	Period          string         `json:"period"`
	TotalOrders     int            `json:"total_orders"`
	TotalRevenue    float64        `json:"total_revenue"`
	AvgOrderValue   float64        `json:"avg_order_value"`
	StatusBreakdown map[string]int `json:"status_breakdown"`
}

// AnalyticsTimelinePoint is one bucket for orders-over-time.
type AnalyticsTimelinePoint struct {
	Date    string  `json:"date"`
	Count   int     `json:"count"`
	Revenue float64 `json:"revenue"`
}

// AnalyticsTopProduct is one row of the top-products report.
type AnalyticsTopProduct struct {
	Name       string  `json:"name"`
	TotalSold  int     `json:"total_sold"`
	TotalValue float64 `json:"total_value"`
}

// AnalyticsDashboard combines overview, timeline, and top-products reports in
// one response so the frontend can render the analytics page with a single call.
type AnalyticsDashboard struct {
	Overview       AnalyticsOverview        `json:"overview"`
	OrdersOverTime []AnalyticsTimelinePoint `json:"orders_over_time"`
	TopProducts    []AnalyticsTopProduct    `json:"top_products"`
}
