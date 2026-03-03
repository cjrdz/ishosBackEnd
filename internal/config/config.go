package config

import (
	"log"
	"os"
	"strconv"

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

	// Supabase Storage (images only)
	SupabaseURL    string // e.g. https://xxx.supabase.co
	SupabaseKey    string // service_role key
	SupabaseBucket string // bucket name, e.g. "product-images"

	// WhatsApp Business API
	WhatsAppPhoneID string // Phone number ID from Meta
	WhatsAppToken   string // Permanent or temporary token
	WhatsAppNumber  string // Business phone shown to customers

	// Rate Limiting
	OrdersPerHour int
	OrdersPerDay  int
	// Max login attempts per IP per minute
	LoginAttemptsPerMinute int

	// CORS
	// Comma separated allowed origins
	// e.g. "http://localhost:4321,https://yourdomain.com"
	AllowedOrigins string
}

// Load reads environment variables (and optional .env file) into Config.
func Load() *Config {
	// In development, load .env file.
	// In production (Railway, Render, Fly.io) env vars are injected directly.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	return &Config{
		Port:                       getEnv("PORT", "8080"),
		Env:                        getEnv("ENV", "development"),
		DatabaseURL:                mustEnv("DATABASE_URL"),
		PBDataDir:                  getEnv("PB_DATA_DIR", "./pb_data"),
		EnablePBAdminUI:            getEnvBool("ENABLE_PB_ADMIN_UI", false),
		PBAdminUIHost:              getEnv("PB_ADMIN_UI_HOST", "127.0.0.1"),
		PBAdminUIPort:              getEnv("PB_ADMIN_UI_PORT", "8090"),
		PBAdminUIAllowedOrigins:    getEnv("PB_ADMIN_UI_ALLOWED_ORIGINS", "http://localhost:8090"),
		AllowPBAdminUIInProduction: getEnvBool("ALLOW_PB_ADMIN_UI_IN_PRODUCTION", false),
		SupabaseURL:                mustEnv("SUPABASE_URL"),
		SupabaseKey:                mustEnv("SUPABASE_KEY"),
		SupabaseBucket:             getEnv("SUPABASE_BUCKET", "product-images"),
		WhatsAppPhoneID:            getEnv("WHATSAPP_PHONE_ID", ""),
		WhatsAppToken:              getEnv("WHATSAPP_TOKEN", ""),
		WhatsAppNumber:             getEnv("WHATSAPP_NUMBER", ""),
		OrdersPerHour:              getEnvInt("ORDERS_PER_HOUR", 5),
		OrdersPerDay:               getEnvInt("ORDERS_PER_DAY", 20),
		LoginAttemptsPerMinute:     getEnvInt("LOGIN_ATTEMPTS_PER_MINUTE", 10),
		AllowedOrigins:             getEnv("ALLOWED_ORIGINS", "http://localhost:4321"),
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
