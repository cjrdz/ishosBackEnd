package integration

import (
	"net/http"
	"os"
	"testing"
)

// TestCatalogWritesRequireAdmin proves, through the real router, that a staff
// (employee) session cannot change the catalog or upload images, while it can
// still reach the availability toggle used during service. Random IDs are used,
// so a passing authorization check yields 404 and nothing is modified.
//
// Requires RUN_INTEGRATION=1 and INTEGRATION_STAFF_EMAIL / INTEGRATION_STAFF_PASSWORD
// for an account with role "employee".
func TestCatalogWritesRequireAdmin(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	email, password := os.Getenv("INTEGRATION_STAFF_EMAIL"), os.Getenv("INTEGRATION_STAFF_PASSWORD")
	if email == "" || password == "" {
		t.Skip("set INTEGRATION_STAFF_EMAIL and INTEGRATION_STAFF_PASSWORD (role employee) to run")
	}
	client := newHTTPClient()

	loginResp, loginBody := doJSONRequest(t, client, http.MethodPost, cfg.BaseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": password}, "")
	requireStatus(t, loginResp, http.StatusOK, loginBody)
	var token string
	for _, c := range loginResp.Cookies() {
		if c.Name == "auth_token" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("login did not set auth_token")
	}

	const randomID = "00000000-0000-4000-8000-000000000001"
	forbidden := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/products"},
		{http.MethodPatch, "/api/v1/products/" + randomID},
		{http.MethodDelete, "/api/v1/products/" + randomID},
		{http.MethodPost, "/api/v1/products/" + randomID + "/flavors/" + randomID},
		{http.MethodPost, "/api/v1/flavors"},
		{http.MethodPatch, "/api/v1/flavors/" + randomID},
		{http.MethodPost, "/api/v1/addons"},
		{http.MethodDelete, "/api/v1/addons/" + randomID},
		{http.MethodPost, "/api/v1/categories"},
		{http.MethodDelete, "/api/v1/categories/" + randomID},
		{http.MethodPost, "/api/v1/upload/image"},
	}
	for _, tc := range forbidden {
		resp, body := doJSONRequest(t, client, tc.method, cfg.BaseURL+tc.path, map[string]any{}, token)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s: staff got %d, want 403 (body=%s)", tc.method, tc.path, resp.StatusCode, body)
		}
	}

	// Staff keep the availability toggle: authorization passes, the random ID 404s.
	resp, body := doJSONRequest(t, client, http.MethodPatch, cfg.BaseURL+"/api/v1/products/"+randomID+"/availability", map[string]any{}, token)
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		t.Errorf("availability toggle: staff got %d, want access (body=%s)", resp.StatusCode, body)
	}
}
