package integration

import (
	"net/http"
	"testing"
)

func TestOrdersCreateValidation(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	client := newHTTPClient()

	resp, body := doJSONRequest(t, client, http.MethodPost, cfg.BaseURL+"/api/v1/orders", map[string]any{}, "")
	requireStatus(t, resp, http.StatusBadRequest, body)
}
