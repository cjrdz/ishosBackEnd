// server_test.go — Unit tests for server-level middleware and routing helpers.
package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "empty falls back to localhost",
			raw:  "",
			want: []string{"http://localhost:4321"},
		},
		{
			name: "single https origin",
			raw:  "https://app.example.com",
			want: []string{"https://app.example.com"},
		},
		{
			name: "bare hostname expands to http and https",
			raw:  "app.example.com",
			want: []string{"https://app.example.com", "http://app.example.com", "app.example.com"},
		},
		{
			name: "multiple origins deduplicated",
			raw:  "https://a.com, http://a.com, https://a.com",
			want: []string{"https://a.com", "http://a.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAllowedOrigins(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("expected %v, got %v", tt.want, got)
				}
			}
		})
	}
}

func TestIsOriginAllowed(t *testing.T) {
	origins := []string{"https://app.example.com", "http://localhost:4321", "*.staging.example.com"}

	tests := []struct {
		origin string
		want   bool
	}{
		{"https://app.example.com", true},
		{"https://app.example.com/", true},
		{"http://localhost:4321", true},
		{"https://foo.staging.example.com", true},
		{"https://other.com", false},
		{"", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			if got := isOriginAllowed(tt.origin, origins); got != tt.want {
				t.Fatalf("isOriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestCORSWithCredentials(t *testing.T) {
	allowedOrigins := parseAllowedOrigins("https://app.example.com")

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return cors.Handler(cors.Options{
			AllowOriginFunc: func(req *http.Request, origin string) bool {
				return isOriginAllowed(origin, allowedOrigins)
			},
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
			AllowCredentials: true,
			MaxAge:           300,
		})(next)
	})
	r.Get("/api/v1/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("rejects wildcard origin when credentials enabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		req.Header.Set("Origin", "*")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("expected no Access-Control-Allow-Origin header for wildcard origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("rejects origin not in configured list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		req.Header.Set("Origin", "https://evil.com")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("expected no Access-Control-Allow-Origin header for disallowed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("allows configured origin and sets allow-origin header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		got := rec.Header().Get("Access-Control-Allow-Origin")
		if got != "https://app.example.com" {
			t.Fatalf("expected Access-Control-Allow-Origin %q, got %q", "https://app.example.com", got)
		}
	})
}

func TestJSONBodySizeLimit(t *testing.T) {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(jsonBodySizeLimit)
	r.Post("/api/v1/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Upload routes must bypass the JSON limit.
	r.Post("/api/v1/upload/image", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("rejects oversized json bodies", func(t *testing.T) {
		body := bytes.Repeat([]byte("a"), maxJSONBodyBytes+1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/test", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
		}
	})

	t.Run("allows small json bodies", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/test", strings.NewReader(`{"ok":true}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("skips upload routes", func(t *testing.T) {
		body := bytes.Repeat([]byte("a"), maxJSONBodyBytes+1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/image", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})
}
