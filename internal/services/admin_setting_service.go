package services

import (
	"context"
	"fmt"

	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

const AdminTabOrderSettingKey = "admin.tabs.order"
const AdminTokenTTLSettingKey = "admin.tokens.ttl"

var defaultAdminTabOrder = []string{"ordenes", "categorias", "productos", "empleados", "usuarios", "herramientas", "configuracion"}

type TokenTTLConfig struct {
	AuthCookieTTLHours      int `json:"auth_cookie_ttl_hours"`
	AuthTokenTTLHours       int `json:"auth_token_ttl_hours"`
	TrackingTokenTTLHours   int `json:"tracking_token_ttl_hours"`
	InactivityLogoutSeconds int `json:"inactivity_logout_seconds"`
}

type AdminSettingService struct {
	repo *repository.AdminSettingRepository
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

func (s *AdminSettingService) UpdateTabOrder(ctx context.Context, tabOrder []string) ([]string, error) {
	sanitized := sanitizeTabOrder(tabOrder)
	if len(sanitized) == 0 {
		return nil, fmt.Errorf("tab_order cannot be empty")
	}

	_, err := s.repo.UpsertByKey(ctx, AdminTabOrderSettingKey, map[string]any{"tab_order": sanitized})
	if err != nil {
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

	if resolved.InactivityLogoutSeconds <= 0 {
		legacyMinutes := parsePositiveInt(raw["inactivity_logout_minutes"])
		if legacyMinutes > 0 {
			resolved.InactivityLogoutSeconds = legacyMinutes * 60
		}
	}

	return sanitizeTokenTTLConfig(resolved, defaults), nil
}

func (s *AdminSettingService) UpdateTokenTTLConfig(ctx context.Context, input TokenTTLConfig, defaults TokenTTLConfig) (TokenTTLConfig, error) {
	sanitized := sanitizeTokenTTLConfig(input, defaults)
	if !isValidTokenTTLConfig(sanitized) {
		return TokenTTLConfig{}, fmt.Errorf("invalid token TTL configuration")
	}

	_, err := s.repo.UpsertByKey(ctx, AdminTokenTTLSettingKey, map[string]any{
		"auth_cookie_ttl_hours":     sanitized.AuthCookieTTLHours,
		"auth_token_ttl_hours":      sanitized.AuthTokenTTLHours,
		"tracking_token_ttl_hours":  sanitized.TrackingTokenTTLHours,
		"inactivity_logout_seconds": sanitized.InactivityLogoutSeconds,
	})
	if err != nil {
		return TokenTTLConfig{}, err
	}

	return sanitized, nil
}

func DefaultAdminTabOrder() []string {
	return append([]string(nil), defaultAdminTabOrder...)
}

func sanitizeTabOrder(order []string) []string {
	allowed := map[string]bool{
		"ordenes":       true,
		"categorias":    true,
		"productos":     true,
		"empleados":     true,
		"usuarios":      true,
		"herramientas":  true,
		"configuracion": true,
	}

	seen := make(map[string]bool, len(order))
	sanitized := make([]string, 0, len(defaultAdminTabOrder))

	for _, item := range order {
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
