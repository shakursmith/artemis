package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// ShopConfigHandler holds the database connection and provides the HTTP handler
// for the composite shop config endpoint. This endpoint aggregates data from
// businesses, locations, reviews, loyalty programs, and loyalty memberships
// into a single response so the client can bootstrap the in-shop experience
// in one API call.
type ShopConfigHandler struct {
	DB *sql.DB
}

// NewShopConfigHandler creates a new ShopConfigHandler with the given database connection.
func NewShopConfigHandler(database *sql.DB) *ShopConfigHandler {
	return &ShopConfigHandler{DB: database}
}

// HandleGetShopConfig handles GET /api/shop-config/{locationId}
// Returns a composite payload with everything needed to render the in-shop experience:
// business branding, location hours/status, feature flags, extras, and summary
// data for reviews and loyalty.
//
// Query parameters:
//   - customerId (optional): UUID of the requesting customer. When provided,
//     loyaltySummary.currentStamps reflects that customer's membership.
func (h *ShopConfigHandler) HandleGetShopConfig(w http.ResponseWriter, r *http.Request) {
	// Parse locationId from the URL path (Go 1.22+ pattern matching)
	locationID := r.PathValue("locationId")
	if locationID == "" {
		writeError(w, http.StatusBadRequest, "locationId is required")
		return
	}

	// Parse optional customerId from query string.
	// When provided, the response includes the customer's loyalty stamp count.
	var customerID *string
	if cid := r.URL.Query().Get("customerId"); cid != "" {
		customerID = &cid
	}

	// Compose the shop config from multiple database tables.
	// GetShopConfig handles all the aggregation logic (location + business +
	// reviews + loyalty program + loyalty membership).
	config, err := db.GetShopConfig(h.DB, locationID, customerID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Location not found")
			return
		}
		log.Printf("❌ Failed to get shop config for location %s: %v", locationID, err)
		writeError(w, http.StatusInternalServerError, "Failed to get shop config")
		return
	}

	log.Printf("✅ Shop config retrieved for location %s (business: %s)", locationID, config.BusinessName)
	writeJSON(w, http.StatusOK, config)
}
