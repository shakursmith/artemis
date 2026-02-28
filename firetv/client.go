package firetv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Base URL for the Python Fire TV Remote microservice.
// This service runs locally and handles the actual communication
// with Fire TV devices via the Android TV Remote protocol v2.
const (
	defaultBaseURL = "http://localhost:9090"

	// Endpoints on the Python microservice.
	discoverEndpoint = "/discover"
	pairEndpoint     = "/pair"
	commandEndpoint  = "/command"
	healthEndpoint   = "/health"

	// Timeout for HTTP requests to the Python service.
	// Discovery can take up to 5 seconds (mDNS scan), so we allow extra headroom.
	requestTimeout = 15 * time.Second
)

// Client communicates with the Python Fire TV Remote microservice.
// It proxies discovery, pairing, and command requests from the Go backend
// to the Python service, which handles the actual Android TV Remote protocol.
type Client struct {
	baseURL    string       // Base URL of the Python microservice (e.g., "http://localhost:9090")
	httpClient *http.Client // HTTP client with timeout configured
}

// NewClient creates a new Fire TV client that connects to the Python microservice.
// The serviceURL parameter is the base URL of the Python Fire TV service
// (e.g., "http://localhost:9090"). If empty, defaults to localhost:9090.
func NewClient(serviceURL string) *Client {
	if serviceURL == "" {
		serviceURL = defaultBaseURL
	}

	return &Client{
		baseURL: serviceURL,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// Discover scans the local network for Fire TV devices.
// Calls the Python service's GET /discover endpoint, which uses mDNS/Zeroconf
// to find devices advertising the Android TV Remote v2 service type.
// The scan takes approximately 5 seconds to collect all device responses.
func (c *Client) Discover() (*DiscoverResponse, error) {
	log.Printf("📺 Requesting Fire TV device discovery from Python service...")

	// Send GET request to the Python service's discover endpoint.
	resp, err := c.httpClient.Get(c.baseURL + discoverEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Fire TV service: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body for logging and parsing.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read discovery response: %w", err)
	}

	// Check for non-success HTTP status.
	if resp.StatusCode != http.StatusOK {
		var errDetail ErrorDetail
		if json.Unmarshal(body, &errDetail) == nil && errDetail.Detail != "" {
			return nil, fmt.Errorf("discovery failed: %s", errDetail.Detail)
		}
		return nil, fmt.Errorf("discovery failed with status %d", resp.StatusCode)
	}

	// Parse the discovery response.
	var result DiscoverResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse discovery response: %w", err)
	}

	log.Printf("📺 Discovery returned %d device(s)", len(result.Devices))
	return &result, nil
}

// StartPairing initiates the pairing process with a Fire TV device.
// This is Step 1 of the pairing flow — the TV will display a 6-digit PIN.
// The user must read the PIN and submit it via FinishPairing().
func (c *Client) StartPairing(host string) (*PairResponse, error) {
	log.Printf("📺 Starting pairing with Fire TV at %s...", host)

	// Build the pairing request with just the host (no PIN = start pairing).
	reqBody := PairRequest{Host: host}
	return c.sendPairRequest(reqBody)
}

// FinishPairing completes the pairing process with the PIN shown on the TV.
// This is Step 2 of the pairing flow — submits the user-entered PIN to verify.
// If successful, the device is paired and can receive remote commands.
func (c *Client) FinishPairing(host, pin string) (*PairResponse, error) {
	log.Printf("📺 Finishing pairing with Fire TV at %s (PIN: %s)...", host, pin)

	// Build the pairing request with both host and PIN (PIN present = finish pairing).
	reqBody := PairRequest{Host: host, PIN: pin}
	return c.sendPairRequest(reqBody)
}

// sendPairRequest sends a pairing request to the Python service.
// Used internally by both StartPairing and FinishPairing.
func (c *Client) sendPairRequest(reqBody PairRequest) (*PairResponse, error) {
	// Encode the request body as JSON.
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode pair request: %w", err)
	}

	// Send POST request to the Python service's pair endpoint.
	resp, err := c.httpClient.Post(
		c.baseURL+pairEndpoint,
		"application/json",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Fire TV service: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read pair response: %w", err)
	}

	// Check for error responses (400 = bad PIN, 500 = service error).
	if resp.StatusCode != http.StatusOK {
		var errDetail ErrorDetail
		if json.Unmarshal(body, &errDetail) == nil && errDetail.Detail != "" {
			return nil, fmt.Errorf("pairing failed: %s", errDetail.Detail)
		}
		return nil, fmt.Errorf("pairing failed with status %d", resp.StatusCode)
	}

	var result PairResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse pair response: %w", err)
	}

	log.Printf("📺 Pair response: success=%v, awaiting_pin=%v", result.Success, result.AwaitingPIN)
	return &result, nil
}

// SendCommand sends a remote control command to a paired Fire TV device.
// Supports navigation, media, power, volume, text input, and app launch commands.
// The device must have been previously paired via StartPairing/FinishPairing.
func (c *Client) SendCommand(host, command, text, appPackage string) (*CommandResponse, error) {
	log.Printf("📺 Sending command '%s' to Fire TV at %s", command, host)

	// Build the command request.
	reqBody := CommandRequest{
		Host:       host,
		Command:    command,
		Text:       text,
		AppPackage: appPackage,
	}

	// Encode the request body as JSON.
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode command request: %w", err)
	}

	// Send POST request to the Python service's command endpoint.
	resp, err := c.httpClient.Post(
		c.baseURL+commandEndpoint,
		"application/json",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Fire TV service: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read command response: %w", err)
	}

	// Check for error responses (400 = device offline, 500 = service error).
	if resp.StatusCode != http.StatusOK {
		var errDetail ErrorDetail
		if json.Unmarshal(body, &errDetail) == nil && errDetail.Detail != "" {
			return nil, fmt.Errorf("command failed: %s", errDetail.Detail)
		}
		return nil, fmt.Errorf("command failed with status %d", resp.StatusCode)
	}

	var result CommandResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse command response: %w", err)
	}

	log.Printf("📺 Command response: success=%v, message=%s", result.Success, result.Message)
	return &result, nil
}

// CheckHealth verifies the Python Fire TV microservice is running.
// Returns nil if the service is reachable and healthy, or an error otherwise.
// Used during Go server startup to warn if the Python service isn't running.
func (c *Client) CheckHealth() error {
	resp, err := c.httpClient.Get(c.baseURL + healthEndpoint)
	if err != nil {
		return fmt.Errorf("fire TV service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fire TV service unhealthy (status %d)", resp.StatusCode)
	}

	return nil
}
