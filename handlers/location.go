package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// LocationHandler holds the database connection and provides HTTP handlers
// for location CRUD operations. Use NewLocationHandler to create one.
type LocationHandler struct {
	DB *sql.DB
}

// NewLocationHandler creates a new LocationHandler with the given database connection.
func NewLocationHandler(database *sql.DB) *LocationHandler {
	return &LocationHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createLocationRequest is the JSON body for POST /api/business/{businessId}/locations
type createLocationRequest struct {
	Name    string  `json:"name"`
	Address *string `json:"address,omitempty"`
}

// addressResponse represents the structured address in the API response.
type addressResponse struct {
	Street     string   `json:"street"`
	Street2    *string  `json:"street2,omitempty"`
	City       string   `json:"city"`
	State      string   `json:"state"`
	PostalCode string   `json:"postalCode"`
	Country    string   `json:"country"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
}

// locationDetailResponse is the enriched location response that includes
// associated rooms and computed isOpenNow field. Used by GET /api/location/{id}.
type locationDetailResponse struct {
	ID           string           `json:"id"`
	BusinessID   string           `json:"businessId"`
	Name         string           `json:"name"`
	Address      *addressResponse `json:"address,omitempty"`
	Timezone     *string          `json:"timezone,omitempty"`
	OpeningHours interface{}      `json:"openingHours,omitempty"`  // decoded JSON or nil
	SpecialHours interface{}      `json:"specialHours,omitempty"`  // decoded JSON or nil
	IsOpenNow    bool             `json:"isOpenNow"`
	Rooms        []db.Room        `json:"rooms"`
	CreatedAt    string           `json:"createdAt"`
	UpdatedAt    string           `json:"updatedAt"`
}

// addressRequest represents the structured address in update requests.
type addressRequest struct {
	Street     string   `json:"street"`
	Street2    *string  `json:"street2,omitempty"`
	City       string   `json:"city"`
	State      string   `json:"state"`
	PostalCode string   `json:"postalCode"`
	Country    string   `json:"country"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
}

// updateLocationRequest is the JSON body for PUT /api/location/{id}
type updateLocationRequest struct {
	Name         string          `json:"name"`
	Address      *addressRequest `json:"address,omitempty"`
	Timezone     *string         `json:"timezone,omitempty"`
	OpeningHours interface{}     `json:"openingHours,omitempty"`
	SpecialHours interface{}     `json:"specialHours,omitempty"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleCreateLocation creates a new location under a business.
// POST /api/business/{businessId}/locations
// Request body: {"name": "Downtown Store", "address": "123 Main St"}
// Response (201): full location object
func (h *LocationHandler) HandleCreateLocation(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Verify business exists
	_, err := db.GetBusiness(h.DB, businessID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Business not found")
			return
		}
		log.Printf("❌ Failed to verify business: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to verify business")
		return
	}

	// Parse request body
	var req createLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Location create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Create the location in the database
	location, err := db.CreateLocation(h.DB, businessID, req.Name, req.Address)
	if err != nil {
		log.Printf("❌ Location create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create location")
		return
	}

	log.Printf("📍 Created location: %s (id: %s) under business %s", location.Name, location.ID, businessID)
	writeJSON(w, http.StatusCreated, location)
}

// HandleListLocations returns all locations for a business.
// GET /api/business/{businessId}/locations
// Response (200): array of location objects
func (h *LocationHandler) HandleListLocations(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Fetch locations
	locations, err := db.ListLocationsByBusiness(h.DB, businessID)
	if err != nil {
		log.Printf("❌ Location list failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list locations")
		return
	}

	// Return empty array instead of null for no locations
	if locations == nil {
		locations = []db.Location{}
	}

	writeJSON(w, http.StatusOK, locations)
}

// HandleGetLocation returns a single location by ID, enriched with its rooms.
// GET /api/location/{id}
// Response (200): location with rooms[]
func (h *LocationHandler) HandleGetLocation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Location ID is required")
		return
	}

	// Fetch the location
	location, err := db.GetLocation(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Location not found")
			return
		}
		log.Printf("❌ Location get failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get location")
		return
	}

	// Fetch associated rooms for the enriched response
	rooms, err := db.ListRoomsByLocation(h.DB, id)
	if err != nil {
		log.Printf("❌ Failed to list rooms for location %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "Failed to get location rooms")
		return
	}

	// Return nil slices as empty arrays in JSON
	if rooms == nil {
		rooms = []db.Room{}
	}

	// Build structured address response if address fields are present
	var addressResp *addressResponse
	if location.Street != nil && location.City != nil && location.State != nil && location.PostalCode != nil && location.Country != nil {
		addressResp = &addressResponse{
			Street:     *location.Street,
			Street2:    location.Street2,
			City:       *location.City,
			State:      *location.State,
			PostalCode: *location.PostalCode,
			Country:    *location.Country,
			Latitude:   location.Latitude,
			Longitude:  location.Longitude,
		}
	}

	// Decode opening hours from JSON if present
	var openingHours interface{}
	if location.OpeningHours != nil && *location.OpeningHours != "" {
		var hours db.OpeningHours
		if err := json.Unmarshal([]byte(*location.OpeningHours), &hours); err == nil {
			openingHours = hours
		}
	}

	// Decode special hours from JSON if present
	var specialHours interface{}
	if location.SpecialHours != nil && *location.SpecialHours != "" {
		var hours []db.SpecialHours
		if err := json.Unmarshal([]byte(*location.SpecialHours), &hours); err == nil {
			specialHours = hours
		}
	}

	// Compute isOpenNow
	isOpenNow := db.IsOpenNow(location)

	// Build enriched response
	resp := locationDetailResponse{
		ID:           location.ID,
		BusinessID:   location.BusinessID,
		Name:         location.Name,
		Address:      addressResp,
		Timezone:     location.Timezone,
		OpeningHours: openingHours,
		SpecialHours: specialHours,
		IsOpenNow:    isOpenNow,
		Rooms:        rooms,
		CreatedAt:    location.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    location.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleUpdateLocation updates a location's name, structured address, timezone, and hours.
// PUT /api/location/{id}
// Request body: {"name": "...", "address": {...}, "timezone": "...", "openingHours": {...}, "specialHours": [...]}
// Response (200): updated location detail with isOpenNow computed
func (h *LocationHandler) HandleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Location ID is required")
		return
	}

	// Parse request body
	var req updateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Location update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Extract structured address fields
	var street, street2, city, state, postalCode, country *string
	var latitude, longitude *float64
	if req.Address != nil {
		// Validate required address fields
		if req.Address.Street == "" || req.Address.City == "" || req.Address.State == "" || req.Address.PostalCode == "" || req.Address.Country == "" {
			writeError(w, http.StatusBadRequest, "Address requires street, city, state, postalCode, and country")
			return
		}
		street = &req.Address.Street
		street2 = req.Address.Street2
		city = &req.Address.City
		state = &req.Address.State
		postalCode = &req.Address.PostalCode
		country = &req.Address.Country
		latitude = req.Address.Latitude
		longitude = req.Address.Longitude

		// Validate latitude/longitude ranges if provided
		if latitude != nil && (*latitude < -90 || *latitude > 90) {
			writeError(w, http.StatusBadRequest, "Latitude must be between -90 and 90")
			return
		}
		if longitude != nil && (*longitude < -180 || *longitude > 180) {
			writeError(w, http.StatusBadRequest, "Longitude must be between -180 and 180")
			return
		}
	}

	// Encode opening hours to JSON if provided
	var openingHoursJSON *string
	if req.OpeningHours != nil {
		hoursBytes, err := json.Marshal(req.OpeningHours)
		if err != nil {
			log.Printf("❌ Failed to encode opening hours: %v", err)
			writeError(w, http.StatusBadRequest, "Invalid opening hours format")
			return
		}
		hoursStr := string(hoursBytes)
		openingHoursJSON = &hoursStr
	}

	// Encode special hours to JSON if provided
	var specialHoursJSON *string
	if req.SpecialHours != nil {
		hoursBytes, err := json.Marshal(req.SpecialHours)
		if err != nil {
			log.Printf("❌ Failed to encode special hours: %v", err)
			writeError(w, http.StatusBadRequest, "Invalid special hours format")
			return
		}
		hoursStr := string(hoursBytes)
		specialHoursJSON = &hoursStr
	}

	// Update the location
	location, err := db.UpdateLocation(h.DB, id, req.Name, street, street2, city, state, postalCode, country, latitude, longitude, req.Timezone, openingHoursJSON, specialHoursJSON)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Location not found")
			return
		}
		log.Printf("❌ Location update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update location")
		return
	}

	// Fetch associated rooms for the enriched response
	rooms, err := db.ListRoomsByLocation(h.DB, id)
	if err != nil {
		log.Printf("❌ Failed to list rooms for location %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "Failed to get location rooms")
		return
	}

	// Return nil slices as empty arrays in JSON
	if rooms == nil {
		rooms = []db.Room{}
	}

	// Build structured address response if address fields are present
	var addressResp *addressResponse
	if location.Street != nil && location.City != nil && location.State != nil && location.PostalCode != nil && location.Country != nil {
		addressResp = &addressResponse{
			Street:     *location.Street,
			Street2:    location.Street2,
			City:       *location.City,
			State:      *location.State,
			PostalCode: *location.PostalCode,
			Country:    *location.Country,
			Latitude:   location.Latitude,
			Longitude:  location.Longitude,
		}
	}

	// Decode opening hours from JSON if present
	var openingHours interface{}
	if location.OpeningHours != nil && *location.OpeningHours != "" {
		var hours db.OpeningHours
		if err := json.Unmarshal([]byte(*location.OpeningHours), &hours); err == nil {
			openingHours = hours
		}
	}

	// Decode special hours from JSON if present
	var specialHours interface{}
	if location.SpecialHours != nil && *location.SpecialHours != "" {
		var hours []db.SpecialHours
		if err := json.Unmarshal([]byte(*location.SpecialHours), &hours); err == nil {
			specialHours = hours
		}
	}

	// Compute isOpenNow
	isOpenNow := db.IsOpenNow(location)

	// Build enriched response
	resp := locationDetailResponse{
		ID:           location.ID,
		BusinessID:   location.BusinessID,
		Name:         location.Name,
		Address:      addressResp,
		Timezone:     location.Timezone,
		OpeningHours: openingHours,
		SpecialHours: specialHours,
		IsOpenNow:    isOpenNow,
		Rooms:        rooms,
		CreatedAt:    location.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    location.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	log.Printf("📍 Updated location: %s (id: %s)", location.Name, location.ID)
	writeJSON(w, http.StatusOK, resp)
}

// HandleDeleteLocation deletes a location and all associated rooms (cascade).
// DELETE /api/location/{id}
// Response (204): no content on success
func (h *LocationHandler) HandleDeleteLocation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Location ID is required")
		return
	}

	// Delete the location
	err := db.DeleteLocation(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Location not found")
			return
		}
		log.Printf("❌ Location delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete location")
		return
	}

	log.Printf("📍 Deleted location: %s", id)
	w.WriteHeader(http.StatusNoContent)
}

// HandleCreateRoomForLocation creates a new room under a location (shop mode).
// POST /api/location/{locationId}/rooms
// Request body: {"name": "Main Hall", "icon": "storefront"}
// Response (201): full room object
func (h *LocationHandler) HandleCreateRoomForLocation(w http.ResponseWriter, r *http.Request) {
	locationID := r.PathValue("locationId")
	if locationID == "" {
		writeError(w, http.StatusBadRequest, "Location ID is required")
		return
	}

	// Verify location exists
	_, err := db.GetLocation(h.DB, locationID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Location not found")
			return
		}
		log.Printf("❌ Failed to verify location: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to verify location")
		return
	}

	// Parse request body (reuse roomHandler's type if needed, or define inline)
	var req struct {
		Name string `json:"name"`
		Icon string `json:"icon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Room create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Icon == "" {
		req.Icon = "house" // default icon
	}

	// Create the room in the database
	room, err := db.CreateRoomForLocation(h.DB, locationID, req.Name, req.Icon)
	if err != nil {
		log.Printf("❌ Room create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create room")
		return
	}

	log.Printf("🏠 Created room: %s (id: %s) under location %s", room.Name, room.ID, locationID)
	writeJSON(w, http.StatusCreated, room)
}
