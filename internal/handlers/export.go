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

const maxExportWindowDays = 90

func (h *ExportHandler) ExportOrders(w http.ResponseWriter, r *http.Request) {
	start, end, err := capDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"), maxExportWindowDays)
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

	actorID := ""
	if rec, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(rec)
	}
	ip := auth.ClientIP(r)

	filename := fmt.Sprintf("orders_%s_to_%s.%s", start.Format("2006-01-02"), end.Format("2006-01-02"), format)
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")

		var rows []dto.ExportOrderRow
		streamErr := h.orderRepo.StreamOrderExportRows(r.Context(), start, end, statuses, 500, func(row dto.ExportOrderRow) error {
			rows = append(rows, row)
			return nil
		})
		if streamErr != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to export orders")
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"start":  start.Format("2006-01-02"),
			"end":    end.Format("2006-01-02"),
			"count":  len(rows),
			"orders": rows,
		})

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

	var exportedCount int
	streamErr := h.orderRepo.StreamOrderExportRows(r.Context(), start, end, statuses, 500, func(row dto.ExportOrderRow) error {
		exportedCount++
		tableNumber := ""
		if row.TableNumber != nil {
			tableNumber = fmt.Sprintf("%d", *row.TableNumber)
		}

		notes := ""
		if row.Notes != nil {
			notes = *row.Notes
		}

		return csvWriter.Write([]string{
			row.ID.String(),
			row.OrderNumber,
			row.CustomerName,
			row.CustomerPhone,
			row.Status,
			row.PaymentMethod,
			row.PaymentStatus,
			row.OrderType,
			tableNumber,
			fmt.Sprintf("%.2f", row.TotalAmount),
			fmt.Sprintf("%d", row.ItemsCount),
			row.ItemsSummary,
			notes,
			row.CreatedAt.Format(time.RFC3339),
			row.UpdatedAt.Format(time.RFC3339),
		})
	})

	csvWriter.Flush()
	if streamErr != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to export orders")
		return
	}
	if err := csvWriter.Error(); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to encode CSV export")
		return
	}

	_ = h.auditSvc.Log(r.Context(), audit.ActionExportTriggered, actorID, "export:orders", map[string]any{
		"format":    format,
		"count":     exportedCount,
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
