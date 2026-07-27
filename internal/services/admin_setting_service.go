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
	ordersEnabledCacheKey        = "settings:orders_enabled"
	tokenTTLConfigCacheKey       = "settings:token_ttl_config"
	archiveConfigCacheKey        = "settings:archive_config"
	publicOrderRateLimitCacheKey = "settings:public_order_rate_limit"
	settingsCacheTTL             = 10 * time.Second
)

// Order archive configuration keys.
const (
	OrderArchiveConfigKey = "order.archive.config"
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

// UpdateTokenTTLConfig persists the panel/session configuration and invalidates
// the local cache so subsequent reads use the new values immediately.
func (s *AdminSettingService) UpdateTokenTTLConfig(ctx context.Context, input TokenTTLConfig, changedBy string, defaults TokenTTLConfig) (TokenTTLConfig, error) {
	sanitized := sanitizeTokenTTLConfig(input, defaults)
	if s.repo == nil {
		return sanitized, nil
	}

	value := map[string]any{
		"auth_cookie_ttl_hours":     sanitized.AuthCookieTTLHours,
		"auth_token_ttl_hours":      sanitized.AuthTokenTTLHours,
		"tracking_token_ttl_hours":  sanitized.TrackingTokenTTLHours,
		"inactivity_logout_seconds": sanitized.InactivityLogoutSeconds,
	}
	if err := s.upsertSetting(ctx, AdminTokenTTLSettingKey, value, changedBy); err != nil {
		return TokenTTLConfig{}, fmt.Errorf("failed to save panel config: %w", err)
	}
	s.cache.Delete(tokenTTLConfigCacheKey)
	return sanitized, nil
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
	ProductID     string   `json:"product_id"`
	Label         string   `json:"label"`
	Note          string   `json:"note,omitempty"`
	DiscountPrice float64  `json:"discount_price,omitempty"`
	FlavorID      string   `json:"flavor_id,omitempty"`
	FlavorIDs     []string `json:"flavor_ids,omitempty"`
	ExpiresAt     string   `json:"expires_at"` // RFC 3339
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
					if v, ok := m["flavor_id"].(string); ok {
						item.FlavorID = v
					}
					if rawIds, ok := m["flavor_ids"].([]any); ok {
						ids := make([]string, 0, len(rawIds))
						for _, raw := range rawIds {
							if id, idOK := raw.(string); idOK {
								ids = append(ids, id)
							}
						}
						item.FlavorIDs = ids
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
			"flavor_id":      o.FlavorID,
			"flavor_ids":     o.FlavorIDs,
			"expires_at":     o.ExpiresAt,
		})
	}
	offersValue := map[string]any{"items": items}
	if err := s.upsertSetting(ctx, StoreOffersKey, offersValue, changedBy); err != nil {
		return StoreSettings{}, fmt.Errorf("failed to save offers: %w", err)
	}
	return input, nil
}

// ArchiveConfig controls automatic order archiving.
type ArchiveConfig struct {
	Enabled         bool `json:"enabled"`
	AgeDays         int  `json:"age_days"`
	IntervalMinutes int  `json:"interval_minutes"`
}

func sanitizeArchiveConfig(input ArchiveConfig, defaults ArchiveConfig) ArchiveConfig {
	resolved := ArchiveConfig{
		Enabled:         input.Enabled,
		AgeDays:         input.AgeDays,
		IntervalMinutes: input.IntervalMinutes,
	}
	if resolved.AgeDays <= 0 {
		resolved.AgeDays = defaults.AgeDays
	}
	if resolved.AgeDays <= 0 {
		resolved.AgeDays = 7
	}
	if resolved.IntervalMinutes <= 0 {
		resolved.IntervalMinutes = defaults.IntervalMinutes
	}
	if resolved.IntervalMinutes <= 0 {
		resolved.IntervalMinutes = 60
	}
	return resolved
}

// GetArchiveSettings reads the persisted archive configuration, falling back to
// the supplied defaults when no setting exists or values are invalid.
func (s *AdminSettingService) GetArchiveSettings(ctx context.Context, defaults ArchiveConfig) (ArchiveConfig, error) {
	result := sanitizeArchiveConfig(ArchiveConfig{}, defaults)
	if s.repo == nil {
		return result, nil
	}

	if v, ok := s.cache.Get(archiveConfigCacheKey); ok {
		return sanitizeArchiveConfig(v.(ArchiveConfig), defaults), nil
	}

	rec, err := s.repo.GetByKey(ctx, OrderArchiveConfigKey)
	if err != nil {
		return result, nil
	}

	resolved := ArchiveConfig{
		Enabled:         parseBoolSetting(rec.Value["enabled"]),
		AgeDays:         parsePositiveInt(rec.Value["age_days"]),
		IntervalMinutes: parsePositiveInt(rec.Value["interval_minutes"]),
	}
	result = sanitizeArchiveConfig(resolved, defaults)
	s.cache.Set(archiveConfigCacheKey, result)
	return result, nil
}

func parseBoolSetting(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	case float64:
		return typed != 0
	}
	return false
}

// UpdateArchiveSettings persists archive configuration and invalidates the cache.
func (s *AdminSettingService) UpdateArchiveSettings(ctx context.Context, input ArchiveConfig, changedBy string, defaults ArchiveConfig) (ArchiveConfig, error) {
	sanitized := sanitizeArchiveConfig(input, defaults)
	if s.repo == nil {
		return sanitized, nil
	}

	value := map[string]any{
		"enabled":          sanitized.Enabled,
		"age_days":         sanitized.AgeDays,
		"interval_minutes": sanitized.IntervalMinutes,
	}
	if err := s.upsertSetting(ctx, OrderArchiveConfigKey, value, changedBy); err != nil {
		return ArchiveConfig{}, fmt.Errorf("failed to save archive settings: %w", err)
	}
	s.cache.Delete(archiveConfigCacheKey)
	return sanitized, nil
}

// PublicOrderRateLimitConfig controls public order throttling.
type PublicOrderRateLimitConfig struct {
	PhoneBurst         int `json:"phone_burst"`
	PhoneWindowMinutes int `json:"phone_window_minutes"`
	PhoneBlockMinutes  int `json:"phone_block_minutes"`
	IPBurst            int `json:"ip_burst"`
	IPWindowMinutes    int `json:"ip_window_minutes"`
	IPBlockMinutes     int `json:"ip_block_minutes"`
	StaffBurst         int `json:"staff_burst"`
	StaffWindowMinutes int `json:"staff_window_minutes"`
}

// PublicOrderRateLimitConfigKey is the admin settings key for public order rate limits.
const PublicOrderRateLimitConfigKey = "public_order.rate_limit.config"

func sanitizePublicOrderRateLimitConfig(input PublicOrderRateLimitConfig, defaults PublicOrderRateLimitConfig) PublicOrderRateLimitConfig {
	resolved := PublicOrderRateLimitConfig{
		PhoneBurst:         input.PhoneBurst,
		PhoneWindowMinutes: input.PhoneWindowMinutes,
		PhoneBlockMinutes:  input.PhoneBlockMinutes,
		IPBurst:            input.IPBurst,
		IPWindowMinutes:    input.IPWindowMinutes,
		IPBlockMinutes:     input.IPBlockMinutes,
		StaffBurst:         input.StaffBurst,
		StaffWindowMinutes: input.StaffWindowMinutes,
	}
	if resolved.PhoneBurst <= 0 {
		resolved.PhoneBurst = defaults.PhoneBurst
	}
	if resolved.PhoneWindowMinutes <= 0 {
		resolved.PhoneWindowMinutes = defaults.PhoneWindowMinutes
	}
	if resolved.PhoneBlockMinutes <= 0 {
		resolved.PhoneBlockMinutes = defaults.PhoneBlockMinutes
	}
	if resolved.IPBurst <= 0 {
		resolved.IPBurst = defaults.IPBurst
	}
	if resolved.IPWindowMinutes <= 0 {
		resolved.IPWindowMinutes = defaults.IPWindowMinutes
	}
	if resolved.IPBlockMinutes <= 0 {
		resolved.IPBlockMinutes = defaults.IPBlockMinutes
	}
	if resolved.StaffBurst <= 0 {
		resolved.StaffBurst = defaults.StaffBurst
	}
	if resolved.StaffWindowMinutes <= 0 {
		resolved.StaffWindowMinutes = defaults.StaffWindowMinutes
	}
	return resolved
}

// GetPublicOrderRateLimitConfig returns the persisted public order rate-limit config.
func (s *AdminSettingService) GetPublicOrderRateLimitConfig(ctx context.Context, defaults PublicOrderRateLimitConfig) (PublicOrderRateLimitConfig, error) {
	result := sanitizePublicOrderRateLimitConfig(PublicOrderRateLimitConfig{}, defaults)
	if s.repo == nil {
		return result, nil
	}

	if v, ok := s.cache.Get(publicOrderRateLimitCacheKey); ok {
		return sanitizePublicOrderRateLimitConfig(v.(PublicOrderRateLimitConfig), defaults), nil
	}

	rec, err := s.repo.GetByKey(ctx, PublicOrderRateLimitConfigKey)
	if err != nil {
		return result, nil
	}

	resolved := PublicOrderRateLimitConfig{
		PhoneBurst:         parsePositiveInt(rec.Value["phone_burst"]),
		PhoneWindowMinutes: parsePositiveInt(rec.Value["phone_window_minutes"]),
		PhoneBlockMinutes:  parsePositiveInt(rec.Value["phone_block_minutes"]),
		IPBurst:            parsePositiveInt(rec.Value["ip_burst"]),
		IPWindowMinutes:    parsePositiveInt(rec.Value["ip_window_minutes"]),
		IPBlockMinutes:     parsePositiveInt(rec.Value["ip_block_minutes"]),
		StaffBurst:         parsePositiveInt(rec.Value["staff_burst"]),
		StaffWindowMinutes: parsePositiveInt(rec.Value["staff_window_minutes"]),
	}
	result = sanitizePublicOrderRateLimitConfig(resolved, defaults)
	s.cache.Set(publicOrderRateLimitCacheKey, result)
	return result, nil
}

// UpdatePublicOrderRateLimitConfig persists public order rate-limit config and invalidates the cache.
func (s *AdminSettingService) UpdatePublicOrderRateLimitConfig(ctx context.Context, input PublicOrderRateLimitConfig, changedBy string, defaults PublicOrderRateLimitConfig) (PublicOrderRateLimitConfig, error) {
	sanitized := sanitizePublicOrderRateLimitConfig(input, defaults)
	if s.repo == nil {
		return sanitized, nil
	}

	value := map[string]any{
		"phone_burst":          sanitized.PhoneBurst,
		"phone_window_minutes": sanitized.PhoneWindowMinutes,
		"phone_block_minutes":  sanitized.PhoneBlockMinutes,
		"ip_burst":             sanitized.IPBurst,
		"ip_window_minutes":    sanitized.IPWindowMinutes,
		"ip_block_minutes":     sanitized.IPBlockMinutes,
		"staff_burst":          sanitized.StaffBurst,
		"staff_window_minutes": sanitized.StaffWindowMinutes,
	}
	if err := s.upsertSetting(ctx, PublicOrderRateLimitConfigKey, value, changedBy); err != nil {
		return PublicOrderRateLimitConfig{}, fmt.Errorf("failed to save public order rate-limit settings: %w", err)
	}
	s.cache.Delete(publicOrderRateLimitCacheKey)
	return sanitized, nil
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
