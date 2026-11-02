package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pantheon/artemis/db"
)

// setupTestLoyaltyHandler creates a LoyaltyHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestLoyaltyHandler(t *testing.T) (*LoyaltyHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewLoyaltyHandler(database), database
}

// =============================================================================
// POST /api/business/{businessId}/loyalty-program — Create Loyalty Program
// =============================================================================

func TestCreateLoyaltyProgram_Success(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed a business
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	body := `{"name": "Cafe Rewards", "pointsMultiplier": 1.0, "rewardThreshold": 100, "rewardDescription": "Free coffee"}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/loyalty-program", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleCreateLoyaltyProgram(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp loyaltyProgramResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Cafe Rewards" {
		t.Errorf("expected name 'Cafe Rewards', got '%s'", resp.Name)
	}
	if resp.PointsMultiplier != 1.0 {
		t.Errorf("expected pointsMultiplier 1.0, got %.2f", resp.PointsMultiplier)
	}
	if resp.RewardThreshold != 100 {
		t.Errorf("expected rewardThreshold 100, got %d", resp.RewardThreshold)
	}
	if resp.RewardDescription != "Free coffee" {
		t.Errorf("expected rewardDescription 'Free coffee', got '%s'", resp.RewardDescription)
	}
}

func TestCreateLoyaltyProgram_AlreadyExists(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed a business with a loyalty program
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)
	_, err := database.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description)
		VALUES ('prog1', ?, 'Existing Program', 1.0, 100, 'Free coffee')
	`, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed loyalty program: %v", err)
	}

	body := `{"name": "New Program", "pointsMultiplier": 2.0, "rewardThreshold": 200, "rewardDescription": "Free pastry"}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/loyalty-program", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleCreateLoyaltyProgram(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateLoyaltyProgram_InvalidData(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	tests := []struct {
		name string
		body string
		want int
	}{
		{"MissingName", `{"pointsMultiplier": 1.0, "rewardThreshold": 100, "rewardDescription": "Free coffee"}`, http.StatusBadRequest},
		{"NegativeMultiplier", `{"name": "Rewards", "pointsMultiplier": -1.0, "rewardThreshold": 100, "rewardDescription": "Free coffee"}`, http.StatusBadRequest},
		{"ZeroThreshold", `{"name": "Rewards", "pointsMultiplier": 1.0, "rewardThreshold": 0, "rewardDescription": "Free coffee"}`, http.StatusBadRequest},
		{"MissingDescription", `{"name": "Rewards", "pointsMultiplier": 1.0, "rewardThreshold": 100, "rewardDescription": ""}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/loyalty-program", bytes.NewBufferString(tt.body))
			req.SetPathValue("businessId", business.ID)
			w := httptest.NewRecorder()

			h.HandleCreateLoyaltyProgram(w, req)

			if w.Code != tt.want {
				t.Errorf("expected status %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

// =============================================================================
// GET /api/business/{businessId}/loyalty-program — Get Loyalty Program
// =============================================================================

func TestGetLoyaltyProgram_Success(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed a business with a loyalty program
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)
	_, err := database.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description)
		VALUES ('prog1', ?, 'Cafe Rewards', 1.5, 150, 'Free pastry')
	`, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed loyalty program: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/business/"+business.ID+"/loyalty-program", nil)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetLoyaltyProgram(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loyaltyProgramResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Cafe Rewards" {
		t.Errorf("expected name 'Cafe Rewards', got '%s'", resp.Name)
	}
	if resp.PointsMultiplier != 1.5 {
		t.Errorf("expected pointsMultiplier 1.5, got %.2f", resp.PointsMultiplier)
	}
}

func TestGetLoyaltyProgram_NotFound(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed a business without a loyalty program
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/business/"+business.ID+"/loyalty-program", nil)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetLoyaltyProgram(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// PUT /api/business/{businessId}/loyalty-program — Update Loyalty Program
// =============================================================================

func TestUpdateLoyaltyProgram_Success(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed a business with a loyalty program
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)
	_, err := database.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description)
		VALUES ('prog1', ?, 'Cafe Rewards', 1.0, 100, 'Free coffee')
	`, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed loyalty program: %v", err)
	}

	body := `{"name": "Premium Rewards", "pointsMultiplier": 2.0, "rewardThreshold": 200, "rewardDescription": "Free meal"}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID+"/loyalty-program", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateLoyaltyProgram(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loyaltyProgramResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Premium Rewards" {
		t.Errorf("expected name 'Premium Rewards', got '%s'", resp.Name)
	}
	if resp.PointsMultiplier != 2.0 {
		t.Errorf("expected pointsMultiplier 2.0, got %.2f", resp.PointsMultiplier)
	}
	if resp.RewardThreshold != 200 {
		t.Errorf("expected rewardThreshold 200, got %d", resp.RewardThreshold)
	}
}

func TestUpdateLoyaltyProgram_NotFound(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed a business without a loyalty program
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	body := `{"name": "Premium Rewards", "pointsMultiplier": 2.0, "rewardThreshold": 200, "rewardDescription": "Free meal"}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID+"/loyalty-program", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateLoyaltyProgram(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /api/customer/{customerId}/loyalty-memberships — List Memberships
// =============================================================================

func TestGetCustomerMemberships_Success(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed customer, businesses, programs, and memberships
	customer, _ := db.CreateCustomer(database, "Alex")
	business1, _ := db.CreateBusiness(database, "Coffee Shop", nil)
	business2, _ := db.CreateBusiness(database, "Bakery", nil)

	_, err := database.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description)
		VALUES ('prog1', ?, 'Coffee Rewards', 1.0, 100, 'Free coffee'), ('prog2', ?, 'Bakery Rewards', 2.0, 200, 'Free pastry')
	`, business1.ID, business2.ID)
	if err != nil {
		t.Fatalf("Failed to seed programs: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO loyalty_memberships (id, customer_id, business_id, points_balance)
		VALUES ('mem1', ?, ?, 50), ('mem2', ?, ?, 150)
	`, customer.ID, business1.ID, customer.ID, business2.ID)
	if err != nil {
		t.Fatalf("Failed to seed memberships: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/loyalty-memberships", nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerMemberships(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []loyaltyMembershipResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 memberships, got %d", len(resp))
	}
}

func TestGetCustomerMemberships_EmptyList(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/loyalty-memberships", nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerMemberships(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []loyaltyMembershipResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected 0 memberships, got %d", len(resp))
	}
}

// =============================================================================
// GET /api/customer/{customerId}/loyalty-membership/{businessId} — Get Membership (Auto-Create)
// =============================================================================

func TestGetCustomerMembership_ExistingMembership(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed customer, business, program, and membership
	customer, _ := db.CreateCustomer(database, "Alex")
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	_, err := database.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description)
		VALUES ('prog1', ?, 'Coffee Rewards', 1.0, 100, 'Free coffee')
	`, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed program: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO loyalty_memberships (id, customer_id, business_id, points_balance)
		VALUES ('mem1', ?, ?, 75)
	`, customer.ID, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed membership: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/loyalty-membership/"+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerMembership(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loyaltyMembershipResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.PointsBalance != 75 {
		t.Errorf("expected pointsBalance 75, got %d", resp.PointsBalance)
	}
	if resp.HasEarnedReward {
		t.Error("expected hasEarnedReward false (75 < 100)")
	}
}

func TestGetCustomerMembership_AutoCreate(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed customer, business, and program (no membership yet)
	customer, _ := db.CreateCustomer(database, "Alex")
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	_, err := database.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description)
		VALUES ('prog1', ?, 'Coffee Rewards', 1.0, 100, 'Free coffee')
	`, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed program: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/loyalty-membership/"+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerMembership(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loyaltyMembershipResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.PointsBalance != 0 {
		t.Errorf("expected pointsBalance 0, got %d", resp.PointsBalance)
	}
	if resp.HasEarnedReward {
		t.Error("expected hasEarnedReward false (0 < 100)")
	}

	// Verify membership was created in DB
	var count int
	database.QueryRow("SELECT COUNT(*) FROM loyalty_memberships WHERE customer_id = ? AND business_id = ?", customer.ID, business.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 membership created, got %d", count)
	}
}

func TestGetCustomerMembership_NoProgramExists(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed customer and business (no loyalty program)
	customer, _ := db.CreateCustomer(database, "Alex")
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/loyalty-membership/"+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerMembership(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetCustomerMembership_HasEarnedReward(t *testing.T) {
	h, database := setupTestLoyaltyHandler(t)

	// Seed customer, business, program, and membership with enough points
	customer, _ := db.CreateCustomer(database, "Alex")
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	_, err := database.Exec(`
		INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description)
		VALUES ('prog1', ?, 'Coffee Rewards', 1.0, 100, 'Free coffee')
	`, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed program: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO loyalty_memberships (id, customer_id, business_id, points_balance)
		VALUES ('mem1', ?, ?, 150)
	`, customer.ID, business.ID)
	if err != nil {
		t.Fatalf("Failed to seed membership: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/loyalty-membership/"+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerMembership(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loyaltyMembershipResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.PointsBalance != 150 {
		t.Errorf("expected pointsBalance 150, got %d", resp.PointsBalance)
	}
	if !resp.HasEarnedReward {
		t.Error("expected hasEarnedReward true (150 >= 100)")
	}
}
