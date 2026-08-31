package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// MenuOptionHandler holds the database connection and provides HTTP handlers
// for menu option CRUD operations. Use NewMenuOptionHandler to create one.
type MenuOptionHandler struct {
	DB *sql.DB
}

// NewMenuOptionHandler creates a new MenuOptionHandler with the given database connection.
func NewMenuOptionHandler(database *sql.DB) *MenuOptionHandler {
	return &MenuOptionHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// copyTemplateRequest is the JSON body for POST /api/business/{businessId}/menu-options/copy
type copyTemplateRequest struct {
	TemplateID string `json:"templateId"`
	Data       string `json:"data"` // JSON string with actual credentials/URLs
}

// createMenuOptionRequest is the JSON body for POST /api/business/{businessId}/menu-options
type createMenuOptionRequest struct {
	TemplateID *string `json:"templateId,omitempty"`
	OptionType string  `json:"optionType"`
	Title      string  `json:"title"`
	Icon       string  `json:"icon"`
	Data       string  `json:"data"`
}

// updateMenuOptionRequest is the JSON body for PUT /api/menu-option/{id}
type updateMenuOptionRequest struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
	Data  string `json:"data"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleListMenuOptions returns all menu options for a specific business.
// GET /api/business/{businessId}/menu-options
// Response (200): array of menu option objects
func (h *MenuOptionHandler) HandleListMenuOptions(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	options, err := db.ListMenuOptionsByBusiness(h.DB, businessID)
	if err != nil {
		log.Printf("❌ Menu options list failed for business %s: %v", businessID, err)
		writeError(w, http.StatusInternalServerError, "Failed to list menu options")
		return
	}

	// Return empty array instead of nil for JSON
	if options == nil {
		options = []db.MenuOption{}
	}

	log.Printf("📋 Listed %d menu options for business %s", len(options), businessID)
	writeJSON(w, http.StatusOK, options)
}

// HandleCopyTemplate copies a template to create a business-specific menu option.
// POST /api/business/{businessId}/menu-options/copy
// Request body: {"templateId": "tpl_wifi_system", "data": "{\"ssid\":\"CafeWiFi\",\"password\":\"coffee123\"}"}
// Response (201): full menu option object
func (h *MenuOptionHandler) HandleCopyTemplate(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Parse request body
	var req copyTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Copy template: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.TemplateID == "" || req.Data == "" {
		writeError(w, http.StatusBadRequest, "templateId and data are required")
		return
	}

	// Copy the template to create a menu option
	option, err := db.CopyMenuOptionFromTemplate(h.DB, businessID, req.TemplateID, req.Data)
	if err != nil {
		log.Printf("❌ Copy template failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to copy template")
		return
	}

	log.Printf("📝 Copied template %s to create menu option: %s", req.TemplateID, option.ID)
	writeJSON(w, http.StatusCreated, option)
}

// HandleCreateMenuOption creates a new menu option (without using a template).
// POST /api/business/{businessId}/menu-options
// Request body: {"optionType": "custom", "title": "My Option", "icon": "star.fill", "data": "{}"}
// Response (201): full menu option object
func (h *MenuOptionHandler) HandleCreateMenuOption(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Parse request body
	var req createMenuOptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Menu option create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.OptionType == "" || req.Title == "" || req.Icon == "" || req.Data == "" {
		writeError(w, http.StatusBadRequest, "optionType, title, icon, and data are required")
		return
	}

	// Create the menu option
	option, err := db.CreateMenuOption(h.DB, businessID, req.TemplateID, req.OptionType, req.Title, req.Icon, req.Data)
	if err != nil {
		log.Printf("❌ Menu option create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create menu option")
		return
	}

	log.Printf("📝 Created menu option: %s (type: %s)", option.Title, option.OptionType)
	writeJSON(w, http.StatusCreated, option)
}

// HandleUpdateMenuOption updates a menu option's title, icon, and data.
// PUT /api/menu-option/{id}
// Request body: {"title": "New Title", "icon": "new.icon", "data": "{}"}
// Response (200): updated menu option object
func (h *MenuOptionHandler) HandleUpdateMenuOption(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Menu option ID is required")
		return
	}

	// Parse request body
	var req updateMenuOptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Menu option update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Title == "" || req.Icon == "" || req.Data == "" {
		writeError(w, http.StatusBadRequest, "title, icon, and data are required")
		return
	}

	// Update the menu option
	option, err := db.UpdateMenuOption(h.DB, id, req.Title, req.Icon, req.Data)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Menu option not found")
			return
		}
		log.Printf("❌ Menu option update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update menu option")
		return
	}

	log.Printf("📝 Updated menu option: %s", option.ID)
	writeJSON(w, http.StatusOK, option)
}

// HandleDeleteMenuOption deletes a menu option.
// This will also remove all assignments of this option to menus (via CASCADE).
// DELETE /api/menu-option/{id}
// Response (204): no content on success
func (h *MenuOptionHandler) HandleDeleteMenuOption(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Menu option ID is required")
		return
	}

	// Delete the menu option
	err := db.DeleteMenuOption(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Menu option not found")
			return
		}
		log.Printf("❌ Menu option delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete menu option")
		return
	}

	log.Printf("📝 Deleted menu option: %s", id)
	w.WriteHeader(http.StatusNoContent)
}
