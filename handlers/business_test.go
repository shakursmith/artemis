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

// setupTestBusinessHandler creates a BusinessHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestBusinessHandler(t *testing.T) (*BusinessHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewBusinessHandler(database), database
}

// =============================================================================
// POST /api/business — Create Business
// =============================================================================

func TestCreateBusiness_Success(t *testing.T) {
	h, _ := setupTestBusinessHandler(t)

	body := `{"name": "Yummy Cafe", "ownerName": "Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/business", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleCreateBusiness(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Business
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Yummy Cafe" {
		t.Errorf("expected name 'Yummy Cafe', got '%s'", resp.Name)
	}
	if resp.OwnerName == nil || *resp.OwnerName != "Alice" {
		t.Errorf("expected owner name 'Alice', got %v", resp.OwnerName)
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateBusiness_WithoutOwner(t *testing.T) {
	h, _ := setupTestBusinessHandler(t)

	body := `{"name": "Coffee Shop"}`
	req := httptest.NewRequest(http.MethodPost, "/api/business", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleCreateBusiness(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Business
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.OwnerName != nil {
		t.Errorf("expected nil owner name, got %v", resp.OwnerName)
	}
}

func TestCreateBusiness_MissingName(t *testing.T) {
	h, _ := setupTestBusinessHandler(t)

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/business", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleCreateBusiness(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateBusiness_InvalidJSON(t *testing.T) {
	h, _ := setupTestBusinessHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/business", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	h.HandleCreateBusiness(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// GET /api/business/{id} — Get Business (enriched)
// =============================================================================

func TestGetBusiness_Success(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	// Seed a business with locations
	ownerName := "Alice"
	business, _ := db.CreateBusiness(database, "Chain Cafe", &ownerName)
	address := "123 Main St"
	db.CreateLocation(database, business.ID, "Downtown Store", &address)
	db.CreateLocation(database, business.ID, "Airport Branch", nil)

	// Create a proper request with the path value
	req := httptest.NewRequest(http.MethodGet, "/api/business/"+business.ID, nil)
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetBusiness(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp businessDetailResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Chain Cafe" {
		t.Errorf("expected name 'Chain Cafe', got '%s'", resp.Name)
	}
	if len(resp.Locations) != 2 {
		t.Errorf("expected 2 locations, got %d", len(resp.Locations))
	}
}

func TestGetBusiness_NotFound(t *testing.T) {
	h, _ := setupTestBusinessHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/business/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetBusiness(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/business/{id} — Update Business
// =============================================================================

func TestUpdateBusiness_Success(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	// Seed a business
	ownerName := "Alice"
	business, _ := db.CreateBusiness(database, "Old Name", &ownerName)

	// Update it
	body := `{"name": "New Name", "ownerName": "Bob"}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateBusiness(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Business
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "New Name" {
		t.Errorf("expected name 'New Name', got '%s'", resp.Name)
	}
	if resp.OwnerName == nil || *resp.OwnerName != "Bob" {
		t.Errorf("expected owner 'Bob', got %v", resp.OwnerName)
	}
}

func TestUpdateBusiness_NotFound(t *testing.T) {
	h, _ := setupTestBusinessHandler(t)

	body := `{"name": "New Name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/nonexistent", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateBusiness(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateBusiness_MissingName(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	business, _ := db.CreateBusiness(database, "Old Name", nil)

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateBusiness(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// DELETE /api/business/{id} — Delete Business
// =============================================================================

func TestDeleteBusiness_Success(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	// Seed a business
	business, _ := db.CreateBusiness(database, "Yummy Cafe", nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/business/"+business.ID, nil)
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteBusiness(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify it's deleted
	_, err := db.GetBusiness(database, business.ID)
	if err == nil {
		t.Error("expected business to be deleted")
	}
}

func TestDeleteBusiness_NotFound(t *testing.T) {
	h, _ := setupTestBusinessHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/business/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleDeleteBusiness(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// Test cascade: deleting business should delete locations
func TestDeleteBusiness_Cascade(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	// Create business → location
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	location, _ := db.CreateLocation(database, business.ID, "Downtown", nil)

	// Delete the business
	req := httptest.NewRequest(http.MethodDelete, "/api/business/"+business.ID, nil)
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteBusiness(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}

	// Verify location is also deleted
	_, err := db.GetLocation(database, location.ID)
	if err == nil {
		t.Error("expected location to be deleted via cascade")
	}
}

// =============================================================================
// Business Branding Fields Tests (Ticket 008)
// =============================================================================

func TestUpdateBusiness_BrandingFields(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	// Seed a business
	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	// Update with all branding fields
	body := `{
		"name": "Coffee Shop",
		"ownerName": "Jane",
		"welcomeMessage": "Welcome to our shop!",
		"website": "https://coffeeshop.com",
		"phone": "+14155551234",
		"logoUrl": "https://cdn.example.com/logo.png",
		"accentColor": "#8B4513",
		"heroImageUrl": "https://cdn.example.com/hero.jpg"
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateBusiness(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Business
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.WelcomeMessage == nil || *resp.WelcomeMessage != "Welcome to our shop!" {
		t.Errorf("expected welcome message, got %v", resp.WelcomeMessage)
	}
	if resp.Website == nil || *resp.Website != "https://coffeeshop.com" {
		t.Errorf("expected website, got %v", resp.Website)
	}
	if resp.Phone == nil || *resp.Phone != "+14155551234" {
		t.Errorf("expected phone, got %v", resp.Phone)
	}
	if resp.LogoURL == nil || *resp.LogoURL != "https://cdn.example.com/logo.png" {
		t.Errorf("expected logo URL, got %v", resp.LogoURL)
	}
	if resp.AccentColor == nil || *resp.AccentColor != "#8B4513" {
		t.Errorf("expected accent color #8B4513, got %v", resp.AccentColor)
	}
	if resp.HeroImageURL == nil || *resp.HeroImageURL != "https://cdn.example.com/hero.jpg" {
		t.Errorf("expected hero image URL, got %v", resp.HeroImageURL)
	}
}

func TestUpdateBusiness_InvalidHexColor(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	body := `{"name": "Coffee Shop", "accentColor": "not-a-color"}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateBusiness(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateBusiness_WelcomeMessageTooLong(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	business, _ := db.CreateBusiness(database, "Coffee Shop", nil)

	// Create a message longer than 500 characters
	longMessage := ""
	for i := 0; i < 501; i++ {
		longMessage += "a"
	}

	body := `{"name": "Coffee Shop", "welcomeMessage": "` + longMessage + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateBusiness(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestGetBusiness_IncludesBrandingFields(t *testing.T) {
	h, database := setupTestBusinessHandler(t)

	// Create business with branding fields
	ownerName := "Jane"
	welcomeMsg := "Welcome!"
	website := "https://example.com"
	phone := "+14155551234"
	logoURL := "https://cdn.example.com/logo.png"
	accentColor := "#FF5733"
	heroImageURL := "https://cdn.example.com/hero.jpg"

	business, _ := db.CreateBusiness(database, "Branded Cafe", &ownerName)
	db.UpdateBusiness(database, business.ID, "Branded Cafe", &ownerName, &welcomeMsg, &website, &phone, &logoURL, &accentColor, &heroImageURL)

	req := httptest.NewRequest(http.MethodGet, "/api/business/"+business.ID, nil)
	req.SetPathValue("id", business.ID)
	w := httptest.NewRecorder()

	h.HandleGetBusiness(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp businessDetailResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.WelcomeMessage == nil || *resp.WelcomeMessage != "Welcome!" {
		t.Errorf("expected welcome message in GET response, got %v", resp.WelcomeMessage)
	}
	if resp.AccentColor == nil || *resp.AccentColor != "#FF5733" {
		t.Errorf("expected accent color in GET response, got %v", resp.AccentColor)
	}
}
