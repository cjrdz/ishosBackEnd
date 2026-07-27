// users.go — Customer (end-user) management handlers.
// Customers are stored in Postgres (not PocketBase); they are created automatically on first order.
package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type UserHandler struct {
	customerSvc services.CustomerServiceInterface
	auditSvc    audit.Service
}

func NewUserHandler(svc services.CustomerServiceInterface, auditSvc audit.Service) *UserHandler {
	return &UserHandler{customerSvc: svc, auditSvc: auditSvc}
}

type userResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	UserType  string    `json:"user_type"`
	Phone     string    `json:"phone"`
	Email     *string   `json:"email,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type userOrderResponse struct {
	ID            uuid.UUID `json:"id"`
	OrderNumber   string    `json:"order_number"`
	CustomerName  string    `json:"customer_name"`
	CustomerPhone string    `json:"customer_phone"`
	TotalAmount   float64   `json:"total_amount"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func toUserResponse(customer dto.Customer) userResponse {
	return userResponse{
		ID:        customer.ID,
		Name:      customer.Name,
		UserType:  customer.UserType,
		Phone:     customer.Phone,
		Email:     customer.Email,
		Status:    customer.Status,
		CreatedAt: customer.CreatedAt,
		UpdatedAt: customer.UpdatedAt,
	}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	params := response.ParsePaginationParams(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("per_page"),
	)

	// Convert page/per_page to offset/limit for service layer
	offset := (params.Page - 1) * params.PerPage

	customers, total, err := h.customerSvc.ListPaginated(r.Context(), status, search, offset, params.PerPage)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}

	result := make([]userResponse, 0, len(customers))
	for _, customer := range customers {
		result = append(result, toUserResponse(customer))
	}

	response.JSON(w, http.StatusOK, response.NewPaginated(result, params.Page, params.PerPage, total))
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de usuario invalido")
		return
	}

	customer, err := h.customerSvc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Usuario no encontrado")
			return
		}
		log.Error().Err(err).Msg("Failed to get user")
		response.Error(w, http.StatusInternalServerError, "No se pudo obtener usuario")
		return
	}

	response.JSON(w, http.StatusOK, toUserResponse(customer))
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.createOrUpsert(r, w, http.StatusCreated, false)
}

// Upsert creates a customer if the phone does not exist, or updates the
// existing customer. Used when saving a customer from an order.
func (h *UserHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	h.createOrUpsert(r, w, http.StatusOK, true)
}

func (h *UserHandler) createOrUpsert(r *http.Request, w http.ResponseWriter, statusCode int, upsert bool) {
	var body struct {
		Name     string `json:"name"`
		UserType string `json:"user_type"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
		Status   string `json:"status"`
	}

	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud invalido")
		return
	}

	if errs := validator.ValidateUserCreate(body.Name, body.UserType, body.Phone, body.Email, body.Status); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	var email *string
	if body.Email != "" {
		email = &body.Email
	}

	input := repository.CreateCustomerInput{
		Name:     body.Name,
		UserType: body.UserType,
		Phone:    body.Phone,
		Email:    email,
		Status:   body.Status,
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	var result dto.Customer
	var err error
	if upsert {
		result, err = h.customerSvc.UpsertByPhone(ctx, input)
	} else {
		result, err = h.customerSvc.Create(ctx, input)
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		response.Error(w, http.StatusInternalServerError, "No se pudo crear usuario")
		return
	}

	action := audit.ActionUserCreated
	if upsert {
		action = audit.ActionUserUpserted
	}
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, action, actorID, fmt.Sprintf("user:%s", result.ID.String()), map[string]any{
		"name":  result.Name,
		"phone": result.Phone,
	}, ip)

	response.JSON(w, statusCode, toUserResponse(result))
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de usuario invalido")
		return
	}

	var body struct {
		Name     *string `json:"name"`
		UserType *string `json:"user_type"`
		Phone    *string `json:"phone"`
		Email    *string `json:"email"`
		Status   *string `json:"status"`
	}

	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud invalido")
		return
	}

	if errs := validator.ValidateUserUpdate(body.Name, body.UserType, body.Phone, body.Email, body.Status); errs.HasErrors() {
		response.Error(w, http.StatusBadRequest, errs.Error())
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	updated, err := h.customerSvc.Update(ctx, id, repository.UpdateCustomerInput{
		Name:     body.Name,
		UserType: body.UserType,
		Phone:    body.Phone,
		Email:    body.Email,
		Status:   body.Status,
	})
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Usuario no encontrado")
			return
		}
		log.Error().Err(err).Msg("Failed to update user")
		response.Error(w, http.StatusInternalServerError, "No se pudo actualizar usuario")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionUserUpdated, actorID, fmt.Sprintf("user:%s", id.String()), map[string]any{
		"name":  updated.Name,
		"phone": updated.Phone,
	}, ip)

	response.JSON(w, http.StatusOK, toUserResponse(updated))
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de usuario invalido")
		return
	}

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)

	if err := h.customerSvc.Delete(ctx, id); err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Usuario no encontrado")
			return
		}
		log.Error().Err(err).Msg("Failed to delete user")
		response.Error(w, http.StatusInternalServerError, "No se pudo eliminar usuario")
		return
	}

	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionUserDeleted, actorID, fmt.Sprintf("user:%s", id.String()), nil, ip)

	response.JSON(w, http.StatusOK, map[string]string{"message": "Usuario eliminado"})
}

func (h *UserHandler) Orders(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de usuario invalido")
		return
	}

	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	orders, err := h.customerSvc.OrdersByCustomer(r.Context(), id, limit)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "Usuario no encontrado")
			return
		}
		log.Error().Err(err).Msg("Failed to list user orders")
		response.Error(w, http.StatusInternalServerError, "No se pudo listar historial")
		return
	}

	result := make([]userOrderResponse, 0, len(orders))
	for _, order := range orders {
		result = append(result, userOrderResponse{
			ID:            order.ID,
			OrderNumber:   order.OrderNumber,
			CustomerName:  order.CustomerName,
			CustomerPhone: order.CustomerPhone,
			TotalAmount:   order.TotalAmount,
			Status:        order.Status,
			CreatedAt:     order.CreatedAt,
		})
	}

	response.JSON(w, http.StatusOK, map[string]any{"orders": result})
}
