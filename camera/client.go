package camera

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Default configuration for the Wyze Bridge connection.
const (
	defaultBridgeURL = "http://localhost:5050"

	// Endpoint on the Wyze Bridge that returns camera info.
	// Returns a JSON object keyed by camera URI name.
	bridgeAPIEndpoint = "/api/"

	// Timeout for HTTP requests to the bridge.
	requestTimeout = 10 * time.Second

	// Default ports for stream URLs.
	// These match the port mappings in docker-compose.yml.
	hlsPort    = "8888"
	rtspPort   = "8554"
	webrtcPort = "8889"
)

// Client communicates with the Docker Wyze Bridge REST API.
// It queries the bridge for camera info and constructs stream URLs
// that the iOS app can use to view live camera feeds.
type Client struct {
	bridgeURL  string       // Base URL of the Wyze Bridge web UI (e.g., "http://localhost:5050")
	apiKey     string       // Optional API key for bridge authentication (WB_API)
	httpClient *http.Client // HTTP client with timeout configured
}

// NewClient creates a new Wyze Bridge client.
// bridgeURL is the base URL of the bridge (e.g., "http://localhost:5050").
// apiKey is optional — only needed if WB_AUTH is enabled on the bridge.
func NewClient(bridgeURL, apiKey string) *Client {
	if bridgeURL == "" {
		bridgeURL = defaultBridgeURL
	}

	// Strip trailing slash to avoid double-slashes in URL construction.
	bridgeURL = strings.TrimRight(bridgeURL, "/")

	return &Client{
		bridgeURL: bridgeURL,
		apiKey:    apiKey,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// GetCameras queries the Wyze Bridge API for all available cameras.
// Returns a list of Camera objects with name, model, status, and stream URLs.
//
// The bridge API returns a JSON object where each key is a camera name-uri:
//
//	{
//	  "front-door": { "name_uri": "front-door", "nickname": "Front Door", ... },
//	  "back-yard":  { "name_uri": "back-yard", "nickname": "Back Yard", ... }
//	}
//
// We iterate over the keys and construct stream URLs for each camera.
func (c *Client) GetCameras() ([]Camera, error) {
	log.Printf("📷 Fetching cameras from Wyze Bridge at %s...", c.bridgeURL)

	// Build the request URL. Include API key if configured.
	reqURL := c.bridgeURL + bridgeAPIEndpoint
	if c.apiKey != "" {
		reqURL += "?api=" + c.apiKey
	}

	// Make the GET request to the bridge API.
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Wyze Bridge at %s: %w", c.bridgeURL, err)
	}
	defer resp.Body.Close()

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bridge response: %w", err)
	}

	// Check for non-success HTTP status.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	// The bridge API returns a JSON object keyed by camera URI name.
	// Each value contains camera metadata fields. We use a flexible map
	// to handle varying response structures across bridge versions.
	var bridgeResponse map[string]json.RawMessage
	if err := json.Unmarshal(body, &bridgeResponse); err != nil {
		return nil, fmt.Errorf("failed to parse bridge response: %w", err)
	}

	// Extract the bridge host from the URL for constructing stream URLs.
	// Stream URLs use different ports on the same host.
	bridgeHost := extractHost(c.bridgeURL)

	// Transform each camera entry into our Camera model.
	var cameras []Camera
	for nameURI, rawData := range bridgeResponse {
		camera := c.parseCameraEntry(nameURI, rawData, bridgeHost)
		cameras = append(cameras, camera)
	}

	log.Printf("📷 Found %d camera(s) from Wyze Bridge", len(cameras))
	return cameras, nil
}

// GetCamera returns info and stream URLs for a specific camera by name.
// The name parameter is the URL-safe camera name (e.g., "front-door").
func (c *Client) GetCamera(nameURI string) (*Camera, error) {
	log.Printf("📷 Fetching camera '%s' from Wyze Bridge...", nameURI)

	// Build the request URL for a specific camera.
	reqURL := c.bridgeURL + "/api/" + nameURI
	if c.apiKey != "" {
		reqURL += "?api=" + c.apiKey
	}

	// Make the GET request.
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Wyze Bridge: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bridge response: %w", err)
	}

	// 404 or empty response means camera not found.
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("camera '%s' not found", nameURI)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bridge returned status %d for camera '%s'", resp.StatusCode, nameURI)
	}

	bridgeHost := extractHost(c.bridgeURL)
	cam := c.parseCameraEntry(nameURI, body, bridgeHost)
	return &cam, nil
}

// parseCameraEntry transforms a raw bridge API camera entry into our Camera model.
// Handles the flexible JSON structure by trying known fields and falling back
// to defaults when fields are missing (bridge response varies by version/model).
func (c *Client) parseCameraEntry(nameURI string, rawData json.RawMessage, bridgeHost string) Camera {
	// Try to parse known fields from the camera data.
	var info BridgeCameraInfo
	_ = json.Unmarshal(rawData, &info) // Best-effort parse; missing fields get zero values.

	// Also try parsing as a generic map to catch additional fields.
	var generic map[string]interface{}
	_ = json.Unmarshal(rawData, &generic)

	// Determine the display name — prefer nickname, fall back to name_uri.
	displayName := info.Nickname
	if displayName == "" {
		displayName = nameURI
	}

	// Determine the model name.
	model := info.ModelName
	if model == "" {
		model = info.ProductModel
	}
	if model == "" {
		// Try alternate field names that some bridge versions use.
		if m, ok := generic["model_name"].(string); ok && m != "" {
			model = m
		} else if m, ok := generic["product_model"].(string); ok && m != "" {
			model = m
		} else {
			model = "Wyze Camera"
		}
	}

	// Determine the URI name for stream URLs.
	uri := info.NameURI
	if uri == "" {
		uri = nameURI
	}

	// Determine online/offline status.
	// The bridge uses "connected" and "enabled" fields.
	status := "offline"
	connected := info.Connected
	enabled := info.Enabled

	// Also check generic map for boolean fields that might be parsed differently.
	if !connected {
		if c, ok := generic["connected"].(bool); ok {
			connected = c
		}
	}
	if !enabled {
		if e, ok := generic["enabled"].(bool); ok {
			enabled = e
		}
	}

	if connected && enabled {
		status = "online"
	}

	// Construct stream URLs using the bridge host and standard ports.
	streams := StreamURLs{
		HLS:    fmt.Sprintf("http://%s:%s/%s/stream.m3u8", bridgeHost, hlsPort, uri),
		RTSP:   fmt.Sprintf("rtsp://%s:%s/%s", bridgeHost, rtspPort, uri),
		WebRTC: fmt.Sprintf("http://%s:%s/%s/", bridgeHost, webrtcPort, uri),
	}

	return Camera{
		Name:      displayName,
		NameURI:   uri,
		Model:     model,
		Status:    status,
		Enabled:   enabled,
		StreamURL: streams.HLS, // HLS is the primary stream for iOS (native AVPlayer support)
		Streams:   streams,
	}
}

// CheckHealth verifies the Wyze Bridge is running and reachable.
// Returns nil if healthy, or an error describing the problem.
func (c *Client) CheckHealth() error {
	reqURL := c.bridgeURL + bridgeAPIEndpoint
	if c.apiKey != "" {
		reqURL += "?api=" + c.apiKey
	}

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return fmt.Errorf("wyze Bridge unreachable at %s: %w", c.bridgeURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wyze Bridge unhealthy (status %d)", resp.StatusCode)
	}

	return nil
}

// extractHost extracts the hostname (without scheme or port) from a URL.
// e.g., "http://192.168.1.100:5050" → "192.168.1.100"
//
//	"http://localhost:5050" → "localhost"
func extractHost(rawURL string) string {
	// Strip the scheme.
	host := rawURL
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}

	// Strip the port.
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Strip any trailing path.
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	return host
}
