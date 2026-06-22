// export.go — Order export (CSV/JSON download) and admin purge endpoint.
// ExportOrders streams directly to the response writer; no temp files are created.
// PurgeOrders deletes completed/cancelled orders in a transaction to keep order items consistent.
package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type ExportHandler struct {
	orderRepo services.OrderRepository
	auditSvc  audit.Service
}

func NewExportHandler(orderRepo services.OrderRepository, auditSvc audit.Service) *ExportHandler {
	return &ExportHandler{orderRepo: orderRepo, auditSvc: auditSvc}
}

func (h *ExportHandler) ExportOrders(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		response.Error(w, http.StatusBadRequest, "format must be one of: csv, json")
		return
	}

	statuses, err := parseOrderStatuses(r.URL.Query().Get("statuses"), nil)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	orders, err := h.orderRepo.FindByDateRangeWithItems(r.Context(), start, end, statuses)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to export orders")
		return
	}

	filename := fmt.Sprintf("orders_%s_to_%s.%s", start.Format("2006-01-02"), end.Format("2006-01-02"), format)
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")

		mapper := services.NewOrderResponseMapper(nil)
		rows := make([]services.OrderResponse, 0, len(orders))
		for _, order := range orders {
			rows = append(rows, mapper.Map(order))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"start":  start.Format("2006-01-02"),
			"end":    end.Format("2006-01-02"),
			"count":  len(rows),
			"orders": rows,
		})

		actorID := ""
		if rec, ok := auth.GetRecord(r.Context()); ok {
			actorID = auth.RecordID(rec)
		}
		ip := auth.ClientIP(r)
		_ = h.auditSvc.Log(r.Context(), audit.ActionExportTriggered, actorID, "export:orders", map[string]any{
			"format":    format,
			"count":     len(rows),
			"date_from": start.Format("2006-01-02"),
			"date_to":   end.Format("2006-01-02"),
		}, ip)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	csvWriter := csv.NewWriter(w)

	_ = csvWriter.Write([]string{
		"order_id",
		"order_number",
		"customer_name",
		"customer_phone",
		"status",
		"payment_method",
		"payment_status",
		"order_type",
		"table_number",
		"total_amount",
		"items_count",
		"items_summary",
		"notes",
		"created_at",
		"updated_at",
	})

	for _, order := range orders {
		tableNumber := ""
		if order.TableNumber != nil {
			tableNumber = fmt.Sprintf("%d", *order.TableNumber)
		}

		notes := ""
		if order.Notes != nil {
			notes = *order.Notes
		}

		_ = csvWriter.Write([]string{
			order.ID.String(),
			order.OrderNumber,
			order.CustomerName,
			order.CustomerPhone,
			order.Status,
			order.PaymentMethod,
			order.PaymentStatus,
			order.OrderType,
			tableNumber,
			fmt.Sprintf("%.2f", order.TotalAmount),
			fmt.Sprintf("%d", len(order.Items)),
			orderItemsSummary(order.Items),
			notes,
			order.CreatedAt.Format(time.RFC3339),
			order.UpdatedAt.Format(time.RFC3339),
		})
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to encode CSV export")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionExportTriggered, actorID, "export:orders", map[string]any{
		"format":    format,
		"count":     len(orders),
		"date_from": start.Format("2006-01-02"),
		"date_to":   end.Format("2006-01-02"),
	}, ip)
}

func (h *ExportHandler) PurgeOrders(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	statuses, err := parseOrderStatuses(r.URL.Query().Get("statuses"), []string{"entregada", "cancelada"})
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	deletedOrdersCount, deletedItemsCount, err := h.orderRepo.Purge(r.Context(), start, end, statuses)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to purge orders")
		return
	}

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionOrderDeleted, actorID, "orders:purge", map[string]any{
		"deleted_orders": deletedOrdersCount,
		"deleted_items":  deletedItemsCount,
		"date_from":      start.Format("2006-01-02"),
		"date_to":        end.Format("2006-01-02"),
		"statuses":       statuses,
	}, ip)

	message := "Orders purged successfully"
	if deletedOrdersCount == 0 {
		message = "No orders matched the selected filters"
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message":             message,
		"deleted_count":       deletedOrdersCount,
		"deleted_items_count": deletedItemsCount,
		"statuses":            statuses,
	})
}

func parseOrderStatuses(raw string, defaults []string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		if len(defaults) == 0 {
			return nil, nil
		}

		copied := make([]string, len(defaults))
		copy(copied, defaults)
		return copied, nil
	}

	parts := strings.Split(trimmed, ",")
	parsed := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, part := range parts {
		status := strings.TrimSpace(part)
		if status == "" {
			continue
		}
		if _, exists := seen[status]; exists {
			continue
		}
		seen[status] = struct{}{}
		parsed = append(parsed, status)
	}

	if len(parsed) == 0 {
		return nil, fmt.Errorf("statuses must include at least one valid status")
	}

	return parsed, nil
}

func orderItemsSummary(items []dto.OrderItem) string {
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%dx %s", item.Quantity, item.ProductName))
	}

	return strings.Join(parts, "; ")
}
