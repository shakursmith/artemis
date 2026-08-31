package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// RoomMenuHandler holds the database connection and provides HTTP handlers
// for room menu management operations. Use NewRoomMenuHandler to create one.
type RoomMenuHandler struct {
	DB *sql.DB
}

// NewRoomMenuHandler creates a new RoomMenuHandler with the given database connection.
func NewRoomMenuHandler(database *sql.DB) *RoomMenuHandler {
	return &RoomMenuHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// roomMenuResponse is the hybrid response that combines menu data + appearance settings.
// Used by GET /api/room/{roomId}/menu.
type roomMenuResponse struct {
	Menu       *db.Menu        `json:"menu"`
	Options    []db.MenuOption `json:"options"`
	Appearance *menuAppearance `json:"appearance,omitempty"` // from Location.metadata
}

// menuAppearance represents the appearance settings stored in Location.metadata.
// This maintains backward compatibility with the existing ShopMenuConfig structure.
type menuAppearance struct {
	Colors              *menuColors  `json:"colors,omitempty"`
	OverlayOpacity      *float64     `json:"overlayOpacity,omitempty"`
	MenuBackgroundColor *string      `json:"menuBackgroundColor,omitempty"`
	MenuButtonColor     *string      `json:"menuButtonColor,omitempty"`
	Icon                *string      `json:"icon,omitempty"`
	Enabled             *bool        `json:"enabled,omitempty"`
}

// menuColors represents the color configuration for the menu.
type menuColors struct {
	Overlay        *string `json:"overlay,omitempty"`
	MenuBackground *string `json:"menuBackground,omitempty"`
	MenuButton     *string `json:"menuButton,omitempty"`
}

// updateMenuEnabledRequest is the JSON body for PUT /api/room/{roomId}/menu/enabled
type updateMenuEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// updateMenuAppearanceRequest is the JSON body for PUT /api/room/{roomId}/menu/appearance
type updateMenuAppearanceRequest struct {
	MenuColor     string  `json:"menuColor"`
	MenuOpacity   float64 `json:"menuOpacity"`
	TextColor     string  `json:"textColor"`
	IconColor     string  `json:"iconColor"`
	CollapsedIcon string  `json:"collapsedIcon"`
}

// assignMenuOptionRequest is the JSON body for POST /api/room/{roomId}/menu/assign
type assignMenuOptionRequest struct {
	MenuOptionID string `json:"menuOptionId"`
	Position     int    `json:"position"`
}

// unassignMenuOptionRequest is the JSON body for DELETE /api/room/{roomId}/menu/unassign
type unassignMenuOptionRequest struct {
	MenuOptionID string `json:"menuOptionId"`
}

// reorderMenuOptionsRequest is the JSON body for PUT /api/room/{roomId}/menu/reorder
type reorderMenuOptionsRequest struct {
	MenuOptionIDs []string `json:"menuOptionIds"` // ordered list of menu option IDs
}

// =============================================================================
// Handlers
// =============================================================================

// HandleGetRoomMenu returns a room's menu with options and appearance settings.
// Menu data and appearance come from the menus table.
// Options come from menu_options + menu_option_assignments.
// GET /api/room/{roomId}/menu
// Response (200): {menu, options[], appearance}
func (h *RoomMenuHandler) HandleGetRoomMenu(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Get menu and options from database
	menu, options, err := db.GetMenuWithOptions(h.DB, roomID)
	if err != nil {
		log.Printf("❌ Failed to get menu for room %s: %v", roomID, err)
		writeError(w, http.StatusInternalServerError, "Failed to get room menu")
		return
	}

	// Convert menu appearance fields to menuAppearance struct
	appearance := menuAppearance{
		Colors: &menuColors{
			Overlay:        &menu.TextColor,
			MenuBackground: &menu.MenuColor,
			MenuButton:     &menu.IconColor,
		},
		OverlayOpacity:      &menu.MenuOpacity,
		MenuBackgroundColor: &menu.MenuColor,
		MenuButtonColor:     &menu.IconColor,
		Icon:                &menu.CollapsedIcon,
		Enabled:             &menu.IsEnabled,
	}

	// Initialize response
	resp := roomMenuResponse{
		Menu:       menu,
		Options:    options,
		Appearance: &appearance,
	}

	// Return empty array instead of nil for JSON
	if resp.Options == nil {
		resp.Options = []db.MenuOption{}
	}

	log.Printf("📋 Retrieved menu for room %s with %d options", roomID, len(options))
	writeJSON(w, http.StatusOK, resp)
}

// HandleUpdateMenuEnabled updates the is_enabled flag for a room's menu.
// PUT /api/room/{roomId}/menu/enabled
// Request body: {"enabled": true}
// Response (200): updated menu object
func (h *RoomMenuHandler) HandleUpdateMenuEnabled(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Parse request body
	var req updateMenuEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Update menu enabled: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update menu enabled status
	menu, err := db.UpdateMenuEnabled(h.DB, roomID, req.Enabled)
	if err != nil {
		log.Printf("❌ Failed to update menu enabled for room %s: %v", roomID, err)
		writeError(w, http.StatusInternalServerError, "Failed to update menu")
		return
	}

	log.Printf("📋 Updated menu enabled for room %s: %v", roomID, req.Enabled)
	writeJSON(w, http.StatusOK, menu)
}

// HandleAssignMenuOption assigns a menu option to a room's menu at the specified position.
// POST /api/room/{roomId}/menu/assign
// Request body: {"menuOptionId": "opt_123", "position": 0}
// Response (201): created assignment object
func (h *RoomMenuHandler) HandleAssignMenuOption(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Parse request body
	var req assignMenuOptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Assign menu option: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.MenuOptionID == "" {
		writeError(w, http.StatusBadRequest, "menuOptionId is required")
		return
	}

	// Get or create the menu for this room
	menu, err := db.GetOrCreateMenu(h.DB, roomID)
	if err != nil {
		log.Printf("❌ Failed to get/create menu for room %s: %v", roomID, err)
		writeError(w, http.StatusInternalServerError, "Failed to get menu")
		return
	}

	// Assign the menu option
	assignment, err := db.AssignMenuOption(h.DB, menu.ID, req.MenuOptionID, req.Position)
	if err != nil {
		log.Printf("❌ Failed to assign menu option %s to menu %s: %v", req.MenuOptionID, menu.ID, err)
		writeError(w, http.StatusBadRequest, "Failed to assign menu option (may already be assigned or position taken)")
		return
	}

	log.Printf("📋 Assigned menu option %s to room %s at position %d", req.MenuOptionID, roomID, req.Position)
	writeJSON(w, http.StatusCreated, assignment)
}

// HandleUnassignMenuOption removes a menu option assignment from a room's menu.
// DELETE /api/room/{roomId}/menu/unassign
// Request body: {"menuOptionId": "opt_123"}
// Response (204): no content on success
func (h *RoomMenuHandler) HandleUnassignMenuOption(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Parse request body
	var req unassignMenuOptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Unassign menu option: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.MenuOptionID == "" {
		writeError(w, http.StatusBadRequest, "menuOptionId is required")
		return
	}

	// Get the menu for this room
	menu, err := db.GetOrCreateMenu(h.DB, roomID)
	if err != nil {
		log.Printf("❌ Failed to get menu for room %s: %v", roomID, err)
		writeError(w, http.StatusInternalServerError, "Failed to get menu")
		return
	}

	// Unassign the menu option
	err = db.UnassignMenuOption(h.DB, menu.ID, req.MenuOptionID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Menu option assignment not found")
			return
		}
		log.Printf("❌ Failed to unassign menu option %s from menu %s: %v", req.MenuOptionID, menu.ID, err)
		writeError(w, http.StatusInternalServerError, "Failed to unassign menu option")
		return
	}

	log.Printf("📋 Unassigned menu option %s from room %s", req.MenuOptionID, roomID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderMenuOptions updates the positions of menu options in a room's menu.
// PUT /api/room/{roomId}/menu/reorder
// Request body: {"menuOptionIds": ["opt_1", "opt_2", "opt_3"]}
// Response (204): no content on success
func (h *RoomMenuHandler) HandleReorderMenuOptions(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Parse request body
	var req reorderMenuOptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Reorder menu options: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.MenuOptionIDs == nil || len(req.MenuOptionIDs) == 0 {
		writeError(w, http.StatusBadRequest, "menuOptionIds is required and must not be empty")
		return
	}

	// Get the menu for this room
	menu, err := db.GetOrCreateMenu(h.DB, roomID)
	if err != nil {
		log.Printf("❌ Failed to get menu for room %s: %v", roomID, err)
		writeError(w, http.StatusInternalServerError, "Failed to get menu")
		return
	}

	// Reorder the menu options
	err = db.ReorderMenuOptions(h.DB, menu.ID, req.MenuOptionIDs)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "One or more menu options not found")
			return
		}
		log.Printf("❌ Failed to reorder menu options for menu %s: %v", menu.ID, err)
		writeError(w, http.StatusInternalServerError, "Failed to reorder menu options")
		return
	}

	log.Printf("📋 Reordered %d menu options for room %s", len(req.MenuOptionIDs), roomID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleUpdateMenuAppearance updates the appearance customization for a room's menu.
// PUT /api/room/{roomId}/menu/appearance
// Request body: {"menuColor": "#000000", "menuOpacity": 0.3, "textColor": "#FFFFFF", "iconColor": "#FF9500", "collapsedIcon": "storefront"}
// Response (200): updated menu object
func (h *RoomMenuHandler) HandleUpdateMenuAppearance(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "Room ID is required")
		return
	}

	// Parse request body
	var req updateMenuAppearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Update menu appearance: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.MenuColor == "" || req.TextColor == "" || req.IconColor == "" || req.CollapsedIcon == "" {
		writeError(w, http.StatusBadRequest, "All appearance fields are required")
		return
	}

	// Validate opacity range
	if req.MenuOpacity < 0.0 || req.MenuOpacity > 1.0 {
		writeError(w, http.StatusBadRequest, "menuOpacity must be between 0.0 and 1.0")
		return
	}

	// Update menu appearance
	menu, err := db.UpdateMenuAppearance(h.DB, roomID, req.MenuColor, req.MenuOpacity, req.TextColor, req.IconColor, req.CollapsedIcon)
	if err != nil {
		log.Printf("❌ Failed to update menu appearance for room %s: %v", roomID, err)
		writeError(w, http.StatusInternalServerError, "Failed to update menu appearance")
		return
	}

	log.Printf("📋 Updated menu appearance for room %s", roomID)
	writeJSON(w, http.StatusOK, menu)
}
