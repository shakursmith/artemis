package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pantheon/artemis/camera"
)

// HandleGetCameras returns all cameras from the Wyze Bridge.
// GET /api/cameras
// Queries the Docker Wyze Bridge REST API for available cameras and
// returns them with name, model, online/offline status, and stream URLs.
// The iOS app uses this to populate the camera list view.
func HandleGetCameras(cameraClient *camera.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests.
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("📷 Camera list request from client: %s", r.RemoteAddr)

		// Query the Wyze Bridge for all cameras.
		cameras, err := cameraClient.GetCameras()
		if err != nil {
			log.Printf("❌ Failed to fetch cameras from Wyze Bridge: %v", err)
			sendCameraError(w, http.StatusInternalServerError, "Failed to fetch cameras: "+err.Error())
			return
		}

		// Handle nil cameras slice (no cameras found but no error).
		if cameras == nil {
			cameras = []camera.Camera{}
		}

		log.Printf("📷 Returning %d camera(s) to client", len(cameras))

		// Build the response for the iOS app.
		response := camera.CamerasResponse{
			Success: true,
			Cameras: cameras,
			Message: formatCameraCountMessage(len(cameras)),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Error encoding cameras response: %v", err)
		}
	}
}

// HandleGetCameraStream returns stream URLs for a specific camera.
// GET /api/cameras/stream?name=<camera-name-uri>
// The name parameter is the URL-safe camera name (e.g., "front-door").
// Returns HLS, RTSP, and WebRTC stream URLs along with camera status.
//
// The iOS app calls this when the user taps a camera in the list to view
// the live stream. HLS is the primary protocol used by iOS (AVPlayer).
func HandleGetCameraStream(cameraClient *camera.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests.
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the camera name from query parameters.
		// Matches the existing pattern used by HandleGetDeviceState (govee.go).
		nameURI := r.URL.Query().Get("name")
		if nameURI == "" {
			sendCameraError(w, http.StatusBadRequest, "Missing required 'name' query parameter")
			return
		}

		log.Printf("📷 Stream request for camera '%s' from client: %s", nameURI, r.RemoteAddr)

		// Query the bridge for this specific camera.
		cam, err := cameraClient.GetCamera(nameURI)
		if err != nil {
			log.Printf("❌ Failed to get camera '%s': %v", nameURI, err)
			sendCameraError(w, http.StatusNotFound, "Camera not found: "+err.Error())
			return
		}

		// Check if the camera is offline — still return URLs but warn the caller.
		statusMsg := "Camera is online and streaming"
		if cam.Status == "offline" {
			statusMsg = "Camera is offline — stream may not be available"
			log.Printf("⚠️  Camera '%s' is offline", nameURI)
		}

		log.Printf("📷 Returning stream URLs for camera '%s' (status: %s)", nameURI, cam.Status)

		// Build the response with all stream URLs.
		response := camera.StreamResponse{
			Success:   true,
			Name:      cam.Name,
			NameURI:   cam.NameURI,
			Status:    cam.Status,
			StreamURL: cam.StreamURL,
			Streams:   cam.Streams,
			Message:   statusMsg,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Error encoding stream response: %v", err)
		}
	}
}

// sendCameraError sends a JSON error response for camera endpoints.
func sendCameraError(w http.ResponseWriter, statusCode int, message string) {
	response := camera.CamerasResponse{
		Success: false,
		Cameras: []camera.Camera{},
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// formatCameraCountMessage returns a human-readable message for camera count.
func formatCameraCountMessage(count int) string {
	if count == 0 {
		return "No cameras found. Make sure Wyze Bridge is running and cameras are connected."
	}
	if count == 1 {
		return "Found 1 camera"
	}
	return fmt.Sprintf("Found %d cameras", count)
}
