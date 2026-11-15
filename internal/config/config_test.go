package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caarlos0/env/v10"
)

func TestApplyDevelopmentTrackingTokenSecretFallbackSetsRandomSecret(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		Env:         "development",
		DatabaseURL: "postgres://user:pass@localhost:5432/ishos?sslmode=disable",
		PBDataDir:   tmpDir,
	}

	cfg.applyDevelopmentTrackingTokenSecretFallback()

	if cfg.TrackingTokenSecret == "" {
		t.Fatal("expected development fallback tracking token secret")
	}
	if cfg.TrackingTokenSecret == cfg.DatabaseURL {
		t.Fatal("expected random fallback, got raw database URL")
	}

	// 32 random bytes encoded as 64 hex characters.
	if len(cfg.TrackingTokenSecret) != 64 {
		t.Fatalf("expected 64-char hex secret, got %d chars", len(cfg.TrackingTokenSecret))
	}

	// Secret should be persisted inside PBDataDir.
	secretPath := filepath.Join(tmpDir, ".tracking_token_secret")
	data, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("expected secret file to be persisted: %v", err)
	}
	if string(data) != cfg.TrackingTokenSecret {
		t.Fatalf("expected persisted secret to match config secret")
	}

	// A second invocation with an empty secret should reload the persisted value.
	original := cfg.TrackingTokenSecret
	cfg.TrackingTokenSecret = ""
	cfg.applyDevelopmentTrackingTokenSecretFallback()
	if cfg.TrackingTokenSecret != original {
		t.Fatalf("expected persisted fallback to be stable across invocations")
	}
}

func TestApplyDevelopmentTrackingTokenSecretFallbackPreservesExplicitSecret(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Env:                 "development",
		DatabaseURL:         "postgres://user:pass@localhost:5432/ishos?sslmode=disable",
		TrackingTokenSecret: "explicit-secret",
	}

	cfg.applyDevelopmentTrackingTokenSecretFallback()

	if cfg.TrackingTokenSecret != "explicit-secret" {
		t.Fatalf("expected explicit secret to be preserved, got %q", cfg.TrackingTokenSecret)
	}
}

func TestApplyDevelopmentTrackingTokenSecretFallbackSkipsProduction(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Env:         "production",
		DatabaseURL: "postgres://user:pass@localhost:5432/ishos?sslmode=disable",
	}

	cfg.applyDevelopmentTrackingTokenSecretFallback()

	if cfg.TrackingTokenSecret != "" {
		t.Fatalf("expected production config to require explicit secret, got %q", cfg.TrackingTokenSecret)
	}
}

// productionConfig returns a configuration that passes Validate in production,
// built from the struct's env defaults plus the required production values.
func productionConfig(t *testing.T) *Config {
	t.Helper()
	cfg := &Config{}
	err := env.ParseWithOptions(cfg, env.Options{Environment: map[string]string{
		"ENV":                   "production",
		"DATABASE_URL":          "postgres://u:p@db.example.com/app?sslmode=require",
		"R2_ACCOUNT_ID":         "acct",
		"R2_ACCESS_KEY_ID":      "key",
		"R2_SECRET_ACCESS_KEY":  "secret",
		"R2_PUBLIC_BASE_URL":    "https://img.example.com",
		"TRACKING_TOKEN_SECRET": "tracking-secret-for-tests",
		"REDIS_URL":             "redis://localhost:6379/0",
		"BFF_SHARED_SECRET":     strings.Repeat("b", minBFFSharedSecretLength),
	}})
	if err != nil {
		t.Fatalf("parse test config: %v", err)
	}
	return cfg
}

func TestValidateProductionConfigAccepted(t *testing.T) {
	if err := productionConfig(t).Validate(); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
}

func TestValidateRequiresBFFSharedSecretInProduction(t *testing.T) {
	for _, secret := range []string{"", "   ", strings.Repeat("x", minBFFSharedSecretLength-1)} {
		cfg := productionConfig(t)
		cfg.BFFSharedSecret = secret
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "BFF_SHARED_SECRET") {
			t.Fatalf("secret %q: expected BFF_SHARED_SECRET error, got %v", secret, err)
		}
	}
}

func TestValidateRejectsDevelopmentEnvOnRailwayProduction(t *testing.T) {
	cfg := productionConfig(t)
	cfg.Env = "development"
	cfg.RailwayEnvironmentName = "production"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "ENV=production") {
		t.Fatalf("expected ENV mismatch error, got %v", err)
	}

	cfg.RailwayEnvironmentName = "staging"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("non-production Railway environment should allow development, got %v", err)
	}
}

func TestNormalizeR2EndpointStripsBucketSuffix(t *testing.T) {
	cases := map[string]string{
		"https://acct.r2.cloudflarestorage.com/ishos-menu":  "https://acct.r2.cloudflarestorage.com",
		"https://acct.r2.cloudflarestorage.com/ishos-menu/": "https://acct.r2.cloudflarestorage.com",
		"https://acct.r2.cloudflarestorage.com":             "https://acct.r2.cloudflarestorage.com",
		"https://acct.r2.cloudflarestorage.com/other":       "https://acct.r2.cloudflarestorage.com/other",
	}
	for in, want := range cases {
		cfg := &Config{R2Endpoint: in, R2BucketName: "ishos-menu"}
		cfg.normalizeR2Endpoint()
		if cfg.R2Endpoint != want {
			t.Errorf("normalizeR2Endpoint(%q) = %q, want %q", in, cfg.R2Endpoint, want)
		}
	}
}
