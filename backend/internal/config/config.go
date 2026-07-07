package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application's configuration.
type Config struct {
	DatabaseURL     string
	Port            string
	GeminiAPIKey    string
	JWTSecret       string
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDContact    string
}

// Load loads the configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/chorecraft?sslmode=disable"),
		Port:            getEnv("PORT", "8080"),
		GeminiAPIKey:    getEnv("GEMINI_API_KEY", ""),
		JWTSecret:       getEnv("JWT_SECRET", "super-secret-key-for-local-dev-12345"),
		VAPIDPublicKey:  getEnv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey: getEnv("VAPID_PRIVATE_KEY", ""),
		VAPIDContact:    getEnv("VAPID_CONTACT", "mailto:admin@chorecraft.app"),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

