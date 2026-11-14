// locations.go — Locations (sucursales): public list, settings and Agotados.
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// LocationHandler serves /locations.
type LocationHandler struct {
	locations *services.LocationService
	auditSvc  audit.Service
}

// NewLocationHandler creates a location handler.
func NewLocationHandler(locations *services.LocationService, auditSvc audit.Service) *LocationHandler {
	return &LocationHandler{locations: locations, auditSvc: auditSvc}
}

// publicLocation is what customers see: contact details and live status,
// without internal switches.
type publicLocation struct {
	ID            uuid.UUID                `json:"id"`
	Slug          string                   `json:"slug"`
	Name          string                   `json:"name"`
	Address       string                   `json:"address"`
	MapsURL       string                   `json:"maps_url"`
	MapsEmbedURL  string                   `json:"maps_embed_url"`
	WhatsApp      string                   `json:"whatsapp"`
	Phone         string                   `json:"phone"`
	AcceptsOnline bool                     `json:"accepts_online_orders"`
	Hours         services.StoreOpenStatus `json:"hours"`
}

// staffLocation adds the switches staff manage.
type staffLocation struct {
	dto.Location
	Status services.StoreOpenStatus `json:"status"`
}

// ListPublic returns the active locations with their open status.
//
// GET /api/v1/locations
func (h *LocationHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	locations, err := h.locations.List(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	now := time.Now()
	out := make([]publicLocation, 0, len(locations))
	for _, l := range locations {
		if !l.IsActive {
			continue
		}
		out = append(out, publicLocation{
			ID: l.ID, Slug: l.Slug, Name: l.Name, Address: l.Address,
			MapsURL: l.MapsURL, MapsEmbedURL: l.MapsEmbedURL, WhatsApp: l.WhatsApp, Phone: l.Phone,
			AcceptsOnline: l.OnlineOrdersEnabled && l.OrdersEnabled,
			Hours:         h.locations.OpenStatus(l, now),
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// ListManaged returns the locations a staff member works with: all of them
// (including inactive ones) for admins, their own for everyone else.
//
// GET /api/v1/locations/manage
func (h *LocationHandler) ListManaged(w http.ResponseWriter, r *http.Request) {
	locations, err := h.locations.List(r.Context())
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	isAdmin := auth.GetRole(r.Context()) == auth.RoleAdmin
	own, _ := scope.FromContext(r.Context())
	now := time.Now()
	out := make([]staffLocation, 0, len(locations))
	for _, l := range locations {
		if !isAdmin {
			if id, ok := own.ID(); !ok || id != l.ID {
				continue
			}
		}
		out = append(out, staffLocation{Location: l, Status: h.locations.OpenStatus(l, now)})
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// Create adds a location (admin).
//
// POST /api/v1/locations
func (h *LocationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body services.LocationPatch
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}
	created, err := h.locations.Create(r.Context(), body)
	if err != nil {
		writeLocationError(w, err)
		return
	}
	actorID := getAdminID(r)
	_ = h.auditSvc.Log(r.Context(), audit.ActionSettingsUpdated, actorID, "location:"+created.ID.String(),
		map[string]any{"action_detail": "created", "name": created.Name}, auth.ClientIP(r))
	response.JSON(w, http.StatusCreated, created)
}

// Update changes a location. Admins may change everything; a manager only
// the opening hours and the orders switch of their own location.
//
// PATCH /api/v1/locations/{id}
func (h *LocationHandler) Update(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Sucursal inválida")
		return
	}
	if err := loc.Check("location", id); err != nil {
		response.Error(w, http.StatusNotFound, "Sucursal no encontrada")
		return
	}
	var body services.LocationPatch
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}
	switch auth.GetRole(r.Context()) {
	case auth.RoleAdmin:
	case auth.RoleManager:
		if !body.ManagerFields() {
			response.Error(w, http.StatusForbidden, "Solo un administrador puede cambiar los datos de la sucursal")
			return
		}
	default:
		response.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	updated, err := h.locations.Update(r.Context(), id, body)
	if err != nil {
		writeLocationError(w, err)
		return
	}
	_ = h.auditSvc.Log(r.Context(), audit.ActionSettingsUpdated, getAdminID(r), "location:"+id.String(),
		map[string]any{"action_detail": "updated"}, auth.ClientIP(r))
	response.JSON(w, http.StatusOK, staffLocation{Location: updated, Status: h.locations.OpenStatus(updated, time.Now())})
}

// GetAvailability lists the products and flavours sold out at a location.
// Public: the store uses it to grey out items.
//
// GET /api/v1/locations/{id}/availability
func (h *LocationHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Sucursal inválida")
		return
	}
	if _, err := h.locations.Get(r.Context(), id); err != nil {
		response.Error(w, http.StatusNotFound, "Sucursal no encontrada")
		return
	}
	items, err := h.locations.SoldOut(r.Context(), id)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

// SetAvailability marks a product or flavour sold out (or available again)
// at a location. Any staff member of that location may do it.
//
// PUT /api/v1/locations/{id}/availability
// Body: {"item_type": "product"|"flavor", "item_id": "…", "sold_out": true}
func (h *LocationHandler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	loc, ok := requestScope(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Sucursal inválida")
		return
	}
	if err := loc.Check("location", id); err != nil {
		response.Error(w, http.StatusNotFound, "Sucursal no encontrada")
		return
	}
	var body struct {
		ItemType string    `json:"item_type"`
		ItemID   uuid.UUID `json:"item_id"`
		SoldOut  bool      `json:"sold_out"`
	}
	if err := decodeJSON(r, &body); err != nil {
		response.Error(w, http.StatusBadRequest, "Cuerpo de solicitud inválido")
		return
	}
	actorID := getAdminID(r)
	if err := h.locations.SetSoldOut(r.Context(), id, body.ItemType, body.ItemID, body.SoldOut, actorID); err != nil {
		writeLocationError(w, err)
		return
	}
	_ = h.auditSvc.Log(r.Context(), audit.ActionProductAvailabilityToggled, actorID, body.ItemType+":"+body.ItemID.String(),
		map[string]any{"location_id": id.String(), "sold_out": body.SoldOut}, auth.ClientIP(r))

	items, err := h.locations.SoldOut(r.Context(), id)
	if err != nil {
		response.WriteOrError(w, nil, err)
		return
	}
	response.JSON(w, http.StatusOK, items)
}

func writeLocationError(w http.ResponseWriter, err error) {
	if fields := validationFields(err); fields != nil {
		response.JSON(w, http.StatusBadRequest, map[string]any{"error": "Revisa los datos de la sucursal", "fields": fields})
		return
	}
	response.WriteOrError(w, nil, err)
}
