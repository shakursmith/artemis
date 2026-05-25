package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// =============================================================================
// POST /api/device/{id}/brightness — Set Brightness
// =============================================================================

// TestSetBrightness_ValidRange verifies that setting brightness within 0-100 succeeds
// (returns 400 because the test device has no external_id, but validates the request parsing).
func TestSetBrightness_ValidRange(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Test Lamp", "govee_light")

	body := `{"brightness": 50}`
	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/brightness",
		strings.NewReader(body))
	req.SetPathValue("id", device.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleSetBrightness(w, req)

	// Should return 400 because the test device has no external_id.
	// This validates the handler parsed the request correctly and got past validation.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 (no external_id), got %d: %s", w.Code, w.Body.String())
	}
	// Verify it's the right 400 — should mention external_id, not brightness range.
	if !strings.Contains(w.Body.String(), "external_id") {
		t.Errorf("expected error about external_id, got: %s", w.Body.String())
	}
}

// TestSetBrightness_InvalidRange verifies that brightness outside 0-100 returns 400.
func TestSetBrightness_InvalidRange(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Test Lamp", "govee_light")

	tests := []struct {
		name       string
		brightness int
	}{
		{"too_high", 101},
		{"too_low", -1},
		{"way_too_high", 200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"brightness": %d}`, tc.brightness)
			req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/brightness",
				strings.NewReader(body))
			req.SetPathValue("id", device.ID)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleSetBrightness(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 for brightness %d, got %d: %s",
					tc.brightness, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "between 0 and 100") {
				t.Errorf("expected brightness range error, got: %s", w.Body.String())
			}
		})
	}
}

// TestSetBrightness_NotFound verifies that brightness on a non-existent device returns 404.
func TestSetBrightness_NotFound(t *testing.T) {
	handler, _ := setupTestDeviceControlHandler(t, "", "")

	body := `{"brightness": 50}`
	req := httptest.NewRequest(http.MethodPost, "/api/device/nonexistent-uuid/brightness",
		strings.NewReader(body))
	req.SetPathValue("id", "nonexistent-uuid")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleSetBrightness(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSetBrightness_WrongDeviceType verifies brightness on non-govee device returns 400.
func TestSetBrightness_WrongDeviceType(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Living Room TV", "fire_tv")

	body := `{"brightness": 50}`
	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/brightness",
		strings.NewReader(body))
	req.SetPathValue("id", device.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleSetBrightness(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for non-govee device, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "only supported for govee_light") {
		t.Errorf("expected device type error, got: %s", w.Body.String())
	}
}

// =============================================================================
// POST /api/device/{id}/color — Set Color
// =============================================================================

// TestSetColor_ValidRGB verifies that setting color with valid RGB succeeds
// (returns 400 because the test device has no external_id, but validates request parsing).
func TestSetColor_ValidRGB(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Test Lamp", "govee_light")

	body := `{"r": 255, "g": 100, "b": 0}`
	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/color",
		strings.NewReader(body))
	req.SetPathValue("id", device.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleSetColor(w, req)

	// Should return 400 because the test device has no external_id.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 (no external_id), got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "external_id") {
		t.Errorf("expected error about external_id, got: %s", w.Body.String())
	}
}

// TestSetColor_InvalidValues verifies that RGB values outside 0-255 return 400.
func TestSetColor_InvalidValues(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Test Lamp", "govee_light")

	tests := []struct {
		name string
		body string
	}{
		{"r_too_high", `{"r": 256, "g": 100, "b": 0}`},
		{"g_negative", `{"r": 100, "g": -1, "b": 0}`},
		{"b_too_high", `{"r": 100, "g": 100, "b": 300}`},
		{"all_invalid", `{"r": 999, "g": -50, "b": 256}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/color",
				strings.NewReader(tc.body))
			req.SetPathValue("id", device.ID)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleSetColor(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "between 0 and 255") {
				t.Errorf("expected RGB range error, got: %s", w.Body.String())
			}
		})
	}
}

// TestSetColor_NotFound verifies that color on a non-existent device returns 404.
func TestSetColor_NotFound(t *testing.T) {
	handler, _ := setupTestDeviceControlHandler(t, "", "")

	body := `{"r": 100, "g": 100, "b": 100}`
	req := httptest.NewRequest(http.MethodPost, "/api/device/nonexistent-uuid/color",
		strings.NewReader(body))
	req.SetPathValue("id", "nonexistent-uuid")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleSetColor(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSetColor_WrongDeviceType verifies color on non-govee device returns 400.
func TestSetColor_WrongDeviceType(t *testing.T) {
	handler, device := setupTestDeviceControlHandler(t, "Office Cam", "wyze_camera")

	body := `{"r": 100, "g": 100, "b": 100}`
	req := httptest.NewRequest(http.MethodPost, "/api/device/"+device.ID+"/color",
		strings.NewReader(body))
	req.SetPathValue("id", device.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleSetColor(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for non-govee device, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "only supported for govee_light") {
		t.Errorf("expected device type error, got: %s", w.Body.String())
	}
}
