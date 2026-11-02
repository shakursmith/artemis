package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pantheon/artemis/db"
)

// LoyaltyHandler handles loyalty program and membership endpoints.
// Loyalty programs are simple points-based systems that businesses can opt into.
// Customers automatically join when placing their first order at a business with
// an active loyalty program. Points are awarded on order completion.
// Redemption is future work - this version is configuration-only.
type LoyaltyHandler struct {
	db *sql.DB
}

// NewLoyaltyHandler creates a new loyalty handler with the given database connection.
func NewLoyaltyHandler(database *sql.DB) *LoyaltyHandler {
	return &LoyaltyHandler{db: database}
}

// =============================================================================
// Request/Response Models
// =============================================================================

// createLoyaltyProgramRequest represents the request body for creating a loyalty program.
type createLoyaltyProgramRequest struct {
	Name             string  `json:"name"`
	PointsMultiplier float64 `json:"pointsMultiplier"`
	RewardThreshold  int     `json:"rewardThreshold"`
	RewardDescription string `json:"rewardDescription"`
}

// updateLoyaltyProgramRequest represents the request body for updating a loyalty program.
type updateLoyaltyProgramRequest struct {
	Name             string  `json:"name"`
	PointsMultiplier float64 `json:"pointsMultiplier"`
	RewardThreshold  int     `json:"rewardThreshold"`
	RewardDescription string `json:"rewardDescription"`
}

// loyaltyProgramResponse formats a loyalty program for JSON responses.
// Timestamps are formatted as ISO 8601 strings per conventions.md.
type loyaltyProgramResponse struct {
	ID               string  `json:"id"`
	BusinessID       string  `json:"businessId"`
	Name             string  `json:"name"`
	PointsMultiplier float64 `json:"pointsMultiplier"`
	RewardThreshold  int     `json:"rewardThreshold"`
	RewardDescription string `json:"rewardDescription"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

// loyaltyMembershipResponse formats a loyalty membership for JSON responses.
// Includes the computed hasEarnedReward field based on points balance vs reward threshold.
type loyaltyMembershipResponse struct {
	ID             string `json:"id"`
	CustomerID     string `json:"customerId"`
	BusinessID     string `json:"businessId"`
	PointsBalance  int    `json:"pointsBalance"`
	HasEarnedReward bool   `json:"hasEarnedReward"` // computed: pointsBalance >= program.rewardThreshold
	JoinedAt       string `json:"joinedAt"`
	LastActivityAt string `json:"lastActivityAt"`
}

// formatLoyaltyProgram converts a db.LoyaltyProgram to a loyaltyProgramResponse.
func formatLoyaltyProgram(program *db.LoyaltyProgram) loyaltyProgramResponse {
	return loyaltyProgramResponse{
		ID:               program.ID,
		BusinessID:       program.BusinessID,
		Name:             program.Name,
		PointsMultiplier: program.PointsMultiplier,
		RewardThreshold:  program.RewardThreshold,
		RewardDescription: program.RewardDescription,
		CreatedAt:        program.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:        program.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// formatLoyaltyMembership converts a db.LoyaltyMembership to a loyaltyMembershipResponse.
// Computes hasEarnedReward by comparing points balance to the program's reward threshold.
func formatLoyaltyMembership(membership *db.LoyaltyMembership, program *db.LoyaltyProgram) loyaltyMembershipResponse {
	hasEarnedReward := false
	if program != nil {
		hasEarnedReward = membership.PointsBalance >= program.RewardThreshold
	}
	return loyaltyMembershipResponse{
		ID:             membership.ID,
		CustomerID:     membership.CustomerID,
		BusinessID:     membership.BusinessID,
		PointsBalance:  membership.PointsBalance,
		HasEarnedReward: hasEarnedReward,
		JoinedAt:       membership.JoinedAt.Format("2006-01-02T15:04:05Z"),
		LastActivityAt: membership.LastActivityAt.Format("2006-01-02T15:04:05Z"),
	}
}

// =============================================================================
// Loyalty Program Endpoints (Business Admin)
// =============================================================================

// HandleCreateLoyaltyProgram creates a new loyalty program for a business.
// POST /api/business/{businessId}/loyalty-program
// A business can have at most one loyalty program. Returns 400 if program already exists.
func (h *LoyaltyHandler) HandleCreateLoyaltyProgram(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")

	var req createLoyaltyProgramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.PointsMultiplier <= 0 {
		writeError(w, http.StatusBadRequest, "Points multiplier must be greater than 0")
		return
	}
	if req.RewardThreshold <= 0 {
		writeError(w, http.StatusBadRequest, "Reward threshold must be greater than 0")
		return
	}
	if req.RewardDescription == "" {
		writeError(w, http.StatusBadRequest, "Reward description is required")
		return
	}

	// Check if program already exists for this business
	var existingID string
	err := h.db.QueryRow("SELECT id FROM loyalty_programs WHERE business_id = ?", businessID).Scan(&existingID)
	if err == nil {
		writeError(w, http.StatusBadRequest, "Loyalty program already exists for this business")
		return
	}
	if err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Create program
	program := &db.LoyaltyProgram{
		ID:               uuid.NewString(),
		BusinessID:       businessID,
		Name:             req.Name,
		PointsMultiplier: req.PointsMultiplier,
		RewardThreshold:  req.RewardThreshold,
		RewardDescription: req.RewardDescription,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	_, err = h.db.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, program.ID, program.BusinessID, program.Name, program.PointsMultiplier, program.RewardThreshold, program.RewardDescription, program.CreatedAt, program.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create loyalty program")
		return
	}

	writeJSON(w, http.StatusCreated, formatLoyaltyProgram(program))
}

// HandleGetLoyaltyProgram retrieves the loyalty program for a business.
// GET /api/business/{businessId}/loyalty-program
// Returns 404 if no program exists (loyalty is opt-in).
func (h *LoyaltyHandler) HandleGetLoyaltyProgram(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")

	program, err := h.getLoyaltyProgram(businessID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "No loyalty program for this business")
			return
		}
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	writeJSON(w, http.StatusOK, formatLoyaltyProgram(program))
}

// HandleUpdateLoyaltyProgram updates the loyalty program for a business.
// PUT /api/business/{businessId}/loyalty-program
// Returns 404 if no program exists.
func (h *LoyaltyHandler) HandleUpdateLoyaltyProgram(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")

	var req updateLoyaltyProgramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.PointsMultiplier <= 0 {
		writeError(w, http.StatusBadRequest, "Points multiplier must be greater than 0")
		return
	}
	if req.RewardThreshold <= 0 {
		writeError(w, http.StatusBadRequest, "Reward threshold must be greater than 0")
		return
	}
	if req.RewardDescription == "" {
		writeError(w, http.StatusBadRequest, "Reward description is required")
		return
	}

	// Check if program exists
	program, err := h.getLoyaltyProgram(businessID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "No loyalty program for this business")
			return
		}
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Update program
	program.Name = req.Name
	program.PointsMultiplier = req.PointsMultiplier
	program.RewardThreshold = req.RewardThreshold
	program.RewardDescription = req.RewardDescription
	program.UpdatedAt = time.Now().UTC()

	_, err = h.db.Exec(`
		UPDATE loyalty_programs
		SET name = ?, points_multiplier = ?, reward_threshold = ?, reward_description = ?, updated_at = ?
		WHERE business_id = ?
	`, program.Name, program.PointsMultiplier, program.RewardThreshold, program.RewardDescription, program.UpdatedAt, businessID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update loyalty program")
		return
	}

	writeJSON(w, http.StatusOK, formatLoyaltyProgram(program))
}

// =============================================================================
// Loyalty Membership Endpoints (Customer)
// =============================================================================

// HandleGetCustomerMemberships lists all loyalty memberships for a customer across all businesses.
// GET /api/customer/{customerId}/loyalty-memberships
// Returns empty array if the customer has no memberships.
func (h *LoyaltyHandler) HandleGetCustomerMemberships(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")

	rows, err := h.db.Query(`
		SELECT id, customer_id, business_id, points_balance, joined_at, last_activity_at
		FROM loyalty_memberships
		WHERE customer_id = ?
		ORDER BY last_activity_at DESC
	`, customerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	memberships := []loyaltyMembershipResponse{}
	for rows.Next() {
		var m db.LoyaltyMembership
		if err := rows.Scan(&m.ID, &m.CustomerID, &m.BusinessID, &m.PointsBalance, &m.JoinedAt, &m.LastActivityAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Database error")
			return
		}

		// Fetch loyalty program to compute hasEarnedReward
		program, _ := h.getLoyaltyProgram(m.BusinessID)
		memberships = append(memberships, formatLoyaltyMembership(&m, program))
	}

	writeJSON(w, http.StatusOK, memberships)
}

// HandleGetCustomerMembership retrieves a customer's loyalty membership for a specific business.
// GET /api/customer/{customerId}/loyalty-membership/{businessId}
// Auto-creates membership if program exists and customer doesn't have one yet.
// Returns 404 if no program exists for the business.
func (h *LoyaltyHandler) HandleGetCustomerMembership(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	businessID := r.PathValue("businessId")

	// Check if loyalty program exists for this business
	program, err := h.getLoyaltyProgram(businessID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "No loyalty program for this business")
			return
		}
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Check if membership already exists
	membership, err := h.getMembership(customerID, businessID)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Auto-create membership if it doesn't exist
	if err == sql.ErrNoRows {
		membership = &db.LoyaltyMembership{
			ID:             uuid.NewString(),
			CustomerID:     customerID,
			BusinessID:     businessID,
			PointsBalance:  0,
			JoinedAt:       time.Now().UTC(),
			LastActivityAt: time.Now().UTC(),
		}

		_, err = h.db.Exec(`
			INSERT INTO loyalty_memberships (id, customer_id, business_id, points_balance, joined_at, last_activity_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, membership.ID, membership.CustomerID, membership.BusinessID, membership.PointsBalance, membership.JoinedAt, membership.LastActivityAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create loyalty membership")
			return
		}
	}

	writeJSON(w, http.StatusOK, formatLoyaltyMembership(membership, program))
}

// =============================================================================
// Helper Functions
// =============================================================================

// getLoyaltyProgram retrieves a loyalty program by business ID.
// Returns sql.ErrNoRows if no program exists.
func (h *LoyaltyHandler) getLoyaltyProgram(businessID string) (*db.LoyaltyProgram, error) {
	var program db.LoyaltyProgram
	err := h.db.QueryRow(`
		SELECT id, business_id, name, points_multiplier, reward_threshold, reward_description, created_at, updated_at
		FROM loyalty_programs
		WHERE business_id = ?
	`, businessID).Scan(&program.ID, &program.BusinessID, &program.Name, &program.PointsMultiplier, &program.RewardThreshold, &program.RewardDescription, &program.CreatedAt, &program.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &program, nil
}

// getMembership retrieves a loyalty membership by customer ID and business ID.
// Returns sql.ErrNoRows if no membership exists.
func (h *LoyaltyHandler) getMembership(customerID, businessID string) (*db.LoyaltyMembership, error) {
	var membership db.LoyaltyMembership
	err := h.db.QueryRow(`
		SELECT id, customer_id, business_id, points_balance, joined_at, last_activity_at
		FROM loyalty_memberships
		WHERE customer_id = ? AND business_id = ?
	`, customerID, businessID).Scan(&membership.ID, &membership.CustomerID, &membership.BusinessID, &membership.PointsBalance, &membership.JoinedAt, &membership.LastActivityAt)
	if err != nil {
		return nil, err
	}
	return &membership, nil
}
