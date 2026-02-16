package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Port                  string
	Host                  string
	Environment           string
	APIBasePath           string
	EnableRequestLogging  bool
}

// Load reads configuration from environment variables
// It first attempts to load from a .env file, then reads the values
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		Host:                  getEnv("HOST", "0.0.0.0"),
		Environment:           getEnv("ENVIRONMENT", "development"),
		APIBasePath:           getEnv("API_BASE_PATH", "/api"),
		EnableRequestLogging:  getEnvAsBool("ENABLE_REQUEST_LOGGING", true),
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsBool retrieves an environment variable as a boolean
func getEnvAsBool(key string, defaultValue bool) bool {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}
	return defaultValue
}

// GetAddress returns the full address string for the server
func (c *Config) GetAddress() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}
