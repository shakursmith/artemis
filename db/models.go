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
// Rooms belong to a profile and can optionally have BLE beacon configuration
// for automatic proximity-based detection.
type Room struct {
	ID          string  `json:"id"`
	ProfileID   string  `json:"profileId"`
	Name        string  `json:"name"`
	Icon        string  `json:"icon"`                    // SF Symbol name for the room icon
	BeaconUUID  *string `json:"beaconUuid,omitempty"`    // iBeacon proximity UUID
	BeaconMajor *int    `json:"beaconMajor,omitempty"`   // iBeacon major value
	BeaconMinor *int    `json:"beaconMinor,omitempty"`   // iBeacon minor value
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
