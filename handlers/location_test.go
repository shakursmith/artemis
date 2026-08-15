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

// setupTestLocationHandler creates a LocationHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestLocationHandler(t *testing.T) (*LocationHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewLocationHandler(database), database
}

// =============================================================================
// POST /api/business/{businessId}/locations — Create Location
// =============================================================================

func TestCreateLocation_Success(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	// Seed a business first
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)

	body := `{"name": "Downtown Store", "address": "123 Main St"}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/locations", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleCreateLocation(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Location
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Downtown Store" {
		t.Errorf("expected name 'Downtown Store', got '%s'", resp.Name)
	}
	if resp.Address == nil || *resp.Address != "123 Main St" {
		t.Errorf("expected address '123 Main St', got %v", resp.Address)
	}
	if resp.BusinessID != business.ID {
		t.Errorf("expected business_id '%s', got '%s'", business.ID, resp.BusinessID)
	}
}

func TestCreateLocation_WithoutAddress(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)

	body := `{"name": "Airport Branch"}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/locations", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleCreateLocation(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	var resp db.Location
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Address != nil {
		t.Errorf("expected nil address, got %v", resp.Address)
	}
}

func TestCreateLocation_BusinessNotFound(t *testing.T) {
	h, _ := setupTestLocationHandler(t)

	body := `{"name": "Store"}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/nonexistent/locations", bytes.NewBufferString(body))
	req.SetPathValue("businessId", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleCreateLocation(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestCreateLocation_MissingName(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/locations", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleCreateLocation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// GET /api/business/{businessId}/locations — List Locations
// =============================================================================

func TestListLocations_Success(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	// Seed a business with locations
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	db.CreateLocation(database, business.ID, "Store A", nil)
	db.CreateLocation(database, business.ID, "Store B", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/business/"+business.ID+"/locations", nil)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleListLocations(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []db.Location
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 locations, got %d", len(resp))
	}
}

func TestListLocations_Empty(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/business/"+business.ID+"/locations", nil)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleListLocations(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp []db.Location
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected 0 locations, got %d", len(resp))
	}
}

// =============================================================================
// GET /api/location/{id} — Get Location (enriched)
// =============================================================================

func TestGetLocation_Success(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	// Seed a business, location, and rooms
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	address := "456 Oak Ave"
	location, _ := db.CreateLocation(database, business.ID, "Westside Store", &address)
	db.CreateRoomForLocation(database, location.ID, "Front", "storefront")
	db.CreateRoomForLocation(database, location.ID, "Back", "warehouse")

	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID, nil)
	req.SetPathValue("id", location.ID)
	w := httptest.NewRecorder()

	h.HandleGetLocation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp locationDetailResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Westside Store" {
		t.Errorf("expected name 'Westside Store', got '%s'", resp.Name)
	}
	if len(resp.Rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(resp.Rooms))
	}
}

func TestGetLocation_NotFound(t *testing.T) {
	h, _ := setupTestLocationHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/location/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetLocation(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/location/{id} — Update Location
// =============================================================================

func TestUpdateLocation_Success(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	// Seed a location
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	address1 := "Old Address"
	location, _ := db.CreateLocation(database, business.ID, "Old Name", &address1)

	// Update it
	body := `{"name": "New Name", "address": "New Address"}`
	req := httptest.NewRequest(http.MethodPut, "/api/location/"+location.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", location.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateLocation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Location
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "New Name" {
		t.Errorf("expected name 'New Name', got '%s'", resp.Name)
	}
	if resp.Address == nil || *resp.Address != "New Address" {
		t.Errorf("expected address 'New Address', got %v", resp.Address)
	}
}

func TestUpdateLocation_NotFound(t *testing.T) {
	h, _ := setupTestLocationHandler(t)

	body := `{"name": "New Name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/location/nonexistent", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateLocation(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// DELETE /api/location/{id} — Delete Location
// =============================================================================

func TestDeleteLocation_Success(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	// Seed a location
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	location, _ := db.CreateLocation(database, business.ID, "Store", nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/location/"+location.ID, nil)
	req.SetPathValue("id", location.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteLocation(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify it's deleted
	_, err := db.GetLocation(database, location.ID)
	if err == nil {
		t.Error("expected location to be deleted")
	}
}

func TestDeleteLocation_NotFound(t *testing.T) {
	h, _ := setupTestLocationHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/location/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleDeleteLocation(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// Test cascade: deleting location should delete rooms
func TestDeleteLocation_Cascade(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	// Create business → location → room
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	location, _ := db.CreateLocation(database, business.ID, "Downtown", nil)
	room, _ := db.CreateRoomForLocation(database, location.ID, "Main Hall", "storefront")

	// Delete the location
	req := httptest.NewRequest(http.MethodDelete, "/api/location/"+location.ID, nil)
	req.SetPathValue("id", location.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteLocation(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}

	// Verify room is also deleted
	_, err := db.GetRoom(database, room.ID)
	if err == nil {
		t.Error("expected room to be deleted via cascade")
	}
}

// =============================================================================
// POST /api/location/{locationId}/rooms — Create Room for Location
// =============================================================================

func TestCreateRoomForLocation_Success(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	// Seed a business and location
	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	location, _ := db.CreateLocation(database, business.ID, "Downtown", nil)

	body := `{"name": "Main Hall", "icon": "storefront"}`
	req := httptest.NewRequest(http.MethodPost, "/api/location/"+location.ID+"/rooms", bytes.NewBufferString(body))
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	h.HandleCreateRoomForLocation(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Room
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Main Hall" {
		t.Errorf("expected name 'Main Hall', got '%s'", resp.Name)
	}
	if resp.Icon != "storefront" {
		t.Errorf("expected icon 'storefront', got '%s'", resp.Icon)
	}
	if resp.LocationID == nil || *resp.LocationID != location.ID {
		t.Errorf("expected location_id '%s', got %v", location.ID, resp.LocationID)
	}
}

func TestCreateRoomForLocation_LocationNotFound(t *testing.T) {
	h, _ := setupTestLocationHandler(t)

	body := `{"name": "Main Hall", "icon": "storefront"}`
	req := httptest.NewRequest(http.MethodPost, "/api/location/nonexistent/rooms", bytes.NewBufferString(body))
	req.SetPathValue("locationId", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleCreateRoomForLocation(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestCreateRoomForLocation_MissingName(t *testing.T) {
	h, database := setupTestLocationHandler(t)

	business, _ := db.CreateBusiness(database, "Chain Cafe", nil)
	location, _ := db.CreateLocation(database, business.ID, "Downtown", nil)

	body := `{"name": "", "icon": "storefront"}`
	req := httptest.NewRequest(http.MethodPost, "/api/location/"+location.ID+"/rooms", bytes.NewBufferString(body))
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	h.HandleCreateRoomForLocation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}
