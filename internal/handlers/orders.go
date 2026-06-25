// orders.go — Thin order lifecycle handlers (create, list, approve, reject, track, purge).
// Public customers submit orders and track them by token; staff manage status transitions.
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

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/handlers/requests"
	"github.com/cjrdz/ishosBackEnd/internal/middleware"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type OrderHandler struct {
	rateLimiter    *middleware.OrderRateLimiter
	orderSvc       services.OrderServiceInterface
	tracker        *services.TrackingTokenService
	mapper         *services.OrderResponseMapper
	settingsSvc    *services.AdminSettingService
	publicStoreURL string
	responseCache  *cache.TTLCache
}

func NewOrderHandler(
	orderSvc services.OrderServiceInterface,
	tracker *services.TrackingTokenService,
	mapper *services.OrderResponseMapper,
	settingsSvc *services.AdminSettingService,
	publicStoreURL string,
	rl *middleware.OrderRateLimiter,
) *OrderHandler {
	return &OrderHandler{
		rateLimiter:    rl,
		orderSvc:       orderSvc,
		tracker:        tracker,
		mapper:         mapper,
		settingsSvc:    settingsSvc,
		publicStoreURL: publicStoreURL,
		responseCache:  cache.New(5 * time.Second),
	}
}

func (h *OrderHandler) invalidateListCache() {
	h.responseCache.InvalidateAll()
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body requests.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	// Optional client-supplied idempotency key. A key identifies a single
	// logical order submission; resending it returns the original order.
	var idempotencyKey *uuid.UUID
	if raw := r.Header.Get("Idempotency-Key"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			response.WriteOrError(w, nil, &apperror.ValidationError{
				Fields: map[string]string{"idempotency_key": "must be a valid UUID"},
			})
			return
		}
		idempotencyKey = &parsed
	}

	if body.OrderType == "" {
		body.OrderType = "para_llevar"
	}

	isStaff, staffID := false, ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		isStaff = true
		staffID = auth.RecordID(record)
	}

	if errs := validator.ValidateOrderCreate(
		body.CustomerName, body.CustomerPhone, body.PaymentMethod, body.OrderType,
		body.TableNumber, len(body.Items), isStaff,
	); errs.HasErrors() {
		response.WriteOrError(w, nil, errs)
		return
	}

	if body.OrderType == "para_llevar" {
		body.TableNumber = nil
	}

	if !isStaff {
		if !h.settingsSvc.AreOrdersEnabled(r.Context()) {
			response.WriteOrError(w, nil, &apperror.ServiceUnavailableError{Message: "Los pedidos están temporalmente desactivados"})
			return
		}
		if !h.rateLimiter.CheckAndRecordPhone(body.CustomerPhone) {
			response.WriteOrError(w, nil, &apperror.RateLimitError{})
			return
		}
	}

	items := make([]services.OrderItemInput, 0, len(body.Items))
	for _, item := range body.Items {
		if item.Quantity <= 0 {
			response.WriteOrError(w, nil, &apperror.ValidationError{Fields: map[string]string{"items.quantity": "must be greater than 0"}})
			return
		}
		items = append(items, services.OrderItemInput{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			Customizations: item.Customizations,
		})
	}

	createdByID := (*string)(nil)
	if isStaff && staffID != "" {
		createdByID = &staffID
	}

	ip := auth.ClientIP(r)
	ctx := audit.ContextFromRequest(r, staffID)

	order, err := h.orderSvc.Create(ctx, services.CreateOrderInput{
		IdempotencyKey: idempotencyKey,
		CustomerName:   body.CustomerName,
		CustomerPhone:  body.CustomerPhone,
		CustomerEmail:  nilIfEmpty(body.CustomerEmail),
		CustomerIP:     &ip,
		PaymentMethod:  body.PaymentMethod,
		OrderType:      body.OrderType,
		TableNumber:    body.TableNumber,
		Notes:          nilIfEmpty(body.Notes),
		Items:          items,
		CreatedByID:    createdByID,
	})
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	h.invalidateListCache()

	trackingAccess, err := h.orderSvc.IssueTrackingAccess(r.Context(), order.OrderNumber)
	if err != nil {
		log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("failed to issue tracking token")
		response.WriteOrError(w, nil, err)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"order_number":        order.OrderNumber,
		"status":              order.Status,
		"total":               order.TotalAmount,
		"tracking_token":      trackingAccess.Token,
		"tracking_expires_at": trackingAccess.ExpiresAt,
		"message":             "Orden recibida, te notificaremos pronto 🍦",
	})
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	lite := r.URL.Query().Get("lite") == "true"
	params := response.ParsePaginationParams(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	cacheKey := fmt.Sprintf("orders:%s:%t:%d:%d", status, lite, params.Page, params.PerPage)

	if cached, ok := h.responseCache.Get(cacheKey); ok {
		response.JSON(w, http.StatusOK, cached)
		return
	}

	orders, total, err := h.orderSvc.ListPaginated(r.Context(), status, params.Page, params.PerPage)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	if lite {
		responses := make([]services.OrderLiteResponse, len(orders))
		for i, o := range orders {
			responses[i] = h.mapper.MapLite(o)
		}
		paginated := response.NewPaginated(responses, params.Page, params.PerPage, total)
		h.responseCache.Set(cacheKey, paginated)
		response.JSON(w, http.StatusOK, paginated)
		return
	}

	// Preload employee cache in one query to avoid N+1 PocketBase lookups.
	h.mapper.PreloadEmployees()
	responses := make([]services.OrderResponse, len(orders))
	for i, o := range orders {
		responses[i] = h.mapper.Map(o)
	}
	paginated := response.NewPaginated(responses, params.Page, params.PerPage, total)
	h.responseCache.Set(cacheKey, paginated)
	response.JSON(w, http.StatusOK, paginated)
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	order, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			err = services.ErrOrderNotFound
		}
		writeAppError(w, err, "Failed to fetch order")
		return
	}

	resp := h.mapper.Map(order)
	response.JSON(w, http.StatusOK, resp)
}

func (h *OrderHandler) TrackPublic(w http.ResponseWriter, r *http.Request) {
	if !h.rateLimiter.CheckTrackingIP(auth.ClientIP(r)) {
		response.Error(w, http.StatusTooManyRequests, "too many tracking attempts, try later")
		return
	}

	orderNumber := strings.TrimSpace(r.URL.Query().Get("order_number"))
	trackingToken := strings.TrimSpace(r.URL.Query().Get("tracking_token"))

	if orderNumber == "" || trackingToken == "" {
		response.Error(w, http.StatusBadRequest, "order_number and tracking_token are required")
		return
	}

	if !validator.IsTrackOrderNumberValid(orderNumber) {
		response.Error(w, http.StatusBadRequest, "invalid order_number format")
		return
	}

	order, err := h.orderSvc.GetByOrderNumber(r.Context(), orderNumber)
	if err != nil {
		writeAppError(w, err, "Failed to fetch order")
		return
	}

	if err := h.tracker.Verify(r.Context(), order.OrderNumber, trackingToken, time.Now()); err != nil {
		if errors.Is(err, services.ErrInvalidTrackingToken) || errors.Is(err, services.ErrExpiredTrackingToken) {
			log.Warn().
				Str("ip", auth.ClientIP(r)).
				Str("order_number", orderNumber).
				Str("token_prefix", services.MaskTrackingToken(trackingToken)).
				Err(err).
				Msg("public tracking verification failed")
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		log.Error().Str("ip", auth.ClientIP(r)).Str("order_number", orderNumber).Err(err).Msg("public tracking verification backend error")
		response.Error(w, http.StatusInternalServerError, "Failed to verify tracking token")
		return
	}

	publicItems := make([]map[string]any, 0, len(order.Items))
	for _, item := range order.Items {
		publicItem := map[string]any{
			"product_name": item.ProductName,
			"quantity":     item.Quantity,
			"unit_price":   item.UnitPrice,
			"subtotal":     item.Subtotal,
		}
		if item.Customizations != nil {
			publicItem["customizations"] = item.Customizations
		}
		publicItems = append(publicItems, publicItem)
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"order_number":      order.OrderNumber,
		"customer_name":     order.CustomerName,
		"status":            order.Status,
		"updated_at":        order.UpdatedAt,
		"created_at":        order.CreatedAt,
		"total_amount":      order.TotalAmount,
		"order_type":        order.OrderType,
		"status_timestamps": order.StatusTimestamps,
		"items":             publicItems,
	})
}

func (h *OrderHandler) TrackPublicHistory(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusGone, "tracking history endpoint has been removed")
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

	updated, err := h.orderSvc.Approve(audit.ContextFromRequest(r, reviewerID), id, reviewerID, allowOverride)
	if err != nil {
		writeAppError(w, err, "Failed to approve order")
		return
	}

	h.invalidateListCache()
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
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

	updated, err := h.orderSvc.Reject(audit.ContextFromRequest(r, reviewerID), id, body.Reason, reviewerID)
	if err != nil {
		writeAppError(w, err, "Failed to reject order")
		return
	}

	h.invalidateListCache()
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
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

	existing, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch order")
		return
	}

	if existing.Status == "cancelada" || existing.Status == "entregada" {
		response.Error(w, http.StatusConflict, "No se puede cambiar el estado de una orden cancelada o entregada")
		return
	}

	actorID := ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(record)
	}

	updated, err := h.orderSvc.UpdateStatus(audit.ContextFromRequest(r, actorID), id, body.Status)
	if err != nil {
		writeAppError(w, err, "Failed to update status")
		return
	}

	h.invalidateListCache()
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

func (h *OrderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	existing, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch order")
		return
	}
	if !h.orderSvc.IsOrderEditable(existing.Status) {
		response.Error(w, http.StatusConflict, "No se puede editar una orden que ya fue aceptada, rechazada, entregada o cancelada")
		return
	}

	var body requests.UpdateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.CustomerName == "" || body.CustomerPhone == "" || body.PaymentMethod == "" || body.OrderType == "" {
		response.Error(w, http.StatusBadRequest, "customer_name, customer_phone, payment_method, order_type are required")
		return
	}

	if errs := validator.ValidateOrderDetails(
		body.CustomerName, body.CustomerPhone, body.PaymentMethod, body.OrderType,
		body.TableNumber, true,
	); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	if body.OrderType == "para_llevar" {
		body.TableNumber = nil
	}

	notes := strings.TrimSpace(body.Notes)
	if notes == "" {
		response.Error(w, http.StatusBadRequest, "notes is required when editing orders")
		return
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

	actorID := ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(record)
	}

	updated, err := h.orderSvc.Update(audit.ContextFromRequest(r, actorID), id, services.UpdateOrderInput{
		CustomerName:  body.CustomerName,
		CustomerPhone: body.CustomerPhone,
		CustomerEmail: nilIfEmpty(body.CustomerEmail),
		PaymentMethod: body.PaymentMethod,
		OrderType:     body.OrderType,
		TableNumber:   body.TableNumber,
		Notes:         &notes,
		Items:         items,
	})
	if err != nil {
		writeAppError(w, err, "Failed to update order")
		return
	}

	h.invalidateListCache()
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
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
		if trimmed := strings.TrimSpace(*body.Notes); trimmed != "" {
			normalized = &trimmed
		}
	}

	actorID := ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(record)
	}

	updated, err := h.orderSvc.UpdateNotes(audit.ContextFromRequest(r, actorID), id, normalized)
	if err != nil {
		writeAppError(w, err, "Failed to update notes")
		return
	}

	h.invalidateListCache()
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

func (h *OrderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	existing, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to fetch order")
		return
	}
	if !h.orderSvc.IsOrderEditable(existing.Status) {
		response.Error(w, http.StatusConflict, "No se puede eliminar una orden que ya fue aceptada, rechazada, entregada o cancelada")
		return
	}

	actorID := ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(record)
	}

	if err := h.orderSvc.Delete(audit.ContextFromRequest(r, actorID), id); err != nil {
		writeAppError(w, err, "Failed to delete order")
		return
	}

	h.invalidateListCache()
	response.JSON(w, http.StatusOK, map[string]string{"message": "Order deleted"})
}
