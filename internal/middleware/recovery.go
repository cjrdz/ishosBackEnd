package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/rs/zerolog/log"
)

// Recovery handles panics, logs stack traces, and returns a JSON error response.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Str("panic", fmt.Sprint(rec)).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered")
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
