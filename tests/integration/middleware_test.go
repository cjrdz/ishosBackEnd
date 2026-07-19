// middleware_test.go — Chi-level integration tests for middleware behavior.
//
// These tests spin up a minimal router with the real middleware under test and
// exercise rate limiting and role enforcement without requiring a running server.
package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	appmiddleware "github.com/cjrdz/ishosBackEnd/internal/middleware"
	"github.com/cjrdz/ishosBackEnd/internal/services"
)

func TestOrderRateLimiterMiddleware(t *testing.T) {
	// Use tight limits so we can trigger blocking quickly.
	defaults := services.PublicOrderRateLimitConfig{
		PhoneBurst:         3,
		PhoneWindowMinutes: 60,
		PhoneBlockMinutes:  120,
		IPBurst:            2,
		IPWindowMinutes:    60,
		IPBlockMinutes:     120,
		StaffBurst:         5,
		StaffWindowMinutes: 60,
	}

	rl := appmiddleware.NewOrderRateLimiter(nil, defaults, 240, time.Hour, "", "")

	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.With(rl.Middleware).Post("/orders", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	t.Run("allows requests under the ip burst", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orders", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
		}
	})

	t.Run("returns 429 with rate-limit headers after burst", func(t *testing.T) {
		for i := 0; i < defaults.IPBurst+1; i++ {
			req := httptest.NewRequest(http.MethodPost, "/orders", nil)
			req.RemoteAddr = "5.6.7.8:1234"
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if i < defaults.IPBurst {
				if rec.Code != http.StatusCreated {
					t.Fatalf("request %d: expected %d, got %d", i, http.StatusCreated, rec.Code)
				}
				continue
			}
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d: expected %d, got %d", i, http.StatusTooManyRequests, rec.Code)
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Fatalf("expected Retry-After header")
			}
			if rec.Header().Get("X-RateLimit-Limit") == "" {
				t.Fatalf("expected X-RateLimit-Limit header")
			}
		}
	})
}

func TestStaffThrottle(t *testing.T) {
	defaults := services.PublicOrderRateLimitConfig{
		StaffBurst:         2,
		StaffWindowMinutes: 60,
	}
	rl := appmiddleware.NewOrderRateLimiter(nil, defaults, 240, time.Hour, "", "")

	for i := 0; i < defaults.StaffBurst+1; i++ {
		res := rl.CheckStaff(context.Background(), "staff-1")
		if i < defaults.StaffBurst {
			if !res.Allowed {
				t.Fatalf("request %d: expected allowed", i)
			}
			continue
		}
		if res.Allowed {
			t.Fatalf("request %d: expected blocked", i)
		}
		if res.RetryAfter == 0 {
			t.Fatalf("expected positive retry-after")
		}
	}
}

func TestTrackingRateLimiter(t *testing.T) {
	rl := appmiddleware.NewOrderRateLimiter(nil, services.PublicOrderRateLimitConfig{}, 2, time.Hour, "", "")

	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Get("/track", func(w http.ResponseWriter, r *http.Request) {
		res := rl.CheckTrackingIP(r.Context(), auth.ClientIP(r))
		res.WriteHeaders(w)
		if !res.Allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/track", nil)
		req.RemoteAddr = "9.10.11.12:1234"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if i < 2 {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected %d, got %d", i, http.StatusOK, rec.Code)
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: expected %d, got %d", i, http.StatusTooManyRequests, rec.Code)
		}
		if rec.Header().Get("X-RateLimit-Remaining") != "0" {
			t.Fatalf("expected remaining 0, got %q", rec.Header().Get("X-RateLimit-Remaining"))
		}
	}
}

func TestRequireAdminRejectsNonAdmin(t *testing.T) {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithRole(r.Context(), "employee")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.With(appmiddleware.RequireAdmin).Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "admin only") {
		t.Fatalf("expected admin-only message, got %q", rec.Body.String())
	}
}

type stubLoginAuthService struct {
	auth.Service
	attempts map[string]int
}

func (s *stubLoginAuthService) CheckLoginAttempt(ctx context.Context, ip string) error {
	s.attempts[ip]++
	if s.attempts[ip] > 2 {
		return &apperror.RateLimitError{RetryAfter: 60}
	}
	return nil
}

func TestLoginRateLimitMiddleware(t *testing.T) {
	svc := &stubLoginAuthService{attempts: make(map[string]int)}
	lrl := appmiddleware.NewLoginRateLimit(svc)

	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.With(lrl.Middleware).Post("/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ip := "1.2.3.4"
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if i < 2 {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected %d, got %d", i, http.StatusOK, rec.Code)
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: expected %d, got %d", i, http.StatusTooManyRequests, rec.Code)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Fatalf("expected Retry-After header")
		}
	}
}

func TestInactivityMiddlewareExpiresIdleSession(t *testing.T) {
	tracker := appmiddleware.NewInactivityTracker()
	mw := appmiddleware.NewInactivityMiddleware(tracker, nil, services.TokenTTLConfig{
		InactivityLogoutSeconds: 1,
	})

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "auth_token", Value: "token-xyz"})
			next.ServeHTTP(w, r)
		})
	})
	r.With(mw.Middleware).Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allows first request and records activity", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("rejects request after inactivity timeout", func(t *testing.T) {
		time.Sleep(1100 * time.Millisecond)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})
}
