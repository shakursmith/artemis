package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pantheon/artemis/db"
)

// setupTestRoomTemplateHandler creates a RoomTemplateHandler backed by an
// in-memory SQLite DB. Also creates a test profile and a room for template lookups.
func setupTestRoomTemplateHandler(t *testing.T, roomName string) (*RoomTemplateHandler, *sql.DB, *db.Room) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// Create a test profile (rooms require a parent profile).
	profile, err := db.CreateProfile(database, "Test User")
	if err != nil {
		t.Fatalf("Failed to create test profile: %v", err)
	}

	// Create a room with the given name.
	room, err := db.CreateRoom(database, profile.ID, roomName, "house")
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}

	return NewRoomTemplateHandler(database), database, room
}

// =============================================================================
// GET /api/room/{id}/template — Get Room Template
// =============================================================================

// TestGetRoomTemplate_LivingRoom verifies the Living Room template is returned
// correctly with all expected fields (wall, floor, 3 interactables).
func TestGetRoomTemplate_LivingRoom(t *testing.T) {
	h, _, room := setupTestRoomTemplateHandler(t, "Living Room")

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleGetRoomTemplate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp roomTemplateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "Living Room" {
		t.Errorf("expected name 'Living Room', got '%s'", resp.Name)
	}
	if resp.ID != "living_room" {
		t.Errorf("expected id 'living_room', got '%s'", resp.ID)
	}
	if len(resp.Elements) < 2 {
		t.Errorf("expected at least 2 elements, got %d", len(resp.Elements))
	}

	// Count interactables — Living Room should have 3 (tv, lamp, camera).
	interactables := 0
	for _, e := range resp.Elements {
		if e.Interaction != nil {
			interactables++
		}
	}
	if interactables != 3 {
		t.Errorf("expected 3 interactables, got %d", interactables)
	}
}

// TestGetRoomTemplate_Office verifies the Office template has 3 interactables
// with correct stateKeys (monitorPowered, deskLampOn, officeCameraActive).
func TestGetRoomTemplate_Office(t *testing.T) {
	h, _, room := setupTestRoomTemplateHandler(t, "Office")

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleGetRoomTemplate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp roomTemplateResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "Office" {
		t.Errorf("expected name 'Office', got '%s'", resp.Name)
	}

	// Verify interactable stateKeys.
	expectedKeys := map[string]bool{
		"monitorPowered":     false,
		"deskLampOn":         false,
		"officeCameraActive": false,
	}
	for _, e := range resp.Elements {
		if e.Interaction != nil {
			expectedKeys[e.Interaction.StateKey] = true
		}
	}
	for key, found := range expectedKeys {
		if !found {
			t.Errorf("expected stateKey '%s' not found in template", key)
		}
	}
}

// TestGetRoomTemplate_UnknownRoom verifies that an unknown room name returns
// a minimal generic template (wall + floor only, no interactables).
func TestGetRoomTemplate_UnknownRoom(t *testing.T) {
	h, _, room := setupTestRoomTemplateHandler(t, "Garage")

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleGetRoomTemplate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp roomTemplateResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Generic template should have the room's actual name.
	if resp.Name != "Garage" {
		t.Errorf("expected name 'Garage', got '%s'", resp.Name)
	}

	// Should only have wall and floor (2 elements), no interactables.
	if len(resp.Elements) != 2 {
		t.Errorf("expected 2 elements (wall + floor), got %d", len(resp.Elements))
	}
	for _, e := range resp.Elements {
		if e.Interaction != nil {
			t.Errorf("generic template should have no interactables, found one: %s", e.ID)
		}
	}
}

// TestGetRoomTemplate_NotFound verifies that a non-existent room ID returns 404.
func TestGetRoomTemplate_NotFound(t *testing.T) {
	h, _, _ := setupTestRoomTemplateHandler(t, "Office")

	req := httptest.NewRequest(http.MethodGet, "/api/room/nonexistent-uuid/template", nil)
	req.SetPathValue("id", "nonexistent-uuid")
	w := httptest.NewRecorder()

	h.HandleGetRoomTemplate(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetRoomTemplate_Description verifies that templates include the description field.
func TestGetRoomTemplate_Description(t *testing.T) {
	h, _, room := setupTestRoomTemplateHandler(t, "Bedroom")

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleGetRoomTemplate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp roomTemplateResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Description == nil {
		t.Fatal("expected description to be non-nil for Bedroom template")
	}
	if *resp.Description == "" {
		t.Error("expected description to be non-empty for Bedroom template")
	}
}
