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
// Includes branding fields (logo, accent color, hero image) shared across all locations.
type Business struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	OwnerName      *string   `json:"ownerName,omitempty"`      // optional shop manager/owner name
	WelcomeMessage *string   `json:"welcomeMessage,omitempty"` // short friendly message from owner to customers (max 500 chars)
	Website        *string   `json:"website,omitempty"`        // business website URL
	Phone          *string   `json:"phone,omitempty"`          // business contact phone (E.164 format recommended)
	LogoURL        *string   `json:"logoUrl,omitempty"`        // URL to business logo image
	AccentColor    *string   `json:"accentColor,omitempty"`    // brand accent color in hex format (#RRGGBB)
	HeroImageURL   *string   `json:"heroImageUrl,omitempty"`   // URL to hero/banner image for the business
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Location represents a physical location belonging to a business.
// For multi-location businesses (e.g. "Downtown Store", "Airport Branch").
// Each location can have multiple rooms with beacon configurations.
// Includes structured address fields with geocoding, timezone, and opening hours.
type Location struct {
	ID           string    `json:"id"`
	BusinessID   string    `json:"businessId"`
	Name         string    `json:"name"`
	Address      *string   `json:"address,omitempty"`  // DEPRECATED: use structured address fields instead
	Metadata     *string   `json:"metadata,omitempty"` // JSON blob for location-specific config
	// Structured address fields for geocoding and display
	Street       *string   `json:"street,omitempty"`
	Street2      *string   `json:"street2,omitempty"`       // suite, unit, etc.
	City         *string   `json:"city,omitempty"`
	State        *string   `json:"state,omitempty"`         // state or province code
	PostalCode   *string   `json:"postalCode,omitempty"`
	Country      *string   `json:"country,omitempty"`       // ISO 3166-1 alpha-2 country code
	Latitude     *float64  `json:"latitude,omitempty"`      // -90 to 90
	Longitude    *float64  `json:"longitude,omitempty"`     // -180 to 180
	// Timezone and hours for isOpenNow computation
	Timezone      *string   `json:"timezone,omitempty"`      // IANA timezone identifier (e.g. "America/Los_Angeles")
	OpeningHours  *string   `json:"openingHours,omitempty"`  // JSON blob following OpeningHours schema
	SpecialHours  *string   `json:"specialHours,omitempty"`  // JSON array following SpecialHours schema
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
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

// OpeningHours represents weekly opening hours with support for multiple ranges per day.
// This structure is stored as JSON in the Location.OpeningHours field.
// Each day can have multiple time ranges (e.g., for split shifts like breakfast and dinner).
// A null day or empty ranges array means the location is closed all day.
type OpeningHours struct {
	Monday    *DayHours `json:"monday,omitempty"`
	Tuesday   *DayHours `json:"tuesday,omitempty"`
	Wednesday *DayHours `json:"wednesday,omitempty"`
	Thursday  *DayHours `json:"thursday,omitempty"`
	Friday    *DayHours `json:"friday,omitempty"`
	Saturday  *DayHours `json:"saturday,omitempty"`
	Sunday    *DayHours `json:"sunday,omitempty"`
}

// DayHours represents opening hours for a single day.
// Null or empty ranges means closed all day.
// Multiple ranges support split shifts (e.g., 07:00-11:00 and 17:00-22:00).
type DayHours struct {
	Ranges []TimeRange `json:"ranges"` // empty array = closed all day
}

// TimeRange represents a single continuous open time range.
// Times are in 24-hour HH:MM format (e.g., "09:00", "21:00").
type TimeRange struct {
	Open  string `json:"open"`  // opening time in HH:MM format
	Close string `json:"close"` // closing time in HH:MM format
}

// SpecialHours represents special hours override for holidays or events.
// These override regular opening hours for specific dates.
// This structure is stored as a JSON array in the Location.SpecialHours field.
type SpecialHours struct {
	Date   string    `json:"date"`   // date in YYYY-MM-DD format
	Reason string    `json:"reason"` // reason for special hours (e.g., "Christmas Day")
	Hours  *DayHours `json:"hours"`  // hours for this day (null means closed all day)
}

// =============================================================================
// Product Catalog Models (Food/Drink Menu System)
// =============================================================================
//
// The product catalog is the food and drink menu system for shop mode.
// This is COMPLETELY SEPARATE from the shop menu system (MenuOption, Menu, etc.)
//
// Shop menu = environment-based actions (WiFi, URLs, room controls)
// Product catalog = browsing and ordering food/drink items
//
// Hierarchy:
// - Business → ProductMenu (e.g., "Breakfast", "Lunch", "Dinner")
// - ProductMenu → Category (e.g., "Appetizers", "Entrees", "Desserts")
// - Category → Item (e.g., "Caesar Salad", "Grilled Salmon")
// - Item → ModifierGroup (e.g., "Size", "Add-ons")
// - ModifierGroup → Modifier (e.g., "Small", "Medium", "Large")

// ProductMenu represents a collection of categories and items that a business offers.
// A business can have multiple menus (e.g., breakfast, lunch, dinner) activated at
// different times or locations. This is separate from the Menu table (shop menu system).
type ProductMenu struct {
	ID           string    `json:"id"`
	BusinessID   string    `json:"businessId"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	IsActive     bool      `json:"isActive"`
	DisplayOrder int       `json:"displayOrder"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Category represents a grouping of related items within a product menu
// (e.g., "Appetizers", "Entrees", "Desserts"). Categories are displayed in order.
type Category struct {
	ID           string    `json:"id"`
	MenuID       string    `json:"menuId"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	IsAvailable  bool      `json:"isAvailable"`
	DisplayOrder int       `json:"displayOrder"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Item represents a specific product (food or drink) that customers can order.
// Items belong to a category and can have modifier groups for customization.
// Prices are stored in minor currency units (cents for USD) to avoid floating point issues.
type Item struct {
	ID               string    `json:"id"`
	CategoryID       string    `json:"categoryId"`
	Name             string    `json:"name"`
	Description      *string   `json:"description,omitempty"`
	PriceMinorUnits  int       `json:"priceMinorUnits"`  // cents for USD
	CurrencyCode     string    `json:"currencyCode"`     // ISO 4217 currency code
	ImageURLs        *string   `json:"imageUrls,omitempty"` // JSON array of image URLs
	DietaryTags      *string   `json:"dietaryTags,omitempty"` // JSON array of dietary tags
	IsAvailable      bool      `json:"isAvailable"`
	IsSoldOut        bool      `json:"isSoldOut"`
	DisplayOrder     int       `json:"displayOrder"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// ModifierGroup represents a set of options for customizing an item
// (e.g., "Size", "Add-ons", "Preparation"). Each group has min/max selection rules.
type ModifierGroup struct {
	ID            string    `json:"id"`
	ItemID        string    `json:"itemId"`
	Name          string    `json:"name"`
	MinSelections int       `json:"minSelections"`
	MaxSelections int       `json:"maxSelections"`
	IsAvailable   bool      `json:"isAvailable"`
	DisplayOrder  int       `json:"displayOrder"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Modifier represents a specific option within a modifier group
// (e.g., "Small", "Medium", "Large" for a Size group). Modifiers can have
// price adjustments (positive or negative).
type Modifier struct {
	ID                        string    `json:"id"`
	ModifierGroupID           string    `json:"modifierGroupId"`
	Name                      string    `json:"name"`
	PriceAdjustmentMinorUnits int       `json:"priceAdjustmentMinorUnits"` // can be negative
	IsAvailable               bool      `json:"isAvailable"`
	IsDefault                 bool      `json:"isDefault"`
	DisplayOrder              int       `json:"displayOrder"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// =============================================================================
// Reviews Models
// =============================================================================

// Review represents a customer review of a business.
// Reviews are business-level (not product-level) as this fits food trucks,
// market stalls, and small food businesses where the overall experience matters
// more than individual products.
// A business can reply to reviews (one reply per review).
// Reviews can be flagged by business admins as inappropriate, but flagged
// reviews remain visible (no auto-hiding or approval workflow yet).
type Review struct {
	ID                string    `json:"id"`
	CustomerID        string    `json:"customerId"`        // UUID of the customer who wrote the review
	BusinessID        string    `json:"businessId"`        // UUID of the business being reviewed
	Rating            int       `json:"rating"`            // Star rating 1-5
	ReviewText        *string   `json:"reviewText,omitempty"` // Optional review text
	PhotoURLs         []string  `json:"photoUrls,omitempty"`  // Optional array of photo URLs (stored as JSON in DB)
	BusinessReply     *BusinessReply `json:"businessReply,omitempty"` // Optional reply from business
	Flagged           bool      `json:"flagged"`           // Whether flagged as inappropriate
	FlaggedAt         *time.Time `json:"flaggedAt,omitempty"` // When flagged (null if not flagged)
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// BusinessReply represents a business's reply to a review.
// This is a nested object within the Review model.
// Each review can have at most one reply from the business.
// Any business admin can add, update, or delete a reply for flexibility.
type BusinessReply struct {
	ReplyText  string    `json:"replyText"`
	RepliedAt  time.Time `json:"repliedAt"`
	RepliedBy  string    `json:"repliedBy"` // UUID of the admin who replied
}

// =============================================================================
// Customer and Arrival Models
// =============================================================================

// Customer represents a customer who interacts with businesses through the app.
// Customers can visit locations, make purchases, and control what information
// businesses can see through their sharing settings.
type Customer struct {
	ID        string    `json:"id"`
	FirstName string    `json:"firstName"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CustomerSharingSettings represents a customer's privacy preferences for sharing
// information with businesses. Controls what data is visible in customer profiles.
// Sharing levels:
//   - minimal: First name only (default for new customers)
//   - standard: First name + purchase history summary + loyalty status
//   - full: All available data including review count
type CustomerSharingSettings struct {
	CustomerID        string    `json:"customerId"`
	SharingLevel      string    `json:"sharingLevel"` // "minimal", "standard", or "full"
	BlockedBusinesses []string  `json:"blockedBusinesses"` // list of business UUIDs where sharing is disabled
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// ArrivalEvent represents a customer's arrival at a business location, detected
// via BLE beacon proximity. Events track the customer's state (entered, dwelling, exited)
// and proximity band (immediate, near, far) as they move through the location.
// These events are broadcast to kiosks via SSE for real-time customer arrival notifications.
type ArrivalEvent struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customerId"`
	BusinessID string    `json:"businessId"` // resolved from beacon
	LocationID string    `json:"locationId"` // resolved from beacon
	BeaconUUID string    `json:"beaconUuid"` // iBeacon proximity UUID
	BeaconMajor int      `json:"beaconMajor"` // iBeacon major value
	BeaconMinor int      `json:"beaconMinor"` // iBeacon minor value
	State      string    `json:"state"`       // "entered", "dwelling", or "exited"
	Proximity  string    `json:"proximity"`   // "immediate", "near", or "far"
	Timestamp  time.Time `json:"timestamp"`   // when the event occurred on the device
	CreatedAt  time.Time `json:"createdAt"`   // when the event was recorded by the backend
}
