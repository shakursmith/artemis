package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// writeJSON encodes the given value as JSON and writes it to the response
// with the specified status code. Sets Content-Type to application/json.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("❌ Error encoding JSON response: %v", err)
	}
}

// writeError sends a JSON error response with the given status code and message.
// Format: {"error": "message here"}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// isNotFound checks if an error message indicates a "not found" condition
// from the repository layer. The db package uses "X not found" error strings.
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found")
}
