package db

import "time"

// Profile represents a user's profile in the system.
// Each profile owns a set of rooms and devices.
// For now there's typically one profile per app install, but the schema
// supports multiple profiles for future multi-user or household features.
type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Room represents a physical space the user has set up (e.g. "Living Room", "Office").
// Rooms belong to either a profile (home mode) or a location (shop mode).
// Can optionally have BLE beacon configuration for automatic proximity-based detection.
type Room struct {
	ID             string    `json:"id"`
	ProfileID      *string   `json:"profileId,omitempty"`        // nullable — nil for shop mode rooms
	Name           string    `json:"name"`
	Icon           string    `json:"icon"`                       // SF Symbol name for the room icon
	BeaconUUID     *string   `json:"beaconUuid,omitempty"`       // iBeacon proximity UUID
	BeaconMajor    *int      `json:"beaconMajor,omitempty"`      // iBeacon major value
	BeaconMinor    *int      `json:"beaconMinor,omitempty"`      // iBeacon minor value
	CustomTemplate *string   `json:"customTemplate,omitempty"`   // User-customized scene layout JSON
	LocationID     *string   `json:"locationId,omitempty"`       // nullable — for shop mode rooms
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Device represents a smart device the user has registered.
// Devices belong to a profile and can optionally be assigned to a room.
// The device_type field maps to integration handlers (govee_light, fire_tv, etc.)
// and external_id connects to the third-party service's device identifier.
type Device struct {
	ID         string  `json:"id"`
	ProfileID  string  `json:"profileId"`
	RoomID     *string `json:"roomId,omitempty"`     // nullable — unassigned devices have no room
	Name       string  `json:"name"`                 // user-given friendly name
	DeviceType string  `json:"deviceType"`           // "govee_light", "fire_tv", "wyze_camera", "generic"
	ExternalID *string `json:"externalId,omitempty"` // ID from the external service (e.g. Govee device ID)
	Model      *string `json:"model,omitempty"`      // device model string from the service
	Metadata   *string `json:"metadata,omitempty"`   // JSON blob for extra device-specific data
	BusinessID *string `json:"businessId,omitempty"` // nullable — for shop mode devices
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Business represents a business entity in shop mode.
// This is the equivalent of Profile for commercial/retail use cases.
// A business can have multiple locations (for chains/franchises).
type Business struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerName *string   `json:"ownerName,omitempty"` // optional shop manager/owner name
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Location represents a physical location belonging to a business.
// For multi-location businesses (e.g. "Downtown Store", "Airport Branch").
// Each location can have multiple rooms with beacon configurations.
type Location struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"businessId"`
	Name       string    `json:"name"`
	Address    *string   `json:"address,omitempty"`  // optional physical address
	Metadata   *string   `json:"metadata,omitempty"` // JSON blob for location-specific config
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// RoomTrigger represents an automated workflow triggered by beacon events.
// Triggers execute actions (notifications, device control, menu display)
// when users enter, exit, or dwell in a room.
type RoomTrigger struct {
	ID           string    `json:"id"`
	RoomID       string    `json:"roomId"`
	TriggerType  string    `json:"triggerType"`            // "entry", "exit", "dwell"
	DwellSeconds *int      `json:"dwellSeconds,omitempty"` // required for dwell triggers
	ActionType   string    `json:"actionType"`             // "notification", "display_menu", "device_control"
	ActionConfig string    `json:"actionConfig"`           // JSON blob with action-specific config
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
