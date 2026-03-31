package handlers

import (
	"database/sql"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// RoomTemplateHandler serves default room scene templates.
// Templates define the visual layout of a room — wall/floor colors, furniture
// positions, and interactable device slots. The iOS app uses these to render
// rooms via GenericRoomScene.
//
// Currently returns hardcoded default templates per room name.
// Future prompts can extend this to support per-user customization.
type RoomTemplateHandler struct {
	DB *sql.DB
}

// NewRoomTemplateHandler creates a new RoomTemplateHandler with the given DB.
func NewRoomTemplateHandler(database *sql.DB) *RoomTemplateHandler {
	return &RoomTemplateHandler{DB: database}
}

// =============================================================================
// Template Types — mirror the iOS RoomTemplate model
// =============================================================================

// roomTemplateResponse is the JSON structure for a room scene template.
// Matches Apollo's RoomTemplate Codable struct exactly.
type roomTemplateResponse struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	BackgroundColor string                 `json:"backgroundColor"`
	Description     *string                `json:"description,omitempty"`
	Elements        []roomElementResponse  `json:"elements"`
}

// roomElementResponse is a single visual element in the room.
type roomElementResponse struct {
	ID          string                    `json:"id"`
	Type        string                    `json:"type"`
	Layer       string                    `json:"layer"`
	Position    normalizedPointResponse   `json:"position"`
	Size        normalizedSizeResponse    `json:"size"`
	ZPosition   float64                   `json:"zPosition"`
	Style       roomElementStyleResponse  `json:"style"`
	Interaction *roomInteractionResponse  `json:"interaction,omitempty"`
	Label       *string                   `json:"label,omitempty"`
}

// normalizedPointResponse is a point in 0.0-1.0 space.
type normalizedPointResponse struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// normalizedSizeResponse is a size in 0.0-1.0 space.
type normalizedSizeResponse struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// roomElementStyleResponse defines visual properties for an element.
type roomElementStyleResponse struct {
	FillColor        string   `json:"fillColor"`
	GradientEndColor *string  `json:"gradientEndColor,omitempty"`
	BorderColor      *string  `json:"borderColor,omitempty"`
	BorderWidth      *float64 `json:"borderWidth,omitempty"`
	CornerRadius     *float64 `json:"cornerRadius,omitempty"`
	Opacity          *float64 `json:"opacity,omitempty"`
}

// roomInteractionResponse defines how a tappable element connects to a device.
type roomInteractionResponse struct {
	Type          string  `json:"type"`
	IdleAnimation string  `json:"idleAnimation"`
	StateKey      string  `json:"stateKey"`
	DeviceID      *string `json:"deviceId,omitempty"`
}

// =============================================================================
// Handler
// =============================================================================

// HandleGetRoomTemplate serves GET /api/room/{id}/template
//
// Looks up the room by ID to get its name, then returns the default template
// for that room type. Unknown room names get a minimal generic template.
// Non-existent room IDs return 404.
func (h *RoomTemplateHandler) HandleGetRoomTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "room id is required")
		return
	}

	// Look up the room to get its name.
	room, err := db.GetRoom(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to look up room")
		return
	}

	// Get the default template for this room name.
	template := defaultTemplate(room.Name)

	writeJSON(w, http.StatusOK, template)
}

// =============================================================================
// Default Templates
// =============================================================================

// defaultTemplate returns a hardcoded default template for a given room name.
// Known rooms (Living Room, Office, Bedroom) get full templates.
// Unknown rooms get a minimal template with just wall and floor.
func defaultTemplate(roomName string) roomTemplateResponse {
	switch roomName {
	case "Living Room":
		return livingRoomTemplate()
	case "Office":
		return officeTemplate()
	case "Bedroom":
		return bedroomTemplate()
	default:
		return genericTemplate(roomName)
	}
}

// Helper to create a string pointer (Go doesn't allow &"string").
func strPtr(s string) *string { return &s }
func floatPtr(f float64) *float64 { return &f }

// livingRoomTemplate returns the default Living Room template.
func livingRoomTemplate() roomTemplateResponse {
	desc := "A cozy living room with smart TV, standing lamp, and pet camera."
	return roomTemplateResponse{
		ID:              "living_room",
		Name:            "Living Room",
		BackgroundColor: "#F5E6D3",
		Description:     &desc,
		Elements: []roomElementResponse{
			{ID: "wall", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.625}, Size: normalizedSizeResponse{1.0, 0.75}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#F7EDE2", GradientEndColor: strPtr("#ECDCC8"), CornerRadius: floatPtr(0)}, Label: strPtr("Wall")},
			{ID: "floor", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.125}, Size: normalizedSizeResponse{1.0, 0.25}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#A0784C", GradientEndColor: strPtr("#7A5C3A"), CornerRadius: floatPtr(0)}, Label: strPtr("Floor")},
			{ID: "tv", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.5, 0.55}, Size: normalizedSizeResponse{0.38, 0.16}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#1C1C1E", BorderColor: strPtr("#2C2C2E"), BorderWidth: floatPtr(3), CornerRadius: floatPtr(4)}, Interaction: &roomInteractionResponse{Type: "firetv", IdleAnimation: "shimmer", StateKey: "tvPowered"}, Label: strPtr("Fire TV")},
			{ID: "lamp", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.85, 0.42}, Size: normalizedSizeResponse{0.12, 0.28}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#E8C9A0", BorderColor: strPtr("#D4B08C"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "lampOn"}, Label: strPtr("Standing Lamp")},
			{ID: "camera", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.58, 0.76}, Size: normalizedSizeResponse{0.07, 0.05}, ZPosition: 22, Style: roomElementStyleResponse{FillColor: "#E8E8E8", BorderColor: strPtr("#CCCCCC"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(4)}, Interaction: &roomInteractionResponse{Type: "camera", IdleAnimation: "blink", StateKey: "cameraActive"}, Label: strPtr("Pet Camera")},
		},
	}
}

// officeTemplate returns the default Office template.
func officeTemplate() roomTemplateResponse {
	desc := "A focused workspace with monitor, desk lamp, and security camera."
	return roomTemplateResponse{
		ID:              "office",
		Name:            "Office",
		BackgroundColor: "#E8E4DF",
		Description:     &desc,
		Elements: []roomElementResponse{
			{ID: "wall", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.625}, Size: normalizedSizeResponse{1.0, 0.75}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#E8E4DF", GradientEndColor: strPtr("#D9D3CC"), CornerRadius: floatPtr(0)}, Label: strPtr("Wall")},
			{ID: "floor", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.125}, Size: normalizedSizeResponse{1.0, 0.25}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#8B7D6B", GradientEndColor: strPtr("#6B5D4B"), CornerRadius: floatPtr(0)}, Label: strPtr("Floor")},
			{ID: "monitor", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.48, 0.50}, Size: normalizedSizeResponse{0.30, 0.14}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#1C1C1E", BorderColor: strPtr("#2C2C2E"), BorderWidth: floatPtr(2), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "firetv", IdleAnimation: "shimmer", StateKey: "monitorPowered"}, Label: strPtr("Monitor")},
			{ID: "desk_lamp", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.25, 0.42}, Size: normalizedSizeResponse{0.08, 0.18}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#C8B898", BorderColor: strPtr("#B8A888"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "deskLampOn"}, Label: strPtr("Desk Lamp")},
			{ID: "office_camera", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.88, 0.78}, Size: normalizedSizeResponse{0.07, 0.05}, ZPosition: 22, Style: roomElementStyleResponse{FillColor: "#E0E0E0", BorderColor: strPtr("#C0C0C0"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(4)}, Interaction: &roomInteractionResponse{Type: "camera", IdleAnimation: "blink", StateKey: "officeCameraActive"}, Label: strPtr("Office Camera")},
		},
	}
}

// bedroomTemplate returns the default Bedroom template.
func bedroomTemplate() roomTemplateResponse {
	desc := "A restful bedroom with ambient lighting and smart devices."
	return roomTemplateResponse{
		ID:              "bedroom",
		Name:            "Bedroom",
		BackgroundColor: "#E8E0F0",
		Description:     &desc,
		Elements: []roomElementResponse{
			{ID: "wall", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.625}, Size: normalizedSizeResponse{1.0, 0.75}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#E8E0F0", GradientEndColor: strPtr("#DDD5E8"), CornerRadius: floatPtr(0)}, Label: strPtr("Wall")},
			{ID: "floor", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.125}, Size: normalizedSizeResponse{1.0, 0.25}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#C4B8A8", GradientEndColor: strPtr("#A89888"), CornerRadius: floatPtr(0)}, Label: strPtr("Floor")},
			{ID: "nightstand_lamp", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.82, 0.40}, Size: normalizedSizeResponse{0.08, 0.16}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#D4C4A8", BorderColor: strPtr("#C4B498"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "nightLampOn"}, Label: strPtr("Nightstand Lamp")},
			{ID: "bedroom_camera", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.14, 0.72}, Size: normalizedSizeResponse{0.07, 0.05}, ZPosition: 22, Style: roomElementStyleResponse{FillColor: "#E0E0E0", BorderColor: strPtr("#C0C0C0"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(4)}, Interaction: &roomInteractionResponse{Type: "camera", IdleAnimation: "blink", StateKey: "bedroomCameraActive"}, Label: strPtr("Bedroom Camera")},
			{ID: "led_strip", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.5, 0.54}, Size: normalizedSizeResponse{0.56, 0.02}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#AA88CC", BorderColor: strPtr("#9977BB"), BorderWidth: floatPtr(0.5), CornerRadius: floatPtr(1), Opacity: floatPtr(0.8)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "ledStripOn"}, Label: strPtr("LED Strip")},
		},
	}
}

// genericTemplate returns a minimal template for unknown room names.
// Just wall and floor — no furniture or interactables.
func genericTemplate(roomName string) roomTemplateResponse {
	desc := "A room in your home."
	return roomTemplateResponse{
		ID:              "generic",
		Name:            roomName,
		BackgroundColor: "#E8E4DF",
		Description:     &desc,
		Elements: []roomElementResponse{
			{ID: "wall", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.625}, Size: normalizedSizeResponse{1.0, 0.75}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#E8E4DF", GradientEndColor: strPtr("#D9D3CC"), CornerRadius: floatPtr(0)}, Label: strPtr("Wall")},
			{ID: "floor", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.125}, Size: normalizedSizeResponse{1.0, 0.25}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#8B7D6B", GradientEndColor: strPtr("#6B5D4B"), CornerRadius: floatPtr(0)}, Label: strPtr("Floor")},
		},
	}
}
