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

// setupTestDeviceHandler creates a DeviceHandler backed by an in-memory SQLite DB.
// Also creates a test profile and room for device operations.
func setupTestDeviceHandler(t *testing.T) (*DeviceHandler, *sql.DB, *db.Profile, *db.Room) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	profile, err := db.CreateProfile(database, "Test User")
	if err != nil {
		t.Fatalf("Failed to create test profile: %v", err)
	}

	room, err := db.CreateRoom(database, profile.ID, "Living Room", "sofa")
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}

	return NewDeviceHandler(database), database, profile, room
}

// =============================================================================
// POST /api/profile/{profileId}/devices — Create Device
// =============================================================================

func TestCreateDevice_Success(t *testing.T) {
	h, _, profile, _ := setupTestDeviceHandler(t)

	body := `{"name": "Desk Lamp", "deviceType": "govee_light", "externalId": "govee-123", "model": "H6160"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/devices", bytes.NewBufferString(body))
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleCreateDevice(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Device
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Desk Lamp" {
		t.Errorf("expected name 'Desk Lamp', got '%s'", resp.Name)
	}
	if resp.DeviceType != "govee_light" {
		t.Errorf("expected deviceType 'govee_light', got '%s'", resp.DeviceType)
	}
	if resp.ExternalID == nil || *resp.ExternalID != "govee-123" {
		t.Errorf("expected externalId 'govee-123', got %v", resp.ExternalID)
	}
	if resp.RoomID != nil {
		t.Error("expected new device to have no room assignment")
	}
}

func TestCreateDevice_MinimalFields(t *testing.T) {
	h, _, profile, _ := setupTestDeviceHandler(t)

	// Only required fields — no externalId or model
	body := `{"name": "Generic Thing", "deviceType": "generic"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/devices", bytes.NewBufferString(body))
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleCreateDevice(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateDevice_MissingName(t *testing.T) {
	h, _, profile, _ := setupTestDeviceHandler(t)

	body := `{"name": "", "deviceType": "govee_light"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/devices", bytes.NewBufferString(body))
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleCreateDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateDevice_MissingDeviceType(t *testing.T) {
	h, _, profile, _ := setupTestDeviceHandler(t)

	body := `{"name": "Lamp", "deviceType": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/devices", bytes.NewBufferString(body))
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleCreateDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCreateDevice_ProfileNotFound(t *testing.T) {
	h, _, _, _ := setupTestDeviceHandler(t)

	body := `{"name": "Lamp", "deviceType": "govee_light"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profile/nonexistent/devices", bytes.NewBufferString(body))
	req.SetPathValue("profileId", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleCreateDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /api/profile/{profileId}/devices — List Devices
// =============================================================================

func TestListDevices_Empty(t *testing.T) {
	h, _, profile, _ := setupTestDeviceHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/"+profile.ID+"/devices", nil)
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleListDevices(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "null\n" {
		t.Error("expected empty array, got null")
	}
}

func TestListDevices_WithData(t *testing.T) {
	h, database, profile, _ := setupTestDeviceHandler(t)

	db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)
	db.CreateDevice(database, profile.ID, "TV", "fire_tv", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/profile/"+profile.ID+"/devices", nil)
	req.SetPathValue("profileId", profile.ID)
	w := httptest.NewRecorder()

	h.HandleListDevices(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var devices []db.Device
	json.NewDecoder(w.Body).Decode(&devices)
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

// =============================================================================
// GET /api/device/{id} — Get Device
// =============================================================================

func TestGetDevice_Success(t *testing.T) {
	h, database, profile, _ := setupTestDeviceHandler(t)

	device, _ := db.CreateDevice(database, profile.ID, "Desk Lamp", "govee_light", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/device/"+device.ID, nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleGetDevice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Device
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Desk Lamp" {
		t.Errorf("expected name 'Desk Lamp', got '%s'", resp.Name)
	}
}

func TestGetDevice_NotFound(t *testing.T) {
	h, _, _, _ := setupTestDeviceHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/device/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleGetDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/device/{id} — Update Device
// =============================================================================

func TestUpdateDevice_Success(t *testing.T) {
	h, database, profile, _ := setupTestDeviceHandler(t)

	device, _ := db.CreateDevice(database, profile.ID, "Old Name", "govee_light", nil, nil)

	body := `{"name": "Fancy Lamp"}`
	req := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateDevice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Device
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Fancy Lamp" {
		t.Errorf("expected name 'Fancy Lamp', got '%s'", resp.Name)
	}
}

func TestUpdateDevice_NotFound(t *testing.T) {
	h, _, _, _ := setupTestDeviceHandler(t)

	body := `{"name": "Whatever"}`
	req := httptest.NewRequest(http.MethodPut, "/api/device/nonexistent", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUpdateDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateDevice_MissingName(t *testing.T) {
	h, database, profile, _ := setupTestDeviceHandler(t)
	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID, bytes.NewBufferString(body))
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleUpdateDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/device/{id}/assign — Assign Device to Room
// =============================================================================

func TestAssignDevice_Success(t *testing.T) {
	h, database, profile, room := setupTestDeviceHandler(t)

	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	body := `{"roomId": "` + room.ID + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID+"/assign", bytes.NewBufferString(body))
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleAssignDevice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Device
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.RoomID == nil || *resp.RoomID != room.ID {
		t.Errorf("expected roomId '%s', got %v", room.ID, resp.RoomID)
	}
}

func TestAssignDevice_DeviceNotFound(t *testing.T) {
	h, _, _, room := setupTestDeviceHandler(t)

	body := `{"roomId": "` + room.ID + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/device/nonexistent/assign", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleAssignDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestAssignDevice_RoomNotFound(t *testing.T) {
	h, database, profile, _ := setupTestDeviceHandler(t)
	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	body := `{"roomId": "nonexistent-room"}`
	req := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID+"/assign", bytes.NewBufferString(body))
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleAssignDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignDevice_MissingRoomId(t *testing.T) {
	h, database, profile, _ := setupTestDeviceHandler(t)
	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	body := `{"roomId": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID+"/assign", bytes.NewBufferString(body))
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleAssignDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// PUT /api/device/{id}/unassign — Unassign Device from Room
// =============================================================================

func TestUnassignDevice_Success(t *testing.T) {
	h, database, profile, room := setupTestDeviceHandler(t)

	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)
	db.AssignDeviceToRoom(database, device.ID, room.ID)

	req := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID+"/unassign", nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleUnassignDevice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Device
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.RoomID != nil {
		t.Errorf("expected roomId to be nil after unassign, got %v", *resp.RoomID)
	}
}

func TestUnassignDevice_NotFound(t *testing.T) {
	h, _, _, _ := setupTestDeviceHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/api/device/nonexistent/unassign", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleUnassignDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// DELETE /api/device/{id} — Delete Device
// =============================================================================

func TestDeleteDevice_Success(t *testing.T) {
	h, database, profile, _ := setupTestDeviceHandler(t)

	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/device/"+device.ID, nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	h.HandleDeleteDevice(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}

	// Verify device is gone
	_, err := db.GetDevice(database, device.ID)
	if err == nil {
		t.Error("expected device to be deleted")
	}
}

func TestDeleteDevice_NotFound(t *testing.T) {
	h, _, _, _ := setupTestDeviceHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/device/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.HandleDeleteDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

// =============================================================================
// Full Flow Test — Create → Assign → Unassign → Delete
// =============================================================================

func TestDeviceFullFlow(t *testing.T) {
	h, _, profile, room := setupTestDeviceHandler(t)

	// Step 1: Create device
	createBody := `{"name": "Smart Lamp", "deviceType": "govee_light", "model": "H6160"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/profile/"+profile.ID+"/devices", bytes.NewBufferString(createBody))
	createReq.SetPathValue("profileId", profile.ID)
	createW := httptest.NewRecorder()
	h.HandleCreateDevice(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", createW.Code)
	}

	var device db.Device
	json.NewDecoder(createW.Body).Decode(&device)

	// Step 2: Assign to room
	assignBody := `{"roomId": "` + room.ID + `"}`
	assignReq := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID+"/assign", bytes.NewBufferString(assignBody))
	assignReq.SetPathValue("id", device.ID)
	assignW := httptest.NewRecorder()
	h.HandleAssignDevice(assignW, assignReq)

	if assignW.Code != http.StatusOK {
		t.Fatalf("assign: expected 200, got %d", assignW.Code)
	}

	var assigned db.Device
	json.NewDecoder(assignW.Body).Decode(&assigned)
	if assigned.RoomID == nil || *assigned.RoomID != room.ID {
		t.Fatalf("assign: expected roomId '%s', got %v", room.ID, assigned.RoomID)
	}

	// Step 3: Unassign
	unassignReq := httptest.NewRequest(http.MethodPut, "/api/device/"+device.ID+"/unassign", nil)
	unassignReq.SetPathValue("id", device.ID)
	unassignW := httptest.NewRecorder()
	h.HandleUnassignDevice(unassignW, unassignReq)

	if unassignW.Code != http.StatusOK {
		t.Fatalf("unassign: expected 200, got %d", unassignW.Code)
	}

	var unassigned db.Device
	json.NewDecoder(unassignW.Body).Decode(&unassigned)
	if unassigned.RoomID != nil {
		t.Fatalf("unassign: expected nil roomId, got %v", *unassigned.RoomID)
	}

	// Step 4: Delete
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/device/"+device.ID, nil)
	deleteReq.SetPathValue("id", device.ID)
	deleteW := httptest.NewRecorder()
	h.HandleDeleteDevice(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", deleteW.Code)
	}

	// Step 5: Verify it's gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/device/"+device.ID, nil)
	getReq.SetPathValue("id", device.ID)
	getW := httptest.NewRecorder()
	h.HandleGetDevice(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", getW.Code)
	}
}
