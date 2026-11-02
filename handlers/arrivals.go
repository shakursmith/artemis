package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pantheon/artemis/db"
)

// ArrivalHandler holds the database connection and provides HTTP handlers
// for arrival events and SSE streaming.
type ArrivalHandler struct {
	DB *sql.DB
	// eventBroadcaster manages SSE connections and broadcasts events to kiosks
	eventBroadcaster *ArrivalEventBroadcaster
}

// NewArrivalHandler creates a new ArrivalHandler with the given database connection.
func NewArrivalHandler(database *sql.DB) *ArrivalHandler {
	return &ArrivalHandler{
		DB:               database,
		eventBroadcaster: NewArrivalEventBroadcaster(),
	}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// arrivalEventRequest is the JSON body for POST /api/arrival
type arrivalEventRequest struct {
	CustomerID  string `json:"customerId"`
	BeaconUUID  string `json:"beaconUuid"`
	BeaconMajor int    `json:"beaconMajor"`
	BeaconMinor int    `json:"beaconMinor"`
	State       string `json:"state"`       // "entered", "dwelling", or "exited"
	Proximity   string `json:"proximity"`   // "immediate", "near", or "far"
	Timestamp   string `json:"timestamp"`   // ISO 8601 timestamp
}

// arrivalEventResponse is the response for POST /api/arrival and SSE events
type arrivalEventResponse struct {
	ID          string `json:"id"`
	CustomerID  string `json:"customerId"`
	BusinessID  string `json:"businessId"`
	LocationID  string `json:"locationId"`
	BeaconUUID  string `json:"beaconUuid"`
	BeaconMajor int    `json:"beaconMajor"`
	BeaconMinor int    `json:"beaconMinor"`
	State       string `json:"state"`
	Proximity   string `json:"proximity"`
	Timestamp   string `json:"timestamp"`   // ISO 8601 from device
	CreatedAt   string `json:"createdAt"`   // ISO 8601 from backend
}

// =============================================================================
// Arrival Event Handlers
// =============================================================================

// HandleReportArrival processes arrival events from customer devices.
// POST /api/arrival
// Request body: {"customerId": "...", "beaconUuid": "...", "beaconMajor": 1, "beaconMinor": 100, "state": "entered", "proximity": "near", "timestamp": "2026-11-01T14:30:00Z"}
// Response (201): arrival event object
//
// This endpoint:
// 1. Validates the event data
// 2. Resolves business_id and location_id from the beacon configuration
// 3. Records the event in the database
// 4. Broadcasts the event to all connected kiosks at the location via SSE
func (h *ArrivalHandler) HandleReportArrival(w http.ResponseWriter, r *http.Request) {
	var req arrivalEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Arrival event: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.CustomerID == "" {
		writeError(w, http.StatusBadRequest, "customerId is required")
		return
	}
	if req.BeaconUUID == "" {
		writeError(w, http.StatusBadRequest, "beaconUuid is required")
		return
	}
	if req.State == "" {
		writeError(w, http.StatusBadRequest, "state is required")
		return
	}
	if req.Proximity == "" {
		writeError(w, http.StatusBadRequest, "proximity is required")
		return
	}
	if req.Timestamp == "" {
		writeError(w, http.StatusBadRequest, "timestamp is required")
		return
	}

	// Validate state enum
	if req.State != "entered" && req.State != "dwelling" && req.State != "exited" {
		writeError(w, http.StatusBadRequest, "Invalid state: must be entered, dwelling, or exited")
		return
	}

	// Validate proximity enum
	if req.Proximity != "immediate" && req.Proximity != "near" && req.Proximity != "far" {
		writeError(w, http.StatusBadRequest, "Invalid proximity: must be immediate, near, or far")
		return
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid timestamp format: must be ISO 8601")
		return
	}

	// Resolve beacon to room/location/business
	roomID, businessID, locationID, err := db.FindRoomByBeacon(h.DB, req.BeaconUUID, req.BeaconMajor, req.BeaconMinor)
	if err != nil {
		log.Printf("❌ Beacon not found: %v", err)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Beacon not found: no room configured with UUID %s, major %d, minor %d", req.BeaconUUID, req.BeaconMajor, req.BeaconMinor))
		return
	}

	// Log the room lookup for debugging
	log.Printf("📍 Beacon resolved: room=%s, business=%s, location=%s", roomID, businessID, locationID)

	// Create arrival event in database
	event, err := db.CreateArrivalEvent(
		h.DB,
		req.CustomerID,
		businessID,
		locationID,
		req.BeaconUUID,
		req.BeaconMajor,
		req.BeaconMinor,
		req.State,
		req.Proximity,
		timestamp,
	)
	if err != nil {
		log.Printf("❌ Failed to create arrival event: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create arrival event")
		return
	}

	// Build response
	response := arrivalEventResponse{
		ID:          event.ID,
		CustomerID:  event.CustomerID,
		BusinessID:  event.BusinessID,
		LocationID:  event.LocationID,
		BeaconUUID:  event.BeaconUUID,
		BeaconMajor: event.BeaconMajor,
		BeaconMinor: event.BeaconMinor,
		State:       event.State,
		Proximity:   event.Proximity,
		Timestamp:   event.Timestamp.Format(time.RFC3339),
		CreatedAt:   event.CreatedAt.Format(time.RFC3339),
	}

	// Broadcast event to all connected kiosks at this location
	h.eventBroadcaster.Broadcast(locationID, response)

	log.Printf("📍 Arrival event recorded: customer=%s, state=%s, location=%s", req.CustomerID, req.State, locationID)
	writeJSON(w, http.StatusCreated, response)
}

// HandleGetRecentArrivals returns recent arrival events for a location.
// GET /api/location/{locationId}/arrivals
// Query params: limit (default: 50)
// Response (200): array of arrival events
func (h *ArrivalHandler) HandleGetRecentArrivals(w http.ResponseWriter, r *http.Request) {
	locationID := r.PathValue("locationId")
	if locationID == "" {
		writeError(w, http.StatusBadRequest, "Location ID is required")
		return
	}

	// Get limit from query params (default 50)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
		if limit < 1 || limit > 500 {
			limit = 50
		}
	}

	// Get recent arrivals from database
	events, err := db.GetRecentArrivalsByLocation(h.DB, locationID, limit)
	if err != nil {
		log.Printf("❌ Failed to get recent arrivals: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get recent arrivals")
		return
	}

	// Convert to response format
	responses := make([]arrivalEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, arrivalEventResponse{
			ID:          event.ID,
			CustomerID:  event.CustomerID,
			BusinessID:  event.BusinessID,
			LocationID:  event.LocationID,
			BeaconUUID:  event.BeaconUUID,
			BeaconMajor: event.BeaconMajor,
			BeaconMinor: event.BeaconMinor,
			State:       event.State,
			Proximity:   event.Proximity,
			Timestamp:   event.Timestamp.Format(time.RFC3339),
			CreatedAt:   event.CreatedAt.Format(time.RFC3339),
		})
	}

	// Return empty array instead of null for consistency
	if responses == nil {
		responses = []arrivalEventResponse{}
	}

	writeJSON(w, http.StatusOK, responses)
}

// =============================================================================
// SSE Stream Handler
// =============================================================================

// HandleArrivalStream streams arrival events to kiosks via Server-Sent Events (SSE).
// GET /api/location/{locationId}/arrivals/stream
// Headers: Last-Event-ID (optional, for reconnection)
// Response (200): text/event-stream
//
// SSE event format:
//   id: evt_123
//   event: arrival
//   data: {"id":"evt_123","customerId":"cust_456",...}
//
// Reconnection:
//   - Browser automatically reconnects on disconnect
//   - Last-Event-ID header contains the last event ID received
//   - Backend replays missed events since that ID (TODO: implement event buffer)
//
// Keep-alive:
//   - Backend sends comment lines (": keep-alive") every 30s to prevent timeout
func (h *ArrivalHandler) HandleArrivalStream(w http.ResponseWriter, r *http.Request) {
	locationID := r.PathValue("locationId")
	if locationID == "" {
		writeError(w, http.StatusBadRequest, "Location ID is required")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow CORS for web clients

	// Get Last-Event-ID for reconnection (TODO: replay missed events)
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID != "" {
		log.Printf("🔄 SSE reconnection: location=%s, lastEventID=%s", locationID, lastEventID)
		// TODO: Replay events since lastEventID from an in-memory buffer
	}

	// Create a channel for this client
	clientChan := make(chan arrivalEventResponse, 10)
	h.eventBroadcaster.Subscribe(locationID, clientChan)
	defer h.eventBroadcaster.Unsubscribe(locationID, clientChan)

	// Set up a ticker for keep-alive comments
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Get flusher for immediate writes
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("❌ SSE stream: ResponseWriter does not support flushing")
		writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	log.Printf("📡 SSE stream opened: location=%s", locationID)

	// Stream loop
	for {
		select {
		case event := <-clientChan:
			// Marshal event to JSON
			eventJSON, err := json.Marshal(event)
			if err != nil {
				log.Printf("❌ SSE: failed to marshal event: %v", err)
				continue
			}

			// Write SSE message
			// Format:
			//   id: evt_123
			//   event: arrival
			//   data: {"id":"evt_123",...}
			//   <blank line>
			fmt.Fprintf(w, "id: %s\n", event.ID)
			fmt.Fprintf(w, "event: arrival\n")
			fmt.Fprintf(w, "data: %s\n\n", eventJSON)
			flusher.Flush()

		case <-ticker.C:
			// Send keep-alive comment
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			log.Printf("📡 SSE stream closed: location=%s", locationID)
			return
		}
	}
}

// =============================================================================
// SSE Broadcaster
// =============================================================================

// ArrivalEventBroadcaster manages SSE connections and broadcasts events to kiosks.
// It maintains a map of location IDs to subscriber channels.
type ArrivalEventBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[string][]chan arrivalEventResponse // locationID -> list of client channels
}

// NewArrivalEventBroadcaster creates a new broadcaster instance.
func NewArrivalEventBroadcaster() *ArrivalEventBroadcaster {
	return &ArrivalEventBroadcaster{
		subscribers: make(map[string][]chan arrivalEventResponse),
	}
}

// Subscribe adds a client channel to the subscribers for a location.
func (b *ArrivalEventBroadcaster) Subscribe(locationID string, ch chan arrivalEventResponse) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[locationID] = append(b.subscribers[locationID], ch)
	log.Printf("📡 New SSE subscriber: location=%s, total=%d", locationID, len(b.subscribers[locationID]))
}

// Unsubscribe removes a client channel from the subscribers for a location.
func (b *ArrivalEventBroadcaster) Unsubscribe(locationID string, ch chan arrivalEventResponse) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[locationID]
	for i, subscriber := range subs {
		if subscriber == ch {
			// Remove subscriber from slice
			b.subscribers[locationID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			log.Printf("📡 SSE subscriber removed: location=%s, remaining=%d", locationID, len(b.subscribers[locationID]))
			return
		}
	}
}

// Broadcast sends an arrival event to all subscribers for a location.
// Non-blocking: if a client's channel is full, the event is dropped for that client.
func (b *ArrivalEventBroadcaster) Broadcast(locationID string, event arrivalEventResponse) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.subscribers[locationID]
	if len(subs) == 0 {
		return
	}

	log.Printf("📡 Broadcasting event to %d subscribers: location=%s, event=%s", len(subs), locationID, event.ID)

	for _, ch := range subs {
		select {
		case ch <- event:
			// Event sent successfully
		default:
			// Channel full, drop event for this client
			log.Printf("⚠️  Dropped event for slow client: location=%s, event=%s", locationID, event.ID)
		}
	}
}
