package platform

import (
	"fmt"
	"os"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL       string
	JWTSecret         string
	ResendAPIKey      string
	GoogleMapsAPIKey  string // optional — delivery geocoding degrades gracefully when blank
	Port              string
	CORSOrigin        string
	NotificationsFrom string // "Name <addr@domain>" — controls the email From header
}

// LoadConfig reads configuration from environment variables.
// It panics with a descriptive message if any required variable is missing.
func LoadConfig() Config {
	return Config{
		DatabaseURL:       requireEnv("DATABASE_URL"),
		JWTSecret:         requireEnv("JWT_SECRET"),
		ResendAPIKey:      requireEnv("RESEND_API_KEY"),
		GoogleMapsAPIKey:  os.Getenv("GOOGLE_MAPS_API_KEY"), // optional
		Port:              envOrDefault("PORT", "8080"),
		CORSOrigin:        envOrDefault("CORS_ORIGIN", "https://protou.pages.dev"),
		NotificationsFrom: envOrDefault("NOTIFICATIONS_FROM", "protou <pedidos@protou.co>"),
	}
}

// requireEnv returns the value of an environment variable or panics with a
// descriptive error if it is not set or empty.
func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return val
}

// envOrDefault returns the value of an environment variable or the provided
// default value if the variable is not set.
func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
