package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/middleware"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type OrderHandler struct {
	rateLimiter *middleware.OrderRateLimiter
	orderSvc    *services.OrderService
}

func NewOrderHandler(db *ent.Client, cfg *config.Config, rl *middleware.OrderRateLimiter) *OrderHandler {
	orderRepo := repository.NewOrderRepository(db)
	productRepo := repository.NewProductRepository(db)
	whatsapp := services.NewWhatsAppService(cfg)

	return &OrderHandler{
		rateLimiter: rl,
		orderSvc: services.NewOrderService(
			db,
			orderRepo,
			productRepo,
			whatsapp,
		),
	}
}

type createOrderBody struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	CustomerEmail string `json:"customer_email,omitempty"`
	PaymentMethod string `json:"payment_method"`
	OrderType     string `json:"order_type"`
	TableNumber   *int   `json:"table_number,omitempty"`
	Notes         string `json:"notes,omitempty"`
	Items         []struct {
		ProductID      uuid.UUID              `json:"product_id"`
		Quantity       int                    `json:"quantity"`
		Customizations map[string]interface{} `json:"customizations,omitempty"`
	} `json:"items"`
}

type updateOrderBody struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	CustomerEmail string `json:"customer_email,omitempty"`
	PaymentMethod string `json:"payment_method"`
	OrderType     string `json:"order_type"`
	TableNumber   *int   `json:"table_number,omitempty"`
	Notes         string `json:"notes,omitempty"`
	Items         []struct {
		ProductID      uuid.UUID              `json:"product_id"`
		Quantity       int                    `json:"quantity"`
		Customizations map[string]interface{} `json:"customizations,omitempty"`
	} `json:"items,omitempty"`
}

type orderItemResponse struct {
	ID             uuid.UUID              `json:"id"`
	ProductName    string                 `json:"product_name"`
	Quantity       int                    `json:"quantity"`
	UnitPrice      float64                `json:"unit_price"`
	Subtotal       float64                `json:"subtotal"`
	Customizations map[string]interface{} `json:"customizations,omitempty"`
}

type orderResponse struct {
	ID              uuid.UUID           `json:"id"`
	OrderNumber     string              `json:"order_number"`
	CustomerName    string              `json:"customer_name"`
	CustomerPhone   string              `json:"customer_phone"`
	CustomerEmail   *string             `json:"customer_email,omitempty"`
	Status          string              `json:"status"`
	PaymentMethod   string              `json:"payment_method"`
	PaymentStatus   string              `json:"payment_status"`
	OrderType       string              `json:"order_type"`
	TableNumber     *int                `json:"table_number,omitempty"`
	TotalAmount     float64             `json:"total_amount"`
	Notes           *string             `json:"notes,omitempty"`
	RejectionReason *string             `json:"rejection_reason,omitempty"`
	CreatedByUserID *string             `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Items           []orderItemResponse `json:"items"`
}

func mapOrderResponse(order *ent.Order) orderResponse {
	items := make([]orderItemResponse, 0)
	if order.Edges.Items != nil {
		items = make([]orderItemResponse, 0, len(order.Edges.Items))
		for _, item := range order.Edges.Items {
			if item == nil {
				continue
			}
			items = append(items, orderItemResponse{
				ID:             item.ID,
				ProductName:    item.ProductName,
				Quantity:       item.Quantity,
				UnitPrice:      item.UnitPrice,
				Subtotal:       item.Subtotal,
				Customizations: item.Customizations,
			})
		}
	}

	return orderResponse{
		ID:              order.ID,
		OrderNumber:     order.OrderNumber,
		CustomerName:    order.CustomerName,
		CustomerPhone:   order.CustomerPhone,
		CustomerEmail:   order.CustomerEmail,
		Status:          string(order.Status),
		PaymentMethod:   string(order.PaymentMethod),
		PaymentStatus:   string(order.PaymentStatus),
		OrderType:       string(order.OrderType),
		TableNumber:     order.TableNumber,
		TotalAmount:     order.TotalAmount,
		Notes:           order.Notes,
		RejectionReason: order.RejectionReason,
		CreatedByUserID: order.CreatedByUserID,
		CreatedAt:       order.CreatedAt,
		UpdatedAt:       order.UpdatedAt,
		Items:           items,
	}
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createOrderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.OrderType == "" {
		body.OrderType = "para_llevar"
	}

	if errs := validator.ValidateOrderCreate(
		body.CustomerName,
		body.CustomerPhone,
		body.PaymentMethod,
		body.OrderType,
		body.TableNumber,
		len(body.Items),
	); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	if body.OrderType == "para_llevar" {
		body.TableNumber = nil
	}

	isStaff := false
	var staffID string
	if record, ok := auth.GetRecord(r.Context()); ok {
		isStaff = true
		staffID = auth.RecordID(record)
		log.Info().
			Str("staff_id", staffID).
			Str("customer_phone", body.CustomerPhone).
			Msg("Order creation by authenticated staff - rate limits bypassed")
	}

	if !isStaff {
		if !h.rateLimiter.CheckAndRecordPhone(body.CustomerPhone) {
			response.Error(w, http.StatusTooManyRequests,
				"too many orders from this phone number, try again tomorrow")
			return
		}
	}

	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}

	items := make([]services.OrderItemInput, 0, len(body.Items))
	for _, item := range body.Items {
		if item.Quantity <= 0 {
			response.Error(w, http.StatusBadRequest, "items.quantity must be greater than 0")
			return
		}
		items = append(items, services.OrderItemInput{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			Customizations: item.Customizations,
		})
	}

	var createdByID *string
	if isStaff && staffID != "" {
		createdByID = &staffID
	}

	order, err := h.orderSvc.Create(r.Context(), services.CreateOrderInput{
		CustomerName:  body.CustomerName,
		CustomerPhone: body.CustomerPhone,
		CustomerEmail: nilIfEmpty(body.CustomerEmail),
		CustomerIP:    &ip,
		PaymentMethod: body.PaymentMethod,
		OrderType:     body.OrderType,
		TableNumber:   body.TableNumber,
		Notes:         nilIfEmpty(body.Notes),
		Items:         items,
		CreatedByID:   createdByID,
	})
	if err != nil {
		if errors.Is(err, services.ErrProductNotFound) || errors.Is(err, services.ErrProductUnavailable) {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Msg("Failed to create order")
		response.Error(w, http.StatusInternalServerError, "Failed to create order")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"order_number": order.OrderNumber,
		"status":       order.Status,
		"total":        order.TotalAmount,
		"message":      "Orden recibida, te notificaremos pronto 🍦",
	})
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	page := 1
	limit := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := fmt.Sscanf(pageStr, "%d", &page); err == nil && p == 1 && page > 0 {
		} else {
			page = 1
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && l == 1 && limit > 0 && limit <= 100 {
		} else {
			limit = 50
		}
	}

	orders, total, err := h.orderSvc.ListPaginated(r.Context(), status, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch orders")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
		"pagination": map[string]interface{}{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	order, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch order")
		return
	}
	response.JSON(w, http.StatusOK, mapOrderResponse(order))
}

func (h *OrderHandler) TrackPublic(w http.ResponseWriter, r *http.Request) {
	orderNumber := strings.TrimSpace(r.URL.Query().Get("order_number"))
	customerPhone := strings.TrimSpace(r.URL.Query().Get("customer_phone"))

	if orderNumber == "" || customerPhone == "" {
		response.Error(w, http.StatusBadRequest, "order_number and customer_phone are required")
		return
	}

	if !isTrackPhoneValid(customerPhone) {
		response.Error(w, http.StatusBadRequest, "invalid customer_phone format")
		return
	}

	order, err := h.orderSvc.GetByOrderNumberAndPhone(r.Context(), orderNumber, customerPhone)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch order")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"order_number": order.OrderNumber,
		"status":       order.Status,
		"updated_at":   order.UpdatedAt,
		"created_at":   order.CreatedAt,
		"total_amount": order.TotalAmount,
		"order_type":   order.OrderType,
	})
}

func (h *OrderHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	reviewerID := ""
	allowOverride := false
	if record, ok := auth.GetRecord(r.Context()); ok {
		reviewerID = auth.RecordID(record)
	}
	if auth.GetRole(r.Context()) == "admin" {
		allowOverride = true
	}

	updated, err := h.orderSvc.Approve(r.Context(), id, reviewerID, allowOverride)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) || errors.Is(err, services.ErrInvalidOrderState) {
			response.Error(w, http.StatusBadRequest,
				"Order must be in 'pendiente_revision' (or 'cancelada' for admin) to approve")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to approve order")
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

func (h *OrderHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if errs := validator.ValidateRejectReason(body.Reason); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	reviewerID := ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		reviewerID = auth.RecordID(record)
	}

	updated, err := h.orderSvc.Reject(r.Context(), id, body.Reason, reviewerID)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to reject order")
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid body")
		return
	}

	if errs := validator.ValidateOrderStatus(body.Status); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	updated, err := h.orderSvc.UpdateStatus(r.Context(), id, body.Status)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to update status")
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

func (h *OrderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var body updateOrderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.CustomerName == "" || body.CustomerPhone == "" || body.PaymentMethod == "" || body.OrderType == "" {
		response.Error(w, http.StatusBadRequest, "customer_name, customer_phone, payment_method, order_type are required")
		return
	}

	if errs := validator.ValidateOrderDetails(
		body.CustomerName,
		body.CustomerPhone,
		body.PaymentMethod,
		body.OrderType,
		body.TableNumber,
	); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	if body.OrderType == "para_llevar" {
		body.TableNumber = nil
	}

	items := make([]services.OrderItemInput, 0, len(body.Items))
	for _, item := range body.Items {
		if item.Quantity <= 0 {
			response.Error(w, http.StatusBadRequest, "items.quantity must be greater than 0")
			return
		}
		items = append(items, services.OrderItemInput{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			Customizations: item.Customizations,
		})
	}

	updated, err := h.orderSvc.Update(r.Context(), id, services.UpdateOrderInput{
		CustomerName:  body.CustomerName,
		CustomerPhone: body.CustomerPhone,
		CustomerEmail: nilIfEmpty(body.CustomerEmail),
		PaymentMethod: body.PaymentMethod,
		OrderType:     body.OrderType,
		TableNumber:   body.TableNumber,
		Notes:         nilIfEmpty(body.Notes),
		Items:         items,
	})
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		if errors.Is(err, services.ErrProductNotFound) || errors.Is(err, services.ErrProductUnavailable) {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to update order")
		return
	}

	response.JSON(w, http.StatusOK, mapOrderResponse(updated))
}

func (h *OrderHandler) UpdateNotes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var body struct {
		Notes *string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid body")
		return
	}

	var normalized *string
	if body.Notes != nil {
		trimmed := strings.TrimSpace(*body.Notes)
		if trimmed != "" {
			normalized = &trimmed
		}
	}

	updated, err := h.orderSvc.UpdateNotes(r.Context(), id, normalized)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to update notes")
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

func (h *OrderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	if err := h.orderSvc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to delete order")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Order deleted"})
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isTrackPhoneValid(phone string) bool {
	digits := 0
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits++
			continue
		}
		if r != '+' && r != ' ' && r != '-' && r != '(' && r != ')' {
			return false
		}
	}
	return digits >= 7 && digits <= 15
}
