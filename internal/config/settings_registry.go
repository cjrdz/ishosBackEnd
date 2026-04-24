package config

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Value schemas
type TabsSettings struct {
	TabOrder []string `json:"tab_order"`
}
type PanelConfig struct {
	AuthCookieTTLHours      int `json:"auth_cookie_ttl_hours"`
	AuthTokenTTLHours       int `json:"auth_token_ttl_hours"`
	TrackingTokenTTLHours   int `json:"tracking_token_ttl_hours"`
	InactivityLogoutSeconds int `json:"inactivity_logout_seconds"`
}

// Registry of allowed keys and their value validators.
// Keys must match the constants defined in services/admin_setting_service.go.
var SettingsRegistry = map[string]struct {
	Validate func(json.RawMessage) error
}{
	// AdminTabOrderSettingKey = "admin.tabs.order"
	"admin.tabs.order": {
		Validate: func(raw json.RawMessage) error {
			var v TabsSettings
			if err := json.Unmarshal(raw, &v); err != nil {
				return err
			}
			if len(v.TabOrder) == 0 {
				return errors.New("tab_order must not be empty")
			}
			return nil
		},
	},
	// AdminTokenTTLSettingKey = "admin.tokens.ttl"
	"admin.tokens.ttl": {
		Validate: func(raw json.RawMessage) error {
			var v PanelConfig
			if err := json.Unmarshal(raw, &v); err != nil {
				return err
			}
			if v.AuthCookieTTLHours <= 0 {
				return errors.New("auth_cookie_ttl_hours must be positive")
			}
			return nil
		},
	},
	// StoreOrdersEnabledKey = "store.orders.enabled"
	// Value shape: {"enabled": bool}
	"store.orders.enabled": {
		Validate: func(raw json.RawMessage) error {
			var v struct {
				Enabled bool `json:"enabled"`
			}
			return json.Unmarshal(raw, &v)
		},
	},
	// StoreOffersKey = "store.offers"
	// Value shape: {"items": [...]}
	"store.offers": {
		Validate: func(raw json.RawMessage) error {
			var v struct {
				Items []json.RawMessage `json:"items"`
			}
			return json.Unmarshal(raw, &v)
		},
	},
}

// Helper to check key and value
func ValidateSetting(key string, value json.RawMessage) error {
	reg, ok := SettingsRegistry[key]
	if !ok {
		return fmt.Errorf("invalid setting key: %s", key)
	}
	return reg.Validate(value)
}
