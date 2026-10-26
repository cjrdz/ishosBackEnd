// client_ip.go — Resolves the real client IP for rate limiting and audit logs.
package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

const (
	// BFFSecretHeader carries the shared secret proving a request came from the
	// frontend BFF (Astro on Cloudflare Workers).
	// #nosec G101 -- this is a header name, not a credential.
	BFFSecretHeader = "X-BFF-Secret"
	// BFFClientIPHeader carries the end user's IP as seen by the BFF
	// (Cloudflare's CF-Connecting-IP).
	BFFClientIPHeader = "X-Client-IP"
)

// ClientIP stores the client IP in the request context (read it with
// auth.ClientIP). Browser traffic reaches this API through the BFF, so the TCP
// peer and the right-most X-Forwarded-For entry are Cloudflare egress IPs shared
// by many customers. When bffSecret is configured and the request carries a
// matching X-BFF-Secret header, X-Client-IP is trusted instead. Otherwise the IP
// falls back to the right-most X-Forwarded-For entry (Railway's edge proxy).
//
// X-Client-IP is ignored without a valid secret, so clients cannot spoof it.
func ClientIP(bffSecret string) func(http.Handler) http.Handler {
	fromBFF := middleware.ClientIPFromHeader(BFFClientIPHeader)
	fromXFF := middleware.ClientIPFromXFFTrustedProxies(1)
	secret := []byte(bffSecret)

	return func(next http.Handler) http.Handler {
		bffNext := fromBFF(next)
		xffNext := fromXFF(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get(BFFSecretHeader)
			if len(secret) > 0 && provided != "" && subtle.ConstantTimeCompare([]byte(provided), secret) == 1 {
				if r.Header.Get(BFFClientIPHeader) == "" {
					// The BFF should always forward the shopper's IP; without it
					// per-IP limits are skipped, so surface the misconfiguration.
					log.Warn().Str("path", r.URL.Path).Msg("BFF request without X-Client-IP; per-IP rate limits skipped")
				}
				bffNext.ServeHTTP(w, r)
				return
			}
			xffNext.ServeHTTP(w, r)
		})
	}
}
