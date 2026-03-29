package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// DeviceHandler holds the database connection and provides HTTP handlers
// for device CRUD operations. Use NewDeviceHandler to create one.
type DeviceHandler struct {
	DB *sql.DB
}

// NewDeviceHandler creates a new DeviceHandler with the given database connection.
func NewDeviceHandler(database *sql.DB) *DeviceHandler {
	return &DeviceHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createDeviceRequest is the JSON body for POST /api/profile/{profileId}/devices
type createDeviceRequest struct {
	Name       string  `json:"name"`
	DeviceType string  `json:"deviceType"`
	ExternalID *string `json:"externalId,omitempty"`
	Model      *string `json:"model,omitempty"`
}

// updateDeviceRequest is the JSON body for PUT /api/device/{id}
type updateDeviceRequest struct {
	Name string `json:"name"`
}

// assignDeviceRequest is the JSON body for PUT /api/device/{id}/assign
type assignDeviceRequest struct {
	RoomID string `json:"roomId"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleCreateDevice registers a new device under the given profile.
// The device starts unassigned (no room).
// POST /api/profile/{profileId}/devices
// Request body: {"name": "Desk Lamp", "deviceType": "govee_light", "externalId": "...", "model": "H6160"}
// Response (201): device object
func (h *DeviceHandler) HandleCreateDevice(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileId")
	if profileID == "" {
		writeError(w, http.StatusBadRequest, "Profile ID is required")
		return
	}

	// Parse request body
	var req createDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Device create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.DeviceType == "" {
		writeError(w, http.StatusBadRequest, "Device type is required")
		return
	}

	// Verify the profile exists before registering a device under it
	_, err := db.GetProfile(h.DB, profileID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Profile not found")
			return
		}
		log.Printf("❌ Device create: failed to verify profile: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to verify profile")
		return
	}

	// Create the device
	device, err := db.CreateDevice(h.DB, profileID, req.Name, req.DeviceType, req.ExternalID, req.Model)
	if err != nil {
		log.Printf("❌ Device create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create device")
		return
	}

	log.Printf("📱 Created device: %s (id: %s, type: %s) for profile %s", device.Name, device.ID, device.DeviceType, profileID)
	writeJSON(w, http.StatusCreated, device)
}

// HandleListDevices returns all devices for the given profile.
// GET /api/profile/{profileId}/devices
// Response (200): array of device objects
func (h *DeviceHandler) HandleListDevices(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileId")
	if profileID == "" {
		writeError(w, http.StatusBadRequest, "Profile ID is required")
		return
	}

	devices, err := db.ListDevicesByProfile(h.DB, profileID)
	if err != nil {
		log.Printf("❌ Device list failed for profile %s: %v", profileID, err)
		writeError(w, http.StatusInternalServerError, "Failed to list devices")
		return
	}

	// Return empty array instead of null
	if devices == nil {
		devices = []db.Device{}
	}

	writeJSON(w, http.StatusOK, devices)
}

// HandleGetDevice returns a single device by ID.
// GET /api/device/{id}
// Response (200): device object
func (h *DeviceHandler) HandleGetDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Device ID is required")
		return
	}

	device, err := db.GetDevice(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Device not found")
			return
		}
		log.Printf("❌ Device get failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get device")
		return
	}

	writeJSON(w, http.StatusOK, device)
}

// HandleUpdateDevice updates a device's friendly name.
// PUT /api/device/{id}
// Request body: {"name": "New Lamp Name"}
// Response (200): updated device object
func (h *DeviceHandler) HandleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Device ID is required")
		return
	}

	// Parse request body
	var req updateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Device update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Update the device
	device, err := db.UpdateDevice(h.DB, id, req.Name)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Device not found")
			return
		}
		log.Printf("❌ Device update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update device")
		return
	}

	log.Printf("📱 Updated device: %s (id: %s)", device.Name, device.ID)
	writeJSON(w, http.StatusOK, device)
}

// HandleAssignDevice assigns a device to a room.
// PUT /api/device/{id}/assign
// Request body: {"roomId": "room-uuid-here"}
// Response (200): updated device object with roomId set
func (h *DeviceHandler) HandleAssignDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Device ID is required")
		return
	}

	// Parse request body
	var req assignDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Device assign: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Verify the room exists before assigning
	_, err := db.GetRoom(h.DB, req.RoomID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}
		log.Printf("❌ Device assign: failed to verify room: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to verify room")
		return
	}

	// Assign the device
	device, err := db.AssignDeviceToRoom(h.DB, id, req.RoomID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Device not found")
			return
		}
		log.Printf("❌ Device assign failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to assign device")
		return
	}

	log.Printf("📱 Assigned device %s to room %s", device.Name, req.RoomID)
	writeJSON(w, http.StatusOK, device)
}

// HandleUnassignDevice removes a device from its room (sets room_id to NULL).
// PUT /api/device/{id}/unassign
// Response (200): updated device object with roomId removed
func (h *DeviceHandler) HandleUnassignDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Device ID is required")
		return
	}

	device, err := db.UnassignDevice(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Device not found")
			return
		}
		log.Printf("❌ Device unassign failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to unassign device")
		return
	}

	log.Printf("📱 Unassigned device %s from its room", device.Name)
	writeJSON(w, http.StatusOK, device)
}

// HandleDeleteDevice permanently removes a device.
// DELETE /api/device/{id}
// Response (204): no content
func (h *DeviceHandler) HandleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Device ID is required")
		return
	}

	if err := db.DeleteDevice(h.DB, id); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Device not found")
			return
		}
		log.Printf("❌ Device delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete device")
		return
	}

	log.Printf("📱 Deleted device: %s", id)
	w.WriteHeader(http.StatusNoContent)
}
