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

// setupTestCustomerHandler creates a CustomerHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestCustomerHandler(t *testing.T) (*CustomerHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewCustomerHandler(database), database
}

// =============================================================================
// POST /api/customer — Create Customer
// =============================================================================

func TestCreateCustomer_Success(t *testing.T) {
	h, _ := setupTestCustomerHandler(t)

	body := `{"firstName": "Shakur"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleCreateCustomer(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Customer
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.FirstName != "Shakur" {
		t.Errorf("expected firstName 'Shakur', got '%s'", resp.FirstName)
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateCustomer_MissingFirstName(t *testing.T) {
	h, _ := setupTestCustomerHandler(t)

	body := `{"firstName": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleCreateCustomer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateCustomer_InvalidJSON(t *testing.T) {
	h, _ := setupTestCustomerHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/customer", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	h.HandleCreateCustomer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// GET /api/customer/{id} — Get Customer
// =============================================================================

func TestGetCustomer_Success(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed a customer
	customer, _ := db.CreateCustomer(database, "Jordan")

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID, nil)
	req.SetPathValue("id", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Customer
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.FirstName != "Jordan" {
		t.Errorf("expected firstName 'Jordan', got '%s'", resp.FirstName)
	}
}

func TestGetCustomer_NotFound(t *testing.T) {
	h, _ := setupTestCustomerHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/customer/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetCustomer(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/customer/{id} — Update Customer
// =============================================================================

func TestUpdateCustomer_Success(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed a customer
	customer, _ := db.CreateCustomer(database, "Original Name")

	body := `{"firstName": "Updated Name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/customer/"+customer.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", customer.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateCustomer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Customer
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.FirstName != "Updated Name" {
		t.Errorf("expected firstName 'Updated Name', got '%s'", resp.FirstName)
	}
}

func TestUpdateCustomer_NotFound(t *testing.T) {
	h, _ := setupTestCustomerHandler(t)

	body := `{"firstName": "New Name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/customer/nonexistent", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateCustomer(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// GET /api/customer/{id}/sharing — Get Sharing Settings
// =============================================================================

func TestGetSharingSettings_Success(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed a customer (also creates default sharing settings)
	customer, _ := db.CreateCustomer(database, "Taylor")

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/sharing", nil)
	req.SetPathValue("id", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetSharingSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.CustomerSharingSettings
	json.NewDecoder(w.Body).Decode(&resp)

	// Verify default settings
	if resp.SharingLevel != "minimal" {
		t.Errorf("expected default sharing level 'minimal', got '%s'", resp.SharingLevel)
	}
	if len(resp.BlockedBusinesses) != 0 {
		t.Errorf("expected empty blocked businesses, got %v", resp.BlockedBusinesses)
	}
}

func TestGetSharingSettings_NotFound(t *testing.T) {
	h, _ := setupTestCustomerHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/customer/nonexistent/sharing", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetSharingSettings(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/customer/{id}/sharing — Update Sharing Settings
// =============================================================================

func TestUpdateSharingSettings_ChangeLevel(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed a customer
	customer, _ := db.CreateCustomer(database, "Alex")

	body := `{"sharingLevel": "standard"}`
	req := httptest.NewRequest(http.MethodPut, "/api/customer/"+customer.ID+"/sharing", bytes.NewBufferString(body))
	req.SetPathValue("id", customer.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateSharingSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.CustomerSharingSettings
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.SharingLevel != "standard" {
		t.Errorf("expected sharing level 'standard', got '%s'", resp.SharingLevel)
	}
}

func TestUpdateSharingSettings_BlockBusiness(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed a customer and a business
	customer, _ := db.CreateCustomer(database, "Sam")
	business, _ := db.CreateBusiness(database, "Test Business", nil)

	body := `{"blockedBusinesses": ["` + business.ID + `"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/customer/"+customer.ID+"/sharing", bytes.NewBufferString(body))
	req.SetPathValue("id", customer.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateSharingSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.CustomerSharingSettings
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.BlockedBusinesses) != 1 || resp.BlockedBusinesses[0] != business.ID {
		t.Errorf("expected blocked businesses to contain %s, got %v", business.ID, resp.BlockedBusinesses)
	}
}

func TestUpdateSharingSettings_InvalidLevel(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed a customer
	customer, _ := db.CreateCustomer(database, "Chris")

	body := `{"sharingLevel": "invalid_level"}`
	req := httptest.NewRequest(http.MethodPut, "/api/customer/"+customer.ID+"/sharing", bytes.NewBufferString(body))
	req.SetPathValue("id", customer.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateSharingSettings(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateSharingSettings_NotFound(t *testing.T) {
	h, _ := setupTestCustomerHandler(t)

	body := `{"sharingLevel": "full"}`
	req := httptest.NewRequest(http.MethodPut, "/api/customer/nonexistent/sharing", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateSharingSettings(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// GET /api/customer/{customerId}/profile — Get Customer Profile (business-facing)
// =============================================================================

func TestGetCustomerProfile_MinimalSharing(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed customer with minimal sharing (default)
	customer, _ := db.CreateCustomer(database, "Riley")
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/profile?businessId="+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerProfile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp customerProfileResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Verify minimal sharing: only firstName and sharingLevel
	if resp.FirstName != "Riley" {
		t.Errorf("expected firstName 'Riley', got '%s'", resp.FirstName)
	}
	if resp.SharingLevel != "minimal" {
		t.Errorf("expected sharingLevel 'minimal', got '%s'", resp.SharingLevel)
	}
	// These should be nil/omitted for minimal sharing
	if resp.PurchaseHistory != nil {
		t.Error("expected purchaseHistory to be nil for minimal sharing")
	}
	if resp.LoyaltyStatus != nil {
		t.Error("expected loyaltyStatus to be nil for minimal sharing")
	}
	if resp.ReviewCount != nil {
		t.Error("expected reviewCount to be nil for minimal sharing")
	}
}

func TestGetCustomerProfile_StandardSharing(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed customer with standard sharing
	customer, _ := db.CreateCustomer(database, "Morgan")
	business, _ := db.CreateBusiness(database, "Test Cafe", nil)

	// Update to standard sharing
	db.UpdateCustomerSharingSettings(database, customer.ID, "standard", []string{})

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/profile?businessId="+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerProfile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp customerProfileResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.SharingLevel != "standard" {
		t.Errorf("expected sharingLevel 'standard', got '%s'", resp.SharingLevel)
	}
	// Note: purchaseHistory and loyaltyStatus are nil here because we don't have orders/loyalty tables yet
	// This simulates a "new customer" scenario
}

func TestGetCustomerProfile_FullSharing(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed customer with full sharing
	customer, _ := db.CreateCustomer(database, "Casey")
	business, _ := db.CreateBusiness(database, "Test Shop", nil)

	// Update to full sharing
	db.UpdateCustomerSharingSettings(database, customer.ID, "full", []string{})

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/profile?businessId="+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerProfile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp customerProfileResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.SharingLevel != "full" {
		t.Errorf("expected sharingLevel 'full', got '%s'", resp.SharingLevel)
	}
}

func TestGetCustomerProfile_BlockedBusiness(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed customer and business
	customer, _ := db.CreateCustomer(database, "Jamie")
	business, _ := db.CreateBusiness(database, "Blocked Business", nil)

	// Block the business
	db.UpdateCustomerSharingSettings(database, customer.ID, "standard", []string{business.ID})

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/profile?businessId="+business.ID, nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerProfile(w, req)

	// Should return 404 for blocked businesses
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for blocked business, got %d", w.Code)
	}
}

func TestGetCustomerProfile_MissingBusinessId(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	// Seed customer
	customer, _ := db.CreateCustomer(database, "Drew")

	// Request without businessId query parameter
	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/profile", nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetCustomerProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing businessId, got %d", w.Code)
	}
}

func TestGetCustomerProfile_NotFound(t *testing.T) {
	h, database := setupTestCustomerHandler(t)

	business, _ := db.CreateBusiness(database, "Test Business", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/customer/nonexistent/profile?businessId="+business.ID, nil)
	req.SetPathValue("customerId", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetCustomerProfile(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}
