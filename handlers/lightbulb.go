package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// LightbulbToggleRequest represents the incoming request body
type LightbulbToggleRequest struct {
	IsOn bool `json:"isOn"`
}

// LightbulbToggleResponse represents the response body
type LightbulbToggleResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	IsOn      bool      `json:"isOn"`
	Timestamp time.Time `json:"timestamp"`
}

// HandleLightbulbToggle processes lightbulb toggle requests from the frontend
// It logs the request and returns a success response
func HandleLightbulbToggle(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var req LightbulbToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the lightbulb toggle event
	log.Printf("🔆 Lightbulb toggled - State: %t (turned %s) - Client: %s",
		req.IsOn,
		map[bool]string{true: "ON", false: "OFF"}[req.IsOn],
		r.RemoteAddr,
	)

	// Create response
	response := LightbulbToggleResponse{
		Success:   true,
		Message:   "Lightbulb state updated successfully",
		IsOn:      req.IsOn,
		Timestamp: time.Now(),
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
