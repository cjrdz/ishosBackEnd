package config

import (
	"encoding/json"
	"fmt"
)

// Registry of allowed keys and their value validators.
// Keys must match the constants defined in services/admin_setting_service.go.
var SettingsRegistry = map[string]struct {
	Validate func(json.RawMessage) error
}{
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
