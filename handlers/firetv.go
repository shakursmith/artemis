package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/pantheon/artemis/firetv"
)

// FireTVDiscoverResponse is the response sent to the iOS app for device discovery.
// Wraps the list of discovered devices with a success flag and message.
type FireTVDiscoverResponse struct {
	Success bool                       `json:"success"` // Whether the discovery scan succeeded
	Devices []firetv.DiscoveredDevice  `json:"devices"` // List of Fire TV devices found on the LAN
	Message string                     `json:"message"` // Human-readable status (e.g., "Found 2 device(s)")
}

// FireTVPairRequest is the request body from the iOS app for pairing.
// The iOS app sends the device host and optionally the PIN code.
type FireTVPairRequest struct {
	Host string `json:"host"`          // IP address of the Fire TV device
	PIN  string `json:"pin,omitempty"` // 6-digit PIN from the TV screen (empty to start pairing)
}

// FireTVPairResponse is the response sent to the iOS app for pairing.
type FireTVPairResponse struct {
	Success     bool   `json:"success"`                // Whether this pairing step succeeded
	Message     string `json:"message"`                // Status message for the UI
	DeviceName  string `json:"deviceName,omitempty"`   // Device name (after successful pairing)
	AwaitingPIN bool   `json:"awaitingPin"`            // True when TV is displaying a PIN
	Timestamp   string `json:"timestamp"`              // When the response was generated
}

// FireTVCommandRequest is the request body from the iOS app for sending commands.
// Matches the format expected by POST /api/firetv/command.
type FireTVCommandRequest struct {
	Host       string `json:"host"`                  // IP address of the target Fire TV device
	Command    string `json:"command"`               // Command name (e.g., "home", "up", "text_input")
	Text       string `json:"text,omitempty"`        // Text to send (for "text_input" command)
	AppPackage string `json:"appPackage,omitempty"`  // Package name (for "launch_app" command)
}

// FireTVCommandResponse is the response sent to the iOS app after a command.
type FireTVCommandResponse struct {
	Success   bool   `json:"success"`   // Whether the command was sent successfully
	Message   string `json:"message"`   // Status message (e.g., "Sent command: home")
	Command   string `json:"command"`   // Echo of the command that was executed
	Timestamp string `json:"timestamp"` // When the command was processed
}

// HandleFireTVDiscover handles device discovery requests from the iOS app.
// GET /api/firetv/discover
// Proxies to the Python Fire TV microservice which scans the LAN via mDNS
// for devices advertising the Android TV Remote v2 service type.
// Returns a JSON list of discovered devices with name, IP, port, and model.
func HandleFireTVDiscover(firetvClient *firetv.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests for discovery.
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("📺 Fire TV discovery request from client: %s", r.RemoteAddr)

		// Proxy the discovery request to the Python Fire TV service.
		// This triggers a ~5 second mDNS scan on the local network.
		result, err := firetvClient.Discover()
		if err != nil {
			log.Printf("❌ Fire TV discovery failed: %v", err)
			sendFireTVError(w, http.StatusInternalServerError, err.Error())
			return
		}

		log.Printf("📺 Returning %d Fire TV device(s) to client", len(result.Devices))

		// Send the discovery results to the iOS app.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("❌ Error encoding Fire TV discover response: %v", err)
		}
	}
}

// HandleFireTVPair handles pairing requests from the iOS app.
// POST /api/firetv/pair
// Proxies to the Python Fire TV microservice which manages the PIN-based
// pairing flow with the Android TV Remote protocol v2.
//
// Two-step flow:
//   Step 1: {"host": "192.168.1.50"} → TV shows a PIN. Response has awaitingPin=true.
//   Step 2: {"host": "192.168.1.50", "pin": "123456"} → Verifies PIN. Response has deviceName.
func HandleFireTVPair(firetvClient *firetv.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests for pairing.
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body from the iOS app.
		var req FireTVPairRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("❌ Error decoding Fire TV pair request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate that host is provided.
		if req.Host == "" {
			sendFireTVError(w, http.StatusBadRequest, "host is required")
			return
		}

		log.Printf("📺 Fire TV pair request - Host: %s, PIN: %s - Client: %s",
			req.Host, maskPIN(req.PIN), r.RemoteAddr)

		var result *firetv.PairResponse
		var err error

		if req.PIN == "" {
			// Step 1: Start pairing — TV will display a PIN.
			result, err = firetvClient.StartPairing(req.Host)
		} else {
			// Step 2: Finish pairing with the user-provided PIN.
			result, err = firetvClient.FinishPairing(req.Host, req.PIN)
		}

		if err != nil {
			log.Printf("❌ Fire TV pairing failed: %v", err)
			sendFireTVError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Build the response for the iOS app.
		response := FireTVPairResponse{
			Success:     result.Success,
			Message:     result.Message,
			DeviceName:  result.DeviceName,
			AwaitingPIN: result.AwaitingPIN,
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		log.Printf("📺 Fire TV pair result: success=%v, awaiting_pin=%v", result.Success, result.AwaitingPIN)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Error encoding Fire TV pair response: %v", err)
		}
	}
}

// HandleFireTVCommand handles remote control command requests from the iOS app.
// POST /api/firetv/command
// Proxies to the Python Fire TV microservice which sends the command to the
// paired device using the Android TV Remote protocol v2.
//
// Request body:
//   {"host": "192.168.1.50", "command": "home"}
//   {"host": "192.168.1.50", "command": "text_input", "text": "Netflix"}
//   {"host": "192.168.1.50", "command": "launch_app", "appPackage": "com.netflix.ninja"}
//
// Supported commands:
//   Navigation: up, down, left, right, select, back, home, menu
//   Media: play_pause, play, pause, fast_forward, rewind, stop
//   Power: power, sleep
//   Volume: volume_up, volume_down, mute
//   Special: text_input (with text field), launch_app (with appPackage field)
func HandleFireTVCommand(firetvClient *firetv.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests for commands.
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body from the iOS app.
		var req FireTVCommandRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("❌ Error decoding Fire TV command request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields.
		if req.Host == "" {
			sendFireTVError(w, http.StatusBadRequest, "host is required")
			return
		}
		if req.Command == "" {
			sendFireTVError(w, http.StatusBadRequest, "command is required")
			return
		}

		log.Printf("📺 Fire TV command request - Host: %s, Command: %s - Client: %s",
			req.Host, req.Command, r.RemoteAddr)

		// Proxy the command to the Python Fire TV service.
		result, err := firetvClient.SendCommand(req.Host, req.Command, req.Text, req.AppPackage)
		if err != nil {
			log.Printf("❌ Fire TV command failed: %v", err)
			sendFireTVError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Build the response for the iOS app.
		response := FireTVCommandResponse{
			Success:   result.Success,
			Message:   result.Message,
			Command:   result.Command,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		log.Printf("✅ Fire TV command successful - Host: %s, Command: %s", req.Host, req.Command)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Error encoding Fire TV command response: %v", err)
		}
	}
}

// sendFireTVError sends a JSON error response for Fire TV endpoints.
// Uses a consistent format matching the other handler error patterns.
func sendFireTVError(w http.ResponseWriter, statusCode int, message string) {
	response := FireTVCommandResponse{
		Success:   false,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// maskPIN partially masks the PIN for logging (shows first 2 digits only).
// Returns "(none)" if no PIN is provided.
func maskPIN(pin string) string {
	if pin == "" {
		return "(none)"
	}
	if len(pin) <= 2 {
		return pin + "****"
	}
	return pin[:2] + "****"
}
