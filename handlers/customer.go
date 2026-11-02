package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pantheon/artemis/db"
)

// CustomerHandler holds the database connection and provides HTTP handlers
// for customer profile and sharing settings operations.
type CustomerHandler struct {
	DB *sql.DB
}

// NewCustomerHandler creates a new CustomerHandler with the given database connection.
func NewCustomerHandler(database *sql.DB) *CustomerHandler {
	return &CustomerHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createCustomerRequest is the JSON body for POST /api/customer
type createCustomerRequest struct {
	FirstName string `json:"firstName"`
}

// updateCustomerRequest is the JSON body for PUT /api/customer/{id}
type updateCustomerRequest struct {
	FirstName string `json:"firstName"`
}

// updateSharingSettingsRequest is the JSON body for PUT /api/customer/{id}/sharing
type updateSharingSettingsRequest struct {
	SharingLevel      *string  `json:"sharingLevel,omitempty"`
	BlockedBusinesses []string `json:"blockedBusinesses,omitempty"`
}

// customerProfileResponse is the response for GET /api/customer/{customerId}/profile/business/{businessId}
// Fields are conditionally included based on sharing level.
type customerProfileResponse struct {
	ID              string                  `json:"id"`
	FirstName       string                  `json:"firstName"`
	SharingLevel    string                  `json:"sharingLevel"`
	PurchaseHistory *purchaseHistorySummary `json:"purchaseHistory,omitempty"`
	LoyaltyStatus   *string                 `json:"loyaltyStatus,omitempty"`
	ReviewCount     *int                    `json:"reviewCount,omitempty"`
}

// purchaseHistorySummary represents a customer's purchase history at a business.
type purchaseHistorySummary struct {
	TotalPurchases int     `json:"totalPurchases"`
	TotalSpent     float64 `json:"totalSpent"`
	LastVisit      string  `json:"lastVisit"` // ISO 8601 timestamp
}

// =============================================================================
// Customer CRUD Handlers
// =============================================================================

// HandleCreateCustomer creates a new customer.
// POST /api/customer
// Request body: {"firstName": "Shakur"}
// Response (201): customer object with default sharing settings created
func (h *CustomerHandler) HandleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req createCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Customer create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.FirstName == "" {
		writeError(w, http.StatusBadRequest, "firstName is required")
		return
	}

	// Create the customer (also creates default sharing settings)
	customer, err := db.CreateCustomer(h.DB, req.FirstName)
	if err != nil {
		log.Printf("❌ Customer create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create customer")
		return
	}

	log.Printf("👤 Created customer: %s (id: %s)", customer.FirstName, customer.ID)
	writeJSON(w, http.StatusCreated, customer)
}

// HandleGetCustomer retrieves a single customer by ID.
// GET /api/customer/{id}
// Response (200): customer object
func (h *CustomerHandler) HandleGetCustomer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	customer, err := db.GetCustomer(h.DB, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
		log.Printf("❌ Customer get failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get customer")
		return
	}

	writeJSON(w, http.StatusOK, customer)
}

// HandleUpdateCustomer updates a customer's first name.
// PUT /api/customer/{id}
// Request body: {"firstName": "Jordan"}
// Response (200): updated customer object
func (h *CustomerHandler) HandleUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	var req updateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Customer update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.FirstName == "" {
		writeError(w, http.StatusBadRequest, "firstName is required")
		return
	}

	customer, err := db.UpdateCustomer(h.DB, id, req.FirstName)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
		log.Printf("❌ Customer update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update customer")
		return
	}

	log.Printf("✏️  Updated customer: %s (id: %s)", customer.FirstName, customer.ID)
	writeJSON(w, http.StatusOK, customer)
}

// =============================================================================
// Customer Sharing Settings Handlers
// =============================================================================

// HandleGetSharingSettings retrieves a customer's sharing settings.
// GET /api/customer/{id}/sharing
// Response (200): sharing settings object
func (h *CustomerHandler) HandleGetSharingSettings(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("id")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	settings, err := db.GetCustomerSharingSettings(h.DB, customerID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
		log.Printf("❌ Get sharing settings failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get sharing settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// HandleUpdateSharingSettings updates a customer's sharing settings.
// PUT /api/customer/{id}/sharing
// Request body: {"sharingLevel": "standard", "blockedBusinesses": ["biz_123"]}
// Response (200): updated sharing settings object
func (h *CustomerHandler) HandleUpdateSharingSettings(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("id")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	var req updateSharingSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Update sharing settings: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get current settings
	settings, err := db.GetCustomerSharingSettings(h.DB, customerID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
		log.Printf("❌ Get sharing settings failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get sharing settings")
		return
	}

	// Apply updates (use current values if not provided in request)
	sharingLevel := settings.SharingLevel
	if req.SharingLevel != nil {
		sharingLevel = *req.SharingLevel
	}

	blockedBusinesses := settings.BlockedBusinesses
	if req.BlockedBusinesses != nil {
		blockedBusinesses = req.BlockedBusinesses
	}

	// Validate sharing level
	if sharingLevel != "minimal" && sharingLevel != "standard" && sharingLevel != "full" {
		writeError(w, http.StatusBadRequest, "Invalid sharing level: must be minimal, standard, or full")
		return
	}

	// Update settings
	updatedSettings, err := db.UpdateCustomerSharingSettings(h.DB, customerID, sharingLevel, blockedBusinesses)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
		log.Printf("❌ Update sharing settings failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update sharing settings")
		return
	}

	log.Printf("🔒 Updated sharing settings for customer %s: level=%s", customerID, sharingLevel)
	writeJSON(w, http.StatusOK, updatedSettings)
}

// =============================================================================
// Customer Profile Handler (Business-Facing)
// =============================================================================

// HandleGetCustomerProfile returns a customer profile filtered by sharing settings.
// GET /api/customer/{customerId}/profile?businessId={businessId}
// Response (200): customer profile with fields based on sharing level
//
// Sharing levels:
//   - minimal: firstName only
//   - standard: firstName + purchaseHistory + loyaltyStatus
//   - full: firstName + purchaseHistory + loyaltyStatus + reviewCount
//
// New customer detection: if purchaseHistory and loyaltyStatus are absent,
// the customer is new to this business.
func (h *CustomerHandler) HandleGetCustomerProfile(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	businessID := r.URL.Query().Get("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "businessId query parameter is required")
		return
	}

	// Get customer
	customer, err := db.GetCustomer(h.DB, customerID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
		log.Printf("❌ Get customer failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get customer")
		return
	}

	// Get sharing settings
	settings, err := db.GetCustomerSharingSettings(h.DB, customerID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
		log.Printf("❌ Get sharing settings failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get sharing settings")
		return
	}

	// Check if business is blocked
	for _, blockedID := range settings.BlockedBusinesses {
		if blockedID == businessID {
			writeError(w, http.StatusNotFound, "Customer not found")
			return
		}
	}

	// Build response based on sharing level
	response := customerProfileResponse{
		ID:           customer.ID,
		FirstName:    customer.FirstName,
		SharingLevel: settings.SharingLevel,
	}

	// For standard and full levels, include purchase history and loyalty
	// TODO: In a real implementation, query orders/reviews tables for this data.
	// For now, we'll return nil for these fields to indicate "new customer"
	// since we don't have orders/loyalty implementation yet.
	if settings.SharingLevel == "standard" || settings.SharingLevel == "full" {
		// Future: query purchase history and loyalty from respective tables
		// For now, omit these fields (nil) to indicate new customer
		response.PurchaseHistory = nil
		response.LoyaltyStatus = nil
	}

	// For full level, include review count
	if settings.SharingLevel == "full" {
		// Future: query review count from reviews table
		// For now, omit this field (nil)
		response.ReviewCount = nil
	}

	writeJSON(w, http.StatusOK, response)
}
