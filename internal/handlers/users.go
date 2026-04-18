// users.go — Customer (end-user) management handlers.
// Customers are stored in Postgres (not PocketBase); they are created automatically on first order.
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/internal/validator"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type UserHandler struct {
	customerSvc services.CustomerServiceInterface
}

func NewUserHandler(db *ent.Client) *UserHandler {
	repo := repository.NewCustomerRepository(db)
	return &UserHandler{customerSvc: services.NewCustomerService(repo)}
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

func toUserResponse(customer *ent.Customer) userResponse {
	return userResponse{
		ID:        customer.ID,
		Name:      customer.Name,
		UserType:  string(customer.UserType),
		Phone:     customer.Phone,
		Email:     customer.Email,
		Status:    string(customer.Status),
		CreatedAt: customer.CreatedAt,
		UpdatedAt: customer.UpdatedAt,
	}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	limit := 50
	offset := 0

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			response.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	if rawOffset := r.URL.Query().Get("offset"); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			response.Error(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = parsed
	}

	customers, total, err := h.customerSvc.ListPaginated(r.Context(), status, search, offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list users")
		response.Error(w, http.StatusInternalServerError, "No se pudieron listar usuarios")
		return
	}

	result := make([]userResponse, 0, len(customers))
	for _, customer := range customers {
		result = append(result, toUserResponse(customer))
	}

	// Preserve array response shape expected by existing admin clients.
	// Pagination metadata is exposed via headers for forward-compatible consumers.
	w.Header().Set("X-Pagination-Limit", strconv.Itoa(limit))
	w.Header().Set("X-Pagination-Offset", strconv.Itoa(offset))
	w.Header().Set("X-Pagination-Total", strconv.Itoa(total))
	response.JSON(w, http.StatusOK, result)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de usuario invalido")
		return
	}

	customer, err := h.customerSvc.GetByID(r.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
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

	created, err := h.customerSvc.Create(r.Context(), repository.CreateCustomerInput{
		Name:     body.Name,
		UserType: body.UserType,
		Phone:    body.Phone,
		Email:    email,
		Status:   body.Status,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		response.Error(w, http.StatusInternalServerError, "No se pudo crear usuario")
		return
	}

	response.JSON(w, http.StatusCreated, toUserResponse(created))
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

	updated, err := h.customerSvc.Update(r.Context(), id, repository.UpdateCustomerInput{
		Name:     body.Name,
		UserType: body.UserType,
		Phone:    body.Phone,
		Email:    body.Email,
		Status:   body.Status,
	})
	if err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Usuario no encontrado")
			return
		}
		log.Error().Err(err).Msg("Failed to update user")
		response.Error(w, http.StatusInternalServerError, "No se pudo actualizar usuario")
		return
	}

	response.JSON(w, http.StatusOK, toUserResponse(updated))
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "ID de usuario invalido")
		return
	}

	if err := h.customerSvc.Delete(r.Context(), id); err != nil {
		if ent.IsNotFound(err) {
			response.Error(w, http.StatusNotFound, "Usuario no encontrado")
			return
		}
		log.Error().Err(err).Msg("Failed to delete user")
		response.Error(w, http.StatusInternalServerError, "No se pudo eliminar usuario")
		return
	}

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
		if ent.IsNotFound(err) {
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
			Status:        string(order.Status),
			CreatedAt:     order.CreatedAt,
		})
	}

	response.JSON(w, http.StatusOK, map[string]any{"orders": result})
}
