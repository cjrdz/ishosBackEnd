// location_service.go — Locations (sucursales): hours, order switches, contact details and Agotados.
package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	cachepkg "github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

// LocationRepository is the data access the location service needs.
type LocationRepository interface {
	List(ctx context.Context) ([]dto.Location, error)
	FindByID(ctx context.Context, id uuid.UUID) (dto.Location, error)
	Create(ctx context.Context, input repository.LocationInput) (dto.Location, error)
	Update(ctx context.Context, id uuid.UUID, input repository.LocationInput) (dto.Location, error)
	SoldOut(ctx context.Context, locationID uuid.UUID) (dto.SoldOutItems, error)
	// itemType is dto.AvailabilityProduct or dto.AvailabilityFlavor.
	SetSoldOut(ctx context.Context, locationID uuid.UUID, itemType string, itemID uuid.UUID, soldOut bool, by string) error
}

const (
	locationsCacheKey = "locations:all"
	locationsCacheTTL = 10 * time.Second
)

// LocationService manages locations. Reads are cached briefly because every
// staff request resolves its location and every public order checks hours.
type LocationService struct {
	repo  LocationRepository
	tz    *time.Location
	cache *cachepkg.TTLCache
}

// NewLocationService creates a location service; tz is the store time zone.
func NewLocationService(repo LocationRepository, tz *time.Location) *LocationService {
	if tz == nil {
		tz = time.UTC
	}
	return &LocationService{repo: repo, tz: tz, cache: cachepkg.New(locationsCacheTTL)}
}

// List returns every location (active and inactive) in display order.
func (s *LocationService) List(ctx context.Context) ([]dto.Location, error) {
	if v, ok := s.cache.Get(locationsCacheKey); ok {
		return v.([]dto.Location), nil
	}
	locations, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	s.cache.Set(locationsCacheKey, locations)
	return locations, nil
}

// ActiveIDs returns the IDs of active locations.
func (s *LocationService) ActiveIDs(ctx context.Context) ([]uuid.UUID, error) {
	locations, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(locations))
	for _, l := range locations {
		if l.IsActive {
			ids = append(ids, l.ID)
		}
	}
	return ids, nil
}

// Get returns one location.
func (s *LocationService) Get(ctx context.Context, id uuid.UUID) (dto.Location, error) {
	locations, err := s.List(ctx)
	if err != nil {
		return dto.Location{}, err
	}
	for _, l := range locations {
		if l.ID == id {
			return l, nil
		}
	}
	return dto.Location{}, &apperror.NotFoundError{Resource: "location"}
}

// OpenStatus is the schedule-derived status of a location at now.
func (s *LocationService) OpenStatus(l dto.Location, now time.Time) StoreOpenStatus {
	return ComputeOpenStatus(weeklyFromDTO(l.Hours), now, s.tz)
}

// ClosedMessage explains in Spanish when a location opens next.
func (s *LocationService) ClosedMessage(status StoreOpenStatus, now time.Time) string {
	return ClosedMessage(status, now, s.tz)
}

// CheckCanOrder returns a 503 error when the location can't take an order
// now. Online orders also need the location's online switch and its opening
// hours; counter staff can always order while the location is active and its
// orders switch is on.
func (s *LocationService) CheckCanOrder(ctx context.Context, id uuid.UUID, online bool, now time.Time) error {
	l, err := s.Get(ctx, id)
	if err != nil {
		return &apperror.ValidationError{Fields: map[string]string{"location_id": "Sucursal no encontrada"}, Cause: err}
	}
	if !l.IsActive {
		return &apperror.ServiceUnavailableError{Message: l.Name + " no está disponible"}
	}
	if !l.OrdersEnabled {
		return &apperror.ServiceUnavailableError{Message: "Los pedidos están temporalmente desactivados en " + l.Name}
	}
	if !online {
		return nil
	}
	if !l.OnlineOrdersEnabled {
		return &apperror.ServiceUnavailableError{Message: "Por ahora " + l.Name + " no recibe pedidos en línea. ¡Visítanos!"}
	}
	if status := s.OpenStatus(l, now); !status.IsOpen {
		return &apperror.ServiceUnavailableError{Message: s.ClosedMessage(status, now)}
	}
	return nil
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// LocationPatch is an update request. Nil fields are left unchanged.
type LocationPatch struct {
	Slug                *string      `json:"slug"`
	Name                *string      `json:"name"`
	Address             *string      `json:"address"`
	MapsURL             *string      `json:"maps_url"`
	MapsEmbedURL        *string      `json:"maps_embed_url"`
	WhatsApp            *string      `json:"whatsapp"`
	Phone               *string      `json:"phone"`
	Hours               *WeeklyHours `json:"hours"`
	OrdersEnabled       *bool        `json:"orders_enabled"`
	OnlineOrdersEnabled *bool        `json:"online_orders_enabled"`
	IsActive            *bool        `json:"is_active"`
	SortOrder           *int         `json:"sort_order"`

	hoursDTO map[string][]dto.HoursWindow // set by normalize
}

// ManagerFields reports whether the patch only touches what a location
// manager may change: opening hours and the orders switch.
func (p LocationPatch) ManagerFields() bool {
	return p.Slug == nil && p.Name == nil && p.Address == nil && p.MapsURL == nil &&
		p.MapsEmbedURL == nil && p.WhatsApp == nil && p.Phone == nil &&
		p.OnlineOrdersEnabled == nil && p.IsActive == nil && p.SortOrder == nil
}

func (p *LocationPatch) normalize() error {
	fields := map[string]string{}
	trim := func(v *string) {
		if v != nil {
			*v = strings.TrimSpace(*v)
		}
	}
	for _, v := range []*string{p.Slug, p.Name, p.Address, p.MapsURL, p.MapsEmbedURL, p.WhatsApp, p.Phone} {
		trim(v)
	}
	if p.Name != nil && (*p.Name == "" || len(*p.Name) > 120) {
		fields["name"] = "Nombre requerido (máximo 120 caracteres)"
	}
	if p.Slug != nil && !slugPattern.MatchString(*p.Slug) {
		fields["slug"] = "Usa minúsculas, números y guiones (ej. sucursal-3)"
	}
	if p.Address != nil && len(*p.Address) > 255 {
		fields["address"] = "Máximo 255 caracteres"
	}
	for name, v := range map[string]*string{"maps_url": p.MapsURL, "maps_embed_url": p.MapsEmbedURL} {
		if v != nil && *v != "" && !strings.HasPrefix(*v, "https://") {
			fields[name] = "Debe empezar con https://"
		}
	}
	if p.MapsEmbedURL != nil && *p.MapsEmbedURL != "" && !strings.HasPrefix(*p.MapsEmbedURL, "https://www.google.com/maps/embed") {
		fields["maps_embed_url"] = "Usa el enlace de \"Insertar un mapa\" de Google Maps"
	}
	if p.WhatsApp != nil {
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, *p.WhatsApp)
		if len(digits) == 8 {
			digits = "503" + digits // El Salvador numbers without country code
		}
		if *p.WhatsApp != "" && (len(digits) < 10 || len(digits) > 15) {
			fields["whatsapp"] = "Número de WhatsApp inválido"
		}
		*p.WhatsApp = digits
	}
	if p.Phone != nil && len(*p.Phone) > 30 {
		fields["phone"] = "Máximo 30 caracteres"
	}
	if p.Hours != nil {
		if err := ValidateWeeklyHours(*p.Hours); err != nil {
			return err
		}
		normalized := NormalizeWeeklyHours(*p.Hours)
		p.hoursDTO = make(map[string][]dto.HoursWindow, len(normalized))
		for day, ranges := range normalized {
			windows := make([]dto.HoursWindow, 0, len(ranges))
			for _, r := range ranges {
				windows = append(windows, dto.HoursWindow{Open: r.Open, Close: r.Close})
			}
			p.hoursDTO[day] = windows
		}
	}
	if len(fields) > 0 {
		return &apperror.ValidationError{Fields: fields, Cause: fmt.Errorf("invalid location")}
	}
	return nil
}

func (p LocationPatch) toInput() repository.LocationInput {
	return repository.LocationInput{
		Slug: p.Slug, Name: p.Name, Address: p.Address, MapsURL: p.MapsURL, MapsEmbedURL: p.MapsEmbedURL,
		WhatsApp: p.WhatsApp, Phone: p.Phone, Hours: p.hoursDTO, SetHours: p.Hours != nil,
		OrdersEnabled: p.OrdersEnabled, OnlineOrdersEnabled: p.OnlineOrdersEnabled,
		IsActive: p.IsActive, SortOrder: p.SortOrder,
	}
}

// Update validates and applies a patch.
func (s *LocationService) Update(ctx context.Context, id uuid.UUID, patch LocationPatch) (dto.Location, error) {
	if err := patch.normalize(); err != nil {
		return dto.Location{}, err
	}
	updated, err := s.repo.Update(ctx, id, patch.toInput())
	if err != nil {
		return dto.Location{}, err
	}
	s.cache.Delete(locationsCacheKey)
	return updated, nil
}

// Create adds a location. Name and slug are required.
func (s *LocationService) Create(ctx context.Context, patch LocationPatch) (dto.Location, error) {
	if patch.Name == nil || patch.Slug == nil {
		return dto.Location{}, &apperror.ValidationError{Fields: map[string]string{"name": "Nombre y slug son requeridos"}}
	}
	if err := patch.normalize(); err != nil {
		return dto.Location{}, err
	}
	created, err := s.repo.Create(ctx, patch.toInput())
	if err != nil {
		return dto.Location{}, err
	}
	s.cache.Delete(locationsCacheKey)
	return created, nil
}

// SoldOut lists what is "Agotado" at a location.
func (s *LocationService) SoldOut(ctx context.Context, locationID uuid.UUID) (dto.SoldOutItems, error) {
	return s.repo.SoldOut(ctx, locationID)
}

// SetSoldOut marks a product or flavour as sold out (or back in stock) at a location.
func (s *LocationService) SetSoldOut(ctx context.Context, locationID uuid.UUID, itemType string, itemID uuid.UUID, soldOut bool, by string) error {
	if itemType != dto.AvailabilityProduct && itemType != dto.AvailabilityFlavor {
		return &apperror.ValidationError{Fields: map[string]string{"item_type": "Indica si es un producto o un sabor"}}
	}
	if itemID == uuid.Nil {
		return &apperror.ValidationError{Fields: map[string]string{"item_id": "Indica el producto o sabor"}}
	}
	if _, err := s.Get(ctx, locationID); err != nil {
		return err
	}
	return s.repo.SetSoldOut(ctx, locationID, itemType, itemID, soldOut, by)
}

func weeklyFromDTO(hours map[string][]dto.HoursWindow) WeeklyHours {
	out := make(WeeklyHours, len(hours))
	for day, windows := range hours {
		for _, w := range windows {
			out[day] = append(out[day], TimeRange{Open: w.Open, Close: w.Close})
		}
	}
	return out
}
