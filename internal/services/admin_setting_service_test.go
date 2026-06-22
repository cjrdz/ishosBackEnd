package services

import (
	"context"
	"testing"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type adminSettingRepoStub struct {
	getByKey      func(ctx context.Context, key string) (dto.AdminSetting, error)
	upsertByKey   func(ctx context.Context, key string, value map[string]any, changedBy string) (dto.AdminSetting, error)
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
		InactivityLogoutSeconds: 4,
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
	if cfg.InactivityLogoutSeconds != 4 {
		t.Fatalf("expected InactivityLogoutSeconds=4, got %d", cfg.InactivityLogoutSeconds)
	}
}

func TestAdminSettingServiceGetTokenTTLConfig_ResolvesFromRepo(t *testing.T) {
	repo := &adminSettingRepoStub{
		getByKey: func(_ context.Context, _ string) (dto.AdminSetting, error) {
			return dto.AdminSetting{
				Value: map[string]any{
					"auth_cookie_ttl_hours":      10,
					"auth_token_ttl_hours":       20,
					"tracking_token_ttl_hours":   30,
					"inactivity_logout_seconds":  40,
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
	if cfg.InactivityLogoutSeconds != 40 {
		t.Fatalf("expected InactivityLogoutSeconds=40, got %d", cfg.InactivityLogoutSeconds)
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
			{ProductID: "prod-1", Label: "Test Offer"},
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
