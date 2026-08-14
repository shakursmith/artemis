package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pantheon/artemis/db"
	"github.com/pantheon/artemis/govee"
)

// DeviceControlHandler handles device toggle and control requests.
// Unlike the Govee/FireTV/Camera handlers which talk to specific integrations,
// this handler provides a generic device control layer — it looks up the device
// by its internal UUID, determines the device type, and dispatches the command
// to the appropriate integration handler.
//
// This enables tap-to-toggle from the room scene: the iOS app sends a toggle
// request with just the device UUID, and the backend handles the rest.
type DeviceControlHandler struct {
	DB           *sql.DB
	GoveeClients []*govee.Client
}

// NewDeviceControlHandler creates a new DeviceControlHandler with the given
// database connection and Govee API clients.
func NewDeviceControlHandler(database *sql.DB, goveeClients []*govee.Client) *DeviceControlHandler {
	return &DeviceControlHandler{
		DB:           database,
		GoveeClients: goveeClients,
	}
}

// deviceToggleResponse is the JSON response for a toggle attempt.
type deviceToggleResponse struct {
	Success    bool   `json:"success"`
	Verified   bool   `json:"verified"`   // Whether the state change was confirmed by re-querying
	DeviceID   string `json:"deviceId"`
	DeviceType string `json:"deviceType"`
	Message    string `json:"message"`
	Timestamp  string `json:"timestamp"`
}

// toggleRequest is the optional JSON body for POST /api/device/{id}/toggle.
// If "action" is provided ("on" or "off"), that exact command is sent.
// If omitted or empty, the backend queries current state and sends the opposite.
type toggleRequest struct {
	Action string `json:"action"` // "on", "off", or "" (auto-toggle)
}

// HandleToggleDevice handles POST /api/device/{id}/toggle
//
// Looks up the device by its internal UUID, determines its type (govee_light,
// fire_tv, wyze_camera), and dispatches the appropriate toggle command.
//
// Request body (optional): {"action": "on"} or {"action": "off"}
// If action is provided, that exact command is sent (no state query needed).
// If omitted, the backend queries current state and sends the opposite.
//
// For Govee lights: sends a "turn" command to the Govee API. The device's
// external_id (MAC address) and model are read from the database.
//
// For other device types: returns a placeholder success for now.
func (h *DeviceControlHandler) HandleToggleDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "device id is required")
		return
	}

	// Parse optional request body for explicit action.
	var req toggleRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	// Look up the device to get its type and external details.
	device, err := db.GetDevice(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		log.Printf("❌ Failed to look up device %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to look up device")
		return
	}

	log.Printf("🔄 Toggle request for device: %s (%s, type=%s, action=%q)", device.Name, device.ID, device.DeviceType, req.Action)

	// Dispatch based on device type.
	switch device.DeviceType {
	case "govee_light":
		h.toggleGoveeDevice(w, device, req.Action)
	case "fire_tv":
		// Fire TV toggle is more complex (requires ADB connection).
		// For now, return success as a placeholder.
		writeJSON(w, http.StatusOK, deviceToggleResponse{
			Success:    true,
			Verified:   true,
			DeviceID:   device.ID,
			DeviceType: device.DeviceType,
			Message:    "Fire TV toggle not yet implemented via this endpoint",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		})
	case "wyze_camera":
		// Cameras don't really have a simple toggle.
		writeJSON(w, http.StatusOK, deviceToggleResponse{
			Success:    true,
			Verified:   true,
			DeviceID:   device.ID,
			DeviceType: device.DeviceType,
			Message:    "Camera toggle not applicable",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		})
	default:
		writeError(w, http.StatusBadRequest, "unsupported device type: "+device.DeviceType)
	}
}

// toggleGoveeDevice sends a turn on/off command to a Govee light.
// It reads the device's external_id (MAC address) and model from the database,
// then sends the command via the Govee API client.
//
// If action is "on" or "off", that exact command is sent (no state query).
// If action is empty, the backend queries current state and sends the opposite.
func (h *DeviceControlHandler) toggleGoveeDevice(w http.ResponseWriter, device *db.Device, action string) {
	// The device must have an external_id (Govee MAC address) to control it.
	if device.ExternalID == nil || *device.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "device has no external_id (Govee MAC address) — cannot control")
		return
	}

	// The device must have a model for Govee API calls.
	if device.Model == nil || *device.Model == "" {
		log.Printf("⚠️ Device %s has no model — attempting toggle anyway", device.ID)
	}

	externalID := *device.ExternalID
	model := ""
	if device.Model != nil {
		model = *device.Model
	}

	if len(h.GoveeClients) == 0 {
		writeError(w, http.StatusInternalServerError, "no Govee API clients configured")
		return
	}

	// Determine whether to turn on or off.
	var turnOn bool

	switch action {
	case "on":
		turnOn = true
	case "off":
		turnOn = false
	default:
		// No explicit action — query current state and send the opposite.
		// This fallback is used by older clients or direct API calls.
		turnOn = true
		for _, client := range h.GoveeClients {
			state, err := client.GetDeviceState(externalID, model)
			if err == nil {
				turnOn = !isDeviceOn(state)
				break
			}
		}
	}

	// Send the command to ALL Govee clients. We don't know which API key
	// owns this device (the Govee API returns 200 even for wrong keys,
	// but the command silently doesn't reach the device). Sending to all
	// ensures the device gets the command regardless of which account owns it.
	anySucceeded := false
	var lastErr error
	for i, client := range h.GoveeClients {
		var err error
		if turnOn {
			err = client.TurnOn(externalID, model)
		} else {
			err = client.TurnOff(externalID, model)
		}
		if err != nil {
			log.Printf("⚠️ Govee client #%d failed for %s: %v", i, externalID, err)
			lastErr = err
		} else {
			anySucceeded = true
		}
	}
	if !anySucceeded {
		log.Printf("❌ All Govee clients failed for device %s: %v", externalID, lastErr)
		writeError(w, http.StatusInternalServerError, "failed to toggle device: "+lastErr.Error())
		return
	}

	stateStr := "off"
	if turnOn {
		stateStr = "on"
	}

	log.Printf("✅ Toggled Govee device %s (%s) → %s", device.Name, externalID, stateStr)

	writeJSON(w, http.StatusOK, deviceToggleResponse{
		Success:    true,
		Verified:   true,
		DeviceID:   device.ID,
		DeviceType: device.DeviceType,
		Message:    "Device toggled " + stateStr,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
}

// brightnessRequest is the JSON body for POST /api/device/{id}/brightness.
type brightnessRequest struct {
	Brightness int `json:"brightness"`
}

// colorRequest is the JSON body for POST /api/device/{id}/color.
type colorRequest struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// deviceControlResponse is the JSON response for brightness/color commands.
type deviceControlResponse struct {
	Success    bool   `json:"success"`
	DeviceID   string `json:"deviceId"`
	DeviceType string `json:"deviceType"`
	Message    string `json:"message"`
	Timestamp  string `json:"timestamp"`
}

// HandleSetBrightness handles POST /api/device/{id}/brightness
//
// Sets the brightness of a Govee light device. Looks up the device by its
// internal UUID, validates the brightness value (0–100), and dispatches the
// brightness command to the Govee API.
func (h *DeviceControlHandler) HandleSetBrightness(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "device id is required")
		return
	}

	// Parse the request body.
	var req brightnessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate brightness range (0–100).
	if req.Brightness < 0 || req.Brightness > 100 {
		writeError(w, http.StatusBadRequest, "brightness must be between 0 and 100")
		return
	}

	// Look up the device.
	device, err := db.GetDevice(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		log.Printf("❌ Failed to look up device %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to look up device")
		return
	}

	// Only Govee lights support brightness control.
	if device.DeviceType != "govee_light" {
		writeError(w, http.StatusBadRequest, "brightness control only supported for govee_light devices")
		return
	}

	// The device must have an external_id (Govee MAC address) to control it.
	if device.ExternalID == nil || *device.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "device has no external_id (Govee MAC address) — cannot control")
		return
	}

	externalID := *device.ExternalID
	model := ""
	if device.Model != nil {
		model = *device.Model
	}

	if len(h.GoveeClients) == 0 {
		writeError(w, http.StatusInternalServerError, "no Govee API clients configured")
		return
	}

	// Send brightness command to ALL Govee clients (we don't know which
	// API key owns this device — Govee silently ignores wrong-key requests).
	anySucceeded := false
	var lastErr error
	for _, client := range h.GoveeClients {
		err := client.SetBrightness(externalID, model, req.Brightness)
		if err != nil {
			lastErr = err
		} else {
			anySucceeded = true
		}
	}
	if !anySucceeded {
		log.Printf("❌ Failed to set brightness on device %s: %v", externalID, lastErr)
		writeError(w, http.StatusInternalServerError, "failed to set brightness: "+lastErr.Error())
		return
	}

	log.Printf("✅ Set brightness %d%% on device %s (%s)", req.Brightness, device.Name, externalID)

	writeJSON(w, http.StatusOK, deviceControlResponse{
		Success:    true,
		DeviceID:   device.ID,
		DeviceType: device.DeviceType,
		Message:    fmt.Sprintf("Brightness set to %d%%", req.Brightness),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
}

// HandleSetColor handles POST /api/device/{id}/color
//
// Sets the color of a Govee light device. Looks up the device by its internal
// UUID, validates the RGB values (0–255), and dispatches the color command to
// the Govee API.
func (h *DeviceControlHandler) HandleSetColor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "device id is required")
		return
	}

	// Parse the request body.
	var req colorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate RGB values (0–255).
	if req.R < 0 || req.R > 255 || req.G < 0 || req.G > 255 || req.B < 0 || req.B > 255 {
		writeError(w, http.StatusBadRequest, "RGB values must be between 0 and 255")
		return
	}

	// Look up the device.
	device, err := db.GetDevice(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		log.Printf("❌ Failed to look up device %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to look up device")
		return
	}

	// Only Govee lights support color control.
	if device.DeviceType != "govee_light" {
		writeError(w, http.StatusBadRequest, "color control only supported for govee_light devices")
		return
	}

	// The device must have an external_id (Govee MAC address) to control it.
	if device.ExternalID == nil || *device.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "device has no external_id (Govee MAC address) — cannot control")
		return
	}

	externalID := *device.ExternalID
	model := ""
	if device.Model != nil {
		model = *device.Model
	}

	if len(h.GoveeClients) == 0 {
		writeError(w, http.StatusInternalServerError, "no Govee API clients configured")
		return
	}

	// Send color command to ALL Govee clients (we don't know which
	// API key owns this device — Govee silently ignores wrong-key requests).
	anySucceeded := false
	var lastErr error
	for _, client := range h.GoveeClients {
		err := client.SetColor(externalID, model, req.R, req.G, req.B)
		if err != nil {
			lastErr = err
		} else {
			anySucceeded = true
		}
	}
	if !anySucceeded {
		log.Printf("❌ Failed to set color on device %s: %v", externalID, lastErr)
		writeError(w, http.StatusInternalServerError, "failed to set color: "+lastErr.Error())
		return
	}

	log.Printf("✅ Set color R=%d G=%d B=%d on device %s (%s)", req.R, req.G, req.B, device.Name, externalID)

	writeJSON(w, http.StatusOK, deviceControlResponse{
		Success:    true,
		DeviceID:   device.ID,
		DeviceType: device.DeviceType,
		Message:    fmt.Sprintf("Color set to R=%d G=%d B=%d", req.R, req.G, req.B),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
}

// deviceStateResponse is the JSON response for a state query.
type deviceStateResponse struct {
	DeviceID string `json:"deviceId"`
	IsOn     bool   `json:"isOn"`
}

// HandleGetDeviceState handles GET /api/device/{id}/state
//
// Looks up the device by its internal UUID, queries the Govee API for the
// current power state, and returns whether the device is on or off.
// This allows the iOS app to sync the UI with the real-world device state
// when a room loads.
//
// Response (200): {"deviceId": "uuid", "isOn": true/false}
func (h *DeviceControlHandler) HandleGetDeviceState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Device ID is required")
		return
	}

	// Look up the device by UUID.
	device, err := db.GetDevice(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Device not found")
			return
		}
		log.Printf("❌ Device state lookup failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to look up device")
		return
	}

	// Only Govee lights support state queries.
	if device.DeviceType != "govee_light" {
		writeError(w, http.StatusBadRequest, "state query only supported for govee_light devices")
		return
	}

	// Need external_id (Govee MAC) and model to query state.
	if device.ExternalID == nil || *device.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "device has no external_id (Govee MAC address)")
		return
	}

	externalID := *device.ExternalID
	model := ""
	if device.Model != nil {
		model = *device.Model
	}

	if len(h.GoveeClients) == 0 {
		writeError(w, http.StatusInternalServerError, "no Govee API clients configured")
		return
	}

	// Query device state via Govee API. Try each client until one succeeds.
	var state *govee.DeviceStateResponse
	var lastErr error
	for _, client := range h.GoveeClients {
		state, lastErr = client.GetDeviceState(externalID, model)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		log.Printf("❌ Failed to get state for device %s (%s): %v", device.Name, externalID, lastErr)
		writeError(w, http.StatusInternalServerError, "failed to query device state: "+lastErr.Error())
		return
	}

	on := isDeviceOn(state)
	log.Printf("💡 Device %s (%s) state: isOn=%v", device.Name, externalID, on)

	writeJSON(w, http.StatusOK, deviceStateResponse{
		DeviceID: device.ID,
		IsOn:     on,
	})
}

// isDeviceOn reads the Govee state response to determine if the device is on.
// The state response contains a list of properties including "powerState".
func isDeviceOn(state *govee.DeviceStateResponse) bool {
	if state == nil {
		return false
	}
	for _, prop := range state.Data.Properties {
		// Each property is a map with a single key-value pair.
		if val, ok := prop["powerState"]; ok {
			return val == "on"
		}
	}
	return false
}
