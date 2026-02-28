package camera

// Data structures for the Wyze Camera Bridge integration.
//
// The Go backend queries the Docker Wyze Bridge REST API (http://<host>:5050/api/)
// to get camera info and status. It then transforms the bridge's response into
// a cleaner format for the iOS frontend.
//
// Stream URLs are constructed by the Go backend based on the bridge host and
// camera name, so the iOS app never needs to know the bridge's direct address.

// Camera represents a Wyze camera as returned to the iOS frontend.
// Contains the camera's identity, status, and all available stream URLs.
type Camera struct {
	Name      string     `json:"name"`      // Camera name from the Wyze app (e.g., "Front Door")
	NameURI   string     `json:"nameUri"`   // URL-safe name used in stream paths (e.g., "front-door")
	Model     string     `json:"model"`     // Camera model (e.g., "Wyze Cam v3")
	Status    string     `json:"status"`    // "online" or "offline"
	Enabled   bool       `json:"enabled"`   // Whether the camera stream is enabled in the bridge
	StreamURL string     `json:"streamUrl"` // Primary HLS stream URL for the iOS app
	Streams   StreamURLs `json:"streams"`   // All available stream URLs (HLS, RTSP, WebRTC)
}

// StreamURLs contains all available streaming protocol URLs for a camera.
// The iOS app primarily uses HLS (native AVPlayer support), but we expose
// all protocols so other clients can choose their preferred format.
type StreamURLs struct {
	HLS    string `json:"hls"`    // http://<host>:8888/<name>/stream.m3u8 — used by iOS AVPlayer
	RTSP   string `json:"rtsp"`   // rtsp://<host>:8554/<name> — standard video streaming
	WebRTC string `json:"webrtc"` // http://<host>:8889/<name>/ — low-latency browser streaming
}

// CamerasResponse is the response from GET /api/cameras.
// Wraps the camera list with a success flag and message.
type CamerasResponse struct {
	Success bool     `json:"success"` // Whether the bridge query succeeded
	Cameras []Camera `json:"cameras"` // List of available cameras
	Message string   `json:"message"` // Human-readable status message
}

// StreamResponse is the response from GET /api/cameras/stream.
// Returns all stream URLs for a specific camera by name.
type StreamResponse struct {
	Success   bool       `json:"success"`   // Whether the camera was found
	Name      string     `json:"name"`      // Camera name
	NameURI   string     `json:"nameUri"`   // URL-safe camera name
	Status    string     `json:"status"`    // "online" or "offline"
	StreamURL string     `json:"streamUrl"` // Primary HLS stream URL
	Streams   StreamURLs `json:"streams"`   // All available stream URLs
	Message   string     `json:"message"`   // Human-readable status message
}

// BridgeCameraInfo represents the raw camera data returned by the Wyze Bridge API.
// The bridge's GET /api/ endpoint returns a JSON object where each key is a camera
// URI name, and the value contains camera metadata. The exact fields vary by camera
// model and bridge version, so we parse selectively.
type BridgeCameraInfo struct {
	NameURI    string `json:"name_uri"`     // URL-safe camera identifier (e.g., "front-door")
	Nickname   string `json:"nickname"`     // Display name from the Wyze app (e.g., "Front Door")
	ModelName  string `json:"model_name"`   // Camera model name (e.g., "Wyze Cam v3")
	ProductModel string `json:"product_model"` // Product model ID (e.g., "WYZE_CAKP2JFUS")
	Connected  bool   `json:"connected"`    // Whether the camera is currently connected
	Enabled    bool   `json:"enabled"`      // Whether streaming is enabled in the bridge
}
