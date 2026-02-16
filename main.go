package main

import (
	"log"
	"net/http"

	"github.com/pantheon/artemis/config"
	"github.com/pantheon/artemis/handlers"
	"github.com/pantheon/artemis/middleware"
)

func main() {
	// Load configuration from environment variables and .env file
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log startup information
	log.Printf("🚀 Starting Artemis server in %s mode", cfg.Environment)
	log.Printf("📍 Server will be available at http://%s", cfg.GetAddress())

	// Create a new HTTP mux (router)
	mux := http.NewServeMux()

	// Register API routes
	// Lightbulb toggle endpoint - called when user taps the lightbulb in the app
	mux.HandleFunc(cfg.APIBasePath+"/lightbulb/toggle", handlers.HandleLightbulbToggle)

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
	log.Printf("   - GET  %s/health - Health check", cfg.APIBasePath)

	if err := http.ListenAndServe(cfg.GetAddress(), handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
