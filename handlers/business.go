package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// BusinessHandler holds the database connection and provides HTTP handlers
// for business CRUD operations. Use NewBusinessHandler to create one.
type BusinessHandler struct {
	DB *sql.DB
}

// NewBusinessHandler creates a new BusinessHandler with the given database connection.
func NewBusinessHandler(database *sql.DB) *BusinessHandler {
	return &BusinessHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createBusinessRequest is the JSON body for POST /api/business
type createBusinessRequest struct {
	Name      string  `json:"name"`
	OwnerName *string `json:"ownerName,omitempty"`
}

// businessDetailResponse is the enriched business response that includes
// associated locations. Used by GET /api/business/{id}.
type businessDetailResponse struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	OwnerName *string       `json:"ownerName,omitempty"`
	Locations []db.Location `json:"locations"`
	CreatedAt string        `json:"createdAt"`
	UpdatedAt string        `json:"updatedAt"`
}

// updateBusinessRequest is the JSON body for PUT /api/business/{id}
type updateBusinessRequest struct {
	Name      string  `json:"name"`
	OwnerName *string `json:"ownerName,omitempty"`
}

// =============================================================================
// Handlers
// =============================================================================

// HandleCreateBusiness creates a new business.
// POST /api/business
// Request body: {"name": "Yummy Cafe", "ownerName": "Alice"}
// Response (201): full business object
func (h *BusinessHandler) HandleCreateBusiness(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req createBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Business create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Create the business in the database
	business, err := db.CreateBusiness(h.DB, req.Name, req.OwnerName)
	if err != nil {
		log.Printf("❌ Business create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create business")
		return
	}

	log.Printf("🏢 Created business: %s (id: %s)", business.Name, business.ID)
	writeJSON(w, http.StatusCreated, business)
}

// HandleGetBusiness returns a single business by ID, enriched with its locations.
// GET /api/business/{id}
// Response (200): business with locations[]
func (h *BusinessHandler) HandleGetBusiness(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Fetch the business
	business, err := db.GetBusiness(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Business not found")
			return
		}
		log.Printf("❌ Business get failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get business")
		return
	}

	// Fetch associated locations for the enriched response
	locations, err := db.ListLocationsByBusiness(h.DB, id)
	if err != nil {
		log.Printf("❌ Failed to list locations for business %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "Failed to get business locations")
		return
	}

	// Return nil slices as empty arrays in JSON
	if locations == nil {
		locations = []db.Location{}
	}

	// Build enriched response
	resp := businessDetailResponse{
		ID:        business.ID,
		Name:      business.Name,
		OwnerName: business.OwnerName,
		Locations: locations,
		CreatedAt: business.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: business.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleUpdateBusiness updates a business's name and owner name.
// PUT /api/business/{id}
// Request body: {"name": "New Name", "ownerName": "New Owner"}
// Response (200): updated business object
func (h *BusinessHandler) HandleUpdateBusiness(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Parse request body
	var req updateBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Business update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Update the business
	business, err := db.UpdateBusiness(h.DB, id, req.Name, req.OwnerName)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Business not found")
			return
		}
		log.Printf("❌ Business update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update business")
		return
	}

	log.Printf("🏢 Updated business: %s (id: %s)", business.Name, business.ID)
	writeJSON(w, http.StatusOK, business)
}

// HandleDeleteBusiness deletes a business and all associated locations/rooms (cascade).
// DELETE /api/business/{id}
// Response (204): no content on success
func (h *BusinessHandler) HandleDeleteBusiness(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Delete the business
	err := db.DeleteBusiness(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Business not found")
			return
		}
		log.Printf("❌ Business delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete business")
		return
	}

	log.Printf("🏢 Deleted business: %s", id)
	w.WriteHeader(http.StatusNoContent)
}
