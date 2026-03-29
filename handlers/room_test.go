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

// setupTestRoomHandler creates a RoomHandler backed by an in-memory SQLite DB.
// Also creates a test profile since rooms require a parent profile.
func setupTestRoomHandler(t *testing.T) (*RoomHandler, *sql.DB, *db.Profile) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// Create a test profile that rooms will belong to
	profile, err := db.CreateProfile(database, "Test User")
	if err != nil {
		t.Fatalf("Failed to create test profile: %v", err)
	}

	return NewRoomHandler(database), database, profile
}

// =============================================================================
// POST /api/profile/{profileId}/rooms — Create Room
// =============================================================================

func TestCreateRoom_Success(t *testing.T) {
	h, _, profile := setupTestRoomHandler(t)

	body := `{"name": "Living Room", "icon": "sofa"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/rooms", bytes.NewBufferString(body))
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleCreateRoom(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Room
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Living Room" {
		t.Errorf("expected name 'Living Room', got '%s'", resp.Name)
	}
	if resp.Icon != "sofa" {
		t.Errorf("expected icon 'sofa', got '%s'", resp.Icon)
	}
	if resp.ProfileID != profile.ID {
		t.Errorf("expected profileId '%s', got '%s'", profile.ID, resp.ProfileID)
	}
}

func TestCreateRoom_DefaultIcon(t *testing.T) {
	h, _, profile := setupTestRoomHandler(t)

	// No icon provided — should default to "house"
	body := `{"name": "Mystery Room"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/rooms", bytes.NewBufferString(body))
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleCreateRoom(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	var resp db.Room
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Icon != "house" {
		t.Errorf("expected default icon 'house', got '%s'", resp.Icon)
	}
}

func TestCreateRoom_MissingName(t *testing.T) {
	h, _, profile := setupTestRoomHandler(t)

	body := `{"name": "", "icon": "sofa"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/rooms", bytes.NewBufferString(body))
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleCreateRoom(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateRoom_ProfileNotFound(t *testing.T) {
	h, _, _ := setupTestRoomHandler(t)

	body := `{"name": "Living Room", "icon": "sofa"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/nonexistent/rooms", bytes.NewBufferString(body))
	req.SetPathValue("profileId", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleCreateRoom(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /api/profile/{profileId}/rooms — List Rooms
// =============================================================================

func TestListRooms_Empty(t *testing.T) {
	h, _, profile := setupTestRoomHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/"+profile.ID+"/rooms", nil)
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleListRooms(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Should return empty array
	body := w.Body.String()
	if body == "null\n" {
		t.Error("expected empty array, got null")
	}
}

func TestListRooms_WithData(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	db.CreateRoom(database, profile.ID, "Living Room", "sofa")
	db.CreateRoom(database, profile.ID, "Office", "desktopcomputer")

	req := httptest.NewRequest(http.MethodGet, "/api/profile/"+profile.ID+"/rooms", nil)
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleListRooms(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var rooms []db.Room
	json.NewDecoder(w.Body).Decode(&rooms)
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}
}

// =============================================================================
// GET /api/room/{id} — Get Room (enriched with devices)
// =============================================================================

func TestGetRoom_Success(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "sofa")
	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)
	db.AssignDeviceToRoom(database, device.ID, room.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID, nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleGetRoom(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp roomDetailResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "Living Room" {
		t.Errorf("expected name 'Living Room', got '%s'", resp.Name)
	}
	if len(resp.Devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(resp.Devices))
	}
}

func TestGetRoom_NotFound(t *testing.T) {
	h, _, _ := setupTestRoomHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/room/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetRoom(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestGetRoom_EmptyDevices(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	room, _ := db.CreateRoom(database, profile.ID, "Empty Room", "house")

	req := httptest.NewRequest(http.MethodGet, "/api/room/"+room.ID, nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleGetRoom(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify devices is empty array, not null
	var raw map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&raw)

	if string(raw["devices"]) == "null" {
		t.Error("expected empty array for devices, got null")
	}
}

// =============================================================================
// PUT /api/room/{id} — Update Room
// =============================================================================

func TestUpdateRoom_Success(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	room, _ := db.CreateRoom(database, profile.ID, "Old Name", "house")

	body := `{"name": "Home Office", "icon": "desktopcomputer"}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/"+room.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateRoom(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Room
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Home Office" {
		t.Errorf("expected name 'Home Office', got '%s'", resp.Name)
	}
	if resp.Icon != "desktopcomputer" {
		t.Errorf("expected icon 'desktopcomputer', got '%s'", resp.Icon)
	}
}

func TestUpdateRoom_NotFound(t *testing.T) {
	h, _, _ := setupTestRoomHandler(t)

	body := `{"name": "Whatever", "icon": "house"}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/nonexistent", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateRoom(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateRoom_MissingFields(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)
	room, _ := db.CreateRoom(database, profile.ID, "Room", "house")

	// Missing icon
	body := `{"name": "New Name", "icon": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/"+room.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateRoom(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/room/{id}/beacon — Update Room Beacon
// =============================================================================

func TestUpdateRoomBeacon_Success(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "sofa")

	body := `{"uuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", "major": 1, "minor": 100}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/"+room.ID+"/beacon", bytes.NewBufferString(body))
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateRoomBeacon(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Room
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.BeaconUUID == nil || *resp.BeaconUUID != "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0" {
		t.Errorf("expected beacon UUID, got %v", resp.BeaconUUID)
	}
	if resp.BeaconMajor == nil || *resp.BeaconMajor != 1 {
		t.Errorf("expected beacon major 1, got %v", resp.BeaconMajor)
	}
	if resp.BeaconMinor == nil || *resp.BeaconMinor != 100 {
		t.Errorf("expected beacon minor 100, got %v", resp.BeaconMinor)
	}
}

func TestUpdateRoomBeacon_MissingUUID(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "sofa")

	body := `{"uuid": "", "major": 1, "minor": 100}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/"+room.ID+"/beacon", bytes.NewBufferString(body))
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateRoomBeacon(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateRoomBeacon_NotFound(t *testing.T) {
	h, _, _ := setupTestRoomHandler(t)

	body := `{"uuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", "major": 1, "minor": 100}`
	req := httptest.NewRequest(http.MethodPut, "/api/room/nonexistent/beacon", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateRoomBeacon(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// DELETE /api/room/{id} — Delete Room
// =============================================================================

func TestDeleteRoom_Success(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "sofa")

	req := httptest.NewRequest(http.MethodDelete, "/api/room/"+room.ID, nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteRoom(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
}

func TestDeleteRoom_NotFound(t *testing.T) {
	h, _, _ := setupTestRoomHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/room/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleDeleteRoom(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteRoom_UnassignsDevices(t *testing.T) {
	h, database, profile := setupTestRoomHandler(t)

	// Create room with an assigned device
	room, _ := db.CreateRoom(database, profile.ID, "Living Room", "sofa")
	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)
	db.AssignDeviceToRoom(database, device.ID, room.ID)

	// Delete the room
	req := httptest.NewRequest(http.MethodDelete, "/api/room/"+room.ID, nil)
	req.SetPathValue("id", room.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteRoom(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}

	// Device should still exist but be unassigned (ON DELETE SET NULL)
	updatedDevice, err := db.GetDevice(database, device.ID)
	if err != nil {
		t.Fatalf("device should still exist after room deletion: %v", err)
	}
	if updatedDevice.RoomID != nil {
		t.Errorf("expected device room_id to be nil after room deletion, got %v", *updatedDevice.RoomID)
	}
}
