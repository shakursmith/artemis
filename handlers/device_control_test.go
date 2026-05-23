package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pantheon/artemis/db"
)

// setupTestDeviceControlHandler creates a DeviceControlHandler backed by an
// in-memory SQLite DB with a test profile, room, and optionally a device.
// Pass deviceType="" and deviceName="" to skip device creation.
func setupTestDeviceControlHandler(t *testing.T, deviceName, deviceType string) (*DeviceControlHandler, *db.Device) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// Create a test profile and room.
	profile, err := db.CreateProfile(database, "Test User")
	if err != nil {
		t.Fatalf("Failed to create test profile: %v", err)
	}

	_, err = db.CreateRoom(database, profile.ID, "Office", "desktopcomputer")
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}

	// No Govee clients for unit tests (we can't call the real API).
	handler := NewDeviceControlHandler(database, nil)

	if deviceName == "" {
		return handler, nil
	}

	// Create a test device.
	device, err := db.CreateDevice(database, profile.ID, deviceName, deviceType, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create test device: %v", err)
	}

	return handler, device
}

// =============================================================================
// POST /api/device/{id}/toggle — Toggle Device
// =============================================================================

// TestToggleDevice_NotFound verifies that toggling a non-existent device returns 404.
func TestToggleDevice_NotFound(t *testing.T) {
	handler, _ := setupTestDeviceControlHandler(t, "", "")

	req := httptest.NewRequest(http.MethodPost, "/api/device/nonexistent-uuid/toggle", nil)
	req.SetPathValue("id", "nonexistent-uuid")
	w := httptest.NewRecorder()

	handler.HandleToggleDevice(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestToggleDevice_MissingID verifies that missing device ID returns 400.
func TestToggleDevice_MissingID(t *testing.T) {
	handler, _ := setupTestDeviceControlHandler(t, "", "")

	req := httptest.NewRequest(http.MethodPost, "/api/device//toggle", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleToggleDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestToggleDevice_GoveeNoExternalID verifies that a Govee device without
// an external_id (MAC address) returns an error.
func TestToggleDevice_GoveeNoExternalID(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Test Light", "govee_light")

	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/toggle", nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	handler.HandleToggleDevice(w, req)

	// Should fail because the device has no external_id.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for Govee device without external_id, got %d: %s", w.Code, w.Body.String())
	}
}

// TestToggleDevice_FireTVPlaceholder verifies that toggling a Fire TV device
// returns a successful placeholder response (full implementation coming later).
func TestToggleDevice_FireTVPlaceholder(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Living Room TV", "fire_tv")

	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/toggle", nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	handler.HandleToggleDevice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp deviceToggleResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Success {
		t.Error("expected success=true for Fire TV toggle placeholder")
	}
	if resp.DeviceType != "fire_tv" {
		t.Errorf("expected deviceType 'fire_tv', got '%s'", resp.DeviceType)
	}
}

// TestToggleDevice_CameraPlaceholder verifies that toggling a camera returns
// a successful placeholder response (cameras don't have a simple toggle).
func TestToggleDevice_CameraPlaceholder(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Office Cam", "wyze_camera")

	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/toggle", nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	handler.HandleToggleDevice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp deviceToggleResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Success {
		t.Error("expected success=true for camera toggle placeholder")
	}
	if resp.DeviceType != "wyze_camera" {
		t.Errorf("expected deviceType 'wyze_camera', got '%s'", resp.DeviceType)
	}
}

// TestToggleDevice_UnsupportedType verifies that an unknown device type returns 400.
func TestToggleDevice_UnsupportedType(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Mystery Box", "unknown_type")

	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/toggle", nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	handler.HandleToggleDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for unsupported device type, got %d: %s", w.Code, w.Body.String())
	}
}

// TestToggleDevice_GoveeNoClients verifies that toggling a Govee device
// when no Govee API clients are configured returns an error.
func TestToggleDevice_GoveeNoClients(t *testing.T) {
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	defer database.Close()

	profile, _ := db.CreateProfile(database, "Test")
	externalID := "AA:BB:CC:DD:EE:FF"
	model := "H6159"
	device, _ := db.CreateDevice(database, profile.ID, "Lamp", "govee_light", &externalID, &model)

	// Handler with nil GoveeClients.
	handler := NewDeviceControlHandler(database, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/toggle", nil)
	req.SetPathValue("id", device.ID)
	w := httptest.NewRecorder()

	handler.HandleToggleDevice(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 when no Govee clients, got %d: %s", w.Code, w.Body.String())
	}
}
