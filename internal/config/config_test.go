package config

import (
	"os"
	"path/filepath"
	"testing"
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
