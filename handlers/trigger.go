package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// TriggerHandler holds the database connection and provides HTTP handlers
// for room trigger CRUD operations. Use NewTriggerHandler to create one.
type TriggerHandler struct {
	DB *sql.DB
}

// NewTriggerHandler creates a new TriggerHandler with the given database connection.
func NewTriggerHandler(database *sql.DB) *TriggerHandler {
	return &TriggerHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createTriggerRequest is the JSON body for POST /api/room/{roomId}/triggers
type createTriggerRequest struct {
	TriggerType  string `json:"triggerType"`            // "entry", "exit", "dwell"
	DwellSeconds *int   `json:"dwellSeconds,omitempty"` // required for dwell triggers
	ActionType   string `json:"actionType"`             // "notification", "display_menu", "device_control"
	ActionConfig string `json:"actionConfig"`           // JSON blob with action-specific config
	Enabled      *bool  `json:"enabled,omitempty"`      // defaults to true if not provided
}

// updateTriggerRequest is the JSON body for PUT /api/trigger/{id}
type updateTriggerRequest struct {
	TriggerType  string `json:"triggerType"`
	DwellSeconds *int   `json:"dwellSeconds,omitempty"`
	ActionType   string `json:"actionType"`
	ActionConfig string `json:"actionConfig"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleCreateTrigger creates a new trigger for a room.
// POST /api/room/{roomId}/triggers
// Request body: {"triggerType": "entry", "actionType": "notification", "actionConfig": "...", "enabled": true}
// Response (201): full trigger object
func (h *TriggerHandler) HandleCreateTrigger(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Verify room exists
	_, err := db.GetRoom(h.DB, roomID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}
		log.Printf("❌ Failed to verify room: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to verify room")
		return
	}

	// Parse request body
	var req createTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Trigger create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.TriggerType == "" {
		writeError(w, http.StatusBadRequest, "triggerType is required")
		return
	}
	if req.ActionType == "" {
		writeError(w, http.StatusBadRequest, "actionType is required")
		return
	}
	if req.ActionConfig == "" {
		writeError(w, http.StatusBadRequest, "actionConfig is required")
		return
	}

	// Validate trigger type
	if req.TriggerType != "entry" && req.TriggerType != "exit" && req.TriggerType != "dwell" {
		writeError(w, http.StatusBadRequest, "triggerType must be 'entry', 'exit', or 'dwell'")
		return
	}

	// Validate dwell seconds for dwell triggers
	if req.TriggerType == "dwell" && (req.DwellSeconds == nil || *req.DwellSeconds <= 0) {
		writeError(w, http.StatusBadRequest, "dwellSeconds is required and must be > 0 for dwell triggers")
		return
	}

	// Validate action type
	if req.ActionType != "notification" && req.ActionType != "display_menu" && req.ActionType != "device_control" {
		writeError(w, http.StatusBadRequest, "actionType must be 'notification', 'display_menu', or 'device_control'")
		return
	}

	// Default enabled to true if not provided
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Create the trigger in the database
	trigger, err := db.CreateRoomTrigger(h.DB, roomID, req.TriggerType, req.ActionType, req.ActionConfig, req.DwellSeconds, enabled)
	if err != nil {
		log.Printf("❌ Trigger create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create trigger")
		return
	}

	log.Printf("⚡ Created trigger: %s (%s) for room %s", trigger.ID, trigger.TriggerType, roomID)
	writeJSON(w, http.StatusCreated, trigger)
}

// HandleListTriggers returns all triggers for a room.
// GET /api/room/{roomId}/triggers
// Response (200): array of trigger objects
func (h *TriggerHandler) HandleListTriggers(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Fetch triggers
	triggers, err := db.ListTriggersByRoom(h.DB, roomID)
	if err != nil {
		log.Printf("❌ Trigger list failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list triggers")
		return
	}

	// Return empty array instead of null for no triggers
	if triggers == nil {
		triggers = []db.RoomTrigger{}
	}

	writeJSON(w, http.StatusOK, triggers)
}

// HandleGetTrigger returns a single trigger by ID.
// GET /api/trigger/{id}
// Response (200): trigger object
func (h *TriggerHandler) HandleGetTrigger(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Trigger ID is required")
		return
	}

	// Fetch the trigger
	trigger, err := db.GetRoomTrigger(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Trigger not found")
			return
		}
		log.Printf("❌ Trigger get failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get trigger")
		return
	}

	writeJSON(w, http.StatusOK, trigger)
}

// HandleUpdateTrigger updates a trigger's configuration.
// PUT /api/trigger/{id}
// Request body: {"triggerType": "dwell", "dwellSeconds": 20, "actionType": "notification", "actionConfig": "...", "enabled": true}
// Response (200): updated trigger object
func (h *TriggerHandler) HandleUpdateTrigger(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Trigger ID is required")
		return
	}

	// Parse request body
	var req updateTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Trigger update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.TriggerType == "" {
		writeError(w, http.StatusBadRequest, "triggerType is required")
		return
	}
	if req.ActionType == "" {
		writeError(w, http.StatusBadRequest, "actionType is required")
		return
	}
	if req.ActionConfig == "" {
		writeError(w, http.StatusBadRequest, "actionConfig is required")
		return
	}

	// Validate trigger type
	if req.TriggerType != "entry" && req.TriggerType != "exit" && req.TriggerType != "dwell" {
		writeError(w, http.StatusBadRequest, "triggerType must be 'entry', 'exit', or 'dwell'")
		return
	}

	// Validate dwell seconds for dwell triggers
	if req.TriggerType == "dwell" && (req.DwellSeconds == nil || *req.DwellSeconds <= 0) {
		writeError(w, http.StatusBadRequest, "dwellSeconds is required and must be > 0 for dwell triggers")
		return
	}

	// Validate action type
	if req.ActionType != "notification" && req.ActionType != "display_menu" && req.ActionType != "device_control" {
		writeError(w, http.StatusBadRequest, "actionType must be 'notification', 'display_menu', or 'device_control'")
		return
	}

	// Default enabled to true if not provided
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Update the trigger
	trigger, err := db.UpdateRoomTrigger(h.DB, id, req.TriggerType, req.ActionType, req.ActionConfig, req.DwellSeconds, enabled)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Trigger not found")
			return
		}
		log.Printf("❌ Trigger update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update trigger")
		return
	}

	log.Printf("⚡ Updated trigger: %s (%s)", trigger.ID, trigger.TriggerType)
	writeJSON(w, http.StatusOK, trigger)
}

// HandleDeleteTrigger deletes a trigger.
// DELETE /api/trigger/{id}
// Response (204): no content on success
func (h *TriggerHandler) HandleDeleteTrigger(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Trigger ID is required")
		return
	}

	// Delete the trigger
	err := db.DeleteRoomTrigger(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Trigger not found")
			return
		}
		log.Printf("❌ Trigger delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete trigger")
		return
	}

	log.Printf("⚡ Deleted trigger: %s", id)
	w.WriteHeader(http.StatusNoContent)
}
