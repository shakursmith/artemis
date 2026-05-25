package main

import (
	"log"
	"net/http"

	"github.com/pantheon/artemis/camera"
	"github.com/pantheon/artemis/config"
	"github.com/pantheon/artemis/db"
	"github.com/pantheon/artemis/firetv"
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

	// Initialize SQLite database for profile, room, and device storage
	database, err := db.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	log.Printf("🗄️  Database ready at %s", cfg.DBPath)

	// Seed dev data if enabled — populates profile, rooms, devices, and beacon configs
	// so you don't have to manually re-add everything after a rebuild.
	// Set SEED_DEV_DATA=true in your .env file to enable.
	if cfg.SeedDevData {
		if err := db.SeedDevData(database); err != nil {
			log.Fatalf("Failed to seed dev data: %v", err)
		}
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
	// Uses Go 1.22+ enhanced pattern matching for path parameters ({id}, {profileId})
	mux := http.NewServeMux()

	// ==========================================================================
	// Profile, Room & Device endpoints — CRUD for user management
	// ==========================================================================

	// Initialize handler structs with database dependency
	profileHandler := handlers.NewProfileHandler(database)
	roomHandler := handlers.NewRoomHandler(database)
	deviceHandler := handlers.NewDeviceHandler(database)
	roomTemplateHandler := handlers.NewRoomTemplateHandler(database)

	// Profile endpoints
	mux.HandleFunc("POST "+cfg.APIBasePath+"/profile", profileHandler.HandleCreateProfile)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/profile/{id}", profileHandler.HandleGetProfile)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/profiles", profileHandler.HandleListProfiles)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/profile/{id}", profileHandler.HandleUpdateProfile)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/profile/{id}", profileHandler.HandleDeleteProfile)

	// Room endpoints
	mux.HandleFunc("POST "+cfg.APIBasePath+"/profile/{profileId}/rooms", roomHandler.HandleCreateRoom)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/profile/{profileId}/rooms", roomHandler.HandleListRooms)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/room/{id}", roomHandler.HandleGetRoom)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{id}", roomHandler.HandleUpdateRoom)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{id}/beacon", roomHandler.HandleUpdateRoomBeacon)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/room/{id}", roomHandler.HandleDeleteRoom)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/room/{id}/template", roomTemplateHandler.HandleGetRoomTemplate)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{id}/template", roomTemplateHandler.HandleSaveRoomTemplate)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/room/{id}/template", roomTemplateHandler.HandleDeleteRoomTemplate)

	// Device endpoints
	mux.HandleFunc("POST "+cfg.APIBasePath+"/profile/{profileId}/devices", deviceHandler.HandleCreateDevice)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/profile/{profileId}/devices", deviceHandler.HandleListDevices)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/device/{id}", deviceHandler.HandleGetDevice)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/device/{id}", deviceHandler.HandleUpdateDevice)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/device/{id}/assign", deviceHandler.HandleAssignDevice)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/device/{id}/unassign", deviceHandler.HandleUnassignDevice)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/device/{id}", deviceHandler.HandleDeleteDevice)

	// Device control — generic control endpoints used by the room scene.
	// Looks up the device by UUID, determines its type, and dispatches commands
	// to the appropriate integration (Govee API, Fire TV, etc.).
	deviceControlHandler := handlers.NewDeviceControlHandler(database, goveeClients)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/device/{id}/toggle", deviceControlHandler.HandleToggleDevice)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/device/{id}/brightness", deviceControlHandler.HandleSetBrightness)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/device/{id}/color", deviceControlHandler.HandleSetColor)

	// ==========================================================================
	// Integration endpoints — External service control
	// ==========================================================================

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

	// Fire TV Remote endpoints - control Fire TV devices via Python microservice
	// Initialize the Fire TV client that communicates with the Python service
	firetvClient := firetv.NewClient(cfg.FireTVServiceURL)
	log.Printf("📺 Fire TV client initialized (service URL: %s)", cfg.FireTVServiceURL)

	// Check if the Python Fire TV service is reachable (non-blocking warning)
	if err := firetvClient.CheckHealth(); err != nil {
		log.Printf("⚠️  Fire TV service not reachable: %v", err)
		log.Printf("⚠️  Fire TV features will not work until the Python service is started")
		log.Printf("⚠️  Start it with: cd ../firestick && uvicorn main:app --host 0.0.0.0 --port 9090")
	} else {
		log.Printf("📺 Fire TV service is healthy and reachable")
	}

	// Discover Fire TV devices on the local network
	mux.HandleFunc(cfg.APIBasePath+"/firetv/discover", handlers.HandleFireTVDiscover(firetvClient))
	// Pair with a Fire TV device (two-step PIN flow)
	mux.HandleFunc(cfg.APIBasePath+"/firetv/pair", handlers.HandleFireTVPair(firetvClient))
	// Send remote control commands to a paired Fire TV device
	mux.HandleFunc(cfg.APIBasePath+"/firetv/command", handlers.HandleFireTVCommand(firetvClient))

	// Wyze Camera Bridge endpoints - view live camera streams
	// Initialize the camera client that communicates with Docker Wyze Bridge
	cameraClient := camera.NewClient(cfg.WyzeBridgeURL, cfg.WyzeBridgeAPIKey)
	log.Printf("📷 Camera client initialized (bridge URL: %s)", cfg.WyzeBridgeURL)

	// Check if the Wyze Bridge is reachable (non-blocking warning)
	if err := cameraClient.CheckHealth(); err != nil {
		log.Printf("⚠️  Wyze Bridge not reachable: %v", err)
		log.Printf("⚠️  Camera features will not work until Wyze Bridge is started")
		log.Printf("⚠️  Start it with: cd .. && docker compose up -d")
	} else {
		log.Printf("📷 Wyze Bridge is healthy and reachable")
	}

	// List all cameras with status and stream URLs
	mux.HandleFunc(cfg.APIBasePath+"/cameras", handlers.HandleGetCameras(cameraClient))
	// Get stream URLs for a specific camera by name
	mux.HandleFunc(cfg.APIBasePath+"/cameras/stream", handlers.HandleGetCameraStream(cameraClient))

	// ==========================================================================
	// Diagnostics endpoints — Beacon signal data capture for tuning
	// ==========================================================================

	diagnosticsHandler := handlers.NewDiagnosticsHandler("./diagnostics")

	// Submit a beacon diagnostic report (from iOS app during test sessions)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/diagnostics/beacon-report", diagnosticsHandler.HandleSubmitBeaconReport)
	// List existing beacon diagnostic reports
	mux.HandleFunc("GET "+cfg.APIBasePath+"/diagnostics/beacon-reports", diagnosticsHandler.HandleListBeaconReports)

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
	log.Printf("  Profile & Room Management:")
	log.Printf("   - POST   %s/profile - Create profile", cfg.APIBasePath)
	log.Printf("   - GET    %s/profile/{id} - Get profile (with rooms & devices)", cfg.APIBasePath)
	log.Printf("   - GET    %s/profiles - List all profiles", cfg.APIBasePath)
	log.Printf("   - PUT    %s/profile/{id} - Update profile", cfg.APIBasePath)
	log.Printf("   - DELETE %s/profile/{id} - Delete profile (cascade)", cfg.APIBasePath)
	log.Printf("   - POST   %s/profile/{id}/rooms - Create room", cfg.APIBasePath)
	log.Printf("   - GET    %s/profile/{id}/rooms - List rooms", cfg.APIBasePath)
	log.Printf("   - GET    %s/room/{id} - Get room (with devices)", cfg.APIBasePath)
	log.Printf("   - PUT    %s/room/{id} - Update room", cfg.APIBasePath)
	log.Printf("   - PUT    %s/room/{id}/beacon - Set beacon config", cfg.APIBasePath)
	log.Printf("   - DELETE %s/room/{id} - Delete room", cfg.APIBasePath)
	log.Printf("   - GET    %s/room/{id}/template - Get room scene template", cfg.APIBasePath)
	log.Printf("   - PUT    %s/room/{id}/template - Save custom room template", cfg.APIBasePath)
	log.Printf("   - DELETE %s/room/{id}/template - Reset room template to default", cfg.APIBasePath)
	log.Printf("   - POST   %s/profile/{id}/devices - Create device", cfg.APIBasePath)
	log.Printf("   - GET    %s/profile/{id}/devices - List devices", cfg.APIBasePath)
	log.Printf("   - GET    %s/device/{id} - Get device", cfg.APIBasePath)
	log.Printf("   - PUT    %s/device/{id} - Update device", cfg.APIBasePath)
	log.Printf("   - PUT    %s/device/{id}/assign - Assign device to room", cfg.APIBasePath)
	log.Printf("   - PUT    %s/device/{id}/unassign - Unassign device", cfg.APIBasePath)
	log.Printf("   - DELETE %s/device/{id} - Delete device", cfg.APIBasePath)
	log.Printf("   - POST   %s/device/{id}/toggle - Toggle device on/off", cfg.APIBasePath)
	log.Printf("   - POST   %s/device/{id}/brightness - Set device brightness", cfg.APIBasePath)
	log.Printf("   - POST   %s/device/{id}/color - Set device color", cfg.APIBasePath)
	log.Printf("  Integrations:")
	log.Printf("   - POST %s/lightbulb/toggle - Toggle lightbulb state", cfg.APIBasePath)
	log.Printf("   - GET  %s/govee/devices - List all Govee devices", cfg.APIBasePath)
	log.Printf("   - POST %s/govee/devices/control - Control Govee device", cfg.APIBasePath)
	log.Printf("   - GET  %s/govee/devices/state - Query device state", cfg.APIBasePath)
	log.Printf("   - GET  %s/firetv/discover - Discover Fire TV devices on LAN", cfg.APIBasePath)
	log.Printf("   - POST %s/firetv/pair - Pair with a Fire TV device", cfg.APIBasePath)
	log.Printf("   - POST %s/firetv/command - Send command to Fire TV", cfg.APIBasePath)
	log.Printf("   - GET  %s/cameras - List Wyze cameras", cfg.APIBasePath)
	log.Printf("   - GET  %s/cameras/stream - Get camera stream URLs", cfg.APIBasePath)
	log.Printf("  Diagnostics:")
	log.Printf("   - POST %s/diagnostics/beacon-report - Submit beacon diagnostic report", cfg.APIBasePath)
	log.Printf("   - GET  %s/diagnostics/beacon-reports - List beacon diagnostic reports", cfg.APIBasePath)
	log.Printf("   - GET  %s/health - Health check", cfg.APIBasePath)

	if err := http.ListenAndServe(cfg.GetAddress(), handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
