package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// ProfileHandler holds the database connection and provides HTTP handlers
// for profile CRUD operations. Use NewProfileHandler to create one.
type ProfileHandler struct {
	DB *sql.DB
}

// NewProfileHandler creates a new ProfileHandler with the given database connection.
func NewProfileHandler(database *sql.DB) *ProfileHandler {
	return &ProfileHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createProfileRequest is the JSON body for POST /api/profile
type createProfileRequest struct {
	Name string `json:"name"`
}

// profileDetailResponse is the enriched profile response that includes
// associated rooms and devices. Used by GET /api/profile/{id}.
type profileDetailResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Rooms     []db.Room   `json:"rooms"`
	Devices   []db.Device `json:"devices"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// updateProfileRequest is the JSON body for PUT /api/profile/{id}
type updateProfileRequest struct {
	Name string `json:"name"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleCreateProfile creates a new user profile.
// POST /api/profile
// Request body: {"name": "Shakur"}
// Response (201): full profile object
func (h *ProfileHandler) HandleCreateProfile(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req createProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Profile create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Create the profile in the database
	profile, err := db.CreateProfile(h.DB, req.Name)
	if err != nil {
		log.Printf("❌ Profile create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create profile")
		return
	}

	log.Printf("👤 Created profile: %s (id: %s)", profile.Name, profile.ID)
	writeJSON(w, http.StatusCreated, profile)
}

// HandleGetProfile returns a single profile by ID, enriched with its rooms and devices.
// GET /api/profile/{id}
// Response (200): profile with rooms[] and devices[]
func (h *ProfileHandler) HandleGetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Profile ID is required")
		return
	}

	// Fetch the profile
	profile, err := db.GetProfile(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Profile not found")
			return
		}
		log.Printf("❌ Profile get failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get profile")
		return
	}

	// Fetch associated rooms and devices for the enriched response
	rooms, err := db.ListRoomsByProfile(h.DB, id)
	if err != nil {
		log.Printf("❌ Failed to list rooms for profile %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "Failed to get profile rooms")
		return
	}

	devices, err := db.ListDevicesByProfile(h.DB, id)
	if err != nil {
		log.Printf("❌ Failed to list devices for profile %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "Failed to get profile devices")
		return
	}

	// Return nil slices as empty arrays in JSON
	if rooms == nil {
		rooms = []db.Room{}
	}
	if devices == nil {
		devices = []db.Device{}
	}

	// Build enriched response
	resp := profileDetailResponse{
		ID:        profile.ID,
		Name:      profile.Name,
		Rooms:     rooms,
		Devices:   devices,
		CreatedAt: profile.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: profile.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleListProfiles returns all profiles. Useful for development and debugging.
// GET /api/profiles
// Response (200): array of profile objects
func (h *ProfileHandler) HandleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := db.ListProfiles(h.DB)
	if err != nil {
		log.Printf("❌ Profile list failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list profiles")
		return
	}

	// Return empty array instead of null
	if profiles == nil {
		profiles = []db.Profile{}
	}

	writeJSON(w, http.StatusOK, profiles)
}

// HandleUpdateProfile updates a profile's name.
// PUT /api/profile/{id}
// Request body: {"name": "New Name"}
// Response (200): updated profile object
func (h *ProfileHandler) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Profile ID is required")
		return
	}

	// Parse request body
	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Profile update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Update the profile
	profile, err := db.UpdateProfile(h.DB, id, req.Name)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Profile not found")
			return
		}
		log.Printf("❌ Profile update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	log.Printf("👤 Updated profile: %s (id: %s)", profile.Name, profile.ID)
	writeJSON(w, http.StatusOK, profile)
}

// HandleDeleteProfile removes a profile and all associated rooms/devices (cascade).
// DELETE /api/profile/{id}
// Response (204): no content
func (h *ProfileHandler) HandleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Profile ID is required")
		return
	}

	if err := db.DeleteProfile(h.DB, id); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Profile not found")
			return
		}
		log.Printf("❌ Profile delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete profile")
		return
	}

	log.Printf("👤 Deleted profile: %s", id)
	w.WriteHeader(http.StatusNoContent)
}
