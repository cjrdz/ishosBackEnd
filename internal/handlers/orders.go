// orders.go — Thin order lifecycle handlers (create, list, approve, reject, track, purge).
// Public customers submit orders and track them by token; staff manage status transitions.
package handlers

import (
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
	orderDomain "github.com/cjrdz/ishosBackEnd/internal/domain/order"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
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
	locations      *services.LocationService
	archiver       *services.OrderArchiver
	publicStoreURL string
	listCache      *cache.TTLCache
	detailCache    *cache.TTLCache
	auditSvc       audit.Service
}

func NewOrderHandler(
	orderSvc services.OrderServiceInterface,
	tracker *services.TrackingTokenService,
	mapper *services.OrderResponseMapper,
	settingsSvc *services.AdminSettingService,
	locations *services.LocationService,
	archiver *services.OrderArchiver,
	publicStoreURL string,
	rl *middleware.OrderRateLimiter,
	auditSvc audit.Service,
) *OrderHandler {
	return &OrderHandler{
		rateLimiter:    rl,
		orderSvc:       orderSvc,
		tracker:        tracker,
		mapper:         mapper,
		settingsSvc:    settingsSvc,
		locations:      locations,
		archiver:       archiver,
		publicStoreURL: publicStoreURL,
		// The frontend polls the orders list every 25 s; a 25 s TTL means most
		// polls are served from memory instead of hitting Neon.
		listCache: cache.New(25 * time.Second),
		// Detail views are read-heavy and invalidated on mutation, so cache a
		// bit longer to make reopening/reprinting the same order fast.
		detailCache: cache.New(60 * time.Second),
		auditSvc:    auditSvc,
	}
}

func (h *OrderHandler) invalidateListCache() {
	h.listCache.InvalidateAll()
}

func (h *OrderHandler) invalidateOrderCache(id uuid.UUID) {
	h.detailCache.Delete(fmt.Sprintf("order:%s", id.String()))
}

type cachedOrderList struct {
	data         any
	lastModified time.Time
}

func maxOrderUpdatedAt(orders []dto.Order) time.Time {
	var max time.Time
	for i := range orders {
		if orders[i].UpdatedAt.After(max) {
			max = orders[i].UpdatedAt
		}
	}
	return max.UTC().Truncate(time.Second)
}

func formatHTTPTime(t time.Time) string {
	return t.UTC().Format(http.TimeFormat)
}

// isNotModified reports whether the client's If-Modified-Since value is
// at or after the resource's last modification time.
func isNotModified(r *http.Request, lastModified time.Time) bool {
	if lastModified.IsZero() {
		return false
	}
	ifModifiedSince := r.Header.Get("If-Modified-Since")
	if ifModifiedSince == "" {
		return false
	}
	clientTime, err := http.ParseTime(ifModifiedSince)
	if err != nil {
		return false
	}
	return !lastModified.After(clientTime)
}

// RequireOrderInScope guards every /orders/{id} route: an order of another
// location answers 404, exactly like an order that does not exist.
func (h *OrderHandler) RequireOrderInScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loc, ok := requestScope(w, r)
		if !ok {
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "ID de orden inválido")
			return
		}
		order, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
		if err == nil {
			err = loc.Check("order", order.LocationID)
		}
		if err != nil {
			if errors.Is(err, apperror.ErrNotFound) {
				response.Error(w, http.StatusNotFound, "Orden no encontrada")
				return
			}
			writeAppError(w, err, "No se pudo cargar la orden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body requests.CreateOrderRequest
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	// Optional client-supplied idempotency key. A key identifies a single
	// logical order submission; resending it returns the original order.
	var idempotencyKey *uuid.UUID
	if raw := r.Header.Get("Idempotency-Key"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			response.WriteOrError(w, nil, &apperror.ValidationError{
				Fields: map[string]string{"idempotency_key": "no es válida"},
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
		body.TableNumber, len(body.Items), isStaff, body.AmountReceived,
	); errs.HasErrors() {
		response.WriteOrError(w, nil, errs)
		return
	}

	if body.OrderType == "para_llevar" {
		body.TableNumber = nil
	}

	// Which location the order goes to. Staff always order for the location
	// they work at (admins: the one they selected); customers choose one.
	var locationID uuid.UUID
	if isStaff {
		loc, ok := requestScope(w, r)
		if !ok {
			return
		}
		id, err := loc.Require()
		if err != nil {
			response.WriteOrError(w, nil, err)
			return
		}
		locationID = id
	} else {
		if body.LocationID == nil || *body.LocationID == uuid.Nil {
			response.WriteOrError(w, nil, &apperror.ValidationError{Fields: map[string]string{"location_id": "Elige una sucursal"}})
			return
		}
		locationID = *body.LocationID
	}
	// Customers also need online ordering on and the location open; counter
	// staff only need the location's orders switch.
	if err := h.locations.CheckCanOrder(r.Context(), locationID, !isStaff, time.Now()); err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	if !isStaff {
		res := h.rateLimiter.CheckAndRecordPhone(r.Context(), body.CustomerPhone)
		res.WriteHeaders(w)
		if !res.Allowed {
			log.Warn().Str("phone", body.CustomerPhone).Msg("Phone order rate limit exceeded")
			response.Error(w, http.StatusTooManyRequests, "Demasiadas órdenes desde este teléfono. Intenta de nuevo más tarde.")
			return
		}
	}

	items := make([]services.OrderItemInput, 0, len(body.Items))
	for _, item := range body.Items {
		if item.Quantity <= 0 {
			response.WriteOrError(w, nil, &apperror.ValidationError{Fields: map[string]string{"items.quantity": "La cantidad debe ser mayor que 0"}})
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
		LocationID:     locationID,
		IdempotencyKey: idempotencyKey,
		CustomerName:   body.CustomerName,
		CustomerPhone:  body.CustomerPhone,
		CustomerEmail:  nilIfEmpty(body.CustomerEmail),
		CustomerIP:     &ip,
		PaymentMethod:  body.PaymentMethod,
		AmountReceived: body.AmountReceived,
		OrderType:      body.OrderType,
		TableNumber:    body.TableNumber,
		Notes:          nilIfEmpty(body.Notes),
		Items:          items,
		CreatedByID:    createdByID,
	})
	if err != nil {
		// Customers get a short message without stock counts or staff
		// instructions; staff get the detailed one.
		var stockErr *apperror.InsufficientStockError
		if !isStaff && errors.As(err, &stockErr) {
			response.Error(w, http.StatusConflict, stockErr.Name+" se agotó en esta sucursal. Quítalo del carrito o elige otro.")
			return
		}
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

	respBody := map[string]any{
		"order_number":        order.OrderNumber,
		"status":              order.Status,
		"total":               order.TotalAmount,
		"tracking_token":      trackingAccess.Token,
		"tracking_expires_at": trackingAccess.ExpiresAt,
		"message":             "Orden recibida, te notificaremos pronto 🍦",
	}
	// Staff fetch the full order by id to print its tickets right away; public
	// customers track by number + token and never need the internal id.
	if isStaff {
		respBody["id"] = order.ID
	}
	if order.AmountReceived != nil {
		respBody["amount_received"] = *order.AmountReceived
		respBody["change"] = *order.AmountReceived - order.TotalAmount
	}
	response.JSON(w, http.StatusCreated, respBody)
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	// status: one status or a comma-separated list (e.g. the board's active columns).
	status := r.URL.Query().Get("status")
	if status != "" {
		for _, s := range strings.Split(status, ",") {
			if _, err := orderDomain.NewOrderStatus(s); err != nil {
				response.Error(w, http.StatusBadRequest, "Estado de orden inválido: "+s)
				return
			}
		}
	}
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	lite := r.URL.Query().Get("lite") == "true"
	archived := r.URL.Query().Get("archived") == "true"
	params := response.ParsePaginationParams(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	cacheKey := fmt.Sprintf("orders:%s:%s:%t:%t:%d:%d", loc, status, lite, archived, params.Page, params.PerPage)

	if cached, ok := h.listCache.Get(cacheKey); ok {
		entry := cached.(cachedOrderList)
		if isNotModified(r, entry.lastModified) {
			w.Header().Set("Last-Modified", formatHTTPTime(entry.lastModified))
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", formatHTTPTime(entry.lastModified))
		response.JSON(w, http.StatusOK, entry.data)
		return
	}

	orders, total, err := h.orderSvc.ListPaginated(r.Context(), loc, status, archived, params.Page, params.PerPage)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	lastModified := maxOrderUpdatedAt(orders)

	if isNotModified(r, lastModified) {
		w.Header().Set("Last-Modified", formatHTTPTime(lastModified))
		w.WriteHeader(http.StatusNotModified)
		return
	}

	var paginated any
	if lite {
		responses := make([]services.OrderLiteResponse, len(orders))
		for i, o := range orders {
			responses[i] = h.mapper.MapLite(o)
		}
		paginated = response.NewPaginated(responses, params.Page, params.PerPage, total)
	} else {
		// Preload employee cache in one query to avoid N+1 PocketBase lookups.
		h.mapper.PreloadEmployees()
		responses := make([]services.OrderResponse, len(orders))
		for i, o := range orders {
			responses[i] = h.mapper.Map(o)
		}
		paginated = response.NewPaginated(responses, params.Page, params.PerPage, total)
	}

	h.listCache.Set(cacheKey, cachedOrderList{data: paginated, lastModified: lastModified})
	w.Header().Set("Last-Modified", formatHTTPTime(lastModified))
	response.JSON(w, http.StatusOK, paginated)
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
		return
	}

	cacheKey := fmt.Sprintf("order:%s", id.String())
	if cached, ok := h.detailCache.Get(cacheKey); ok {
		response.JSON(w, http.StatusOK, cached)
		return
	}

	order, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			err = services.ErrOrderNotFound
		}
		writeAppError(w, err, "No se pudo cargar la orden")
		return
	}

	// Warm employee cache so created_by resolution does not hit PocketBase
	// on a cold cache for the first detail request after server start.
	h.mapper.PreloadEmployees()
	resp := h.mapper.Map(order)
	h.detailCache.Set(cacheKey, resp)
	response.JSON(w, http.StatusOK, resp)
}

func (h *OrderHandler) TrackPublic(w http.ResponseWriter, r *http.Request) {
	res := h.rateLimiter.CheckTrackingIP(r.Context(), auth.ClientIP(r))
	res.WriteHeaders(w)
	if !res.Allowed {
		response.Error(w, http.StatusTooManyRequests, "Demasiados intentos de seguimiento. Intenta de nuevo en unos minutos.")
		return
	}

	// The admin can switch public tracking off (e.g. while only staff test
	// ordering). Answer as if the order did not exist.
	if h.settingsSvc != nil {
		if settings, err := h.settingsSvc.GetStoreSettings(r.Context()); err == nil && !settings.TrackingEnabled {
			response.Error(w, http.StatusNotFound, "El seguimiento de órdenes no está disponible por ahora")
			return
		}
	}

	orderNumber := strings.TrimSpace(r.URL.Query().Get("order_number"))
	trackingToken := strings.TrimSpace(r.URL.Query().Get("tracking_token"))

	if orderNumber == "" || trackingToken == "" {
		response.Error(w, http.StatusBadRequest, "Faltan el número de orden o el código de seguimiento")
		return
	}

	if !validator.IsTrackOrderNumberValid(orderNumber) {
		response.Error(w, http.StatusBadRequest, "El número de orden no es válido")
		return
	}

	order, err := h.orderSvc.GetByOrderNumber(r.Context(), orderNumber)
	if err != nil {
		writeAppError(w, err, "No se pudo cargar la orden")
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
			response.Error(w, http.StatusNotFound, "Orden no encontrada")
			return
		}
		log.Error().Str("ip", auth.ClientIP(r)).Str("order_number", orderNumber).Err(err).Msg("public tracking verification backend error")
		response.Error(w, http.StatusInternalServerError, "No se pudo verificar el seguimiento")
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

	// Where to pick the order up.
	var pickup map[string]any
	if l, err := h.locations.Get(r.Context(), order.LocationID); err == nil {
		pickup = map[string]any{
			"id": l.ID, "name": l.Name, "address": l.Address,
			"maps_url": l.MapsURL, "whatsapp": l.WhatsApp, "phone": l.Phone,
		}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"order_number":      order.OrderNumber,
		"location":          pickup,
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

func (h *OrderHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
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

	ctx := audit.ContextFromRequest(r, reviewerID)
	updated, err := h.orderSvc.Approve(ctx, id, reviewerID, allowOverride)
	if err != nil {
		writeAppError(w, err, "No se pudo aprobar la orden")
		return
	}

	h.invalidateListCache()
	h.invalidateOrderCache(id)
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

func (h *OrderHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
		return
	}

	var body struct {
		Reason string `json:"reason"`
		// Restock, when sent, says how much of each inventory item the order
		// took goes back to stock; the rest is recorded as waste (merma).
		// Omitted: everything goes back.
		Restock *[]struct {
			InventoryItemID uuid.UUID `json:"inventory_item_id"`
			Quantity        int       `json:"quantity"`
		} `json:"restock"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	var restock map[uuid.UUID]int
	if body.Restock != nil {
		if len(*body.Restock) > 200 {
			response.Error(w, http.StatusBadRequest, "Demasiados artículos para devolver")
			return
		}
		restock = make(map[uuid.UUID]int, len(*body.Restock))
		for _, line := range *body.Restock {
			if line.Quantity < 0 || line.InventoryItemID == uuid.Nil {
				response.Error(w, http.StatusBadRequest, "Cantidad a devolver inválida")
				return
			}
			restock[line.InventoryItemID] += line.Quantity
		}
	}

	if errs := validator.ValidateRejectReason(body.Reason); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	reviewerID, role := "", ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		reviewerID = auth.RecordID(record)
		role = auth.RoleFromRecord(record)
	}

	// Rejecting a pending online order is routine. Cancelling one that was
	// accepted (maybe already paid, or in preparation) gives its stock back
	// and voids a sale, so only admins and managers may do it.
	existing, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		writeAppError(w, err, "No se pudo cargar la orden")
		return
	}
	if existing.Status != "pendiente_revision" && role != auth.RoleAdmin && role != auth.RoleManager {
		response.Error(w, http.StatusForbidden, "Solo un administrador o encargado puede cancelar una orden aceptada")
		return
	}

	ctx := audit.ContextFromRequest(r, reviewerID)
	updated, err := h.orderSvc.Reject(ctx, id, body.Reason, reviewerID, restock)
	if err != nil {
		writeAppError(w, err, "No se pudo rechazar la orden")
		return
	}

	h.invalidateListCache()
	h.invalidateOrderCache(id)
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	if errs := validator.ValidateOrderStatus(body.Status); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	existing, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Orden no encontrada")
			return
		}
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar la orden")
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

	ctx := audit.ContextFromRequest(r, actorID)
	updated, err := h.orderSvc.UpdateStatus(ctx, id, body.Status)
	if err != nil {
		writeAppError(w, err, "No se pudo actualizar el estado")
		return
	}

	h.invalidateListCache()
	h.invalidateOrderCache(id)
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

func (h *OrderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
		return
	}

	existing, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Orden no encontrada")
			return
		}
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar la orden")
		return
	}
	if !h.orderSvc.IsOrderEditable(existing) {
		response.Error(w, http.StatusConflict, "No se puede editar una orden que ya fue aceptada, rechazada, entregada o cancelada")
		return
	}

	var body requests.UpdateOrderRequest
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	if body.CustomerName == "" || body.PaymentMethod == "" || body.OrderType == "" {
		response.Error(w, http.StatusBadRequest, "Faltan el nombre, el método de pago o el tipo de orden")
		return
	}

	if errs := validator.ValidateOrderDetails(
		body.CustomerName, body.CustomerPhone, body.PaymentMethod, body.OrderType,
		body.TableNumber, true, body.AmountReceived,
	); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	if body.OrderType == "para_llevar" {
		body.TableNumber = nil
	}

	notes := strings.TrimSpace(body.Notes)
	if notes == "" {
		response.Error(w, http.StatusBadRequest, "Escribe una nota con el motivo del cambio")
		return
	}

	items := make([]services.OrderItemInput, 0, len(body.Items))
	for _, item := range body.Items {
		if item.Quantity <= 0 {
			response.Error(w, http.StatusBadRequest, "La cantidad de cada producto debe ser mayor que 0")
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

	ctx := audit.ContextFromRequest(r, actorID)
	updated, err := h.orderSvc.Update(ctx, id, services.UpdateOrderInput{
		CustomerName:   body.CustomerName,
		CustomerPhone:  body.CustomerPhone,
		CustomerEmail:  nilIfEmpty(body.CustomerEmail),
		PaymentMethod:  body.PaymentMethod,
		AmountReceived: body.AmountReceived,
		OrderType:      body.OrderType,
		TableNumber:    body.TableNumber,
		Notes:          &notes,
		Items:          items,
	})
	if err != nil {
		writeAppError(w, err, "No se pudo actualizar la orden")
		return
	}

	h.invalidateListCache()
	h.invalidateOrderCache(id)
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

func (h *OrderHandler) UpdateNotes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
		return
	}

	var body struct {
		Notes *string `json:"notes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
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

	ctx := audit.ContextFromRequest(r, actorID)
	updated, err := h.orderSvc.UpdateNotes(ctx, id, normalized)
	if err != nil {
		writeAppError(w, err, "No se pudo guardar la nota")
		return
	}

	h.invalidateListCache()
	h.invalidateOrderCache(id)
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

func (h *OrderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
		return
	}

	existing, err := h.orderSvc.GetByIDWithItems(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Orden no encontrada")
			return
		}
		response.Error(w, http.StatusInternalServerError, "No se pudo cargar la orden")
		return
	}
	if !h.orderSvc.IsOrderEditable(existing) {
		response.Error(w, http.StatusConflict, "No se puede eliminar una orden que ya fue aceptada, rechazada, entregada o cancelada")
		return
	}

	actorID := ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(record)
	}

	ctx := audit.ContextFromRequest(r, actorID)
	if err := h.orderSvc.Delete(ctx, id); err != nil {
		writeAppError(w, err, "No se pudo eliminar la orden")
		return
	}

	h.invalidateListCache()
	h.invalidateOrderCache(id)
	response.JSON(w, http.StatusOK, map[string]string{"message": "Orden eliminada"})
}

// Archive toggles the archive flag of an order.
//
// PATCH /api/v1/orders/{id}/archive
// Body: { "archived": bool }
func (h *OrderHandler) Archive(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de orden inválido")
		return
	}

	var body struct {
		Archived bool `json:"archived"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}

	actorID := ""
	if record, ok := auth.GetRecord(r.Context()); ok {
		actorID = auth.RecordID(record)
	}

	ctx := audit.ContextFromRequest(r, actorID)
	updated, err := h.orderSvc.SetArchived(ctx, id, body.Archived)
	if err != nil {
		writeAppError(w, err, "No se pudo archivar la orden")
		return
	}

	h.invalidateListCache()
	h.invalidateOrderCache(id)
	response.JSON(w, http.StatusOK, h.mapper.Map(updated))
}

// RunArchive triggers an immediate auto-archive pass.
//
// POST /api/v1/admin/orders/archive/run
func (h *OrderHandler) RunArchive(w http.ResponseWriter, r *http.Request) {
	if h.archiver == nil {
		response.Error(w, http.StatusServiceUnavailable, "El archivado automático no está configurado")
		return
	}

	archived, err := h.archiver.RunOnce(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Manual archive run failed")
		response.Error(w, http.StatusInternalServerError, "No se pudo archivar")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"archived": archived,
		"message":  "Archivo ejecutado",
	})
}
