package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string
	Env  string // "development" | "production"

	// Database — Neon PostgreSQL
	// Connection string format:
	// postgres://user:pass@ep-xxx.us-east-2.aws.neon.tech/ishosFactory?sslmode=require
	DatabaseURL string

	// PocketBase
	// Directory where PocketBase stores its SQLite data (auth collections, logs, etc.)
	// Defaults to "./pb_data"
	PBDataDir string

	// PocketBase Admin UI
	// When true, PocketBase's built-in HTTP server (admin dashboard) starts on
	// PBAdminUIPort alongside the Chi API server.
	// Development: true  — manage employees via http://localhost:8090/_/
	// Production:  false — all management goes through the Chi API only.
	EnablePBAdminUI bool
	PBAdminUIHost   string // default "127.0.0.1"
	PBAdminUIPort   string // default "8090"
	// Comma separated list of allowed origins for PocketBase Admin UI CORS.
	PBAdminUIAllowedOrigins string
	// Explicit break-glass option. Keep false in production.
	AllowPBAdminUIInProduction bool

	// Cloudflare R2 (images only)
	R2AccountID       string // Cloudflare account ID
	R2AccessKeyID     string // S3-compatible access key ID
	R2SecretAccessKey string // S3-compatible secret access key
	R2BucketName      string // bucket name, e.g. "product-images"
	R2Region          string // typically "auto"
	R2Endpoint        string // e.g. https://<account_id>.r2.cloudflarestorage.com
	R2PublicBaseURL   string // e.g. https://pub-xxx.r2.dev

	// WhatsApp Business API
	WhatsAppPhoneID string // Phone number ID from Meta
	WhatsAppToken   string // Permanent or temporary token
	WhatsAppNumber  string // Business phone shown to customers
	PublicStoreURL  string // Public storefront base URL, e.g. https://store.example.com
	// Browser cookie TTL for auth_token (in hours)
	AuthCookieTTLHours int

	// Rate Limiting
	OrdersPerHour int
	OrdersPerDay  int
	// Max login attempts per IP per minute
	LoginAttemptsPerMinute int
	// Failed-login lockout controls
	LoginLockoutMaxFailures     int
	LoginLockoutWindowMinutes   int
	LoginLockoutDurationMinutes int

	// CORS
	// Comma separated allowed origins
	// e.g. "http://localhost:4321,https://yourdomain.com"
	AllowedOrigins string

	// Public tracking token signing key. Set a dedicated secret in production.
	TrackingTokenSecret string

	// Redis (optional, used for distributed rate limiting).
	RedisURL    string
	RedisPrefix string

	// Tracking endpoint abuse controls.
	TrackingRequestsPerHour int

	// One-time bootstrap: if set and no admin employee exists, the server will
	// create the first admin on startup then ignore the vars on subsequent starts.
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	BootstrapAdminName     string
}

// Load reads environment variables (and optional .env file) into Config.
func Load() *Config {
	// In development, load .env file.
	// In production (Railway, Render, Fly.io) env vars are injected directly.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	databaseURL := mustEnv("DATABASE_URL")

	return &Config{
		Port:                        getEnv("PORT", "8080"),
		Env:                         getEnv("ENV", "development"),
		DatabaseURL:                 databaseURL,
		PBDataDir:                   getEnv("PB_DATA_DIR", "./pb_data"),
		EnablePBAdminUI:             getEnvBool("ENABLE_PB_ADMIN_UI", false),
		PBAdminUIHost:               getEnv("PB_ADMIN_UI_HOST", "127.0.0.1"),
		PBAdminUIPort:               getEnv("PB_ADMIN_UI_PORT", "8090"),
		PBAdminUIAllowedOrigins:     getEnv("PB_ADMIN_UI_ALLOWED_ORIGINS", "http://localhost:8090"),
		AllowPBAdminUIInProduction:  getEnvBool("ALLOW_PB_ADMIN_UI_IN_PRODUCTION", false),
		R2AccountID:                 getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:               mustEnv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:           mustEnv("R2_SECRET_ACCESS_KEY"),
		R2BucketName:                getEnv("R2_BUCKET_NAME", "product-images"),
		R2Region:                    getEnv("R2_REGION", "auto"),
		R2Endpoint:                  getR2Endpoint(),
		R2PublicBaseURL:             mustEnv("R2_PUBLIC_BASE_URL"),
		WhatsAppPhoneID:             getEnv("WHATSAPP_PHONE_ID", ""),
		WhatsAppToken:               getEnv("WHATSAPP_TOKEN", ""),
		WhatsAppNumber:              getEnv("WHATSAPP_NUMBER", ""),
		PublicStoreURL:              getEnv("PUBLIC_STORE_URL", "http://localhost:4321"),
		AuthCookieTTLHours:          getEnvInt("AUTH_COOKIE_TTL_HOURS", 24),
		OrdersPerHour:               getEnvInt("ORDERS_PER_HOUR", 5),
		OrdersPerDay:                getEnvInt("ORDERS_PER_DAY", 20),
		LoginAttemptsPerMinute:      getEnvInt("LOGIN_ATTEMPTS_PER_MINUTE", 10),
		LoginLockoutMaxFailures:     getEnvInt("LOGIN_LOCKOUT_MAX_FAILURES", 5),
		LoginLockoutWindowMinutes:   getEnvInt("LOGIN_LOCKOUT_WINDOW_MINUTES", 15),
		LoginLockoutDurationMinutes: getEnvInt("LOGIN_LOCKOUT_DURATION_MINUTES", 15),
		AllowedOrigins:              getEnv("ALLOWED_ORIGINS", "http://localhost:4321"),
		TrackingTokenSecret:         getEnv("TRACKING_TOKEN_SECRET", databaseURL),
		RedisURL:                    getEnv("REDIS_URL", ""),
		RedisPrefix:                 getEnv("REDIS_PREFIX", "ishos"),
		TrackingRequestsPerHour:     getEnvInt("TRACKING_REQUESTS_PER_HOUR", 60),
		BootstrapAdminEmail:         getEnv("BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPassword:      getEnv("BOOTSTRAP_ADMIN_PASSWORD", ""),
		BootstrapAdminName:          getEnv("BOOTSTRAP_ADMIN_NAME", "Admin"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return v
}

func getR2Endpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT")); endpoint != "" {
		return endpoint
	}

	accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	if accountID == "" {
		log.Fatalf("Required environment variable R2_ENDPOINT or R2_ACCOUNT_ID is not set")
	}

	return "https://" + accountID + ".r2.cloudflarestorage.com"
}

func (c *Config) PublicObjectURL(objectPath string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(c.R2PublicBaseURL), "/")
	cleanPath := strings.TrimLeft(strings.TrimSpace(objectPath), "/")
	if cleanPath == "" {
		return baseURL
	}
	return baseURL + "/" + cleanPath
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
