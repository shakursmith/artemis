package firetv

// Fire TV Remote data structures for communicating with the Python microservice.
// These models mirror the Pydantic models defined in the Python service (firestick/models.py).
// The Go backend proxies requests from the iOS app to the Python service and transforms
// responses back to JSON for the frontend.

// DiscoveredDevice represents a Fire TV device found on the local network.
// Returned by the Python service's GET /discover endpoint via mDNS/Zeroconf scanning.
type DiscoveredDevice struct {
	Name  string `json:"name"`            // Device name from mDNS advertisement (e.g., "Living Room Fire TV")
	Host  string `json:"host"`            // Device IP address on the LAN (e.g., "192.168.1.50")
	Port  int    `json:"port"`            // Android TV Remote service port (usually 6466)
	Model string `json:"model,omitempty"` // Device model from mDNS TXT records (may be empty)
}

// DiscoverResponse is the response from the Python service's /discover endpoint.
// Contains the list of all Fire TV devices found during the network scan.
type DiscoverResponse struct {
	Success bool               `json:"success"` // Whether the scan completed without errors
	Devices []DiscoveredDevice `json:"devices"` // List of discovered Fire TV devices
	Message string             `json:"message"` // Human-readable status message (e.g., "Found 2 device(s)")
}

// PairRequest is sent to the Python service to start or complete pairing.
// Two-step flow:
//   - Step 1: Send with just Host → TV displays a PIN.
//   - Step 2: Send with Host + PIN → Completes pairing.
type PairRequest struct {
	Host string `json:"host"`           // IP address of the Fire TV device to pair with
	PIN  string `json:"pin,omitempty"`  // 6-digit PIN displayed on the TV (empty for step 1)
}

// PairResponse is the response from the Python service's /pair endpoint.
type PairResponse struct {
	Success     bool   `json:"success"`                // Whether the pairing step succeeded
	Message     string `json:"message"`                // Status message for the user
	DeviceName  string `json:"device_name,omitempty"`  // Device name (populated after successful pairing)
	AwaitingPIN bool   `json:"awaiting_pin"`           // True when the TV is displaying a PIN
}

// CommandRequest is sent to the Python service to execute a remote command.
// Supports three types of commands:
//   - Standard key commands: Set Command to a key name (e.g., "home", "play_pause")
//   - Text input: Set Command to "text_input" and provide Text field
//   - App launch: Set Command to "launch_app" and provide AppPackage field
type CommandRequest struct {
	Host       string `json:"host"`                    // IP address of the target Fire TV device
	Command    string `json:"command"`                 // Command name (e.g., "home", "up", "text_input")
	Text       string `json:"text,omitempty"`          // Text to send (for "text_input" command)
	AppPackage string `json:"app_package,omitempty"`   // Android package name (for "launch_app" command)
}

// CommandResponse is the response from the Python service's /command endpoint.
type CommandResponse struct {
	Success bool   `json:"success"` // Whether the command was sent successfully
	Message string `json:"message"` // Status message (e.g., "Sent command: home (HOME)")
	Command string `json:"command"` // Echo of the command that was executed
}

// ErrorDetail is returned by the Python service when a request fails.
// FastAPI wraps errors in a {"detail": "message"} format.
type ErrorDetail struct {
	Detail string `json:"detail"` // Error message from the Python service
}
