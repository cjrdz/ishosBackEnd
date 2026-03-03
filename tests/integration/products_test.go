package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestProductsPublicList(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	client := newHTTPClient()

	resp, body := doJSONRequest(t, client, http.MethodGet, cfg.BaseURL+"/api/v1/products", nil, "")
	requireStatus(t, resp, http.StatusOK, body)

	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("failed to decode products list response: %v", err)
	}
}
