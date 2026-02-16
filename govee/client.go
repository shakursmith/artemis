package govee

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	// Govee Developer API base URL
	// All API requests go to this base + endpoint path
	baseURL = "https://developer-api.govee.com"

	// API endpoints
	devicesEndpoint = "/v1/devices"         // GET - list all devices
	controlEndpoint = "/v1/devices/control" // PUT - control a device
	stateEndpoint   = "/v1/devices/state"   // GET - query device state

	// HTTP timeout for API requests
	// Govee API typically responds within 1-2 seconds
	requestTimeout = 10 * time.Second
)

// Client handles all communication with the Govee Developer API
// It maintains the API key and HTTP client for making requests
type Client struct {
	apiKey     string       // Govee API key from developer.govee.com
	httpClient *http.Client // Reusable HTTP client with timeout
}

// NewClient creates a new Govee API client with the provided API key
// The API key can be obtained from https://developer.govee.com
// after creating an application in the developer portal
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// GetDevices retrieves all Govee devices associated with the API key
// Returns a list of devices with their capabilities and support commands
// This should be called once on app startup to discover available devices
func (c *Client) GetDevices() ([]Device, error) {
	log.Println("💡 Fetching Govee devices...")

	// Create GET request to devices endpoint
	req, err := http.NewRequest("GET", baseURL+devicesEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add required Govee API key header
	// Without this header, the API returns 401 Unauthorized
	req.Header.Set("Govee-API-Key", c.apiKey)

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch devices: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("govee API error (code %d): %s", errResp.Code, errResp.Message)
		}
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var devicesResp DevicesResponse
	if err := json.Unmarshal(body, &devicesResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("💡 Found %d Govee device(s)", len(devicesResp.Data.Devices))
	return devicesResp.Data.Devices, nil
}

// GetDeviceState queries the current state of a Govee device
// Returns the device's current power state (on/off), brightness, color, etc.
// deviceID: Device MAC address from GetDevices()
// model: Device model number from GetDevices()
func (c *Client) GetDeviceState(deviceID, model string) (*DeviceStateResponse, error) {
	// Build URL with query parameters
	// The Govee state endpoint requires device and model as query params
	url := fmt.Sprintf("%s%s?device=%s&model=%s", baseURL, stateEndpoint, deviceID, model)

	// Create GET request to state endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add required Govee API key header
	req.Header.Set("Govee-API-Key", c.apiKey)

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query device state: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("govee API error (code %d): %s", errResp.Code, errResp.Message)
		}
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var stateResp DeviceStateResponse
	if err := json.Unmarshal(body, &stateResp); err != nil {
		return nil, fmt.Errorf("failed to parse state response: %w", err)
	}

	return &stateResp, nil
}

// TurnOn turns on a Govee device
// deviceID: Device MAC address from GetDevices()
// model: Device model number from GetDevices()
func (c *Client) TurnOn(deviceID, model string) error {
	log.Printf("💡 Turning ON device %s", deviceID)
	return c.sendControlCommand(deviceID, model, "turn", "on")
}

// TurnOff turns off a Govee device
// deviceID: Device MAC address from GetDevices()
// model: Device model number from GetDevices()
func (c *Client) TurnOff(deviceID, model string) error {
	log.Printf("💡 Turning OFF device %s", deviceID)
	return c.sendControlCommand(deviceID, model, "turn", "off")
}

// SetBrightness sets the brightness level of a Govee device
// deviceID: Device MAC address from GetDevices()
// model: Device model number from GetDevices()
// level: Brightness level from 0 (dimmest) to 100 (brightest)
//
// Note: Only works if device.SupportCmds contains "brightness"
func (c *Client) SetBrightness(deviceID, model string, level int) error {
	// Validate brightness range
	if level < 0 || level > 100 {
		return fmt.Errorf("brightness must be between 0 and 100, got %d", level)
	}

	log.Printf("💡 Setting brightness to %d for device %s", level, deviceID)
	return c.sendControlCommand(deviceID, model, "brightness", level)
}

// SetColor sets the RGB color of a Govee device
// deviceID: Device MAC address from GetDevices()
// model: Device model number from GetDevices()
// r, g, b: RGB color channels, each from 0 to 255
//
// Note: Only works if device.SupportCmds contains "color"
func (c *Client) SetColor(deviceID, model string, r, g, b int) error {
	// Validate RGB values
	if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
		return fmt.Errorf("RGB values must be between 0 and 255, got R=%d G=%d B=%d", r, g, b)
	}

	log.Printf("💡 Setting color to RGB(%d, %d, %d) for device %s", r, g, b, deviceID)

	// Create color value struct
	color := ColorValue{R: r, G: g, B: b}
	return c.sendControlCommand(deviceID, model, "color", color)
}

// sendControlCommand is the internal method that sends control commands to Govee API
// It handles creating the request, setting headers, and parsing the response
//
// cmdName: Command name ("turn", "brightness", "color", "colorTem")
// value: Command-specific value (string, int, or ColorValue struct)
func (c *Client) sendControlCommand(deviceID, model, cmdName string, value interface{}) error {
	// Build control request payload
	// The Govee API requires device, model, and cmd fields
	controlReq := ControlRequest{
		Device: deviceID,
		Model:  model,
		Cmd: ControlCommand{
			Name:  cmdName,
			Value: value,
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(controlReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create PUT request to control endpoint
	// The Govee API uses PUT (not POST) for control commands
	req, err := http.NewRequest("PUT", baseURL+controlEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Govee-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send control command: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return fmt.Errorf("govee API error (code %d): %s", errResp.Code, errResp.Message)
		}
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var controlResp ControlResponse
	if err := json.Unmarshal(body, &controlResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Check API response code
	if controlResp.Code != 200 {
		return fmt.Errorf("govee API error: %s (code %d)", controlResp.Message, controlResp.Code)
	}

	log.Printf("💡 Control command successful: %s", controlResp.Message)
	return nil
}
