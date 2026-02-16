package main

import (
	"log"
	"net/http"

	"github.com/pantheon/artemis/config"
	"github.com/pantheon/artemis/govee"
	"github.com/pantheon/artemis/handlers"
	"github.com/pantheon/artemis/middleware"
)

func main() {
	// Load configuration from environment variables and .env file
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate that all required configuration is present
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Initialize Govee API clients for controlling smart lights
	// Create primary client (required)
	goveeClients := []*govee.Client{
		govee.NewClient(cfg.GoveeAPIKey),
	}
	log.Printf("💡 Primary Govee client initialized")

	// Create secondary client if API key is configured
	if cfg.GoveeAPIKeySecondary != "" {
		goveeClients = append(goveeClients, govee.NewClient(cfg.GoveeAPIKeySecondary))
		log.Printf("💡 Secondary Govee client initialized (devices from both accounts will be shown)")
	}

	// Log startup information
	log.Printf("🚀 Starting Artemis server in %s mode", cfg.Environment)
	log.Printf("📍 Server will be available at http://%s", cfg.GetAddress())

	// Create a new HTTP mux (router)
	mux := http.NewServeMux()

	// Register API routes
	// Lightbulb toggle endpoint - called when user taps the lightbulb in the app
	mux.HandleFunc(cfg.APIBasePath+"/lightbulb/toggle", handlers.HandleLightbulbToggle)

	// Govee smart light endpoints - control real Govee devices
	// List all Govee devices from all configured accounts
	mux.HandleFunc(cfg.APIBasePath+"/govee/devices", handlers.HandleGetDevices(goveeClients))
	// Control a specific Govee device (turn on/off, brightness, color)
	mux.HandleFunc(cfg.APIBasePath+"/govee/devices/control", handlers.HandleControlDevice(goveeClients))
	// Query current state of a specific device
	mux.HandleFunc(cfg.APIBasePath+"/govee/devices/state", handlers.HandleGetDeviceState(goveeClients))

	// Health check endpoint - useful for monitoring server status
	mux.HandleFunc(cfg.APIBasePath+"/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"artemis"}`))
	})

	// Apply middleware
	var handler http.Handler = mux

	// Add CORS middleware (allows frontend to make requests)
	handler = middleware.CORS(handler)

	// Add request logging middleware if enabled
	if cfg.EnableRequestLogging {
		handler = middleware.RequestLogger(handler)
	}

	// Start the server
	log.Printf("✅ Server is listening on %s", cfg.GetAddress())
	log.Printf("📝 API endpoints:")
	log.Printf("   - POST %s/lightbulb/toggle - Toggle lightbulb state", cfg.APIBasePath)
	log.Printf("   - GET  %s/govee/devices - List all Govee devices", cfg.APIBasePath)
	log.Printf("   - POST %s/govee/devices/control - Control Govee device", cfg.APIBasePath)
	log.Printf("   - GET  %s/govee/devices/state - Query device state", cfg.APIBasePath)
	log.Printf("   - GET  %s/health - Health check", cfg.APIBasePath)

	if err := http.ListenAndServe(cfg.GetAddress(), handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
