package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// RoomHandler holds the database connection and provides HTTP handlers
// for room CRUD operations. Use NewRoomHandler to create one.
type RoomHandler struct {
	DB *sql.DB
}

// NewRoomHandler creates a new RoomHandler with the given database connection.
func NewRoomHandler(database *sql.DB) *RoomHandler {
	return &RoomHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createRoomRequest is the JSON body for POST /api/profile/{profileId}/rooms
type createRoomRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// updateRoomRequest is the JSON body for PUT /api/room/{id}
type updateRoomRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// updateRoomBeaconRequest is the JSON body for PUT /api/room/{id}/beacon
type updateRoomBeaconRequest struct {
	UUID  string `json:"uuid"`
	Major int    `json:"major"`
	Minor int    `json:"minor"`
}

// roomDetailResponse is the enriched room response that includes
// its assigned devices. Used by GET /api/room/{id}.
type roomDetailResponse struct {
	ID          string      `json:"id"`
	ProfileID   string      `json:"profileId"`
	Name        string      `json:"name"`
	Icon        string      `json:"icon"`
	BeaconUUID  *string     `json:"beaconUuid,omitempty"`
	BeaconMajor *int        `json:"beaconMajor,omitempty"`
	BeaconMinor *int        `json:"beaconMinor,omitempty"`
	Devices     []db.Device `json:"devices"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleCreateRoom creates a new room under the given profile.
// POST /api/profile/{profileId}/rooms
// Request body: {"name": "Living Room", "icon": "sofa"}
// Response (201): room object
func (h *RoomHandler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileId")
	if profileID == "" {
		writeError(w, http.StatusBadRequest, "Profile ID is required")
		return
	}

	// Parse request body
	var req createRoomRequest
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

	// Default icon if not provided
	icon := req.Icon
	if icon == "" {
		icon = "house"
	}

	// Verify the profile exists before creating a room under it
	_, err := db.GetProfile(h.DB, profileID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Profile not found")
			return
		}
		log.Printf("❌ Room create: failed to verify profile: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to verify profile")
		return
	}

	// Create the room
	room, err := db.CreateRoom(h.DB, profileID, req.Name, icon)
	if err != nil {
		log.Printf("❌ Room create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create room")
		return
	}

	log.Printf("🏠 Created room: %s (id: %s) for profile %s", room.Name, room.ID, profileID)
	writeJSON(w, http.StatusCreated, room)
}

// HandleListRooms returns all rooms for the given profile.
// GET /api/profile/{profileId}/rooms
// Response (200): array of room objects
func (h *RoomHandler) HandleListRooms(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileId")
	if profileID == "" {
		writeError(w, http.StatusBadRequest, "Profile ID is required")
		return
	}

	rooms, err := db.ListRoomsByProfile(h.DB, profileID)
	if err != nil {
		log.Printf("❌ Room list failed for profile %s: %v", profileID, err)
		writeError(w, http.StatusInternalServerError, "Failed to list rooms")
		return
	}

	// Return empty array instead of null
	if rooms == nil {
		rooms = []db.Room{}
	}

	writeJSON(w, http.StatusOK, rooms)
}

// HandleGetRoom returns a single room by ID, enriched with its assigned devices.
// GET /api/room/{id}
// Response (200): room object with devices[]
func (h *RoomHandler) HandleGetRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Fetch the room
	room, err := db.GetRoom(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}
		log.Printf("❌ Room get failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get room")
		return
	}

	// Fetch devices assigned to this room
	devices, err := db.ListDevicesByRoom(h.DB, id)
	if err != nil {
		log.Printf("❌ Failed to list devices for room %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "Failed to get room devices")
		return
	}

	// Return nil slices as empty arrays in JSON
	if devices == nil {
		devices = []db.Device{}
	}

	// Build enriched response
	resp := roomDetailResponse{
		ID:          room.ID,
		ProfileID:   room.ProfileID,
		Name:        room.Name,
		Icon:        room.Icon,
		BeaconUUID:  room.BeaconUUID,
		BeaconMajor: room.BeaconMajor,
		BeaconMinor: room.BeaconMinor,
		Devices:     devices,
		CreatedAt:   room.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   room.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleUpdateRoom updates a room's name and icon.
// PUT /api/room/{id}
// Request body: {"name": "Home Office", "icon": "desktopcomputer"}
// Response (200): updated room object
func (h *RoomHandler) HandleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Parse request body
	var req updateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Room update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Icon == "" {
		writeError(w, http.StatusBadRequest, "Icon is required")
		return
	}

	// Update the room
	room, err := db.UpdateRoom(h.DB, id, req.Name, req.Icon)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}
		log.Printf("❌ Room update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update room")
		return
	}

	log.Printf("🏠 Updated room: %s (id: %s)", room.Name, room.ID)
	writeJSON(w, http.StatusOK, room)
}

// HandleUpdateRoomBeacon sets the iBeacon configuration for a room.
// This links the room to a physical BLE beacon for proximity detection.
// PUT /api/room/{id}/beacon
// Request body: {"uuid": "E2C56DB5-...", "major": 1, "minor": 100}
// Response (200): updated room object with beacon fields
func (h *RoomHandler) HandleUpdateRoomBeacon(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Parse request body
	var req updateRoomBeaconRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Room beacon update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.UUID == "" {
		writeError(w, http.StatusBadRequest, "Beacon UUID is required")
		return
	}

	// Update beacon configuration
	room, err := db.UpdateRoomBeacon(h.DB, id, req.UUID, req.Major, req.Minor)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}
		log.Printf("❌ Room beacon update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update room beacon")
		return
	}

	log.Printf("📡 Updated beacon for room %s: uuid=%s major=%d minor=%d", room.Name, req.UUID, req.Major, req.Minor)
	writeJSON(w, http.StatusOK, room)
}

// HandleDeleteRoom removes a room. Devices assigned to this room will have
// their room_id set to NULL (unassigned) via the ON DELETE SET NULL constraint.
// DELETE /api/room/{id}
// Response (204): no content
func (h *RoomHandler) HandleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	if err := db.DeleteRoom(h.DB, id); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}
		log.Printf("❌ Room delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete room")
		return
	}

	log.Printf("🏠 Deleted room: %s", id)
	w.WriteHeader(http.StatusNoContent)
}
