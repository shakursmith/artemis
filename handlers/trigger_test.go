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

// setupTestTriggerHandler creates a TriggerHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestTriggerHandler(t *testing.T) (*TriggerHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewTriggerHandler(database), database
}

// =============================================================================
// POST /api/room/{roomId}/triggers — Create Trigger
// =============================================================================

func TestCreateTrigger_Entry_Success(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	// Seed a profile and room
	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")

	body := `{"triggerType": "entry", "actionType": "notification", "actionConfig": "{\"title\":\"Welcome!\"}", "enabled": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/room/"+room.ID+"/triggers", bytes.NewBufferString(body))
	req.SetPathValue("roomId", room.ID)
	w := httptest.NewRecorder()

	h.HandleCreateTrigger(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.RoomTrigger
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TriggerType != "entry" {
		t.Errorf("expected trigger type 'entry', got '%s'", resp.TriggerType)
	}
	if resp.ActionType != "notification" {
		t.Errorf("expected action type 'notification', got '%s'", resp.ActionType)
	}
	if !resp.Enabled {
		t.Error("expected trigger to be enabled")
	}
}

func TestCreateTrigger_Dwell_Success(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")

	body := `{"triggerType": "dwell", "dwellSeconds": 30, "actionType": "device_control", "actionConfig": "{\"device_id\":\"123\"}", "enabled": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/room/"+room.ID+"/triggers", bytes.NewBufferString(body))
	req.SetPathValue("roomId", room.ID)
	w := httptest.NewRecorder()

	h.HandleCreateTrigger(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.RoomTrigger
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TriggerType != "dwell" {
		t.Errorf("expected trigger type 'dwell', got '%s'", resp.TriggerType)
	}
	if resp.DwellSeconds == nil || *resp.DwellSeconds != 30 {
		t.Errorf("expected dwell_seconds 30, got %v", resp.DwellSeconds)
	}
	if resp.Enabled {
		t.Error("expected trigger to be disabled")
	}
}

func TestCreateTrigger_RoomNotFound(t *testing.T) {
	h, _ := setupTestTriggerHandler(t)

	body := `{"triggerType": "entry", "actionType": "notification", "actionConfig": "{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/room/nonexistent/triggers", bytes.NewBufferString(body))
	req.SetPathValue("roomId", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleCreateTrigger(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestCreateTrigger_InvalidTriggerType(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")

	body := `{"triggerType": "invalid", "actionType": "notification", "actionConfig": "{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/room/"+room.ID+"/triggers", bytes.NewBufferString(body))
	req.SetPathValue("roomId", room.ID)
	w := httptest.NewRecorder()

	h.HandleCreateTrigger(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateTrigger_DwellMissingSeconds(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")

	body := `{"triggerType": "dwell", "actionType": "notification", "actionConfig": "{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/room/"+room.ID+"/triggers", bytes.NewBufferString(body))
	req.SetPathValue("roomId", room.ID)
	w := httptest.NewRecorder()

	h.HandleCreateTrigger(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateTrigger_MissingActionType(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")

	body := `{"triggerType": "entry", "actionConfig": "{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/room/"+room.ID+"/triggers", bytes.NewBufferString(body))
	req.SetPathValue("roomId", room.ID)
	w := httptest.NewRecorder()

	h.HandleCreateTrigger(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// GET /api/room/{roomId}/triggers — List Triggers
// =============================================================================

func TestListTriggers_Success(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	// Seed a room with triggers
	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")
	db.CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Entry"}`, nil, true)
	db.CreateRoomTrigger(database, room.ID, "exit", "notification", `{"title":"Exit"}`, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/triggers", nil)
	req.SetPathValue("roomId", room.ID)
	w := httptest.NewRecorder()

	h.HandleListTriggers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []db.RoomTrigger
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 triggers, got %d", len(resp))
	}
}

func TestListTriggers_Empty(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID+"/triggers", nil)
	req.SetPathValue("roomId", room.ID)
	w := httptest.NewRecorder()

	h.HandleListTriggers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp []db.RoomTrigger
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected 0 triggers, got %d", len(resp))
	}
}

// =============================================================================
// GET /api/trigger/{id} — Get Trigger
// =============================================================================

func TestGetTrigger_Success(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")
	trigger, _ := db.CreateRoomTrigger(database, room.ID, "exit", "device_control", `{"device_id":"456"}`, nil, false)

	req := httptest.NewRequest(http.MethodGet, "/api/trigger/"+trigger.ID, nil)
	req.SetPathValue("id", trigger.ID)
	w := httptest.NewRecorder()

	h.HandleGetTrigger(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.RoomTrigger
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TriggerType != "exit" {
		t.Errorf("expected trigger type 'exit', got '%s'", resp.TriggerType)
	}
	if resp.ActionType != "device_control" {
		t.Errorf("expected action type 'device_control', got '%s'", resp.ActionType)
	}
}

func TestGetTrigger_NotFound(t *testing.T) {
	h, _ := setupTestTriggerHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/trigger/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetTrigger(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/trigger/{id} — Update Trigger
// =============================================================================

func TestUpdateTrigger_Success(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")
	trigger, _ := db.CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Old"}`, nil, true)

	body := `{"triggerType": "dwell", "dwellSeconds": 20, "actionType": "device_control", "actionConfig": "{\"device_id\":\"789\"}", "enabled": false}`
	req := httptest.NewRequest(http.MethodPut, "/api/trigger/"+trigger.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", trigger.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateTrigger(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.RoomTrigger
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TriggerType != "dwell" {
		t.Errorf("expected trigger type 'dwell', got '%s'", resp.TriggerType)
	}
	if resp.DwellSeconds == nil || *resp.DwellSeconds != 20 {
		t.Errorf("expected dwell_seconds 20, got %v", resp.DwellSeconds)
	}
	if resp.ActionType != "device_control" {
		t.Errorf("expected action type 'device_control', got '%s'", resp.ActionType)
	}
	if resp.Enabled {
		t.Error("expected trigger to be disabled")
	}
}

func TestUpdateTrigger_NotFound(t *testing.T) {
	h, _ := setupTestTriggerHandler(t)

	body := `{"triggerType": "entry", "actionType": "notification", "actionConfig": "{}"}`
	req := httptest.NewRequest(http.MethodPut, "/api/trigger/nonexistent", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateTrigger(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// DELETE /api/trigger/{id} — Delete Trigger
// =============================================================================

func TestDeleteTrigger_Success(t *testing.T) {
	h, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")
	trigger, _ := db.CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Hi"}`, nil, true)

	req := httptest.NewRequest(http.MethodDelete, "/api/trigger/"+trigger.ID, nil)
	req.SetPathValue("id", trigger.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteTrigger(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify it's deleted
	_, err := db.GetRoomTrigger(database, trigger.ID)
	if err == nil {
		t.Error("expected trigger to be deleted")
	}
}

func TestDeleteTrigger_NotFound(t *testing.T) {
	h, _ := setupTestTriggerHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/trigger/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleDeleteTrigger(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// Test cascade: deleting a room should delete all triggers
func TestDeleteRoom_CascadesTriggers(t *testing.T) {
	_, database := setupTestTriggerHandler(t)

	profile, _ := db.CreateProfile(database, "Shakur")
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "house")
	trigger, _ := db.CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Hi"}`, nil, true)

	// Delete the room
	err := db.DeleteRoom(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error deleting room, got: %v", err)
	}

	// Verify trigger is deleted
	_, err = db.GetRoomTrigger(database, trigger.ID)
	if err == nil {
		t.Error("expected trigger to be deleted via cascade")
	}
}
