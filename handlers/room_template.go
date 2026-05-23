package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
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
	CanvasScale     *float64               `json:"canvasScale,omitempty"`
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
// DeviceID links this scene element to a specific registered device, enabling
// tap-to-toggle to control exactly that physical device (e.g., a specific Govee bulb).
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
// If the room has a custom_template saved (from the scene editor), that JSON
// is returned directly. Otherwise, falls through to the default hardcoded template
// based on the room name. Non-existent room IDs return 404.
func (h *RoomTemplateHandler) HandleGetRoomTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "room id is required")
		return
	}

	// Look up the room to get its name and custom template.
	room, err := db.GetRoom(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to look up room")
		return
	}

	// If the room has a custom template, return it directly.
	// The custom template is already valid JSON in the response format.
	if room.CustomTemplate != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(*room.CustomTemplate))
		return
	}

	// Fall back to the default template for this room name.
	template := defaultTemplate(room.Name)
	writeJSON(w, http.StatusOK, template)
}

// HandleSaveRoomTemplate serves PUT /api/room/{id}/template
//
// Validates that the request body is valid JSON, then saves it as the room's
// custom template. The next GET /api/room/{id}/template will return this
// custom template instead of the default.
func (h *RoomTemplateHandler) HandleSaveRoomTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "room id is required")
		return
	}

	// Read the request body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Validate that the body is valid JSON.
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	// Save the template JSON to the database.
	templateJSON := string(body)
	if err := db.SaveRoomTemplate(h.DB, id, templateJSON); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save template")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "template saved successfully",
	})
}

// HandleDeleteRoomTemplate serves DELETE /api/room/{id}/template
//
// Clears the room's custom template, reverting to the default template.
// The next GET /api/room/{id}/template will return the default template
// for the room's name.
func (h *RoomTemplateHandler) HandleDeleteRoomTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "room id is required")
		return
	}

	if err := db.ClearRoomTemplate(h.DB, id); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to clear template")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "template reset to default",
	})
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
// Uses canvasScale 2.5 for a panoramic view — the office has two desks and
// five Govee bulbs spread across the room, so the scene needs to be wider
// than the phone screen to capture the full layout.
func officeTemplate() roomTemplateResponse {
	desc := "A focused workspace with two desks, five Govee lights, and a security camera."
	// Device IDs reference the seeded devices in db/seed.go.
	shakurDesk := "ff342a65-411e-4cc0-9c3f-b4aa12d3a2dc"
	shakurLamp := "e56f0365-af78-4b02-9f80-ad3a73987463"
	officeLampBottom := "636618e1-0a19-470e-b22d-7645853cffc9"
	officeLampTop := "d2069915-b2e2-4737-92bd-f569ed3ade55"
	najahDesk := "7ae7296b-988b-49e4-a67b-05da2a549ced"

	return roomTemplateResponse{
		ID:              "office",
		Name:            "Office",
		BackgroundColor: "#E8E4DF",
		Description:     &desc,
		CanvasScale:     floatPtr(2.5),
		Elements: []roomElementResponse{
			// Background
			{ID: "wall", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.625}, Size: normalizedSizeResponse{1.0, 0.75}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#E8E4DF", GradientEndColor: strPtr("#D9D3CC"), CornerRadius: floatPtr(0)}, Label: strPtr("Wall")},
			{ID: "floor", Type: "decoration", Layer: "background", Position: normalizedPointResponse{0.5, 0.125}, Size: normalizedSizeResponse{1.0, 0.25}, ZPosition: 0, Style: roomElementStyleResponse{FillColor: "#8B7D6B", GradientEndColor: strPtr("#6B5D4B"), CornerRadius: floatPtr(0)}, Label: strPtr("Floor")},

			// Furniture — two desks and two chairs spread across the panorama
			{ID: "desk_left", Type: "decoration", Layer: "furniture", Position: normalizedPointResponse{0.22, 0.30}, Size: normalizedSizeResponse{0.28, 0.07}, ZPosition: 10, Style: roomElementStyleResponse{FillColor: "#6B5B4A", BorderColor: strPtr("#5A4A39"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Label: strPtr("Shakur's Desk")},
			{ID: "desk_right", Type: "decoration", Layer: "furniture", Position: normalizedPointResponse{0.78, 0.30}, Size: normalizedSizeResponse{0.28, 0.07}, ZPosition: 10, Style: roomElementStyleResponse{FillColor: "#6B5B4A", BorderColor: strPtr("#5A4A39"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Label: strPtr("Najah's Desk")},
			{ID: "office_chair_left", Type: "decoration", Layer: "furniture", Position: normalizedPointResponse{0.22, 0.18}, Size: normalizedSizeResponse{0.14, 0.12}, ZPosition: 25, Style: roomElementStyleResponse{FillColor: "#3A3A3C", BorderColor: strPtr("#2C2C2E"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(6)}, Label: strPtr("Office Chair")},
			{ID: "office_chair_right", Type: "decoration", Layer: "furniture", Position: normalizedPointResponse{0.78, 0.18}, Size: normalizedSizeResponse{0.14, 0.12}, ZPosition: 25, Style: roomElementStyleResponse{FillColor: "#3A3A3C", BorderColor: strPtr("#2C2C2E"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(6)}, Label: strPtr("Office Chair")},
			{ID: "bookshelf", Type: "decoration", Layer: "furniture", Position: normalizedPointResponse{0.50, 0.52}, Size: normalizedSizeResponse{0.12, 0.38}, ZPosition: 8, Style: roomElementStyleResponse{FillColor: "#7A6A58", BorderColor: strPtr("#695A48"), BorderWidth: floatPtr(1.5), CornerRadius: floatPtr(2)}, Label: strPtr("Bookshelf")},
			{ID: "whiteboard", Type: "decoration", Layer: "decoration", Position: normalizedPointResponse{0.50, 0.72}, Size: normalizedSizeResponse{0.18, 0.14}, ZPosition: 15, Style: roomElementStyleResponse{FillColor: "#F5F5F0", BorderColor: strPtr("#CCCCCC"), BorderWidth: floatPtr(2), CornerRadius: floatPtr(2)}, Label: strPtr("Whiteboard")},

			// Interactables — monitor, five Govee lights, camera
			{ID: "monitor", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.22, 0.50}, Size: normalizedSizeResponse{0.14, 0.10}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#1C1C1E", BorderColor: strPtr("#2C2C2E"), BorderWidth: floatPtr(2), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "firetv", IdleAnimation: "shimmer", StateKey: "monitorPowered"}, Label: strPtr("Monitor")},
			{ID: "desk_light_shakur", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.12, 0.42}, Size: normalizedSizeResponse{0.06, 0.14}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#C8B898", BorderColor: strPtr("#B8A888"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "shakurDeskLightOn", DeviceID: &shakurDesk}, Label: strPtr("Shakur's Desk")},
			{ID: "desk_lamp_shakur", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.32, 0.42}, Size: normalizedSizeResponse{0.06, 0.14}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#C8B898", BorderColor: strPtr("#B8A888"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "shakurDeskLampOn", DeviceID: &shakurLamp}, Label: strPtr("Shakur's Desk Lamp")},
			{ID: "office_lamp_bottom", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.50, 0.36}, Size: normalizedSizeResponse{0.06, 0.14}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#D4C4A8", BorderColor: strPtr("#C4B498"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "officeLampBottomOn", DeviceID: &officeLampBottom}, Label: strPtr("Office Lamp (Bottom)")},
			{ID: "office_lamp_top", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.50, 0.60}, Size: normalizedSizeResponse{0.06, 0.14}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#D4C4A8", BorderColor: strPtr("#C4B498"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "officeLampTopOn", DeviceID: &officeLampTop}, Label: strPtr("Office Lamp (Top)")},
			{ID: "najah_desk_light", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.85, 0.42}, Size: normalizedSizeResponse{0.06, 0.14}, ZPosition: 20, Style: roomElementStyleResponse{FillColor: "#C8B898", BorderColor: strPtr("#B8A888"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(3)}, Interaction: &roomInteractionResponse{Type: "govee", IdleAnimation: "glow", StateKey: "najahDeskLightOn", DeviceID: &najahDesk}, Label: strPtr("Najah Desk Light")},
			{ID: "office_camera", Type: "interactable", Layer: "interactable", Position: normalizedPointResponse{0.50, 0.82}, Size: normalizedSizeResponse{0.04, 0.03}, ZPosition: 22, Style: roomElementStyleResponse{FillColor: "#E0E0E0", BorderColor: strPtr("#C0C0C0"), BorderWidth: floatPtr(1), CornerRadius: floatPtr(4)}, Interaction: &roomInteractionResponse{Type: "camera", IdleAnimation: "blink", StateKey: "officeCameraActive"}, Label: strPtr("Office Camera")},
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
