// orders.go — Order lifecycle handlers (create, list, approve, reject, track, purge).
// Public customers submit orders and track them by token; staff manage status transitions.
// WhatsApp notifications are dispatched asynchronously via the event dispatcher.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/events"
	"github.com/cjrdz/ishosBackEnd/internal/middleware"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type OrderHandler struct {
	rateLimiter    *middleware.OrderRateLimiter
	orderSvc       services.OrderServiceInterface
	tracker        *services.TrackingTokenService
	pb             *auth.PBAuth
	settingsSvc    *services.AdminSettingService
	publicStoreURL string
}

func NewOrderHandler(db *ent.Client, tracker *services.TrackingTokenService, pb *auth.PBAuth, cfg *config.Config, rl *middleware.OrderRateLimiter, settingsSvc *services.AdminSettingService) *OrderHandler {
	orderRepo := repository.NewOrderRepository(db)
	productRepo := repository.NewProductRepository(db)
	whatsapp := services.NewWhatsAppService(cfg)
	dispatcher := events.NewDispatcher()
	dispatcher.SubscribeOrderStatusChanged(func(evt events.OrderStatusChanged) {
		switch evt.Status {
		case events.OrderStatusCreated:
			whatsapp.SendOrderCreated(evt.Phone, evt.OrderNumber, evt.Total, evt.TrackingURL)
		case events.OrderStatusApproved:
			whatsapp.SendOrderApproved(evt.Phone, evt.OrderNumber, evt.TrackingURL)
		case events.OrderStatusRejected:
			whatsapp.SendOrderRejected(evt.Phone, evt.OrderNumber, evt.Reason, evt.TrackingURL)
		case events.OrderStatusReady:
			whatsapp.SendOrderReady(evt.Phone, evt.OrderNumber, evt.TrackingURL)
		}
	})

	return &OrderHandler{
		rateLimiter:    rl,
		pb:             pb,
		tracker:        tracker,
		settingsSvc:    settingsSvc,
		publicStoreURL: strings.TrimRight(strings.TrimSpace(cfg.PublicStoreURL), "/"),
		orderSvc: services.NewOrderService(
			db,
			orderRepo,
			productRepo,
			tracker,
			dispatcher,
			cfg.PublicStoreURL,
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
	ID                     uuid.UUID           `json:"id"`
	OrderNumber            string              `json:"order_number"`
	CustomerName           string              `json:"customer_name"`
	CustomerPhone          string              `json:"customer_phone"`
	CustomerEmail          *string             `json:"customer_email,omitempty"`
	Status                 string              `json:"status"`
	PaymentMethod          string              `json:"payment_method"`
	PaymentStatus          string              `json:"payment_status"`
	OrderType              string              `json:"order_type"`
	TableNumber            *int                `json:"table_number,omitempty"`
	TotalAmount            float64             `json:"total_amount"`
	Notes                  *string             `json:"notes,omitempty"`
	RejectionReason        *string             `json:"rejection_reason,omitempty"`
	CreatedByUserID        *string             `json:"created_by_user_id,omitempty"`
	CreatedByName          string              `json:"created_by_name"`
	CreatedByRole          string              `json:"created_by_role"`
	CreatedAt              time.Time           `json:"created_at"`
	UpdatedAt              time.Time           `json:"updated_at"`
	Items                  []orderItemResponse `json:"items"`
	TrackingToken          *string             `json:"tracking_token,omitempty"`
	TrackingTokenExpiresAt *time.Time          `json:"tracking_token_expires_at,omitempty"`
	TrackingURL            *string             `json:"tracking_url,omitempty"`
}

func mapOrderResponse(order *ent.Order) orderResponse {
	return mapOrderResponseWithPB(order, nil)
}

func (h *OrderHandler) mapOrderResponse(ctx context.Context, order *ent.Order) orderResponse {
	resp := mapOrderResponseWithPB(order, h.pb)
	if trackingAccess, err := h.orderSvc.IssueTrackingAccess(ctx, order.OrderNumber); err == nil {
		resp.TrackingToken = &trackingAccess.Token
		resp.TrackingTokenExpiresAt = &trackingAccess.ExpiresAt
		if trackingAccess.URL != "" {
			resp.TrackingURL = &trackingAccess.URL
		}
	}
	return resp
}

func mapOrderResponseWithPB(order *ent.Order, pb *auth.PBAuth) orderResponse {
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

	createdByName := order.CustomerName
	createdByRole := "customer"
	if order.CreatedByUserID != nil && *order.CreatedByUserID != "" {
		createdByRole = "employee"
		if pb != nil {
			record, err := pb.FindEmployeeByID(*order.CreatedByUserID)
			if err == nil && record != nil {
				if name := strings.TrimSpace(record.GetString("name")); name != "" {
					createdByName = name
				} else {
					createdByName = record.Email()
				}
				createdByRole = auth.RoleFromRecord(record)
			} else {
				createdByName = *order.CreatedByUserID
			}
		} else {
			createdByName = *order.CreatedByUserID
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
		CreatedByName:   createdByName,
		CreatedByRole:   createdByRole,
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
		if !h.settingsSvc.AreOrdersEnabled(r.Context()) {
			response.Error(w, http.StatusServiceUnavailable, "Los pedidos están temporalmente desactivados")
			return
		}
		if !h.rateLimiter.CheckAndRecordPhone(body.CustomerPhone) {
			response.Error(w, http.StatusTooManyRequests,
				"too many orders from this phone number, try again tomorrow")
			return
		}
	}

	ip := clientIPFromRequest(r)

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
		if errors.Is(err, services.ErrProductNotFound) ||
			errors.Is(err, services.ErrProductUnavailable) ||
			errors.Is(err, services.ErrFlavorRequired) ||
			errors.Is(err, services.ErrFlavorNotAllowed) ||
			errors.Is(err, services.ErrAddonNotAllowed) {
			writeAppError(w, err, "Failed to create order")
			return
		}
		log.Error().Err(err).Msg("Failed to create order")
		response.Error(w, http.StatusInternalServerError, "Failed to create order")
		return
	}

	trackingAccess, tokenErr := h.orderSvc.IssueTrackingAccess(r.Context(), order.OrderNumber)
	if tokenErr != nil {
		log.Error().Err(tokenErr).Str("order_number", order.OrderNumber).Msg("failed to issue tracking token")
		response.Error(w, http.StatusInternalServerError, "Failed to create tracking token")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"order_number":        order.OrderNumber,
		"status":              order.Status,
		"total":               order.TotalAmount,
		"tracking_token":      trackingAccess.Token,
		"tracking_expires_at": trackingAccess.ExpiresAt,
		"message":             "Orden recibida, te notificaremos pronto 🍦",
	})
}

func clientIPFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	page := 1
	limit := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		parsed, err := strconv.Atoi(pageStr)
		if err != nil || parsed <= 0 {
			response.Error(w, http.StatusBadRequest, "page must be a positive integer")
			return
		}
		page = parsed
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 {
			response.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
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
			err = services.ErrOrderNotFound
		}
		writeAppError(w, err, "Failed to fetch order")
		return
	}
	response.JSON(w, http.StatusOK, h.mapOrderResponse(r.Context(), order))
}

func (h *OrderHandler) TrackPublic(w http.ResponseWriter, r *http.Request) {
	if !h.rateLimiter.CheckTrackingIP(clientIPFromRequest(r)) {
		response.Error(w, http.StatusTooManyRequests, "too many tracking attempts, try later")
		return
	}

	orderNumber := strings.TrimSpace(r.URL.Query().Get("order_number"))
	trackingToken := strings.TrimSpace(r.URL.Query().Get("tracking_token"))

	if orderNumber == "" || trackingToken == "" {
		response.Error(w, http.StatusBadRequest, "order_number and tracking_token are required")
		return
	}

	if !isTrackOrderNumberValid(orderNumber) {
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
				Str("ip", clientIPFromRequest(r)).
				Str("order_number", orderNumber).
				Str("token_prefix", maskTrackingToken(trackingToken)).
				Err(err).
				Msg("public tracking verification failed")
			// Return 404 rather than 401/403 to avoid revealing whether the order number exists.
			response.Error(w, http.StatusNotFound, "Order not found")
			return
		}

		log.Error().
			Str("ip", clientIPFromRequest(r)).
			Str("order_number", orderNumber).
			Err(err).
			Msg("public tracking verification backend error")
		response.Error(w, http.StatusInternalServerError, "Failed to verify tracking token")
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

	updated, err := h.orderSvc.Approve(r.Context(), id, reviewerID, allowOverride)
	if err != nil {
		writeAppError(w, err, "Failed to approve order")
		return
	}

	response.JSON(w, http.StatusOK, h.mapOrderResponse(r.Context(), updated))
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
		writeAppError(w, err, "Failed to reject order")
		return
	}

	response.JSON(w, http.StatusOK, h.mapOrderResponse(r.Context(), updated))
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
		writeAppError(w, err, "Failed to update status")
		return
	}

	response.JSON(w, http.StatusOK, h.mapOrderResponse(r.Context(), updated))
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
		writeAppError(w, err, "Failed to update order")
		return
	}

	response.JSON(w, http.StatusOK, h.mapOrderResponse(r.Context(), updated))
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
		writeAppError(w, err, "Failed to update notes")
		return
	}

	response.JSON(w, http.StatusOK, h.mapOrderResponse(r.Context(), updated))
}

func (h *OrderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	if err := h.orderSvc.Delete(r.Context(), id); err != nil {
		writeAppError(w, err, "Failed to delete order")
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

// maskTrackingToken returns only the first 8 chars of a token for safe logging.
func maskTrackingToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:8]
}

func isTrackOrderNumberValid(orderNumber string) bool {
	orderRe := regexp.MustCompile(`^ORD-\d{8}-\d{4}$`)
	return orderRe.MatchString(orderNumber)
}
