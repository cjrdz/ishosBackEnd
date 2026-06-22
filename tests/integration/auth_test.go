package integration

import (
	"net/http"
	"testing"
)

func TestAuthLoginAndSession(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		t.Skip("set INTEGRATION_ADMIN_EMAIL and INTEGRATION_ADMIN_PASSWORD to run auth integration tests")
	}

	client := newHTTPClient()

	loginResp, loginBody := doJSONRequest(t, client, http.MethodPost, cfg.BaseURL+"/api/v1/auth/login", map[string]string{
		"email":    cfg.AdminEmail,
		"password": cfg.AdminPassword,
	}, "")
	requireStatus(t, loginResp, http.StatusOK, loginBody)

	// Extract token from HttpOnly cookie set by backend
	var token string
	for _, c := range loginResp.Cookies() {
		if c.Name == "auth_token" {
			token = c.Value
			break
		}
	}
	if token == "" {
		t.Fatalf("expected auth_token cookie in login response")
	}

	sessionResp, sessionBody := doJSONRequest(t, client, http.MethodGet, cfg.BaseURL+"/api/v1/auth/session", nil, token)
	requireStatus(t, sessionResp, http.StatusOK, sessionBody)
}
