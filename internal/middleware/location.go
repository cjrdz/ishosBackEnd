// location.go — Resolves which location (sucursal) a staff request may see.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/scope"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

// LocationHeader is how an admin picks the location to work on ("all" or
// missing means every location). Staff never choose: theirs comes from their
// account and this header is ignored.
const LocationHeader = "X-Location-Id"

type locationLookup interface {
	Get(ctx context.Context, id uuid.UUID) (dto.Location, error)
}

// ResolveLocation stores the request's scope.Location in the context. It must
// run after RequireSession.
//
//   - admin: X-Location-Id (or ?location_id=) when set, otherwise all locations;
//   - manager / employee: the location on their account, always. An account
//     without a valid location is refused (403) rather than shown everything.
func ResolveLocation(locations locationLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record, ok := auth.GetRecord(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			var loc scope.Location
			if auth.GetRole(r.Context()) == auth.RoleAdmin {
				raw := strings.TrimSpace(r.Header.Get(LocationHeader))
				if raw == "" {
					raw = strings.TrimSpace(r.URL.Query().Get("location_id"))
				}
				if raw == "" || raw == "all" {
					loc = scope.All()
				} else {
					id, err := uuid.Parse(raw)
					if err != nil {
						response.Error(w, http.StatusBadRequest, "Sucursal inválida")
						return
					}
					if _, err := locations.Get(r.Context(), id); err != nil {
						response.Error(w, http.StatusBadRequest, "Sucursal inválida")
						return
					}
					loc = scope.Only(id)
				}
			} else {
				id, err := uuid.Parse(auth.LocationIDFromRecord(record))
				if err != nil {
					log.Warn().Str("employee", auth.RecordID(record)).Msg("staff account has no location assigned")
					response.Error(w, http.StatusForbidden, "Tu cuenta no tiene una sucursal asignada. Pide a un administrador que la asigne.")
					return
				}
				loc = scope.Only(id)
			}

			next.ServeHTTP(w, r.WithContext(scope.WithLocation(r.Context(), loc)))
		})
	}
}
