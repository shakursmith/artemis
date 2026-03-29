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

// setupTestProfileHandler creates a ProfileHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestProfileHandler(t *testing.T) (*ProfileHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewProfileHandler(database), database
}

// =============================================================================
// POST /api/profile — Create Profile
// =============================================================================

func TestCreateProfile_Success(t *testing.T) {
	h, _ := setupTestProfileHandler(t)

	body := `{"name": "Shakur"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleCreateProfile(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Profile
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Shakur" {
		t.Errorf("expected name 'Shakur', got '%s'", resp.Name)
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateProfile_MissingName(t *testing.T) {
	h, _ := setupTestProfileHandler(t)

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleCreateProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateProfile_InvalidJSON(t *testing.T) {
	h, _ := setupTestProfileHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/profile", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	h.HandleCreateProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// GET /api/profile/{id} — Get Profile (enriched)
// =============================================================================

func TestGetProfile_Success(t *testing.T) {
	h, database := setupTestProfileHandler(t)

	// Seed a profile with a room and device
	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "sofa")
	db.CreateDevice(database, profile.ID, "Desk Lamp", "govee_light", nil, nil)
	db.AssignDeviceToRoom(database, profile.ID, room.ID) // This won't work since we need device ID

	// Create a proper request with the path value
	req := httptest.NewRequest(http.MethodGet, "/api/profile/"+profile.ID, nil)
	req.SetPathValue("id", profile.ID)
	w := httptest.NewRecorder()

	h.HandleGetProfile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp profileDetailResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "Shakur" {
		t.Errorf("expected name 'Shakur', got '%s'", resp.Name)
	}
	if len(resp.Rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(resp.Rooms))
	}
	if len(resp.Devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(resp.Devices))
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	h, _ := setupTestProfileHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetProfile(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestGetProfile_EmptyRoomsAndDevices(t *testing.T) {
	h, database := setupTestProfileHandler(t)

	// Create a profile with no rooms or devices
	profile, _ := db.CreateProfile(database, "Empty Profile")

	req := httptest.NewRequest(http.MethodGet, "/api/profile/"+profile.ID, nil)
	req.SetPathValue("id", profile.ID)
	w := httptest.NewRecorder()

	h.HandleGetProfile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify we get empty arrays, not null
	var raw map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&raw)

	if string(raw["rooms"]) == "null" {
		t.Error("expected empty array for rooms, got null")
	}
	if string(raw["devices"]) == "null" {
		t.Error("expected empty array for devices, got null")
	}
}

// =============================================================================
// GET /api/profiles — List Profiles
// =============================================================================

func TestListProfiles_Empty(t *testing.T) {
	h, _ := setupTestProfileHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	w := httptest.NewRecorder()

	h.HandleListProfiles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Should return empty array, not null
	body := w.Body.String()
	if body == "null\n" {
		t.Error("expected empty array, got null")
	}
}

func TestListProfiles_WithData(t *testing.T) {
	h, database := setupTestProfileHandler(t)

	db.CreateProfile(database, "Alice")
	db.CreateProfile(database, "Bob")

	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	w := httptest.NewRecorder()

	h.HandleListProfiles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var profiles []db.Profile
	json.NewDecoder(w.Body).Decode(&profiles)
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}
}

// =============================================================================
// PUT /api/profile/{id} — Update Profile
// =============================================================================

func TestUpdateProfile_Success(t *testing.T) {
	h, database := setupTestProfileHandler(t)

	profile, _ := db.CreateProfile(database, "Old Name")

	body := `{"name": "New Name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/"+profile.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", profile.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateProfile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Profile
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "New Name" {
		t.Errorf("expected name 'New Name', got '%s'", resp.Name)
	}
}

func TestUpdateProfile_NotFound(t *testing.T) {
	h, _ := setupTestProfileHandler(t)

	body := `{"name": "Whatever"}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/nonexistent", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateProfile(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateProfile_MissingName(t *testing.T) {
	h, database := setupTestProfileHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile/"+profile.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", profile.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// DELETE /api/profile/{id} — Delete Profile
// =============================================================================

func TestDeleteProfile_Success(t *testing.T) {
	h, database := setupTestProfileHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/"+profile.ID, nil)
	req.SetPathValue("id", profile.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteProfile(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}

	// Verify profile is gone
	_, err := db.GetProfile(database, profile.ID)
	if err == nil {
		t.Error("expected profile to be deleted")
	}
}

func TestDeleteProfile_NotFound(t *testing.T) {
	h, _ := setupTestProfileHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleDeleteProfile(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteProfile_CascadesToRoomsAndDevices(t *testing.T) {
	h, database := setupTestProfileHandler(t)

	// Create profile with room and device
	profile, _ := db.CreateProfile(database, "Shakur")
	db.CreateRoom(database, profile.ID, "Living Room", "sofa")
	db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	// Delete the profile
	req := httptest.NewRequest(http.MethodDelete, "/api/profile/"+profile.ID, nil)
	req.SetPathValue("id", profile.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteProfile(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}

	// Rooms and devices should be gone too (CASCADE)
	rooms, _ := db.ListRoomsByProfile(database, profile.ID)
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms after cascade delete, got %d", len(rooms))
	}

	devices, _ := db.ListDevicesByProfile(database, profile.ID)
	if len(devices) != 0 {
		t.Errorf("expected 0 devices after cascade delete, got %d", len(devices))
	}
}
