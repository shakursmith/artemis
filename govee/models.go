package govee

// Device represents a Govee smart device (e.g., light bulb, LED strip)
// The Govee API returns devices with these fields when calling GET /v1/devices
type Device struct {
	// Device MAC address or unique identifier from Govee
	// Format: "AA:BB:CC:DD:EE:FF:GG:HH"
	Device string `json:"device"`

	// Model number of the device (e.g., "H6159", "H6046")
	// Different models support different capabilities
	Model string `json:"model"`

	// User-friendly name set in Govee Home app (e.g., "Living Room Light")
	DeviceName string `json:"deviceName"`

	// Whether device can be controlled (true for most devices)
	Controllable bool `json:"controllable"`

	// Whether device can be queried for state (not all devices support this)
	Retrievable bool `json:"retrievable"`

	// List of supported commands: "turn", "brightness", "color", "colorTem"
	// Not all devices support all commands - check this before sending commands
	SupportCmds []string `json:"supportCmds"`
}

// DevicesResponse is the wrapper returned by GET /v1/devices endpoint
// The Govee API wraps the device list in a nested structure
type DevicesResponse struct {
	Data struct {
		Devices []Device `json:"devices"`
	} `json:"data"`
	Message string `json:"message"` // Success message or error description
	Code    int    `json:"code"`    // Response code: 200 = success
}

// ControlRequest is sent to PUT /v1/devices/control to change device state
// The cmd field determines what the value should be:
// - "turn": value = "on" or "off"
// - "brightness": value = integer 0-100
// - "color": value = {"r": 0-255, "g": 0-255, "b": 0-255}
// - "colorTem": value = integer 2000-9000 (Kelvin temperature)
type ControlRequest struct {
	// Device MAC address to control
	Device string `json:"device"`

	// Model number (required by Govee API for some commands)
	Model string `json:"model"`

	// Command to execute - must be in device's SupportCmds list
	Cmd ControlCommand `json:"cmd"`
}

// ControlCommand specifies what action to perform and its parameters
type ControlCommand struct {
	// Command name: "turn", "brightness", "color", "colorTem"
	Name string `json:"name"`

	// Command value - type varies by command:
	// - turn: string "on" or "off"
	// - brightness: int 0-100
	// - color: ColorValue{R, G, B}
	// - colorTem: int 2000-9000
	Value interface{} `json:"value"`
}

// ColorValue represents RGB color for "color" command
// Each channel ranges from 0-255
type ColorValue struct {
	R int `json:"r"` // Red channel (0-255)
	G int `json:"g"` // Green channel (0-255)
	B int `json:"b"` // Blue channel (0-255)
}

// ControlResponse is returned by PUT /v1/devices/control
// Indicates success or failure of the control command
type ControlResponse struct {
	Code    int    `json:"code"`    // 200 = success, 400 = bad request, 401 = unauthorized
	Message string `json:"message"` // Success message or error description
	Data    struct {
	} `json:"data"` // Usually empty on success
}

// ErrorResponse represents an error returned by the Govee API
// Common error codes:
// - 400: Invalid request (bad device ID, unsupported command, etc.)
// - 401: Invalid API key
// - 429: Rate limit exceeded (max 60 requests/minute)
// - 500: Govee server error
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// DeviceStateResponse is returned by GET /v1/devices/state endpoint
// Contains the current state of a device (on/off, brightness, color, etc.)
type DeviceStateResponse struct {
	Data struct {
		Device     string                 `json:"device"`     // Device MAC address
		Model      string                 `json:"model"`      // Device model
		Properties []map[string]interface{} `json:"properties"` // Array of property objects with varying keys
	} `json:"data"`
	Message string `json:"message"` // Success message or error description
	Code    int    `json:"code"`    // Response code: 200 = success
}
