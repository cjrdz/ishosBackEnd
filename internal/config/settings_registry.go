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
	// StoreTrackingEnabledKey = "store.tracking.enabled"
	// Value shape: {"enabled": bool}
	"store.tracking.enabled": {
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
	// StoreHoursKey = "store.hours"
	// Value shape: {"days": {"mon": [{"open": "HH:MM", "close": "HH:MM"}], ...}}
	// Semantic rules (format, order, overlaps) live in services.ValidateWeeklyHours.
	"store.hours": {
		Validate: func(raw json.RawMessage) error {
			var v struct {
				Days map[string][]struct {
					Open  string `json:"open"`
					Close string `json:"close"`
				} `json:"days"`
			}
			return json.Unmarshal(raw, &v)
		},
	},
	// OrderArchiveConfigKey = "order.archive.config"
	// Value shape: {"enabled": bool, "age_days": int, "interval_minutes": int}
	"order.archive.config": {
		Validate: func(raw json.RawMessage) error {
			var v struct {
				Enabled         bool `json:"enabled"`
				AgeDays         int  `json:"age_days"`
				IntervalMinutes int  `json:"interval_minutes"`
			}
			return json.Unmarshal(raw, &v)
		},
	},
	// AdminTokenTTLSettingKey = "admin.tokens.ttl"
	// Value shape: {"auth_cookie_ttl_hours": int, "auth_token_ttl_hours": int,
	//               "tracking_token_ttl_hours": int, "inactivity_logout_seconds": int}
	"admin.tokens.ttl": {
		Validate: func(raw json.RawMessage) error {
			var v struct {
				AuthCookieTTLHours      int `json:"auth_cookie_ttl_hours"`
				AuthTokenTTLHours       int `json:"auth_token_ttl_hours"`
				TrackingTokenTTLHours   int `json:"tracking_token_ttl_hours"`
				InactivityLogoutSeconds int `json:"inactivity_logout_seconds"`
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
