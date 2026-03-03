package integration

import (
	"os"
	"testing"
)

type IntegrationConfig struct {
	BaseURL       string
	AdminEmail    string
	AdminPassword string
}

func loadIntegrationConfig(t *testing.T) IntegrationConfig {
	t.Helper()

	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("integration tests are disabled; set RUN_INTEGRATION=1 to enable")
	}

	baseURL := os.Getenv("INTEGRATION_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return IntegrationConfig{
		BaseURL:       baseURL,
		AdminEmail:    os.Getenv("INTEGRATION_ADMIN_EMAIL"),
		AdminPassword: os.Getenv("INTEGRATION_ADMIN_PASSWORD"),
	}
}
