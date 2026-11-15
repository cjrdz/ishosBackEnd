// admin_setting_service.go — Manages persisted operational settings (token TTL and store settings).
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
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
	loc   *time.Location // store time zone for opening hours
}

const (
	ordersEnabledCacheKey        = "settings:orders_enabled"
	tokenTTLConfigCacheKey       = "settings:token_ttl_config"
	archiveConfigCacheKey        = "settings:archive_config"
	publicOrderRateLimitCacheKey = "settings:public_order_rate_limit"
	storeHoursCacheKey           = "settings:store_hours"
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
		loc:   time.UTC,
	}
}

// WithLocation sets the store time zone used for opening hours.
func (s *AdminSettingService) WithLocation(loc *time.Location) *AdminSettingService {
	if loc != nil {
		s.loc = loc
	}
	return s
}

// GetStoreHours returns the saved weekly schedule (empty when none is saved).
func (s *AdminSettingService) GetStoreHours(ctx context.Context) (WeeklyHours, error) {
	if s.repo == nil {
		return WeeklyHours{}, nil
	}
	if v, ok := s.cache.Get(storeHoursCacheKey); ok {
		return v.(WeeklyHours), nil
	}
	hours := WeeklyHours{}
	rec, err := s.repo.GetByKey(ctx, StoreHoursKey)
	if err != nil {
		if !errors.Is(err, apperror.ErrNotFound) {
			return nil, err
		}
	} else if rawDays, ok := rec.Value["days"]; ok {
		raw, _ := json.Marshal(rawDays)
		if err := json.Unmarshal(raw, &hours); err != nil {
			return nil, fmt.Errorf("decode store hours: %w", err)
		}
	}
	s.cache.Set(storeHoursCacheKey, hours)
	return hours, nil
}

// UpdateStoreHours validates and saves the weekly schedule. An empty
// schedule turns hours off (the store is then always open).
func (s *AdminSettingService) UpdateStoreHours(ctx context.Context, hours WeeklyHours, changedBy string) (WeeklyHours, error) {
	if err := ValidateWeeklyHours(hours); err != nil {
		return nil, err
	}
	normalized := NormalizeWeeklyHours(hours)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	var days map[string]any
	if err := json.Unmarshal(raw, &days); err != nil {
		return nil, err
	}
	if err := s.upsertSetting(ctx, StoreHoursKey, map[string]any{"days": days}, changedBy); err != nil {
		return nil, fmt.Errorf("failed to save store hours: %w", err)
	}
	s.cache.Delete(storeHoursCacheKey)
	return normalized, nil
}

// StoreOpenStatus reports whether the store is open at now per its schedule.
// A read failure fails open: the manual orders switch still applies.
func (s *AdminSettingService) StoreOpenStatus(ctx context.Context, now time.Time) StoreOpenStatus {
	hours, err := s.GetStoreHours(ctx)
	if err != nil {
		log.Error().Err(err).Msg("could not load store hours; treating store as open")
		hours = nil
	}
	return ComputeOpenStatus(hours, now, s.loc)
}

// ClosedMessage explains when the store opens next, in Spanish.
func (s *AdminSettingService) ClosedMessage(status StoreOpenStatus, now time.Time) string {
	return ClosedMessage(status, now, s.loc)
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
	if err := ValidateTokenTTLConfig(input); err != nil {
		return TokenTTLConfig{}, err
	}
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

// Bounds for the "Seguridad y sesiones" settings. Values outside them are
// refused on save (ValidateTokenTTLConfig) and clamped when read, so a bad
// stored value can neither lock everyone out nor keep sessions open for months.
const (
	MinSessionTTLHours        = 1
	MaxSessionTTLHours        = 24 * 30 // cookie and login token: 30 days
	MinTrackingTokenTTLHours  = 1
	MaxTrackingTokenTTLHours  = 24 * 90
	MinInactivityLogoutSecond = 60
	MaxInactivityLogoutSecond = 24 * 60 * 60
)

// ValidateTokenTTLConfig rejects values outside the allowed bounds with
// Spanish messages for the settings form.
func ValidateTokenTTLConfig(c TokenTTLConfig) error {
	fields := map[string]string{}
	if c.AuthCookieTTLHours < MinSessionTTLHours || c.AuthCookieTTLHours > MaxSessionTTLHours {
		fields["auth_cookie_ttl_hours"] = fmt.Sprintf("La cookie de sesión debe durar entre %d y %d horas", MinSessionTTLHours, MaxSessionTTLHours)
	}
	if c.AuthTokenTTLHours < MinSessionTTLHours || c.AuthTokenTTLHours > MaxSessionTTLHours {
		fields["auth_token_ttl_hours"] = fmt.Sprintf("El token de login debe durar entre %d y %d horas", MinSessionTTLHours, MaxSessionTTLHours)
	}
	if c.TrackingTokenTTLHours < MinTrackingTokenTTLHours || c.TrackingTokenTTLHours > MaxTrackingTokenTTLHours {
		fields["tracking_token_ttl_hours"] = fmt.Sprintf("El token de seguimiento debe durar entre %d y %d horas", MinTrackingTokenTTLHours, MaxTrackingTokenTTLHours)
	}
	if c.InactivityLogoutSeconds < MinInactivityLogoutSecond || c.InactivityLogoutSeconds > MaxInactivityLogoutSecond {
		fields["inactivity_logout_seconds"] = fmt.Sprintf("El cierre por inactividad debe estar entre %d y %d segundos", MinInactivityLogoutSecond, MaxInactivityLogoutSecond)
	}
	if len(fields) > 0 {
		return &apperror.ValidationError{Fields: fields, Cause: fmt.Errorf("invalid panel config")}
	}
	return nil
}

// SessionMaxAge is how long a browser session lasts after login: the
// shorter of the cookie and the login token lifetimes.
func (c TokenTTLConfig) SessionMaxAge() time.Duration {
	hours := c.AuthCookieTTLHours
	if c.AuthTokenTTLHours > 0 && c.AuthTokenTTLHours < hours {
		hours = c.AuthTokenTTLHours
	}
	return time.Duration(hours) * time.Hour
}

func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
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

	resolved.AuthCookieTTLHours = clampInt(resolved.AuthCookieTTLHours, MinSessionTTLHours, MaxSessionTTLHours)
	resolved.AuthTokenTTLHours = clampInt(resolved.AuthTokenTTLHours, MinSessionTTLHours, MaxSessionTTLHours)
	resolved.TrackingTokenTTLHours = clampInt(resolved.TrackingTokenTTLHours, MinTrackingTokenTTLHours, MaxTrackingTokenTTLHours)
	resolved.InactivityLogoutSeconds = clampInt(resolved.InactivityLogoutSeconds, MinInactivityLogoutSecond, MaxInactivityLogoutSecond)

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

// StoreTrackingEnabledKey turns the public "Seguir mi orden" page on or off.
// It is off until the admin enables it, so it stays hidden while staff test
// ordering before the public launch.
const StoreTrackingEnabledKey = "store.tracking.enabled"

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
	// TrackingEnabled shows the public order-tracking page and link.
	TrackingEnabled bool `json:"tracking_enabled"`
}

// ActiveOffers returns the offers that have not expired at now. An offer
// without a parseable expiry is treated as active.
func ActiveOffers(offers []OfferItem, now time.Time) []OfferItem {
	active := make([]OfferItem, 0, len(offers))
	for _, o := range offers {
		if o.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, o.ExpiresAt); err == nil && t.Before(now) {
				continue
			}
		}
		active = append(active, o)
	}
	return active
}

// ActiveOfferPrices maps product ID → special price for every active offer
// with a positive discount price (the lowest one if several overlap). Order
// pricing uses it as the product's base price so the server charges what the
// storefront shows.
func (s *AdminSettingService) ActiveOfferPrices(ctx context.Context) (map[uuid.UUID]float64, error) {
	settings, err := s.GetStoreSettings(ctx)
	if err != nil {
		return nil, err
	}
	prices := make(map[uuid.UUID]float64)
	for _, o := range ActiveOffers(settings.Offers, time.Now()) {
		id, parseErr := uuid.Parse(o.ProductID)
		if parseErr != nil || o.DiscountPrice <= 0 {
			continue
		}
		// The storefront treats an offer without a valid expiry as inactive,
		// so never charge an offer price the customer was not shown.
		if _, expErr := time.Parse(time.RFC3339, o.ExpiresAt); expErr != nil {
			continue
		}
		if current, ok := prices[id]; !ok || o.DiscountPrice < current {
			prices[id] = o.DiscountPrice
		}
	}
	return prices, nil
}

func (s *AdminSettingService) GetStoreSettings(ctx context.Context) (StoreSettings, error) {
	result := StoreSettings{OrdersEnabled: true, Offers: []OfferItem{}}
	if s.repo == nil {
		return result, nil
	}

	// Public order tracking (off unless enabled)
	if trackRec, trackErr := s.repo.GetByKey(ctx, StoreTrackingEnabledKey); trackErr == nil {
		if b, ok := trackRec.Value["enabled"].(bool); ok {
			result.TrackingEnabled = b
		}
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

// validateOffers rejects offers the order pricing would silently ignore:
// ActiveOfferPrices needs a product UUID, a non-negative price and an RFC 3339
// expiry (the admin UI sends toISOString()).
func validateOffers(offers []OfferItem) error {
	fields := map[string]string{}
	for i, o := range offers {
		if _, err := uuid.Parse(o.ProductID); err != nil {
			fields[fmt.Sprintf("offers[%d].product_id", i)] = "must be a product UUID"
		}
		if o.DiscountPrice < 0 {
			fields[fmt.Sprintf("offers[%d].discount_price", i)] = "must not be negative"
		}
		if _, err := time.Parse(time.RFC3339, o.ExpiresAt); err != nil {
			fields[fmt.Sprintf("offers[%d].expires_at", i)] = "must be an RFC 3339 date-time"
		}
	}
	if len(fields) > 0 {
		return &apperror.ValidationError{Fields: fields, Cause: fmt.Errorf("invalid offers")}
	}
	return nil
}

func (s *AdminSettingService) UpdateStoreSettings(ctx context.Context, input StoreSettings, changedBy string) (StoreSettings, error) {
	if err := validateOffers(input.Offers); err != nil {
		return StoreSettings{}, err
	}
	if s.repo == nil {
		return input, nil
	}
	// Persist orders enabled
	ordersValue := map[string]any{"enabled": input.OrdersEnabled}
	if err := s.upsertSetting(ctx, StoreOrdersEnabledKey, ordersValue, changedBy); err != nil {
		return StoreSettings{}, fmt.Errorf("failed to save orders enabled: %w", err)
	}
	s.cache.Delete(ordersEnabledCacheKey)

	trackingValue := map[string]any{"enabled": input.TrackingEnabled}
	if err := s.upsertSetting(ctx, StoreTrackingEnabledKey, trackingValue, changedBy); err != nil {
		return StoreSettings{}, fmt.Errorf("failed to save tracking enabled: %w", err)
	}

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
