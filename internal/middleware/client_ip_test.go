package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
)

func TestClientIP(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"

	tests := []struct {
		name    string
		secret  string
		headers map[string]string
		want    string
	}{
		{
			name:    "trusted BFF header with valid secret",
			secret:  secret,
			headers: map[string]string{BFFSecretHeader: secret, BFFClientIPHeader: "203.0.113.7", "X-Forwarded-For": "104.16.0.1"},
			want:    "203.0.113.7",
		},
		{
			name:    "wrong secret falls back to X-Forwarded-For",
			secret:  secret,
			headers: map[string]string{BFFSecretHeader: "nope", BFFClientIPHeader: "203.0.113.7", "X-Forwarded-For": "104.16.0.1"},
			want:    "104.16.0.1",
		},
		{
			name:    "no secret configured ignores spoofed client IP",
			secret:  "",
			headers: map[string]string{BFFSecretHeader: "", BFFClientIPHeader: "203.0.113.7", "X-Forwarded-For": "198.51.100.2, 104.16.0.1"},
			want:    "104.16.0.1",
		},
		{
			name:    "valid secret without client IP leaves IP unknown",
			secret:  secret,
			headers: map[string]string{BFFSecretHeader: secret, "X-Forwarded-For": "104.16.0.1"},
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := ClientIP(tc.secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = auth.ClientIP(r)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tc.headers {
				if v != "" {
					req.Header.Set(k, v)
				}
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Fatalf("client IP = %q, want %q", got, tc.want)
			}
		})
	}
}
