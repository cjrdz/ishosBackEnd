package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata" // IANA zones for STORE_TIMEZONE; the Alpine image ships none.

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string `env:"PORT" envDefault:"8080"`
	Env  string `env:"ENV" envDefault:"development"` // "development" | "production"

	// Database — Neon PostgreSQL
	// Connection string format:
	// postgres://user:pass@ep-xxx.us-east-2.aws.neon.tech/ishosFactory?sslmode=require
	DatabaseURL string `env:"DATABASE_URL,required"`

	// PocketBase
	// Directory where PocketBase stores its SQLite data (auth collections, logs, etc.)
	PBDataDir string `env:"PB_DATA_DIR" envDefault:"./pb_data"`

	// PocketBase Admin UI
	// When true, PocketBase's built-in HTTP server (admin dashboard) starts on
	// PBAdminUIPort alongside the Chi API server.
	// Development: true  — manage employees via http://localhost:8090/_/
	// Production:  false — all management goes through the Chi API only.
	EnablePBAdminUI bool `env:"ENABLE_PB_ADMIN_UI" envDefault:"false"`

	PBAdminUIHost              string `env:"PB_ADMIN_UI_HOST" envDefault:"127.0.0.1"`
	PBAdminUIPort              string `env:"PB_ADMIN_UI_PORT" envDefault:"8090"`
	PBAdminUIAllowedOrigins    string `env:"PB_ADMIN_UI_ALLOWED_ORIGINS" envDefault:"http://localhost:8090"`
	AllowPBAdminUIInProduction bool   `env:"ALLOW_PB_ADMIN_UI_IN_PRODUCTION" envDefault:"false"`

	// Cloudflare R2 (images only)
	R2AccountID       string `env:"R2_ACCOUNT_ID,required"`
	R2AccessKeyID     string `env:"R2_ACCESS_KEY_ID,required"`
	R2SecretAccessKey string `env:"R2_SECRET_ACCESS_KEY,required"`
	R2BucketName      string `env:"R2_BUCKET_NAME" envDefault:"product-images"`
	R2Region          string `env:"R2_REGION" envDefault:"auto"`
	R2Endpoint        string `env:"R2_ENDPOINT"`
	R2PublicBaseURL   string `env:"R2_PUBLIC_BASE_URL,required"`
	// Maximum uploaded image size in MB.
	AdminImageUploadMaxMB int `env:"ADMIN_IMAGE_UPLOAD_MAX_MB" envDefault:"15"`
	// Maximum uploaded image dimensions in pixels.
	AdminImageUploadMaxWidth  int `env:"ADMIN_IMAGE_UPLOAD_MAX_WIDTH" envDefault:"4096"`
	AdminImageUploadMaxHeight int `env:"ADMIN_IMAGE_UPLOAD_MAX_HEIGHT" envDefault:"4096"`
	// Comma-separated allowlist of upload folder names.
	AllowedUploadFolders string `env:"ALLOWED_UPLOAD_FOLDERS" envDefault:"products,categories,flavors,addons,container-types,profile,general"`

	// Database connection pool tuning.
	DBMaxOpenConns           int `env:"DB_MAX_OPEN_CONNS" envDefault:"25"`
	DBMaxIdleConns           int `env:"DB_MAX_IDLE_CONNS" envDefault:"10"`
	DBConnMaxLifetimeMinutes int `env:"DB_CONN_MAX_LIFETIME_MINUTES" envDefault:"30"`
	DBConnMaxIdleTimeMinutes int `env:"DB_CONN_MAX_IDLE_TIME_MINUTES" envDefault:"5"`

	// WhatsApp Business API (optional, but all-or-nothing if set)
	WhatsAppPhoneID string `env:"WHATSAPP_PHONE_ID"`
	WhatsAppToken   string `env:"WHATSAPP_TOKEN"`
	WhatsAppNumber  string `env:"WHATSAPP_NUMBER"`
	PublicStoreURL  string `env:"PUBLIC_STORE_URL" envDefault:"http://localhost:4321"`

	// Authentication & Session
	// Browser cookie TTL for auth_token (in hours)
	AuthCookieTTLHours int `env:"AUTH_COOKIE_TTL_HOURS" envDefault:"24"`
	// PocketBase auth JWT TTL for employee logins (in hours)
	AuthTokenTTLHours int `env:"AUTH_TOKEN_TTL_HOURS" envDefault:"168"`
	// Frontend session inactivity timeout (in seconds)
	InactivityLogoutSeconds int `env:"INACTIVITY_LOGOUT_SECONDS" envDefault:"900"` // 15 minutes

	// Rate Limiting
	OrdersPerHour int `env:"ORDERS_PER_HOUR" envDefault:"5"`
	OrdersPerDay  int `env:"ORDERS_PER_DAY" envDefault:"20"`
	// Public order rate-limit defaults (phone-first, IP-secondary, staff throttle).
	PublicOrderPhoneBurst         int `env:"PUBLIC_ORDER_PHONE_BURST" envDefault:"3"`
	PublicOrderPhoneWindowMinutes int `env:"PUBLIC_ORDER_PHONE_WINDOW_MINUTES" envDefault:"60"`
	PublicOrderPhoneBlockMinutes  int `env:"PUBLIC_ORDER_PHONE_BLOCK_MINUTES" envDefault:"120"`
	PublicOrderIPBurst            int `env:"PUBLIC_ORDER_IP_BURST" envDefault:"10"`
	PublicOrderIPWindowMinutes    int `env:"PUBLIC_ORDER_IP_WINDOW_MINUTES" envDefault:"60"`
	PublicOrderIPBlockMinutes     int `env:"PUBLIC_ORDER_IP_BLOCK_MINUTES" envDefault:"120"`
	PublicOrderStaffBurst         int `env:"PUBLIC_ORDER_STAFF_BURST" envDefault:"100"`
	PublicOrderStaffWindowMinutes int `env:"PUBLIC_ORDER_STAFF_WINDOW_MINUTES" envDefault:"60"`
	// Max login attempts per IP per minute
	LoginAttemptsPerMinute int `env:"LOGIN_ATTEMPTS_PER_MINUTE" envDefault:"10"`
	// Failed-login lockout controls
	LoginLockoutMaxFailures     int `env:"LOGIN_LOCKOUT_MAX_FAILURES" envDefault:"5"`
	LoginLockoutWindowMinutes   int `env:"LOGIN_LOCKOUT_WINDOW_MINUTES" envDefault:"15"`
	LoginLockoutDurationMinutes int `env:"LOGIN_LOCKOUT_DURATION_MINUTES" envDefault:"15"`

	// Shared secret the frontend BFF sends in X-BFF-Secret so its X-Client-IP
	// header (the end user's IP) is trusted for rate limiting. Must match the
	// BFF_SHARED_SECRET configured on the Cloudflare Worker.
	BFFSharedSecret string `env:"BFF_SHARED_SECRET"`

	// CORS — Comma separated allowed origins
	AllowedOrigins string `env:"ALLOWED_ORIGINS" envDefault:"http://localhost:4321"`

	// Public tracking token signing key. Set a dedicated secret in production.
	TrackingTokenSecret string `env:"TRACKING_TOKEN_SECRET"`
	// Public order tracking token TTL (in hours)
	TrackingTokenTTLHours int `env:"TRACKING_TOKEN_TTL_HOURS" envDefault:"720"`

	// Redis (optional, used for distributed rate limiting).
	RedisURL    string `env:"REDIS_URL"`
	RedisPrefix string `env:"REDIS_PREFIX" envDefault:"ishos"`

	// Tracking endpoint abuse controls.
	TrackingRequestsPerHour int `env:"TRACKING_REQUESTS_PER_HOUR" envDefault:"240"`

	// Order auto-archiving.
	OrderArchiveEnabled         bool `env:"ORDER_ARCHIVE_ENABLED" envDefault:"true"`
	OrderArchiveAgeDays         int  `env:"ORDER_ARCHIVE_AGE_DAYS" envDefault:"7"`
	OrderArchiveIntervalMinutes int  `env:"ORDER_ARCHIVE_INTERVAL_MINUTES" envDefault:"60"`

	// One-time bootstrap: if set and no admin employee exists, the server will
	// create the first admin on startup then ignore the vars on subsequent starts.
	BootstrapAdminEmail    string `env:"BOOTSTRAP_ADMIN_EMAIL"`
	BootstrapAdminPassword string `env:"BOOTSTRAP_ADMIN_PASSWORD"`
	BootstrapAdminName     string `env:"BOOTSTRAP_ADMIN_NAME" envDefault:"Admin"`

	// Store time zone (IANA name). Analytics day/week/month buckets and date
	// filters are computed in this zone, not UTC.
	StoreTimeZone string `env:"STORE_TIMEZONE" envDefault:"America/El_Salvador"`

	// Set automatically by Railway; used to catch a production deploy that is
	// missing ENV=production.
	RailwayEnvironmentName string `env:"RAILWAY_ENVIRONMENT_NAME"`
}

// minBFFSharedSecretLength is the minimum accepted BFF_SHARED_SECRET length in
// production (e.g. `openssl rand -hex 32` yields 64).
const minBFFSharedSecretLength = 32

// Load reads environment variables (and optional .env file) into Config.
// It validates the configuration and exits if required vars are missing or invalid.
func Load() *Config {
	// In development, load .env file.
	// In production (Railway, Render, Fly.io) env vars are injected directly.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	cfg := &Config{}

	// Parse environment variables using caarlos0/env
	if err := env.Parse(cfg); err != nil {
		log.Fatalf("Failed to parse environment configuration: %v", err)
	}

	// Set R2 endpoint if not explicitly provided
	if cfg.R2Endpoint == "" {
		if cfg.R2AccountID == "" {
			log.Fatalf("Required: either R2_ENDPOINT or R2_ACCOUNT_ID must be set")
		}
		cfg.R2Endpoint = "https://" + cfg.R2AccountID + ".r2.cloudflarestorage.com"
	}

	cfg.normalizeR2Endpoint()
	cfg.applyDevelopmentTrackingTokenSecretFallback()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	return cfg
}

// normalizeR2Endpoint strips a trailing "/<bucket>" from R2_ENDPOINT. The
// Cloudflare dashboard shows the S3 API URL with the bucket appended, but the
// S3 client adds the bucket itself; keeping it makes every request target
// "<bucket>/<bucket>/..." and fail with NoSuchKey.
func (c *Config) normalizeR2Endpoint() {
	endpoint := strings.TrimRight(strings.TrimSpace(c.R2Endpoint), "/")
	bucketSuffix := "/" + strings.TrimSpace(c.R2BucketName)
	if c.R2BucketName != "" && strings.HasSuffix(endpoint, bucketSuffix) {
		endpoint = strings.TrimSuffix(endpoint, bucketSuffix)
		log.Printf("R2_ENDPOINT included the bucket name; using %s (set it without /%s)", endpoint, c.R2BucketName)
	}
	c.R2Endpoint = endpoint
}

func (c *Config) applyDevelopmentTrackingTokenSecretFallback() {
	if c == nil || c.Env == "production" {
		return
	}

	if strings.TrimSpace(c.TrackingTokenSecret) != "" {
		return
	}

	// In development, persist a generated secret so tracking tokens remain valid
	// across server restarts. Stored inside PBDataDir so it is kept with the rest
	// of the local state and ignored in production.
	secretPath := filepath.Join(c.PBDataDir, ".tracking_token_secret")
	// #nosec G304 -- Path is derived from operator-controlled PBDataDir config, not user input.
	if data, err := os.ReadFile(secretPath); err == nil && len(data) > 0 {
		c.TrackingTokenSecret = strings.TrimSpace(string(data))
		log.Println("TRACKING_TOKEN_SECRET not set; using persisted development fallback")
		return
	}

	// Generate a cryptographically random 32-byte secret for local development.
	// This avoids deriving the secret from predictable inputs such as DATABASE_URL.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("TRACKING_TOKEN_SECRET not set and failed to generate random fallback: %v", err)
	}
	c.TrackingTokenSecret = hex.EncodeToString(b)

	if err := os.MkdirAll(c.PBDataDir, 0o700); err != nil {
		log.Printf("TRACKING_TOKEN_SECRET: failed to create PBDataDir for persistence: %v", err)
	} else if err := os.WriteFile(secretPath, []byte(c.TrackingTokenSecret), 0o600); err != nil {
		log.Printf("TRACKING_TOKEN_SECRET: failed to persist development fallback: %v", err)
	}

	log.Println("TRACKING_TOKEN_SECRET not set; generated and persisted development fallback")
}

// Validate checks that the configuration is logically valid.
func (c *Config) Validate() error {
	// Port validation
	if c.Port == "" {
		return fmt.Errorf("PORT cannot be empty")
	}

	// Environment
	if c.Env != "development" && c.Env != "production" {
		return fmt.Errorf("ENV must be 'development' or 'production', got %q", c.Env)
	}

	// ENV defaults to development, which silently relaxes production guards
	// (Redis, tracking secret, debug logging). Refuse to boot in Railway's
	// production environment without it.
	if c.RailwayEnvironmentName == "production" && c.Env != "production" {
		return fmt.Errorf("running in Railway environment %q but ENV=%q; set ENV=production", c.RailwayEnvironmentName, c.Env)
	}

	// In production, PocketBase admin UI should not be enabled unless explicitly allowed
	if c.Env == "production" && c.EnablePBAdminUI && !c.AllowPBAdminUIInProduction {
		return fmt.Errorf("PocketBase admin UI cannot be enabled in production unless ALLOW_PB_ADMIN_UI_IN_PRODUCTION=true")
	}

	// In production, tracking token secret must be explicitly set (not DATABASE_URL)
	if c.Env == "production" && (c.TrackingTokenSecret == "" || c.TrackingTokenSecret == c.DatabaseURL) {
		return fmt.Errorf("TRACKING_TOKEN_SECRET must be explicitly set in production (cannot use DATABASE_URL as fallback)")
	}

	// WhatsApp: all-or-nothing (if any is set, all must be set)
	whatsappSet := c.WhatsAppPhoneID != "" || c.WhatsAppToken != "" || c.WhatsAppNumber != ""
	if whatsappSet {
		if c.WhatsAppPhoneID == "" || c.WhatsAppToken == "" || c.WhatsAppNumber == "" {
			return fmt.Errorf("WhatsApp configuration is incomplete: all of WHATSAPP_PHONE_ID, WHATSAPP_TOKEN, and WHATSAPP_NUMBER must be set together")
		}
	}

	// TTL values must be positive
	if c.AuthCookieTTLHours <= 0 {
		return fmt.Errorf("AUTH_COOKIE_TTL_HOURS must be positive, got %d", c.AuthCookieTTLHours)
	}
	if c.AuthTokenTTLHours <= 0 {
		return fmt.Errorf("AUTH_TOKEN_TTL_HOURS must be positive, got %d", c.AuthTokenTTLHours)
	}
	if c.InactivityLogoutSeconds <= 0 {
		return fmt.Errorf("INACTIVITY_LOGOUT_SECONDS must be positive, got %d", c.InactivityLogoutSeconds)
	}

	// Rate limiting values must be positive
	if c.OrdersPerHour <= 0 {
		return fmt.Errorf("ORDERS_PER_HOUR must be positive, got %d", c.OrdersPerHour)
	}
	if c.OrdersPerDay <= 0 {
		return fmt.Errorf("ORDERS_PER_DAY must be positive, got %d", c.OrdersPerDay)
	}
	if c.PublicOrderPhoneBurst <= 0 {
		return fmt.Errorf("PUBLIC_ORDER_PHONE_BURST must be positive, got %d", c.PublicOrderPhoneBurst)
	}
	if c.PublicOrderPhoneWindowMinutes <= 0 {
		return fmt.Errorf("PUBLIC_ORDER_PHONE_WINDOW_MINUTES must be positive, got %d", c.PublicOrderPhoneWindowMinutes)
	}
	if c.PublicOrderPhoneBlockMinutes <= 0 {
		return fmt.Errorf("PUBLIC_ORDER_PHONE_BLOCK_MINUTES must be positive, got %d", c.PublicOrderPhoneBlockMinutes)
	}
	if c.PublicOrderIPBurst <= 0 {
		return fmt.Errorf("PUBLIC_ORDER_IP_BURST must be positive, got %d", c.PublicOrderIPBurst)
	}
	if c.PublicOrderIPWindowMinutes <= 0 {
		return fmt.Errorf("PUBLIC_ORDER_IP_WINDOW_MINUTES must be positive, got %d", c.PublicOrderIPWindowMinutes)
	}
	if c.PublicOrderIPBlockMinutes < 0 {
		return fmt.Errorf("PUBLIC_ORDER_IP_BLOCK_MINUTES must be non-negative, got %d", c.PublicOrderIPBlockMinutes)
	}
	if c.PublicOrderStaffBurst <= 0 {
		return fmt.Errorf("PUBLIC_ORDER_STAFF_BURST must be positive, got %d", c.PublicOrderStaffBurst)
	}
	if c.PublicOrderStaffWindowMinutes <= 0 {
		return fmt.Errorf("PUBLIC_ORDER_STAFF_WINDOW_MINUTES must be positive, got %d", c.PublicOrderStaffWindowMinutes)
	}
	if c.LoginAttemptsPerMinute <= 0 {
		return fmt.Errorf("LOGIN_ATTEMPTS_PER_MINUTE must be positive, got %d", c.LoginAttemptsPerMinute)
	}

	if _, err := time.LoadLocation(c.StoreTimeZone); err != nil {
		return fmt.Errorf("STORE_TIMEZONE %q is not a valid IANA time zone: %w", c.StoreTimeZone, err)
	}

	// Archive configuration
	if c.OrderArchiveAgeDays <= 0 {
		return fmt.Errorf("ORDER_ARCHIVE_AGE_DAYS must be positive, got %d", c.OrderArchiveAgeDays)
	}
	if c.OrderArchiveIntervalMinutes <= 0 {
		return fmt.Errorf("ORDER_ARCHIVE_INTERVAL_MINUTES must be positive, got %d", c.OrderArchiveIntervalMinutes)
	}

	// Lockout configuration
	if c.LoginLockoutMaxFailures <= 0 {
		return fmt.Errorf("LOGIN_LOCKOUT_MAX_FAILURES must be positive, got %d", c.LoginLockoutMaxFailures)
	}
	if c.LoginLockoutWindowMinutes <= 0 {
		return fmt.Errorf("LOGIN_LOCKOUT_WINDOW_MINUTES must be positive, got %d", c.LoginLockoutWindowMinutes)
	}
	if c.LoginLockoutDurationMinutes <= 0 {
		return fmt.Errorf("LOGIN_LOCKOUT_DURATION_MINUTES must be positive, got %d", c.LoginLockoutDurationMinutes)
	}

	// Image upload size
	if c.AdminImageUploadMaxMB <= 0 {
		return fmt.Errorf("ADMIN_IMAGE_UPLOAD_MAX_MB must be positive, got %d", c.AdminImageUploadMaxMB)
	}
	if c.AdminImageUploadMaxWidth <= 0 {
		return fmt.Errorf("ADMIN_IMAGE_UPLOAD_MAX_WIDTH must be positive, got %d", c.AdminImageUploadMaxWidth)
	}
	if c.AdminImageUploadMaxHeight <= 0 {
		return fmt.Errorf("ADMIN_IMAGE_UPLOAD_MAX_HEIGHT must be positive, got %d", c.AdminImageUploadMaxHeight)
	}
	if strings.TrimSpace(c.AllowedUploadFolders) == "" {
		return fmt.Errorf("ALLOWED_UPLOAD_FOLDERS cannot be empty")
	}

	// Database pool tuning
	if c.DBMaxOpenConns <= 0 {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be positive, got %d", c.DBMaxOpenConns)
	}
	if c.DBMaxIdleConns <= 0 {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must be positive, got %d", c.DBMaxIdleConns)
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		return fmt.Errorf("DB_MAX_IDLE_CONNS (%d) cannot exceed DB_MAX_OPEN_CONNS (%d)", c.DBMaxIdleConns, c.DBMaxOpenConns)
	}
	if c.DBConnMaxLifetimeMinutes <= 0 {
		return fmt.Errorf("DB_CONN_MAX_LIFETIME_MINUTES must be positive, got %d", c.DBConnMaxLifetimeMinutes)
	}
	if c.DBConnMaxIdleTimeMinutes <= 0 {
		return fmt.Errorf("DB_CONN_MAX_IDLE_TIME_MINUTES must be positive, got %d", c.DBConnMaxIdleTimeMinutes)
	}

	// Tracking configuration
	if c.TrackingTokenTTLHours <= 0 {
		return fmt.Errorf("TRACKING_TOKEN_TTL_HOURS must be positive, got %d", c.TrackingTokenTTLHours)
	}
	if c.TrackingRequestsPerHour <= 0 {
		return fmt.Errorf("TRACKING_REQUESTS_PER_HOUR must be positive, got %d", c.TrackingRequestsPerHour)
	}

	// Bootstrap credentials must not be present in production; provisioning
	// should use the explicit `make bootstrap-admin` command instead.
	if c.Env == "production" && (c.BootstrapAdminEmail != "" || c.BootstrapAdminPassword != "") {
		return fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL/PASSWORD must not be set in production; use the bootstrap command instead")
	}

	// Without the BFF secret every customer shares the BFF's IP for rate
	// limiting, so one busy hour can block the whole store.
	if c.Env == "production" && len(strings.TrimSpace(c.BFFSharedSecret)) < minBFFSharedSecretLength {
		return fmt.Errorf("BFF_SHARED_SECRET must be set in production (at least %d characters) so per-customer rate limits work behind the frontend BFF", minBFFSharedSecretLength)
	}

	// Redis is optional: rate limits, login lockouts and the analytics cache
	// fall back to memory. That is enough for one instance (counters reset on
	// redeploy); set REDIS_URL (Redis or Valkey) before running more than one
	// replica so they share that state.
	if c.Env == "production" && strings.TrimSpace(c.RedisURL) == "" {
		log.Println("REDIS_URL not set: rate limits and login lockouts are kept in memory (fine for a single instance)")
	}

	return nil
}

// StoreLocation returns the store's time zone. Validate guarantees it loads;
// UTC is only a defensive fallback.
func (c *Config) StoreLocation() *time.Location {
	loc, err := time.LoadLocation(c.StoreTimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// PublicObjectURL constructs the full public URL for an R2 object.
func (c *Config) PublicObjectURL(objectPath string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(c.R2PublicBaseURL), "/")
	cleanPath := strings.TrimLeft(strings.TrimSpace(objectPath), "/")
	if cleanPath == "" {
		return baseURL
	}
	return baseURL + "/" + cleanPath
}
