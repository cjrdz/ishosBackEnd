// admin_setting_service.go — Manages persisted operational settings (token TTL and store settings).
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/cache"
	"github.com/cjrdz/ishosBackEnd/internal/config"
)

const AdminTokenTTLSettingKey = "admin.tokens.ttl" // #nosec G101 -- Settings lookup key, not a credential.

type TokenTTLConfig struct {
	AuthCookieTTLHours      int `json:"auth_cookie_ttl_hours"`
	AuthTokenTTLHours       int `json:"auth_token_ttl_hours"`
	TrackingTokenTTLHours   int `json:"tracking_token_ttl_hours"`
	InactivityLogoutSeconds int `json:"inactivity_logout_seconds"`
}

type AdminSettingService struct {
	repo  AdminSettingRepository
	cache *cache.TTLCache
}

const (
	ordersEnabledCacheKey = "settings:orders_enabled"
	tokenTTLConfigCacheKey = "settings:token_ttl_config"
	settingsCacheTTL       = 10 * time.Second
)

// Helper to validate and upsert a setting with audit
func (s *AdminSettingService) upsertSetting(ctx context.Context, key string, value map[string]any, changedBy string) error {
	if s.repo == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	if err := config.ValidateSetting(key, raw); err != nil {
		return fmt.Errorf("invalid setting for key %s: %w", key, err)
	}
	_, err = s.repo.UpsertByKeyWithAudit(ctx, key, value, changedBy)
	return err
}

func NewAdminSettingService(repo AdminSettingRepository) *AdminSettingService {
	return &AdminSettingService{
		repo:  repo,
		cache: cache.New(settingsCacheTTL),
	}
}

func (s *AdminSettingService) GetTokenTTLConfig(ctx context.Context, defaults TokenTTLConfig) (TokenTTLConfig, error) {
	if s.repo == nil {
		return sanitizeTokenTTLConfig(TokenTTLConfig{}, defaults), nil
	}

	if v, ok := s.cache.Get(tokenTTLConfigCacheKey); ok {
		return sanitizeTokenTTLConfig(v.(TokenTTLConfig), defaults), nil
	}

	record, err := s.repo.GetByKey(ctx, AdminTokenTTLSettingKey)
	if err != nil {
		return sanitizeTokenTTLConfig(TokenTTLConfig{}, defaults), nil
	}

	raw := record.Value
	if raw == nil {
		return sanitizeTokenTTLConfig(TokenTTLConfig{}, defaults), nil
	}

	resolved := TokenTTLConfig{
		AuthCookieTTLHours:      parsePositiveInt(raw["auth_cookie_ttl_hours"]),
		AuthTokenTTLHours:       parsePositiveInt(raw["auth_token_ttl_hours"]),
		TrackingTokenTTLHours:   parsePositiveInt(raw["tracking_token_ttl_hours"]),
		InactivityLogoutSeconds: parsePositiveInt(raw["inactivity_logout_seconds"]),
	}

	// Legacy migration: older records stored minutes; new records use seconds.
	if resolved.InactivityLogoutSeconds <= 0 {
		legacyMinutes := parsePositiveInt(raw["inactivity_logout_minutes"])
		if legacyMinutes > 0 {
			resolved.InactivityLogoutSeconds = legacyMinutes * 60
		}
	}

	result := sanitizeTokenTTLConfig(resolved, defaults)
	s.cache.Set(tokenTTLConfigCacheKey, result)
	return result, nil
}

func sanitizeTokenTTLConfig(input TokenTTLConfig, defaults TokenTTLConfig) TokenTTLConfig {
	resolved := TokenTTLConfig{
		AuthCookieTTLHours:      input.AuthCookieTTLHours,
		AuthTokenTTLHours:       input.AuthTokenTTLHours,
		TrackingTokenTTLHours:   input.TrackingTokenTTLHours,
		InactivityLogoutSeconds: input.InactivityLogoutSeconds,
	}

	if resolved.AuthCookieTTLHours <= 0 {
		resolved.AuthCookieTTLHours = defaults.AuthCookieTTLHours
	}
	if resolved.AuthTokenTTLHours <= 0 {
		resolved.AuthTokenTTLHours = defaults.AuthTokenTTLHours
	}
	if resolved.TrackingTokenTTLHours <= 0 {
		resolved.TrackingTokenTTLHours = defaults.TrackingTokenTTLHours
	}
	if resolved.InactivityLogoutSeconds <= 0 {
		resolved.InactivityLogoutSeconds = defaults.InactivityLogoutSeconds
	}

	if resolved.AuthCookieTTLHours <= 0 {
		resolved.AuthCookieTTLHours = 24
	}
	if resolved.AuthTokenTTLHours <= 0 {
		resolved.AuthTokenTTLHours = 168
	}
	if resolved.TrackingTokenTTLHours <= 0 {
		resolved.TrackingTokenTTLHours = 720
	}
	if resolved.InactivityLogoutSeconds <= 0 {
		resolved.InactivityLogoutSeconds = 900
	}

	return resolved
}

func parsePositiveInt(value any) int {
	maxInt := int(^uint(0) >> 1)

	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int64:
		if typed > 0 && typed <= int64(maxInt) {
			return int(typed)
		}
	case float64:
		if typed > 0 && typed <= float64(maxInt) {
			return int(typed)
		}
	}

	return 0
}

// ── Store Settings (orders kill-switch + special offers) ─────────────

const StoreOrdersEnabledKey = "store.orders.enabled"
const StoreOffersKey = "store.offers"

type OfferItem struct {
	ProductID     string  `json:"product_id"`
	Label         string  `json:"label"`
	Note          string  `json:"note,omitempty"`
	DiscountPrice float64 `json:"discount_price,omitempty"`
	ExpiresAt     string  `json:"expires_at"` // RFC 3339
}

type StoreSettings struct {
	OrdersEnabled bool        `json:"orders_enabled"`
	Offers        []OfferItem `json:"offers"`
}

func (s *AdminSettingService) GetStoreSettings(ctx context.Context) (StoreSettings, error) {
	result := StoreSettings{OrdersEnabled: true, Offers: []OfferItem{}}
	if s.repo == nil {
		return result, nil
	}

	// Orders enabled
	rec, err := s.repo.GetByKey(ctx, StoreOrdersEnabledKey)
	if err == nil {
		if enabled, ok := rec.Value["enabled"]; ok {
			if b, castOK := enabled.(bool); castOK {
				result.OrdersEnabled = b
			}
		}
	}

	// Offers
	offerRec, err := s.repo.GetByKey(ctx, StoreOffersKey)
	if err == nil {
		if rawItems, ok := offerRec.Value["items"]; ok {
			if list, castOK := rawItems.([]any); castOK {
				for _, raw := range list {
					m, mOK := raw.(map[string]any)
					if !mOK {
						continue
					}
					item := OfferItem{}
					if v, ok := m["product_id"].(string); ok {
						item.ProductID = v
					}
					if v, ok := m["label"].(string); ok {
						item.Label = v
					}
					if v, ok := m["note"].(string); ok {
						item.Note = v
					}
					if v, ok := m["discount_price"].(float64); ok {
						item.DiscountPrice = v
					}
					if v, ok := m["expires_at"].(string); ok {
						item.ExpiresAt = v
					}
					if item.ProductID != "" {
						result.Offers = append(result.Offers, item)
					}
				}
			}
		}
	}

	return result, nil
}

func (s *AdminSettingService) UpdateStoreSettings(ctx context.Context, input StoreSettings, changedBy string) (StoreSettings, error) {
	if s.repo == nil {
		return input, nil
	}
	// Persist orders enabled
	ordersValue := map[string]any{"enabled": input.OrdersEnabled}
	if err := s.upsertSetting(ctx, StoreOrdersEnabledKey, ordersValue, changedBy); err != nil {
		return StoreSettings{}, fmt.Errorf("failed to save orders enabled: %w", err)
	}
	s.cache.Delete(ordersEnabledCacheKey)

	// Persist offers
	items := make([]map[string]any, 0, len(input.Offers))
	for _, o := range input.Offers {
		items = append(items, map[string]any{
			"product_id":     o.ProductID,
			"label":          o.Label,
			"note":           o.Note,
			"discount_price": o.DiscountPrice,
			"expires_at":     o.ExpiresAt,
		})
	}
	offersValue := map[string]any{"items": items}
	if err := s.upsertSetting(ctx, StoreOffersKey, offersValue, changedBy); err != nil {
		return StoreSettings{}, fmt.Errorf("failed to save offers: %w", err)
	}
	return input, nil
}

// AreOrdersEnabled is a convenience helper for the kill-switch check.
// It returns true on error (fail-open) so a DB hiccup does not block orders.
func (s *AdminSettingService) AreOrdersEnabled(ctx context.Context) bool {
	if s.repo == nil {
		return true
	}

	if v, ok := s.cache.Get(ordersEnabledCacheKey); ok {
		return v.(bool)
	}

	rec, err := s.repo.GetByKey(ctx, StoreOrdersEnabledKey)
	if err != nil {
		return true // fail-open
	}

	enabled := true
	if raw, ok := rec.Value["enabled"]; ok {
		if b, castOK := raw.(bool); castOK {
			enabled = b
		}
	}

	s.cache.Set(ordersEnabledCacheKey, enabled)
	return enabled
}
