package integration

import (
	"encoding/json"
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

	var loginResult struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginBody, &loginResult); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResult.Token == "" {
		t.Fatalf("expected non-empty token in login response")
	}

	sessionResp, sessionBody := doJSONRequest(t, client, http.MethodGet, cfg.BaseURL+"/api/v1/auth/session", nil, loginResult.Token)
	requireStatus(t, sessionResp, http.StatusOK, sessionBody)
}
