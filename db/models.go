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

// MenuOptionTemplate represents a global template for menu options.
// Templates can be system-provided (wifi, url) or business-created.
// Other businesses can copy templates to create their own menu options.
type MenuOptionTemplate struct {
	ID                 string    `json:"id"`
	CreatedByBusinessID *string  `json:"createdByBusinessId,omitempty"` // nullable for system templates
	OptionType         string    `json:"optionType"`                    // "wifi", "url", "qr_code", etc.
	DefaultTitle       string    `json:"defaultTitle"`                  // default title for this template
	DefaultIcon        string    `json:"defaultIcon"`                   // SF Symbol name for icon
	DataSchema         string    `json:"dataSchema"`                    // JSON schema defining required fields
	Description        string    `json:"description"`                   // description of what this template does
	IsSystem           bool      `json:"isSystem"`                      // true for built-in templates
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// MenuOption represents a business-specific menu option (copied from a template).
// Contains actual data like WiFi credentials or menu URLs.
type MenuOption struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"businessId"`
	TemplateID *string   `json:"templateId,omitempty"` // nullable — can create custom options without a template
	OptionType string    `json:"optionType"`           // "wifi", "url", etc. — immutable after creation
	Title      string    `json:"title"`                // customizable title for this option
	Icon       string    `json:"icon"`                 // SF Symbol name, customizable
	Data       string    `json:"data"`                 // JSON blob with actual credentials/URLs/etc.
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Menu represents a menu for a specific room.
// One-to-one relationship: each room has exactly one menu.
// Includes appearance customization fields.
type Menu struct {
	ID             string    `json:"id"`
	RoomID         string    `json:"roomId"`         // unique — each room has one menu
	IsEnabled      bool      `json:"isEnabled"`      // whether the menu is shown when entering the room
	MenuColor      string    `json:"menuColor"`      // hex color for menu background (e.g., "#000000")
	MenuOpacity    float64   `json:"menuOpacity"`    // opacity for menu background (0.0 to 1.0)
	TextColor      string    `json:"textColor"`      // hex color for menu text (e.g., "#FFFFFF")
	IconColor      string    `json:"iconColor"`      // hex color for menu icons (e.g., "#FF9500")
	CollapsedIcon  string    `json:"collapsedIcon"`  // SF Symbol name for collapsed state icon
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// MenuOptionAssignment represents a menu option assigned to a room's menu.
// Junction table with ordering — manages the many-to-many relationship
// between menus and menu options.
type MenuOptionAssignment struct {
	ID           string    `json:"id"`
	MenuID       string    `json:"menuId"`
	MenuOptionID string    `json:"menuOptionId"`
	Position     int       `json:"position"`  // display order (0-indexed)
	CreatedAt    time.Time `json:"createdAt"`
}

// WiFiData represents the typed data structure for WiFi menu options.
// This is what gets stored in the MenuOption.Data field (as JSON).
type WiFiData struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

// URLData represents the typed data structure for URL menu options.
// This is what gets stored in the MenuOption.Data field (as JSON).
type URLData struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"` // optional display title
}
