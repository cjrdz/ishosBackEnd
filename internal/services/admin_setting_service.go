// admin_setting_service.go — Manages persisted admin configuration such as tab order and token TTL settings.
package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
	"github.com/google/uuid"
)

const AdminTabOrderSettingKey = "admin.tabs.order"
const AdminTokenTTLSettingKey = "admin.tokens.ttl" // #nosec G101 -- Settings lookup key, not a credential.

//nolint:misspell // Spanish UI tab names, not English typos.
var defaultAdminTabOrder = []string{"ordenes", "categorias", "productos", "personas", "ofertas", "herramientas", "configuracion"}

type TokenTTLConfig struct {
	AuthCookieTTLHours      int `json:"auth_cookie_ttl_hours"`
	AuthTokenTTLHours       int `json:"auth_token_ttl_hours"`
	TrackingTokenTTLHours   int `json:"tracking_token_ttl_hours"`
	InactivityLogoutSeconds int `json:"inactivity_logout_seconds"`
}

type AdminSettingService struct {
	repo *repository.AdminSettingRepository
}

// Helper to validate and upsert a setting with audit
func (s *AdminSettingService) upsertSetting(ctx context.Context, key string, value map[string]any, changedBy uuid.UUID) error {
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

func NewAdminSettingService(repo *repository.AdminSettingRepository) *AdminSettingService {
	return &AdminSettingService{repo: repo}
}

func (s *AdminSettingService) GetTabOrder(ctx context.Context) ([]string, error) {
	record, err := s.repo.GetByKey(ctx, AdminTabOrderSettingKey)
	if err != nil {
		return append([]string(nil), defaultAdminTabOrder...), nil
	}

	rawOrder, ok := record.Value["tab_order"]
	if !ok {
		return append([]string(nil), defaultAdminTabOrder...), nil
	}

	// JSONB round-trips arrays as []any when decoded by the Postgres driver, not []string.
	list, ok := rawOrder.([]any)
	if !ok {
		if typed, castOK := rawOrder.([]string); castOK {
			return sanitizeTabOrder(typed), nil
		}
		return append([]string(nil), defaultAdminTabOrder...), nil
	}

	order := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			continue
		}
		order = append(order, text)
	}

	return sanitizeTabOrder(order), nil
}

func (s *AdminSettingService) UpdateTabOrder(ctx context.Context, tabOrder []string, changedBy uuid.UUID) ([]string, error) {
	sanitized := sanitizeTabOrder(tabOrder)
	if len(sanitized) == 0 {
		return nil, fmt.Errorf("tab_order cannot be empty")
	}
	value := map[string]any{"tab_order": sanitized}
	if err := s.upsertSetting(ctx, AdminTabOrderSettingKey, value, changedBy); err != nil {
		return nil, err
	}
	return sanitized, nil
}

func (s *AdminSettingService) GetTokenTTLConfig(ctx context.Context, defaults TokenTTLConfig) (TokenTTLConfig, error) {
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

	return sanitizeTokenTTLConfig(resolved, defaults), nil
}

func (s *AdminSettingService) UpdateTokenTTLConfig(ctx context.Context, input TokenTTLConfig, defaults TokenTTLConfig, changedBy uuid.UUID) (TokenTTLConfig, error) {
	sanitized := sanitizeTokenTTLConfig(input, defaults)
	if !isValidTokenTTLConfig(sanitized) {
		return TokenTTLConfig{}, fmt.Errorf("invalid token TTL configuration")
	}
	value := map[string]any{
		"auth_cookie_ttl_hours":     sanitized.AuthCookieTTLHours,
		"auth_token_ttl_hours":      sanitized.AuthTokenTTLHours,
		"tracking_token_ttl_hours":  sanitized.TrackingTokenTTLHours,
		"inactivity_logout_seconds": sanitized.InactivityLogoutSeconds,
	}
	if err := s.upsertSetting(ctx, AdminTokenTTLSettingKey, value, changedBy); err != nil {
		return TokenTTLConfig{}, err
	}
	return sanitized, nil
}

func DefaultAdminTabOrder() []string {
	return append([]string(nil), defaultAdminTabOrder...)
}

func sanitizeTabOrder(order []string) []string {
	//nolint:misspell // Spanish UI tab names, not English typos.
	allowed := map[string]bool{
		"ordenes":       true,
		"categorias":    true,
		"productos":     true,
		"personas":      true,
		"ofertas":       true,
		"herramientas":  true,
		"configuracion": true,
	}

	seen := make(map[string]bool, len(order))
	sanitized := make([]string, 0, len(defaultAdminTabOrder))

	for _, item := range order {
		if item == "empleados" || item == "usuarios" {
			item = "personas"
		}
		if !allowed[item] || seen[item] {
			continue
		}
		seen[item] = true
		sanitized = append(sanitized, item)
	}

	for _, fallback := range defaultAdminTabOrder {
		if !seen[fallback] {
			sanitized = append(sanitized, fallback)
		}
	}

	return sanitized
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

func isValidTokenTTLConfig(cfg TokenTTLConfig) bool {
	const maxHours = 24 * 365 * 3
	const maxSeconds = 24 * 60 * 60
	return cfg.AuthCookieTTLHours > 0 && cfg.AuthCookieTTLHours <= maxHours &&
		cfg.AuthTokenTTLHours > 0 && cfg.AuthTokenTTLHours <= maxHours &&
		cfg.TrackingTokenTTLHours > 0 && cfg.TrackingTokenTTLHours <= maxHours &&
		cfg.InactivityLogoutSeconds > 0 && cfg.InactivityLogoutSeconds <= maxSeconds
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

func (s *AdminSettingService) UpdateStoreSettings(ctx context.Context, input StoreSettings, changedBy uuid.UUID) (StoreSettings, error) {
	// Persist orders enabled
	ordersValue := map[string]any{"enabled": input.OrdersEnabled}
	if err := s.upsertSetting(ctx, StoreOrdersEnabledKey, ordersValue, changedBy); err != nil {
		return StoreSettings{}, fmt.Errorf("failed to save orders enabled: %w", err)
	}
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
	rec, err := s.repo.GetByKey(ctx, StoreOrdersEnabledKey)
	if err != nil {
		return true // fail-open
	}
	if enabled, ok := rec.Value["enabled"]; ok {
		if b, castOK := enabled.(bool); castOK {
			return b
		}
	}
	return true
}
