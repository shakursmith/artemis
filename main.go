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
	businessHandler := handlers.NewBusinessHandler(database)
	locationHandler := handlers.NewLocationHandler(database)
	triggerHandler := handlers.NewTriggerHandler(database)
	menuTemplateHandler := handlers.NewMenuTemplateHandler(database)
	menuOptionHandler := handlers.NewMenuOptionHandler(database)
	roomMenuHandler := handlers.NewRoomMenuHandler(database)
	productCatalogHandler := handlers.NewProductCatalogHandler(database)
	customerHandler := handlers.NewCustomerHandler(database)
	arrivalHandler := handlers.NewArrivalHandler(database)

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
	mux.HandleFunc("GET "+cfg.APIBasePath+"/device/{id}/state", deviceControlHandler.HandleGetDeviceState)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/device/{id}/brightness", deviceControlHandler.HandleSetBrightness)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/device/{id}/color", deviceControlHandler.HandleSetColor)

	// Business endpoints (shop mode)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/business", businessHandler.HandleCreateBusiness)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/business/{id}", businessHandler.HandleGetBusiness)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/business/{id}", businessHandler.HandleUpdateBusiness)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/business/{id}", businessHandler.HandleDeleteBusiness)

	// Location endpoints (shop mode)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/business/{businessId}/locations", locationHandler.HandleCreateLocation)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/business/{businessId}/locations", locationHandler.HandleListLocations)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/location/{id}", locationHandler.HandleGetLocation)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/location/{id}", locationHandler.HandleUpdateLocation)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/location/{id}", locationHandler.HandleDeleteLocation)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/location/{locationId}/rooms", locationHandler.HandleCreateRoomForLocation)

	// Room Trigger endpoints
	mux.HandleFunc("POST "+cfg.APIBasePath+"/room/{roomId}/triggers", triggerHandler.HandleCreateTrigger)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/room/{roomId}/triggers", triggerHandler.HandleListTriggers)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/trigger/{id}", triggerHandler.HandleGetTrigger)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/trigger/{id}", triggerHandler.HandleUpdateTrigger)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/trigger/{id}", triggerHandler.HandleDeleteTrigger)

	// Menu Template endpoints (shop mode menu system)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/menu-templates", menuTemplateHandler.HandleListTemplates)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/business/{businessId}/menu-templates", menuTemplateHandler.HandleCreateTemplate)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/menu-template/{id}", menuTemplateHandler.HandleUpdateTemplate)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/menu-template/{id}", menuTemplateHandler.HandleDeleteTemplate)

	// Menu Option endpoints (shop mode menu system)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/business/{businessId}/menu-options", menuOptionHandler.HandleListMenuOptions)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/business/{businessId}/menu-options/copy", menuOptionHandler.HandleCopyTemplate)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/business/{businessId}/menu-options", menuOptionHandler.HandleCreateMenuOption)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/menu-option/{id}", menuOptionHandler.HandleUpdateMenuOption)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/menu-option/{id}", menuOptionHandler.HandleDeleteMenuOption)

	// Room Menu endpoints (shop mode menu system)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/room/{roomId}/menu", roomMenuHandler.HandleGetRoomMenu)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{roomId}/menu/enabled", roomMenuHandler.HandleUpdateMenuEnabled)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{roomId}/menu/appearance", roomMenuHandler.HandleUpdateMenuAppearance)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/room/{roomId}/menu/assign", roomMenuHandler.HandleAssignMenuOption)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/room/{roomId}/menu/unassign", roomMenuHandler.HandleUnassignMenuOption)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{roomId}/menu/reorder", roomMenuHandler.HandleReorderMenuOptions)

	// Product Catalog endpoints (food/drink menu system for shop mode)
	// Menu management
	mux.HandleFunc("GET "+cfg.APIBasePath+"/business/{businessId}/product-menus", productCatalogHandler.HandleListProductMenus)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/business/{businessId}/product-menus/create", productCatalogHandler.HandleCreateProductMenu)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/product-menu/{menuId}", productCatalogHandler.HandleGetProductMenu)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/product-menu/{menuId}/full", productCatalogHandler.HandleGetFullMenu)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/product-menu/{menuId}", productCatalogHandler.HandleUpdateProductMenu)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/product-menu/{menuId}", productCatalogHandler.HandleDeleteProductMenu)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/business/{businessId}/product-menus/reorder", productCatalogHandler.HandleReorderProductMenus)
	// Category management
	mux.HandleFunc("GET "+cfg.APIBasePath+"/product-menu/{menuId}/categories", productCatalogHandler.HandleListCategories)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/product-menu/{menuId}/categories", productCatalogHandler.HandleCreateCategory)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/category/{categoryId}", productCatalogHandler.HandleGetCategory)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/category/{categoryId}", productCatalogHandler.HandleUpdateCategory)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/category/{categoryId}", productCatalogHandler.HandleDeleteCategory)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/product-menu/{menuId}/categories/reorder", productCatalogHandler.HandleReorderCategories)
	// Item management
	mux.HandleFunc("GET "+cfg.APIBasePath+"/category/{categoryId}/items", productCatalogHandler.HandleListItems)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/category/{categoryId}/items", productCatalogHandler.HandleCreateItem)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/item/{itemId}", productCatalogHandler.HandleGetItem)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/item/{itemId}", productCatalogHandler.HandleUpdateItem)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/item/{itemId}", productCatalogHandler.HandleDeleteItem)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/item/{itemId}/sold-out", productCatalogHandler.HandleToggleSoldOut)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/category/{categoryId}/items/reorder", productCatalogHandler.HandleReorderItems)
	// Modifier Group management
	mux.HandleFunc("GET "+cfg.APIBasePath+"/item/{itemId}/modifier-groups", productCatalogHandler.HandleListModifierGroups)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/item/{itemId}/modifier-groups", productCatalogHandler.HandleCreateModifierGroup)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/modifier-group/{modifierGroupId}", productCatalogHandler.HandleGetModifierGroup)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/modifier-group/{modifierGroupId}", productCatalogHandler.HandleUpdateModifierGroup)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/modifier-group/{modifierGroupId}", productCatalogHandler.HandleDeleteModifierGroup)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/item/{itemId}/modifier-groups/reorder", productCatalogHandler.HandleReorderModifierGroups)
	// Modifier management
	mux.HandleFunc("GET "+cfg.APIBasePath+"/modifier-group/{modifierGroupId}/modifiers", productCatalogHandler.HandleListModifiers)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/modifier-group/{modifierGroupId}/modifiers", productCatalogHandler.HandleCreateModifier)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/modifier/{modifierId}", productCatalogHandler.HandleGetModifier)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/modifier/{modifierId}", productCatalogHandler.HandleUpdateModifier)
	mux.HandleFunc("DELETE "+cfg.APIBasePath+"/modifier/{modifierId}", productCatalogHandler.HandleDeleteModifier)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/modifier-group/{modifierGroupId}/modifiers/reorder", productCatalogHandler.HandleReorderModifiers)
	// Image upload and bulk import
	mux.HandleFunc("POST "+cfg.APIBasePath+"/product-catalog/image-upload", productCatalogHandler.HandleImageUpload)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/business/{businessId}/product-menus/bulk-import", productCatalogHandler.HandleBulkMenuImport)

	// Customer endpoints (shop mode)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/customer", customerHandler.HandleCreateCustomer)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/customer/{id}", customerHandler.HandleGetCustomer)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/customer/{id}", customerHandler.HandleUpdateCustomer)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/customer/{id}/sharing", customerHandler.HandleGetSharingSettings)
	mux.HandleFunc("PUT "+cfg.APIBasePath+"/customer/{id}/sharing", customerHandler.HandleUpdateSharingSettings)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/customer/{customerId}/profile", customerHandler.HandleGetCustomerProfile)

	// Arrival event endpoints (shop mode)
	mux.HandleFunc("POST "+cfg.APIBasePath+"/arrival", arrivalHandler.HandleReportArrival)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/location/{locationId}/arrivals", arrivalHandler.HandleGetRecentArrivals)
	mux.HandleFunc("GET "+cfg.APIBasePath+"/location/{locationId}/arrivals/stream", arrivalHandler.HandleArrivalStream)

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
	log.Printf("  Business & Location Management (Shop Mode):")
	log.Printf("   - POST   %s/business - Create business", cfg.APIBasePath)
	log.Printf("   - GET    %s/business/{id} - Get business (with locations)", cfg.APIBasePath)
	log.Printf("   - PUT    %s/business/{id} - Update business", cfg.APIBasePath)
	log.Printf("   - DELETE %s/business/{id} - Delete business (cascade)", cfg.APIBasePath)
	log.Printf("   - POST   %s/business/{id}/locations - Create location", cfg.APIBasePath)
	log.Printf("   - GET    %s/business/{id}/locations - List locations", cfg.APIBasePath)
	log.Printf("   - GET    %s/location/{id} - Get location (with rooms)", cfg.APIBasePath)
	log.Printf("   - PUT    %s/location/{id} - Update location", cfg.APIBasePath)
	log.Printf("   - DELETE %s/location/{id} - Delete location (cascade)", cfg.APIBasePath)
	log.Printf("   - POST   %s/location/{id}/rooms - Create room for location", cfg.APIBasePath)
	log.Printf("  Room Triggers:")
	log.Printf("   - POST   %s/room/{id}/triggers - Create trigger", cfg.APIBasePath)
	log.Printf("   - GET    %s/room/{id}/triggers - List triggers for room", cfg.APIBasePath)
	log.Printf("   - GET    %s/trigger/{id} - Get trigger", cfg.APIBasePath)
	log.Printf("   - PUT    %s/trigger/{id} - Update trigger", cfg.APIBasePath)
	log.Printf("   - DELETE %s/trigger/{id} - Delete trigger", cfg.APIBasePath)
	log.Printf("  Menu System (Shop Mode):")
	log.Printf("   - GET    %s/menu-templates - List all templates", cfg.APIBasePath)
	log.Printf("   - POST   %s/business/{id}/menu-templates - Create template", cfg.APIBasePath)
	log.Printf("   - PUT    %s/menu-template/{id} - Update template", cfg.APIBasePath)
	log.Printf("   - DELETE %s/menu-template/{id} - Delete template", cfg.APIBasePath)
	log.Printf("   - GET    %s/business/{id}/menu-options - List menu options", cfg.APIBasePath)
	log.Printf("   - POST   %s/business/{id}/menu-options/copy - Copy template to menu option", cfg.APIBasePath)
	log.Printf("   - POST   %s/business/{id}/menu-options - Create custom menu option", cfg.APIBasePath)
	log.Printf("   - PUT    %s/menu-option/{id} - Update menu option", cfg.APIBasePath)
	log.Printf("   - DELETE %s/menu-option/{id} - Delete menu option", cfg.APIBasePath)
	log.Printf("   - GET    %s/room/{id}/menu - Get room menu (with options)", cfg.APIBasePath)
	log.Printf("   - PUT    %s/room/{id}/menu/enabled - Enable/disable room menu", cfg.APIBasePath)
	log.Printf("   - PUT    %s/room/{id}/menu/appearance - Update menu appearance", cfg.APIBasePath)
	log.Printf("   - POST   %s/room/{id}/menu/assign - Assign option to room menu", cfg.APIBasePath)
	log.Printf("   - DELETE %s/room/{id}/menu/unassign - Unassign option from room menu", cfg.APIBasePath)
	log.Printf("   - PUT    %s/room/{id}/menu/reorder - Reorder room menu options", cfg.APIBasePath)
	log.Printf("  Product Catalog (Food/Drink Menu System):")
	log.Printf("   - GET    %s/business/{id}/product-menus - List product menus", cfg.APIBasePath)
	log.Printf("   - POST   %s/business/{id}/product-menus/create - Create product menu", cfg.APIBasePath)
	log.Printf("   - GET    %s/product-menu/{id}/full - Get full menu with nested data", cfg.APIBasePath)
	log.Printf("   - PUT    %s/product-menu/{id} - Update product menu", cfg.APIBasePath)
	log.Printf("   - DELETE %s/product-menu/{id} - Delete product menu", cfg.APIBasePath)
	log.Printf("   - POST   %s/product-menu/{id}/categories - Create category", cfg.APIBasePath)
	log.Printf("   - POST   %s/category/{id}/items - Create item", cfg.APIBasePath)
	log.Printf("   - PUT    %s/item/{id}/sold-out - Toggle item sold-out status", cfg.APIBasePath)
	log.Printf("   - POST   %s/item/{id}/modifier-groups - Create modifier group", cfg.APIBasePath)
	log.Printf("   - POST   %s/modifier-group/{id}/modifiers - Create modifier", cfg.APIBasePath)
	log.Printf("   - POST   %s/product-catalog/image-upload - Request image upload URL", cfg.APIBasePath)
	log.Printf("   - POST   %s/business/{id}/product-menus/bulk-import - Bulk import menu", cfg.APIBasePath)
	log.Printf("  Customer Management (Shop Mode):")
	log.Printf("   - POST   %s/customer - Create customer", cfg.APIBasePath)
	log.Printf("   - GET    %s/customer/{id} - Get customer", cfg.APIBasePath)
	log.Printf("   - PUT    %s/customer/{id} - Update customer", cfg.APIBasePath)
	log.Printf("   - GET    %s/customer/{id}/sharing - Get sharing settings", cfg.APIBasePath)
	log.Printf("   - PUT    %s/customer/{id}/sharing - Update sharing settings", cfg.APIBasePath)
	log.Printf("   - GET    %s/customer/{id}/profile?businessId={bid} - Get customer profile for business", cfg.APIBasePath)
	log.Printf("  Arrival Events (Shop Mode):")
	log.Printf("   - POST   %s/arrival - Report arrival event (customer device)", cfg.APIBasePath)
	log.Printf("   - GET    %s/location/{id}/arrivals - Get recent arrivals", cfg.APIBasePath)
	log.Printf("   - GET    %s/location/{id}/arrivals/stream - SSE stream for kiosk", cfg.APIBasePath)
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
