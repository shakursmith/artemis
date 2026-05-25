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

// deviceToggleResponse is the JSON response for a successful toggle.
type deviceToggleResponse struct {
	Success   bool   `json:"success"`
	DeviceID  string `json:"deviceId"`
	DeviceType string `json:"deviceType"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// HandleToggleDevice handles POST /api/device/{id}/toggle
//
// Looks up the device by its internal UUID, determines its type (govee_light,
// fire_tv, wyze_camera), and dispatches the appropriate toggle command.
//
// For Govee lights: sends a "turn" command to the Govee API. The device's
// external_id (MAC address) and model are read from the database.
//
// For other device types: returns a placeholder success for now (the full
// control flow uses the integration-specific endpoints).
func (h *DeviceControlHandler) HandleToggleDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "device id is required")
		return
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

	log.Printf("🔄 Toggle request for device: %s (%s, type=%s)", device.Name, device.ID, device.DeviceType)

	// Dispatch based on device type.
	switch device.DeviceType {
	case "govee_light":
		h.toggleGoveeDevice(w, device)
	case "fire_tv":
		// Fire TV toggle is more complex (requires ADB connection).
		// For now, return success as a placeholder.
		writeJSON(w, http.StatusOK, deviceToggleResponse{
			Success:    true,
			DeviceID:   device.ID,
			DeviceType: device.DeviceType,
			Message:    "Fire TV toggle not yet implemented via this endpoint",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		})
	case "wyze_camera":
		// Cameras don't really have a simple toggle.
		writeJSON(w, http.StatusOK, deviceToggleResponse{
			Success:    true,
			DeviceID:   device.ID,
			DeviceType: device.DeviceType,
			Message:    "Camera toggle not applicable",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		})
	default:
		writeError(w, http.StatusBadRequest, "unsupported device type: "+device.DeviceType)
	}
}

// toggleGoveeDevice sends a "turn" toggle command to a Govee light.
// It reads the device's external_id (MAC address) and model from the database,
// then sends the command via the Govee API client.
//
// Since we don't know the current state from the database alone, we query
// the device's current state first and then send the opposite command.
func (h *DeviceControlHandler) toggleGoveeDevice(w http.ResponseWriter, device *db.Device) {
	// The device must have an external_id (Govee MAC address) to control it.
	if device.ExternalID == nil || *device.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "device has no external_id (Govee MAC address) — cannot control")
		return
	}

	// The device must have a model for Govee API calls.
	if device.Model == nil || *device.Model == "" {
		// Try to control without model — some Govee API calls work without it.
		// But log a warning.
		log.Printf("⚠️ Device %s has no model — attempting toggle anyway", device.ID)
	}

	externalID := *device.ExternalID
	model := ""
	if device.Model != nil {
		model = *device.Model
	}

	// Determine which API key to use. For now, try the first client.
	// TODO: Store the API key index on the device record for proper routing.
	if len(h.GoveeClients) == 0 {
		writeError(w, http.StatusInternalServerError, "no Govee API clients configured")
		return
	}

	// Try to get current state to toggle it.
	// If we can't determine state, default to toggling on.
	var turnOn bool = true
	client := h.GoveeClients[0]

	state, err := client.GetDeviceState(externalID, model)
	if err != nil {
		log.Printf("⚠️ Could not get device state for %s, defaulting to turn on: %v", externalID, err)
	} else {
		// Read the current on/off state and send the opposite.
		turnOn = !isDeviceOn(state)
	}

	// Send the toggle command using TurnOn/TurnOff.
	if turnOn {
		err = client.TurnOn(externalID, model)
	} else {
		err = client.TurnOff(externalID, model)
	}
	if err != nil {
		// Try other clients if the first one fails.
		toggled := false
		for i := 1; i < len(h.GoveeClients); i++ {
			if turnOn {
				err = h.GoveeClients[i].TurnOn(externalID, model)
			} else {
				err = h.GoveeClients[i].TurnOff(externalID, model)
			}
			if err == nil {
				toggled = true
				break
			}
		}
		if !toggled {
			log.Printf("❌ Failed to toggle Govee device %s: %v", externalID, err)
			writeError(w, http.StatusInternalServerError, "failed to toggle device: "+err.Error())
			return
		}
	}

	stateStr := "off"
	if turnOn {
		stateStr = "on"
	}
	log.Printf("✅ Toggled Govee device %s (%s) → %s", device.Name, externalID, stateStr)

	writeJSON(w, http.StatusOK, deviceToggleResponse{
		Success:    true,
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

	// Send brightness command via Govee API. Try each client until one succeeds.
	var lastErr error
	for _, client := range h.GoveeClients {
		lastErr = client.SetBrightness(externalID, model, req.Brightness)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
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

	// Send color command via Govee API. Try each client until one succeeds.
	var lastErr error
	for _, client := range h.GoveeClients {
		lastErr = client.SetColor(externalID, model, req.R, req.G, req.B)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
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
