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

	// Govee Smart Light Integration
	// Primary API key from https://developer.govee.com
	// Required to control Govee smart lights and devices
	GoveeAPIKey           string

	// Secondary Govee API key (optional)
	// Used to access devices from a second Govee account (e.g., spouse's account)
	// If set, devices from both accounts will be combined in the UI
	GoveeAPIKeySecondary  string

	// Fire TV Remote Integration
	// URL of the Python Fire TV microservice that handles device communication.
	// The Python service runs locally and uses the Android TV Remote protocol v2
	// to discover, pair with, and control Fire TV devices on the LAN.
	// Default: http://localhost:9090
	FireTVServiceURL      string

	// Wyze Camera Bridge Integration
	// URL of the Docker Wyze Bridge web UI / REST API.
	// The bridge runs as a Docker container and provides camera info at /api/
	// and streams via HLS (port 8888), RTSP (port 8554), and WebRTC (port 8889).
	// Default: http://localhost:5050
	WyzeBridgeURL         string

	// Optional API key for the Wyze Bridge.
	// Only required if WB_AUTH is enabled on the bridge container.
	// Must match the WYZE_BRIDGE_API_KEY set in the bridge's environment.
	WyzeBridgeAPIKey      string
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
		GoveeAPIKey:           getEnv("GOVEE_API_KEY", ""),
		GoveeAPIKeySecondary:  getEnv("GOVEE_API_KEY_SECONDARY", ""),
		FireTVServiceURL:      getEnv("FIRETV_SERVICE_URL", "http://localhost:9090"),
		WyzeBridgeURL:         getEnv("WYZE_BRIDGE_URL", "http://localhost:5050"),
		WyzeBridgeAPIKey:      getEnv("WYZE_BRIDGE_API_KEY", ""),
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

// Validate checks that all required configuration values are present
// Returns an error if any critical configuration is missing
func (c *Config) Validate() error {
	// Check for Govee API key
	// Get your API key from https://developer.govee.com
	// 1. Sign up or log in with your Govee account
	// 2. Navigate to "My Applications"
	// 3. Click "Create Application"
	// 4. Fill in application name and description
	// 5. Copy the generated API key to .env file as GOVEE_API_KEY=your_key
	if c.GoveeAPIKey == "" {
		return fmt.Errorf("GOVEE_API_KEY is required but not set in .env file")
	}

	return nil
}
