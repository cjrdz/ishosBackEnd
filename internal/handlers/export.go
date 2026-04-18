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

	"github.com/cjrdz/ishosBackEnd/ent"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	entOrderItem "github.com/cjrdz/ishosBackEnd/ent/orderitem"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type ExportHandler struct {
	db *ent.Client
}

func NewExportHandler(db *ent.Client) *ExportHandler {
	return &ExportHandler{db: db}
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

	query := h.db.Order.Query().
		Where(
			entOrder.CreatedAtGTE(start),
			entOrder.CreatedAtLTE(end),
		).
		WithItems()

	if len(statuses) > 0 {
		query = query.Where(entOrder.StatusIn(statuses...))
	}

	orders, err := query.All(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to export orders")
		return
	}

	filename := fmt.Sprintf("orders_%s_to_%s.%s", start.Format("2006-01-02"), end.Format("2006-01-02"), format)
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")

		rows := make([]orderResponse, 0, len(orders))
		for _, order := range orders {
			rows = append(rows, mapOrderResponse(order))
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"start":  start.Format("2006-01-02"),
			"end":    end.Format("2006-01-02"),
			"count":  len(rows),
			"orders": rows,
		})
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
			string(order.Status),
			string(order.PaymentMethod),
			string(order.PaymentStatus),
			string(order.OrderType),
			tableNumber,
			fmt.Sprintf("%.2f", order.TotalAmount),
			fmt.Sprintf("%d", len(order.Edges.Items)),
			orderItemsSummary(order.Edges.Items),
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
}

func (h *ExportHandler) PurgeOrders(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	statuses, err := parseOrderStatuses(r.URL.Query().Get("statuses"), []entOrder.Status{
		entOrder.StatusEntregada,
		entOrder.StatusCancelada,
	})
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.db.Tx(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to start purge transaction")
		return
	}

	defer func() {
		// err is the named loop variable; rollback only if it was set by one of the
		// query/delete calls below, not by the commit itself.
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	ids, queryErr := tx.Order.Query().
		Where(
			entOrder.CreatedAtGTE(start),
			entOrder.CreatedAtLTE(end),
			entOrder.StatusIn(statuses...),
		).
		IDs(r.Context())
	if queryErr != nil {
		err = queryErr
		response.Error(w, http.StatusInternalServerError, "Failed to find orders to purge")
		return
	}

	if len(ids) == 0 {
		if commitErr := tx.Commit(); commitErr != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to finish purge")
			return
		}

		response.JSON(w, http.StatusOK, map[string]interface{}{
			"message":             "No orders matched the selected filters",
			"deleted_count":       0,
			"deleted_items_count": 0,
			"statuses":            stringifyStatuses(statuses),
		})
		return
	}

	deletedItemsCount, deleteItemsErr := tx.OrderItem.Delete().
		Where(entOrderItem.HasOrderWith(entOrder.IDIn(ids...))).
		Exec(r.Context())
	if deleteItemsErr != nil {
		err = deleteItemsErr
		response.Error(w, http.StatusInternalServerError, "Failed to delete order items")
		return
	}

	deletedOrdersCount, deleteOrdersErr := tx.Order.Delete().
		Where(entOrder.IDIn(ids...)).
		Exec(r.Context())
	if deleteOrdersErr != nil {
		err = deleteOrdersErr
		response.Error(w, http.StatusInternalServerError, "Failed to delete orders")
		return
	}

	if commitErr := tx.Commit(); commitErr != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to finalize purge")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":             "Orders purged successfully",
		"deleted_count":       deletedOrdersCount,
		"deleted_items_count": deletedItemsCount,
		"statuses":            stringifyStatuses(statuses),
	})
}

func parseOrderStatuses(raw string, defaults []entOrder.Status) ([]entOrder.Status, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		if len(defaults) == 0 {
			return nil, nil
		}

		copied := make([]entOrder.Status, len(defaults))
		copy(copied, defaults)
		return copied, nil
	}

	parts := strings.Split(trimmed, ",")
	parsed := make([]entOrder.Status, 0, len(parts))
	seen := map[entOrder.Status]struct{}{}

	for _, part := range parts {
		status := entOrder.Status(strings.TrimSpace(part))
		if status == "" {
			continue
		}
		if err := entOrder.StatusValidator(status); err != nil {
			return nil, fmt.Errorf("invalid status %q", status)
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

func stringifyStatuses(statuses []entOrder.Status) []string {
	out := make([]string, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, string(status))
	}
	return out
}

func orderItemsSummary(items []*ent.OrderItem) string {
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%dx %s", item.Quantity, item.ProductName))
	}

	return strings.Join(parts, "; ")
}
