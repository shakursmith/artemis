package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// MenuTemplateHandler holds the database connection and provides HTTP handlers
// for menu template CRUD operations. Use NewMenuTemplateHandler to create one.
type MenuTemplateHandler struct {
	DB *sql.DB
}

// NewMenuTemplateHandler creates a new MenuTemplateHandler with the given database connection.
func NewMenuTemplateHandler(database *sql.DB) *MenuTemplateHandler {
	return &MenuTemplateHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createTemplateRequest is the JSON body for POST /api/business/{businessId}/menu-templates
type createTemplateRequest struct {
	OptionType   string `json:"optionType"`
	DefaultTitle string `json:"defaultTitle"`
	DefaultIcon  string `json:"defaultIcon"`
	DataSchema   string `json:"dataSchema"`
	Description  string `json:"description"`
}

// updateTemplateRequest is the JSON body for PUT /api/menu-template/{id}
type updateTemplateRequest struct {
	DefaultTitle string `json:"defaultTitle"`
	DefaultIcon  string `json:"defaultIcon"`
	DataSchema   string `json:"dataSchema"`
	Description  string `json:"description"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleListTemplates returns all menu option templates (system + business-created).
// GET /api/menu-templates
// Response (200): array of template objects
func (h *MenuTemplateHandler) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := db.ListMenuOptionTemplates(h.DB)
	if err != nil {
		log.Printf("❌ Menu templates list failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list menu templates")
		return
	}

	// Return empty array instead of nil for JSON
	if templates == nil {
		templates = []db.MenuOptionTemplate{}
	}

	log.Printf("📋 Listed %d menu templates", len(templates))
	writeJSON(w, http.StatusOK, templates)
}

// HandleCreateTemplate creates a new business-specific menu template.
// POST /api/business/{businessId}/menu-templates
// Request body: {"optionType": "custom", "defaultTitle": "My Template", ...}
// Response (201): full template object
func (h *MenuTemplateHandler) HandleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Parse request body
	var req createTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Template create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.OptionType == "" || req.DefaultTitle == "" || req.DefaultIcon == "" || req.DataSchema == "" || req.Description == "" {
		writeError(w, http.StatusBadRequest, "All fields are required")
		return
	}

	// Create the template
	template, err := db.CreateMenuOptionTemplate(h.DB, &businessID, req.OptionType, req.DefaultTitle, req.DefaultIcon, req.DataSchema, req.Description)
	if err != nil {
		log.Printf("❌ Template create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create template")
		return
	}

	log.Printf("📝 Created menu template: %s (type: %s)", template.DefaultTitle, template.OptionType)
	writeJSON(w, http.StatusCreated, template)
}

// HandleUpdateTemplate updates a menu template's fields.
// System templates cannot be edited.
// PUT /api/menu-template/{id}
// Request body: {"defaultTitle": "New Title", ...}
// Response (200): updated template object
func (h *MenuTemplateHandler) HandleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Template ID is required")
		return
	}

	// Parse request body
	var req updateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Template update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.DefaultTitle == "" || req.DefaultIcon == "" || req.DataSchema == "" || req.Description == "" {
		writeError(w, http.StatusBadRequest, "All fields are required")
		return
	}

	// Update the template (this will fail if it's a system template)
	template, err := db.UpdateMenuOptionTemplate(h.DB, id, req.DefaultTitle, req.DefaultIcon, req.DataSchema, req.Description)
	if err != nil {
		if isNotFound(err) || err.Error() == "cannot edit system template: "+id {
			writeError(w, http.StatusForbidden, "Cannot edit system template")
			return
		}
		log.Printf("❌ Template update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update template")
		return
	}

	log.Printf("📝 Updated menu template: %s", template.ID)
	writeJSON(w, http.StatusOK, template)
}

// HandleDeleteTemplate deletes a menu template.
// System templates cannot be deleted.
// DELETE /api/menu-template/{id}
// Response (204): no content on success
func (h *MenuTemplateHandler) HandleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Template ID is required")
		return
	}

	// Delete the template (this will fail if it's a system template)
	err := db.DeleteMenuOptionTemplate(h.DB, id)
	if err != nil {
		if isNotFound(err) || err.Error() == "cannot delete system template: "+id {
			writeError(w, http.StatusForbidden, "Cannot delete system template")
			return
		}
		log.Printf("❌ Template delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete template")
		return
	}

	log.Printf("📝 Deleted menu template: %s", id)
	w.WriteHeader(http.StatusNoContent)
}
