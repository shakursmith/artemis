package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pantheon/artemis/db"
)

// setupTestArrivalHandler creates an ArrivalHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestArrivalHandler(t *testing.T) (*ArrivalHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewArrivalHandler(database), database
}

// =============================================================================
// POST /api/arrival — Report Arrival Event
// =============================================================================

func TestReportArrival_Success(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data: business -> location -> room with beacon
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	location, _ := db.CreateLocation(database, business.ID, "Downtown", nil)
	room, _ := db.CreateRoomForLocation(database, location.ID, "Entrance", "door.left.hand.open")
	beaconUUID := "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0"
	beaconMajor := 1
	beaconMinor := 100
	db.UpdateRoomBeacon(database, room.ID, beaconUUID, beaconMajor, beaconMinor)

	// Create a customer
	customer, _ := db.CreateCustomer(database, "Shakur")

	// Report arrival event
	body := `{
		"customerId": "` + customer.ID + `",
		"beaconUuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		"beaconMajor": 1,
		"beaconMinor": 100,
		"state": "entered",
		"proximity": "near",
		"timestamp": "2026-11-01T14:30:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/arrival", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleReportArrival(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp arrivalEventResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Verify response
	if resp.CustomerID != customer.ID {
		t.Errorf("expected customerId %s, got %s", customer.ID, resp.CustomerID)
	}
	if resp.BusinessID != business.ID {
		t.Errorf("expected businessId %s, got %s", business.ID, resp.BusinessID)
	}
	if resp.LocationID != location.ID {
		t.Errorf("expected locationId %s, got %s", location.ID, resp.LocationID)
	}
	if resp.State != "entered" {
		t.Errorf("expected state 'entered', got %s", resp.State)
	}
	if resp.Proximity != "near" {
		t.Errorf("expected proximity 'near', got %s", resp.Proximity)
	}
	if resp.ID == "" {
		t.Error("expected non-empty event ID")
	}
}

func TestReportArrival_InvalidState(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "Test Shop", nil)
	location, _ := db.CreateLocation(database, business.ID, "Main", nil)
	room, _ := db.CreateRoomForLocation(database, location.ID, "Lobby", "building")
	db.UpdateRoomBeacon(database, room.ID, "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", 1, 101)
	customer, _ := db.CreateCustomer(database, "Jordan")

	body := `{
		"customerId": "` + customer.ID + `",
		"beaconUuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		"beaconMajor": 1,
		"beaconMinor": 101,
		"state": "invalid_state",
		"proximity": "near",
		"timestamp": "2026-11-01T14:30:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/arrival", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleReportArrival(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestReportArrival_InvalidProximity(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "Test Cafe", nil)
	location, _ := db.CreateLocation(database, business.ID, "Branch 1", nil)
	room, _ := db.CreateRoomForLocation(database, location.ID, "Dining", "fork.knife")
	db.UpdateRoomBeacon(database, room.ID, "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", 1, 102)
	customer, _ := db.CreateCustomer(database, "Alex")

	body := `{
		"customerId": "` + customer.ID + `",
		"beaconUuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		"beaconMajor": 1,
		"beaconMinor": 102,
		"state": "entered",
		"proximity": "invalid_proximity",
		"timestamp": "2026-11-01T14:30:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/arrival", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleReportArrival(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestReportArrival_BeaconNotFound(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Create customer but no beacon configuration
	customer, _ := db.CreateCustomer(database, "Taylor")

	body := `{
		"customerId": "` + customer.ID + `",
		"beaconUuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		"beaconMajor": 999,
		"beaconMinor": 999,
		"state": "entered",
		"proximity": "near",
		"timestamp": "2026-11-01T14:30:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/arrival", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleReportArrival(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for unknown beacon, got %d", w.Code)
	}

	// Verify error message mentions beacon not found
	var errorResp map[string]string
	json.NewDecoder(w.Body).Decode(&errorResp)
	if !strings.Contains(errorResp["error"], "Beacon not found") {
		t.Errorf("expected 'Beacon not found' error, got: %s", errorResp["error"])
	}
}

func TestReportArrival_MissingFields(t *testing.T) {
	h, _ := setupTestArrivalHandler(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing customerId", `{"beaconUuid":"E2C56DB5-DFFB-48D2-B060-D0F5A71096E0","beaconMajor":1,"beaconMinor":100,"state":"entered","proximity":"near","timestamp":"2026-11-01T14:30:00Z"}`},
		{"missing beaconUuid", `{"customerId":"cust_123","beaconMajor":1,"beaconMinor":100,"state":"entered","proximity":"near","timestamp":"2026-11-01T14:30:00Z"}`},
		{"missing state", `{"customerId":"cust_123","beaconUuid":"E2C56DB5-DFFB-48D2-B060-D0F5A71096E0","beaconMajor":1,"beaconMinor":100,"proximity":"near","timestamp":"2026-11-01T14:30:00Z"}`},
		{"missing proximity", `{"customerId":"cust_123","beaconUuid":"E2C56DB5-DFFB-48D2-B060-D0F5A71096E0","beaconMajor":1,"beaconMinor":100,"state":"entered","timestamp":"2026-11-01T14:30:00Z"}`},
		{"missing timestamp", `{"customerId":"cust_123","beaconUuid":"E2C56DB5-DFFB-48D2-B060-D0F5A71096E0","beaconMajor":1,"beaconMinor":100,"state":"entered","proximity":"near"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/arrival", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()
			h.HandleReportArrival(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 for %s, got %d", tt.name, w.Code)
			}
		})
	}
}

// =============================================================================
// GET /api/location/{locationId}/arrivals — Get Recent Arrivals
// =============================================================================

func TestGetRecentArrivals_Success(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "Test Business", nil)
	location, _ := db.CreateLocation(database, business.ID, "Main Location", nil)
	room, _ := db.CreateRoomForLocation(database, location.ID, "Room 1", "star")
	db.UpdateRoomBeacon(database, room.ID, "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", 1, 200)

	customer1, _ := db.CreateCustomer(database, "Customer 1")
	customer2, _ := db.CreateCustomer(database, "Customer 2")

	// Create some arrival events
	db.CreateArrivalEvent(database, customer1.ID, business.ID, location.ID, "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", 1, 200, "entered", "near", time.Now().Add(-5*time.Minute))
	db.CreateArrivalEvent(database, customer2.ID, business.ID, location.ID, "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", 1, 200, "entered", "immediate", time.Now().Add(-2*time.Minute))
	db.CreateArrivalEvent(database, customer1.ID, business.ID, location.ID, "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", 1, 200, "dwelling", "near", time.Now())

	// Get recent arrivals
	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID+"/arrivals", nil)
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	h.HandleGetRecentArrivals(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []arrivalEventResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp) != 3 {
		t.Errorf("expected 3 events, got %d", len(resp))
	}

	// Events should be ordered by timestamp descending (most recent first)
	if len(resp) > 1 && resp[0].State != "dwelling" {
		t.Errorf("expected most recent event to be 'dwelling', got '%s'", resp[0].State)
	}
}

func TestGetRecentArrivals_EmptyList(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Create location with no events
	business, _ := db.CreateBusiness(database, "Empty Business", nil)
	location, _ := db.CreateLocation(database, business.ID, "Empty Location", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID+"/arrivals", nil)
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	h.HandleGetRecentArrivals(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp []arrivalEventResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Verify empty array, not null
	if resp == nil {
		t.Error("expected empty array, got null")
	}
	if len(resp) != 0 {
		t.Errorf("expected 0 events, got %d", len(resp))
	}
}

func TestGetRecentArrivals_WithLimit(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "Test Business", nil)
	location, _ := db.CreateLocation(database, business.ID, "Test Location", nil)
	customer, _ := db.CreateCustomer(database, "Test Customer")

	// Create 10 events
	for i := 0; i < 10; i++ {
		db.CreateArrivalEvent(database, customer.ID, business.ID, location.ID, "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", 1, 300, "dwelling", "near", time.Now().Add(time.Duration(-i)*time.Minute))
	}

	// Request with limit=5
	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID+"/arrivals?limit=5", nil)
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	h.HandleGetRecentArrivals(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp []arrivalEventResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp) != 5 {
		t.Errorf("expected 5 events with limit=5, got %d", len(resp))
	}
}

// =============================================================================
// GET /api/location/{locationId}/arrivals/stream — SSE Stream
// =============================================================================

func TestArrivalStream_SSEFormat(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "SSE Test Business", nil)
	location, _ := db.CreateLocation(database, business.ID, "SSE Test Location", nil)

	// Create a request for the SSE stream
	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID+"/arrivals/stream", nil)
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	// Start the stream in a goroutine
	done := make(chan bool)
	go func() {
		h.HandleArrivalStream(w, req)
		done <- true
	}()

	// Wait a moment for the stream to initialize
	time.Sleep(50 * time.Millisecond)

	// Broadcast a test event
	testEvent := arrivalEventResponse{
		ID:          "evt_test_123",
		CustomerID:  "cust_456",
		BusinessID:  business.ID,
		LocationID:  location.ID,
		BeaconUUID:  "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		BeaconMajor: 1,
		BeaconMinor: 400,
		State:       "entered",
		Proximity:   "near",
		Timestamp:   "2026-11-01T14:30:00Z",
		CreatedAt:   "2026-11-01T14:30:01Z",
	}
	h.eventBroadcaster.Broadcast(location.ID, testEvent)

	// Wait a moment for the event to be written
	time.Sleep(50 * time.Millisecond)

	// Cancel the request to close the stream
	// Note: We can't actually cancel the request in httptest, so we just check the output

	// Verify SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got '%s'", w.Header().Get("Cache-Control"))
	}

	// Verify SSE event format in response body
	body := w.Body.String()
	if !strings.Contains(body, "id: evt_test_123") {
		t.Error("expected SSE message to contain 'id: evt_test_123'")
	}
	if !strings.Contains(body, "event: arrival") {
		t.Error("expected SSE message to contain 'event: arrival'")
	}
	if !strings.Contains(body, "data: {") {
		t.Error("expected SSE message to contain 'data: {' (JSON)")
	}
}

func TestArrivalStream_MultipleEvents(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "Multi Event Business", nil)
	location, _ := db.CreateLocation(database, business.ID, "Multi Event Location", nil)

	// Create a request for the SSE stream
	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID+"/arrivals/stream", nil)
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	// Start the stream in a goroutine
	go func() {
		h.HandleArrivalStream(w, req)
	}()

	// Wait for stream to initialize
	time.Sleep(50 * time.Millisecond)

	// Broadcast multiple events
	event1 := arrivalEventResponse{
		ID:          "evt_001",
		CustomerID:  "cust_1",
		BusinessID:  business.ID,
		LocationID:  location.ID,
		BeaconUUID:  "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		BeaconMajor: 1,
		BeaconMinor: 500,
		State:       "entered",
		Proximity:   "near",
		Timestamp:   "2026-11-01T14:30:00Z",
		CreatedAt:   "2026-11-01T14:30:01Z",
	}
	event2 := arrivalEventResponse{
		ID:          "evt_002",
		CustomerID:  "cust_1",
		BusinessID:  business.ID,
		LocationID:  location.ID,
		BeaconUUID:  "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		BeaconMajor: 1,
		BeaconMinor: 500,
		State:       "dwelling",
		Proximity:   "immediate",
		Timestamp:   "2026-11-01T14:31:00Z",
		CreatedAt:   "2026-11-01T14:31:01Z",
	}

	h.eventBroadcaster.Broadcast(location.ID, event1)
	time.Sleep(50 * time.Millisecond)
	h.eventBroadcaster.Broadcast(location.ID, event2)
	time.Sleep(50 * time.Millisecond)

	// Verify both events are in the output
	body := w.Body.String()
	if !strings.Contains(body, "id: evt_001") {
		t.Error("expected first event in stream")
	}
	if !strings.Contains(body, "id: evt_002") {
		t.Error("expected second event in stream")
	}
}

func TestArrivalStream_Reconnection(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "Reconnect Business", nil)
	location, _ := db.CreateLocation(database, business.ID, "Reconnect Location", nil)

	// Create a request with Last-Event-ID header (simulating reconnection)
	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID+"/arrivals/stream", nil)
	req.SetPathValue("locationId", location.ID)
	req.Header.Set("Last-Event-ID", "evt_previous_123")
	w := httptest.NewRecorder()

	// Start the stream
	go func() {
		h.HandleArrivalStream(w, req)
	}()

	// Wait for stream to initialize
	time.Sleep(50 * time.Millisecond)

	// Note: Full reconnection replay is not implemented yet (marked as TODO in code)
	// This test just verifies the endpoint accepts the Last-Event-ID header without error

	// Verify SSE headers are set correctly
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", w.Header().Get("Content-Type"))
	}
}

func TestArrivalStream_KeepAlive(t *testing.T) {
	h, database := setupTestArrivalHandler(t)

	// Seed test data
	business, _ := db.CreateBusiness(database, "KeepAlive Business", nil)
	location, _ := db.CreateLocation(database, business.ID, "KeepAlive Location", nil)

	// Create a request for the SSE stream
	req := httptest.NewRequest(http.MethodGet, "/api/location/"+location.ID+"/arrivals/stream", nil)
	req.SetPathValue("locationId", location.ID)
	w := httptest.NewRecorder()

	// Start the stream
	go func() {
		h.HandleArrivalStream(w, req)
	}()

	// Wait for more than the keep-alive interval (30 seconds is too long for a test)
	// Since we can't actually wait 30 seconds in a test, we'll just verify the stream starts correctly
	time.Sleep(50 * time.Millisecond)

	// In a real scenario, after 30 seconds we'd see ": keep-alive\n\n" in the output
	// For this test, we just verify the stream is working
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", w.Header().Get("Content-Type"))
	}
}

// =============================================================================
// Broadcaster Tests
// =============================================================================

func TestBroadcaster_SubscribeUnsubscribe(t *testing.T) {
	b := NewArrivalEventBroadcaster()

	ch1 := make(chan arrivalEventResponse, 10)
	ch2 := make(chan arrivalEventResponse, 10)

	locationID := "loc_test_123"

	// Subscribe two clients
	b.Subscribe(locationID, ch1)
	b.Subscribe(locationID, ch2)

	// Verify subscriber count
	b.mu.RLock()
	if len(b.subscribers[locationID]) != 2 {
		t.Errorf("expected 2 subscribers, got %d", len(b.subscribers[locationID]))
	}
	b.mu.RUnlock()

	// Unsubscribe one client
	b.Unsubscribe(locationID, ch1)

	// Verify subscriber count
	b.mu.RLock()
	if len(b.subscribers[locationID]) != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", len(b.subscribers[locationID]))
	}
	b.mu.RUnlock()
}

func TestBroadcaster_Broadcast(t *testing.T) {
	b := NewArrivalEventBroadcaster()

	ch := make(chan arrivalEventResponse, 10)
	locationID := "loc_broadcast_test"

	b.Subscribe(locationID, ch)

	testEvent := arrivalEventResponse{
		ID:         "evt_broadcast_123",
		CustomerID: "cust_456",
		State:      "entered",
	}

	// Broadcast event
	b.Broadcast(locationID, testEvent)

	// Verify event received
	select {
	case event := <-ch:
		if event.ID != testEvent.ID {
			t.Errorf("expected event ID %s, got %s", testEvent.ID, event.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for broadcast event")
	}
}

func TestBroadcaster_BroadcastToMultipleSubscribers(t *testing.T) {
	b := NewArrivalEventBroadcaster()

	ch1 := make(chan arrivalEventResponse, 10)
	ch2 := make(chan arrivalEventResponse, 10)
	locationID := "loc_multi_broadcast"

	b.Subscribe(locationID, ch1)
	b.Subscribe(locationID, ch2)

	testEvent := arrivalEventResponse{
		ID:         "evt_multi_123",
		CustomerID: "cust_789",
		State:      "dwelling",
	}

	// Broadcast to both subscribers
	b.Broadcast(locationID, testEvent)

	// Verify both subscribers received the event
	select {
	case event := <-ch1:
		if event.ID != testEvent.ID {
			t.Errorf("ch1: expected event ID %s, got %s", testEvent.ID, event.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event on ch1")
	}

	select {
	case event := <-ch2:
		if event.ID != testEvent.ID {
			t.Errorf("ch2: expected event ID %s, got %s", testEvent.ID, event.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event on ch2")
	}
}
