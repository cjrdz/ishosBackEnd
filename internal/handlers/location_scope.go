// location_scope.go — Reading the request's location scope in handlers.
package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/scope"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// requestScope returns the location scope set by middleware.ResolveLocation.
// It fails closed: a route wired without that middleware answers 403 instead
// of silently showing every location.
func requestScope(w http.ResponseWriter, r *http.Request) (scope.Location, bool) {
	loc, ok := scope.FromContext(r.Context())
	if !ok {
		log.Error().Str("path", r.URL.Path).Msg("location scope missing; route is not behind ResolveLocation")
		response.Error(w, http.StatusForbidden, "Sucursal no definida")
		return scope.Location{}, false
	}
	return loc, true
}
