// Package scope carries the location (sucursal) a request is allowed to see.
//
// Every repository method that reads or writes location-bound data (orders,
// stock, Agotados, per-location settings) takes a Location argument with no
// default, so a forgotten filter is a compile error instead of a data leak.
package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
)

// Location is either one location or all of them. The zero value is "all":
// only admins can get it (see middleware.ResolveLocation).
type Location struct {
	id  uuid.UUID
	one bool
}

// All returns the scope covering every location (admin reads).
func All() Location { return Location{} }

// Only returns the scope of a single location.
func Only(id uuid.UUID) Location { return Location{id: id, one: true} }

// IsAll reports whether the scope covers every location.
func (l Location) IsAll() bool { return !l.one }

// ID returns the location and true when the scope is a single location.
func (l Location) ID() (uuid.UUID, bool) { return l.id, l.one }

// Allows reports whether data that belongs to locationID is visible in scope.
func (l Location) Allows(locationID uuid.UUID) bool {
	return !l.one || l.id == locationID
}

// ErrLocationRequired is returned when an action creates location-bound data
// (an order, a stock entry, hours) while the admin is looking at "Todas".
var ErrLocationRequired = &apperror.ValidationError{
	Fields: map[string]string{"location_id": "Selecciona una sucursal"},
	Cause:  errors.New("location required"),
}

// Require returns the single location of the scope, or ErrLocationRequired.
func (l Location) Require() (uuid.UUID, error) {
	if !l.one {
		return uuid.Nil, ErrLocationRequired
	}
	return l.id, nil
}

// String is used in cache keys and logs.
func (l Location) String() string {
	if !l.one {
		return "all"
	}
	return l.id.String()
}

// Check returns a not-found error when locationID is outside the scope, so a
// staff member cannot even learn that another location's record exists.
func (l Location) Check(resource string, locationID uuid.UUID) error {
	if l.Allows(locationID) {
		return nil
	}
	return &apperror.NotFoundError{Resource: resource, Cause: fmt.Errorf("outside location scope")}
}

type contextKey struct{}

// WithLocation stores the request's location scope in ctx.
func WithLocation(ctx context.Context, l Location) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext returns the scope stored by the location middleware.
// ok is false when no middleware ran (public routes): callers must then not
// treat the zero value as "all".
func FromContext(ctx context.Context) (Location, bool) {
	l, ok := ctx.Value(contextKey{}).(Location)
	return l, ok
}
