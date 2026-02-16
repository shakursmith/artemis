package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pantheon/artemis/govee"
)

// DeviceResponse represents a simplified device for the frontend
// Transforms Govee's complex API response into a cleaner format
type DeviceResponse struct {
	ID           string   `json:"id"`           // Device MAC address
	Name         string   `json:"name"`         // User-friendly name
	Model        string   `json:"model"`        // Device model number
	Type         string   `json:"type"`         // Device type (e.g., "light")
	Capabilities []string `json:"capabilities"` // Supported commands
	APIKeyIndex  int      `json:"apiKeyIndex"`  // Which API key owns this device (0 = primary, 1 = secondary)
}

// ControlRequest represents a device control request from the frontend
// The command field determines what the value should be:
// - "turn": value should be boolean (true = on, false = off)
// - "brightness": value should be number 0-100
// - "color": value should be object with r, g, b fields (each 0-255)
type ControlRequest struct {
	DeviceID    string      `json:"deviceId"`    // Device MAC address
	Model       string      `json:"model"`       // Device model (needed for some commands)
	Command     string      `json:"command"`     // Command type: "turn", "brightness", "color"
	Value       interface{} `json:"value"`       // Command value (type depends on command)
	APIKeyIndex int         `json:"apiKeyIndex"` // Which API key owns this device (0 = primary, 1 = secondary)
}

// ControlResponse represents the response after controlling a device
type ControlResponse struct {
	Success   bool   `json:"success"`   // Whether the command succeeded
	Message   string `json:"message"`   // Success or error message
	DeviceID  string `json:"deviceId"`  // Which device was controlled
	Timestamp string `json:"timestamp"` // When the command was executed
}

// RGBValue represents an RGB color from the frontend
// Used when command is "color"
type RGBValue struct {
	R int `json:"r"` // Red (0-255)
	G int `json:"g"` // Green (0-255)
	B int `json:"b"` // Blue (0-255)
}

// HandleGetDevices returns all Govee devices from all configured API keys
// GET /api/govee/devices
// Returns: JSON array of DeviceResponse objects from both primary and secondary accounts
func HandleGetDevices(goveeClients []*govee.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("💡 Fetching Govee devices from %d account(s) - Client: %s", len(goveeClients), r.RemoteAddr)

		// Collect all devices from all API keys
		var allDevices []DeviceResponse

		// Fetch devices from each API key
		for apiKeyIndex, client := range goveeClients {
			devices, err := client.GetDevices()
			if err != nil {
				log.Printf("❌ Error fetching devices from API key #%d: %v", apiKeyIndex, err)
				// Continue with other API keys even if one fails
				continue
			}

			log.Printf("💡 Found %d device(s) from API key #%d", len(devices), apiKeyIndex)

			// Transform and tag each device with its API key index
			for _, device := range devices {
				allDevices = append(allDevices, DeviceResponse{
					ID:           device.Device,
					Name:         device.DeviceName,
					Model:        device.Model,
					Type:         "light", // Most Govee devices are lights
					Capabilities: device.SupportCmds,
					APIKeyIndex:  apiKeyIndex, // Track which API key owns this device
				})
			}
		}

		log.Printf("💡 Returning %d total device(s) to client", len(allDevices))

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(allDevices); err != nil {
			log.Printf("❌ Error encoding response: %v", err)
		}
	}
}

// HandleControlDevice processes device control requests from the frontend
// POST /api/govee/devices/control
// Accepts: ControlRequest JSON body
// Returns: ControlResponse JSON
//
// The handler routes commands to the appropriate Govee client method:
// - "turn": Calls TurnOn or TurnOff based on boolean value
// - "brightness": Calls SetBrightness with integer value (0-100)
// - "color": Calls SetColor with RGB values from object
// Uses the apiKeyIndex from the request to select the correct API key
func HandleControlDevice(goveeClients []*govee.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body
		var req ControlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("❌ Error decoding control request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		log.Printf("💡 Control request - Device: %s, Command: %s, API Key Index: %d - Client: %s",
			req.DeviceID, req.Command, req.APIKeyIndex, r.RemoteAddr)

		// Validate API key index
		if req.APIKeyIndex < 0 || req.APIKeyIndex >= len(goveeClients) {
			log.Printf("❌ Invalid API key index: %d (have %d clients)", req.APIKeyIndex, len(goveeClients))
			sendErrorResponse(w, req.DeviceID, "Invalid API key index")
			return
		}

		// Select the correct client based on API key index
		goveeClient := goveeClients[req.APIKeyIndex]

		// Execute the appropriate command based on command type
		var err error
		switch req.Command {
		case "turn":
			// Value should be boolean
			isOn, ok := req.Value.(bool)
			if !ok {
				sendErrorResponse(w, req.DeviceID, "Invalid value for 'turn' command - expected boolean")
				return
			}

			if isOn {
				err = goveeClient.TurnOn(req.DeviceID, req.Model)
			} else {
				err = goveeClient.TurnOff(req.DeviceID, req.Model)
			}

		case "brightness":
			// Value should be number (will come as float64 from JSON)
			brightness, ok := req.Value.(float64)
			if !ok {
				sendErrorResponse(w, req.DeviceID, "Invalid value for 'brightness' command - expected number")
				return
			}

			err = goveeClient.SetBrightness(req.DeviceID, req.Model, int(brightness))

		case "color":
			// Value should be object with r, g, b fields
			// JSON unmarshals objects as map[string]interface{}
			colorMap, ok := req.Value.(map[string]interface{})
			if !ok {
				sendErrorResponse(w, req.DeviceID, "Invalid value for 'color' command - expected object with r, g, b")
				return
			}

			// Extract RGB values (they come as float64 from JSON)
			r, okR := colorMap["r"].(float64)
			g, okG := colorMap["g"].(float64)
			b, okB := colorMap["b"].(float64)

			if !okR || !okG || !okB {
				sendErrorResponse(w, req.DeviceID, "Color object must have r, g, b numeric fields")
				return
			}

			err = goveeClient.SetColor(req.DeviceID, req.Model, int(r), int(g), int(b))

		default:
			sendErrorResponse(w, req.DeviceID, "Unknown command: "+req.Command)
			return
		}

		// Check if command execution failed
		if err != nil {
			log.Printf("❌ Error executing command: %v", err)
			sendErrorResponse(w, req.DeviceID, err.Error())
			return
		}

		// Send success response
		response := ControlResponse{
			Success:   true,
			Message:   "Device controlled successfully",
			DeviceID:  req.DeviceID,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		log.Printf("✅ Control command successful - Device: %s, Command: %s", req.DeviceID, req.Command)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Error encoding response: %v", err)
		}
	}
}

// sendErrorResponse is a helper function to send error responses
// Encapsulates the common error response pattern
func sendErrorResponse(w http.ResponseWriter, deviceID, message string) {
	response := ControlResponse{
		Success:   false,
		Message:   message,
		DeviceID:  deviceID,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// StateResponse represents the simplified device state for the frontend
type StateResponse struct {
	DeviceID string `json:"deviceId"` // Device MAC address
	IsOn     bool   `json:"isOn"`     // Whether device is currently on
}

// HandleGetDeviceState queries the current state of a specific device
// GET /api/govee/devices/state?deviceId=X&model=Y&apiKeyIndex=Z
// Returns: StateResponse JSON with current on/off state
func HandleGetDeviceState(goveeClients []*govee.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse query parameters
		deviceID := r.URL.Query().Get("deviceId")
		model := r.URL.Query().Get("model")
		apiKeyIndex := 0 // Default to primary

		// Parse apiKeyIndex if provided
		if apiKeyIndexStr := r.URL.Query().Get("apiKeyIndex"); apiKeyIndexStr != "" {
			var err error
			if _, err = fmt.Sscanf(apiKeyIndexStr, "%d", &apiKeyIndex); err != nil {
				http.Error(w, "Invalid apiKeyIndex", http.StatusBadRequest)
				return
			}
		}

		// Validate parameters
		if deviceID == "" || model == "" {
			http.Error(w, "Missing deviceId or model parameter", http.StatusBadRequest)
			return
		}

		// Validate API key index
		if apiKeyIndex < 0 || apiKeyIndex >= len(goveeClients) {
			http.Error(w, "Invalid API key index", http.StatusBadRequest)
			return
		}

		// Get the appropriate client
		client := goveeClients[apiKeyIndex]

		// Query device state
		stateResp, err := client.GetDeviceState(deviceID, model)
		if err != nil {
			log.Printf("❌ Error querying device state: %v", err)
			http.Error(w, "Failed to query device state", http.StatusInternalServerError)
			return
		}

		// Extract power state from properties
		// The Govee API returns properties as an array of objects with varying keys
		// Common keys: "online" (bool), "powerState" (string "on"/"off"), "brightness" (int)
		isOn := false
		for _, prop := range stateResp.Data.Properties {
			// Check for "online" property (boolean)
			if onlineVal, exists := prop["online"]; exists {
				if boolVal, ok := onlineVal.(bool); ok {
					isOn = boolVal
					break
				}
			}
			// Check for "powerState" property (string)
			if powerStateVal, exists := prop["powerState"]; exists {
				if strVal, ok := powerStateVal.(string); ok {
					isOn = (strVal == "on")
					break
				}
			}
		}

		// Send simplified response
		response := StateResponse{
			DeviceID: deviceID,
			IsOn:     isOn,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Error encoding response: %v", err)
		}
	}
}
