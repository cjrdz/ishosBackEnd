package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type adminSettingRepoStub struct {
	getByKey    func(ctx context.Context, key string) (dto.AdminSetting, error)
	upsertByKey func(ctx context.Context, key string, value map[string]any, changedBy string) (dto.AdminSetting, error)
}

func (s *adminSettingRepoStub) GetByKey(ctx context.Context, key string) (dto.AdminSetting, error) {
	if s.getByKey != nil {
		return s.getByKey(ctx, key)
	}
	return dto.AdminSetting{}, &apperror.NotFoundError{Resource: "admin_setting"}
}

func (s *adminSettingRepoStub) UpsertByKeyWithAudit(ctx context.Context, key string, value map[string]any, changedBy string) (dto.AdminSetting, error) {
	if s.upsertByKey != nil {
		return s.upsertByKey(ctx, key, value, changedBy)
	}
	return dto.AdminSetting{}, nil
}

func TestAdminSettingServiceGetTokenTTLConfig_ReturnsDefaultsOnNotFound(t *testing.T) {
	repo := &adminSettingRepoStub{}
	svc := NewAdminSettingService(repo)

	defaults := TokenTTLConfig{
		AuthCookieTTLHours:      1,
		AuthTokenTTLHours:       2,
		TrackingTokenTTLHours:   3,
		InactivityLogoutSeconds: 400,
	}
	cfg, err := svc.GetTokenTTLConfig(context.Background(), defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthCookieTTLHours != 1 {
		t.Fatalf("expected AuthCookieTTLHours=1, got %d", cfg.AuthCookieTTLHours)
	}
	if cfg.AuthTokenTTLHours != 2 {
		t.Fatalf("expected AuthTokenTTLHours=2, got %d", cfg.AuthTokenTTLHours)
	}
	if cfg.TrackingTokenTTLHours != 3 {
		t.Fatalf("expected TrackingTokenTTLHours=3, got %d", cfg.TrackingTokenTTLHours)
	}
	if cfg.InactivityLogoutSeconds != 400 {
		t.Fatalf("expected InactivityLogoutSeconds=400, got %d", cfg.InactivityLogoutSeconds)
	}
}

func TestAdminSettingServiceGetTokenTTLConfig_ResolvesFromRepo(t *testing.T) {
	repo := &adminSettingRepoStub{
		getByKey: func(_ context.Context, _ string) (dto.AdminSetting, error) {
			return dto.AdminSetting{
				Value: map[string]any{
					"auth_cookie_ttl_hours":     10,
					"auth_token_ttl_hours":      20,
					"tracking_token_ttl_hours":  30,
					"inactivity_logout_seconds": 600,
				},
			}, nil
		},
	}
	svc := NewAdminSettingService(repo)

	cfg, err := svc.GetTokenTTLConfig(context.Background(), TokenTTLConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthCookieTTLHours != 10 {
		t.Fatalf("expected AuthCookieTTLHours=10, got %d", cfg.AuthCookieTTLHours)
	}
	if cfg.AuthTokenTTLHours != 20 {
		t.Fatalf("expected AuthTokenTTLHours=20, got %d", cfg.AuthTokenTTLHours)
	}
	if cfg.TrackingTokenTTLHours != 30 {
		t.Fatalf("expected TrackingTokenTTLHours=30, got %d", cfg.TrackingTokenTTLHours)
	}
	if cfg.InactivityLogoutSeconds != 600 {
		t.Fatalf("expected InactivityLogoutSeconds=600, got %d", cfg.InactivityLogoutSeconds)
	}
}

func TestAdminSettingServiceAreOrdersEnabled_FailOpen(t *testing.T) {
	repo := &adminSettingRepoStub{}
	svc := NewAdminSettingService(repo)

	if !svc.AreOrdersEnabled(context.Background()) {
		t.Fatal("expected AreOrdersEnabled to be true when repo returns not found")
	}
}

func TestAdminSettingServiceAreOrdersEnabled_RespectsSetting(t *testing.T) {
	repo := &adminSettingRepoStub{
		getByKey: func(_ context.Context, _ string) (dto.AdminSetting, error) {
			return dto.AdminSetting{
				Value: map[string]any{"enabled": false},
			}, nil
		},
	}
	svc := NewAdminSettingService(repo)

	if svc.AreOrdersEnabled(context.Background()) {
		t.Fatal("expected AreOrdersEnabled to be false when setting is disabled")
	}
}

func TestAdminSettingServiceGetStoreSettings_Defaults(t *testing.T) {
	repo := &adminSettingRepoStub{}
	svc := NewAdminSettingService(repo)

	settings, err := svc.GetStoreSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !settings.OrdersEnabled {
		t.Fatal("expected default OrdersEnabled=true")
	}
	if len(settings.Offers) != 0 {
		t.Fatalf("expected default empty offers, got %d", len(settings.Offers))
	}
}

func TestAdminSettingServiceUpdateStoreSettings_Persists(t *testing.T) {
	var storedOrders map[string]any
	var storedOffers map[string]any

	repo := &adminSettingRepoStub{
		upsertByKey: func(_ context.Context, key string, value map[string]any, _ string) (dto.AdminSetting, error) {
			if key == StoreOrdersEnabledKey {
				storedOrders = value
			}
			if key == StoreOffersKey {
				storedOffers = value
			}
			return dto.AdminSetting{Value: value}, nil
		},
	}
	svc := NewAdminSettingService(repo)

	_, err := svc.UpdateStoreSettings(context.Background(), StoreSettings{
		OrdersEnabled: false,
		Offers: []OfferItem{
			// Shaped like the admin UI's payload: product UUID + RFC 3339 expiry.
			{ProductID: "6a1f0e9e-2c1b-4c8a-9f3e-1d2c3b4a5f60", Label: "Test Offer", ExpiresAt: "2026-12-31T23:59:00Z"},
		},
	}, "admin-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if storedOrders == nil || storedOrders["enabled"] != false {
		t.Fatal("expected orders enabled to be persisted as false")
	}
	if storedOffers == nil {
		t.Fatal("expected offers to be persisted")
	}
}

func TestAdminSettingServiceNilRepo(t *testing.T) {
	svc := NewAdminSettingService(nil)

	cfg, err := svc.GetTokenTTLConfig(context.Background(), TokenTTLConfig{AuthCookieTTLHours: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthCookieTTLHours != 5 {
		t.Fatalf("expected fallback default, got %d", cfg.AuthCookieTTLHours)
	}

	if !svc.AreOrdersEnabled(context.Background()) {
		t.Fatal("expected true with nil repo")
	}

	settings, err := svc.GetStoreSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !settings.OrdersEnabled {
		t.Fatal("expected true with nil repo")
	}
}

func TestTokenTTLConfig_BoundsAndSessionMaxAge(t *testing.T) {
	good := TokenTTLConfig{AuthCookieTTLHours: 12, AuthTokenTTLHours: 24, TrackingTokenTTLHours: 72, InactivityLogoutSeconds: 900}
	if err := ValidateTokenTTLConfig(good); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	if got := good.SessionMaxAge(); got != 12*time.Hour {
		t.Fatalf("session max age should be the shorter TTL, got %v", got)
	}
	for _, bad := range []TokenTTLConfig{
		{AuthCookieTTLHours: 12, AuthTokenTTLHours: 24, TrackingTokenTTLHours: 72, InactivityLogoutSeconds: 5},
		{AuthCookieTTLHours: 0, AuthTokenTTLHours: 24, TrackingTokenTTLHours: 72, InactivityLogoutSeconds: 900},
		{AuthCookieTTLHours: 12, AuthTokenTTLHours: 24 * 365, TrackingTokenTTLHours: 72, InactivityLogoutSeconds: 900},
	} {
		if err := ValidateTokenTTLConfig(bad); !errors.Is(err, apperror.ErrValidation) {
			t.Errorf("expected validation error for %+v, got %v", bad, err)
		}
	}
	// A stored 1-second timeout is clamped instead of locking everyone out.
	clamped := sanitizeTokenTTLConfig(TokenTTLConfig{InactivityLogoutSeconds: 1, AuthTokenTTLHours: 100000}, TokenTTLConfig{})
	if clamped.InactivityLogoutSeconds != MinInactivityLogoutSecond || clamped.AuthTokenTTLHours != MaxSessionTTLHours {
		t.Fatalf("unexpected clamp result %+v", clamped)
	}
}
