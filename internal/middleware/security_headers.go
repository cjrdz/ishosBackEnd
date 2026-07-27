// security_headers.go — Sets defensive HTTP response headers on every response.
package middleware

import "net/http"

// SecurityHeaders sets baseline hardening headers on every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; img-src 'self' https://pub-*.r2.dev data:; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'",
		)

		next.ServeHTTP(w, r)
	})
}
