package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestGetRoomTemplate_Office verifies the Office template has 7 interactables
// (monitor, 5 Govee lights, camera) with correct stateKeys and canvasScale.
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

	// Office uses canvasScale 2.5 for panoramic view.
	if resp.CanvasScale == nil {
		t.Fatal("expected canvasScale to be non-nil for Office template")
	}
	if *resp.CanvasScale != 2.5 {
		t.Errorf("expected canvasScale 2.5, got %f", *resp.CanvasScale)
	}

	// Verify interactable stateKeys — 7 interactables total:
	// 1 monitor, 5 Govee lights, 1 camera.
	expectedKeys := map[string]bool{
		"monitorPowered":     false,
		"shakurDeskLightOn":  false,
		"shakurDeskLampOn":   false,
		"officeLampBottomOn": false,
		"officeLampTopOn":    false,
		"najahDeskLightOn":   false,
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

	// Verify that Govee lights have deviceId bindings.
	goveeWithDeviceId := 0
	for _, e := range resp.Elements {
		if e.Interaction != nil && e.Interaction.Type == "govee" && e.Interaction.DeviceID != nil {
			goveeWithDeviceId++
		}
	}
	if goveeWithDeviceId != 5 {
		t.Errorf("expected 5 Govee lights with deviceId, got %d", goveeWithDeviceId)
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

// =============================================================================
// PUT /api/room/{id}/template — Save Custom Template
// =============================================================================

// TestSaveCustomTemplate verifies that PUT saves a custom template and GET
// returns it instead of the default.
func TestSaveCustomTemplate(t *testing.T) {
	h, database, room := setupTestRoomTemplateHandler(t, "Living Room")

	// Save a custom template via PUT.
	customJSON := `{"id":"custom","name":"Custom Living Room","backgroundColor":"#FF0000","elements":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/"+room.ID+"/template",
		strings.NewReader(customJSON))
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleSaveRoomTemplate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on PUT, got %d: %s", w.Code, w.Body.String())
	}

	// Now GET should return the custom template, not the default.
	req2 := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req2.SetPathValue("id", room.ID)
	w2 := httptest.NewRecorder()

	h.HandleGetRoomTemplate(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 on GET, got %d: %s", w2.Code, w2.Body.String())
	}

	// Decode the custom template response.
	var resp roomTemplateResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode custom template response: %v", err)
	}

	// Should be our custom template, not the default Living Room template.
	if resp.Name != "Custom Living Room" {
		t.Errorf("expected custom name 'Custom Living Room', got '%s'", resp.Name)
	}
	if resp.BackgroundColor != "#FF0000" {
		t.Errorf("expected custom backgroundColor '#FF0000', got '%s'", resp.BackgroundColor)
	}

	_ = database // keep the DB alive
}

// TestDeleteCustomTemplate verifies that DELETE clears the custom template
// and GET reverts to the default.
func TestDeleteCustomTemplate(t *testing.T) {
	h, _, room := setupTestRoomTemplateHandler(t, "Office")

	// Save a custom template first.
	customJSON := `{"id":"custom_office","name":"My Office","backgroundColor":"#00FF00","elements":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/"+room.ID+"/template",
		strings.NewReader(customJSON))
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()
	h.HandleSaveRoomTemplate(w, req)

	// DELETE the custom template.
	req2 := httptest.NewRequest(http.MethodDelete, "/api/room/"+room.ID+"/template", nil)
	req2.SetPathValue("id", room.ID)
	w2 := httptest.NewRecorder()
	h.HandleDeleteRoomTemplate(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 on DELETE, got %d: %s", w2.Code, w2.Body.String())
	}

	// GET should now return the default Office template.
	req3 := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req3.SetPathValue("id", room.ID)
	w3 := httptest.NewRecorder()
	h.HandleGetRoomTemplate(w3, req3)

	var resp roomTemplateResponse
	json.NewDecoder(w3.Body).Decode(&resp)

	// Default Office template has name "Office".
	if resp.Name != "Office" {
		t.Errorf("expected default name 'Office' after delete, got '%s'", resp.Name)
	}
}

// TestCustomTemplateSurvivesRoomUpdate verifies that updating a room's name/icon
// does not affect the stored custom template.
func TestCustomTemplateSurvivesRoomUpdate(t *testing.T) {
	h, database, room := setupTestRoomTemplateHandler(t, "Living Room")

	// Save a custom template.
	customJSON := `{"id":"custom","name":"My Custom Room","backgroundColor":"#123456","elements":[]}`
	db.SaveRoomTemplate(database, room.ID, customJSON)

	// Update the room's name and icon.
	db.UpdateRoom(database, room.ID, "Updated Room", "star")

	// GET template should still return the custom template.
	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()
	h.HandleGetRoomTemplate(w, req)

	var resp roomTemplateResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "My Custom Room" {
		t.Errorf("expected custom template to survive room update, got name '%s'", resp.Name)
	}
}

// TestCustomTemplateDeletedWithRoom verifies that deleting a room removes its
// custom template along with it (the row is deleted entirely).
func TestCustomTemplateDeletedWithRoom(t *testing.T) {
	h, database, room := setupTestRoomTemplateHandler(t, "Bedroom")

	// Save a custom template.
	customJSON := `{"id":"custom_bedroom","name":"My Bedroom","backgroundColor":"#000000","elements":[]}`
	db.SaveRoomTemplate(database, room.ID, customJSON)

	// Delete the room entirely.
	db.DeleteRoom(database, room.ID)

	// GET template for the deleted room should return 404.
	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/template", nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()
	h.HandleGetRoomTemplate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleted room's template, got %d", w.Code)
	}
}

// TestSaveInvalidTemplateJSON verifies that PUT with non-JSON body returns 400.
func TestSaveInvalidTemplateJSON(t *testing.T) {
	h, _, room := setupTestRoomTemplateHandler(t, "Living Room")

	// Send invalid JSON.
	req := httptest.NewRequest(http.MethodPut, "/api/room/"+room.ID+"/template",
		strings.NewReader("this is not json"))
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleSaveRoomTemplate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
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
