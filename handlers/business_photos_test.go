package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pantheon/artemis/db"
)

// setupPhotosTestDB creates an in-memory database for testing business photos
func setupPhotosTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// createTestBusinessForPhotos creates a test business and returns its ID
func createTestBusinessForPhotos(t *testing.T, database *sql.DB) string {
	t.Helper()
	business, err := db.CreateBusiness(database, "Test Photo Business", nil)
	if err != nil {
		t.Fatalf("Failed to create test business: %v", err)
	}
	return business.ID
}

// insertReviewWithPhotos inserts a review directly into the database with the given
// customer ID, rating, and photo URLs. This bypasses the handler so we can control
// the customer_id (instead of getting a random stub UUID from the handler).
func insertReviewWithPhotos(t *testing.T, database *sql.DB, businessID, customerID string, rating int, photoURLs []string) string {
	t.Helper()
	reviewID := generateUUID()
	now := time.Now()

	photoURLsJSON := "[]"
	if len(photoURLs) > 0 {
		bytes, _ := json.Marshal(photoURLs)
		photoURLsJSON = string(bytes)
	}

	_, err := database.Exec(`
		INSERT INTO reviews (id, customer_id, business_id, rating, photo_urls, flagged, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
	`, reviewID, customerID, businessID, rating, photoURLsJSON, now, now)
	if err != nil {
		t.Fatalf("Failed to insert test review: %v", err)
	}
	return reviewID
}

// TestGetBusinessPhotos_ReturnsPhotosFromReviews verifies that the endpoint
// aggregates photos from multiple reviews into a flat list with attribution.
func TestGetBusinessPhotos_ReturnsPhotosFromReviews(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	// Create a customer so the JOIN picks up a real name
	customer, err := db.CreateCustomer(database, "Alex")
	if err != nil {
		t.Fatalf("Failed to create customer: %v", err)
	}

	// Insert reviews with photos
	insertReviewWithPhotos(t, database, businessID, customer.ID, 5,
		[]string{"https://cdn.example.com/photo1.jpg", "https://cdn.example.com/photo2.jpg"})
	insertReviewWithPhotos(t, database, businessID, customer.ID, 4,
		[]string{"https://cdn.example.com/photo3.jpg"})

	// Fetch photos
	req := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos", nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp businessPhotosResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have 3 total photos (2 from first review + 1 from second)
	if resp.Pagination.Total != 3 {
		t.Errorf("Expected total 3 photos, got %d", resp.Pagination.Total)
	}
	if len(resp.Data) != 3 {
		t.Errorf("Expected 3 photos in data, got %d", len(resp.Data))
	}

	// Verify each photo has attribution
	for i, photo := range resp.Data {
		if photo.URL == "" {
			t.Errorf("Photo %d: expected URL to be set", i)
		}
		if photo.ReviewID == "" {
			t.Errorf("Photo %d: expected reviewId to be set", i)
		}
		if photo.CustomerName == "" {
			t.Errorf("Photo %d: expected customerName to be set", i)
		}
		if photo.Rating < 1 || photo.Rating > 5 {
			t.Errorf("Photo %d: expected rating 1-5, got %d", i, photo.Rating)
		}
		if photo.CreatedAt == "" {
			t.Errorf("Photo %d: expected createdAt to be set", i)
		}
	}

	// Verify customer name is "Alex" (from the customer we created)
	if resp.Data[0].CustomerName != "Alex" {
		t.Errorf("Expected customer name 'Alex', got '%s'", resp.Data[0].CustomerName)
	}
}

// TestGetBusinessPhotos_Pagination verifies limit and offset work correctly.
func TestGetBusinessPhotos_Pagination(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	customerID := generateUUID()

	// Insert a review with 5 photos to have enough for pagination
	photos := []string{
		"https://cdn.example.com/p1.jpg",
		"https://cdn.example.com/p2.jpg",
		"https://cdn.example.com/p3.jpg",
		"https://cdn.example.com/p4.jpg",
		"https://cdn.example.com/p5.jpg",
	}
	insertReviewWithPhotos(t, database, businessID, customerID, 5, photos)

	// Request first page (limit 2, offset 0)
	req := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos?limit=2&offset=0", nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp businessPhotosResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 2 {
		t.Errorf("Expected 2 photos on first page, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 5 {
		t.Errorf("Expected total 5, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", resp.Pagination.Offset)
	}
	if resp.Pagination.Limit != 2 {
		t.Errorf("Expected limit 2, got %d", resp.Pagination.Limit)
	}

	// Request second page (limit 2, offset 2)
	req2 := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos?limit=2&offset=2", nil)
	req2.SetPathValue("businessId", businessID)
	w2 := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w2, req2)

	var resp2 businessPhotosResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if len(resp2.Data) != 2 {
		t.Errorf("Expected 2 photos on second page, got %d", len(resp2.Data))
	}
	if resp2.Pagination.Total != 5 {
		t.Errorf("Expected total 5, got %d", resp2.Pagination.Total)
	}

	// Request last page (limit 2, offset 4) — should return 1 photo
	req3 := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos?limit=2&offset=4", nil)
	req3.SetPathValue("businessId", businessID)
	w3 := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w3, req3)

	var resp3 businessPhotosResponse
	json.NewDecoder(w3.Body).Decode(&resp3)

	if len(resp3.Data) != 1 {
		t.Errorf("Expected 1 photo on last page, got %d", len(resp3.Data))
	}
}

// TestGetBusinessPhotos_EmptyBusiness verifies that a business with no reviews
// returns an empty array with total 0.
func TestGetBusinessPhotos_EmptyBusiness(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	req := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos", nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp businessPhotosResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 0 {
		t.Errorf("Expected 0 photos, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 0 {
		t.Errorf("Expected total 0, got %d", resp.Pagination.Total)
	}
}

// TestGetBusinessPhotos_ExcludesReviewsWithNoPhotos verifies that reviews
// without photos (empty array or null) are excluded from the results.
func TestGetBusinessPhotos_ExcludesReviewsWithNoPhotos(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	customerID := generateUUID()

	// Insert a review WITH photos
	insertReviewWithPhotos(t, database, businessID, customerID, 5,
		[]string{"https://cdn.example.com/photo1.jpg"})

	// Insert a review WITHOUT photos (empty array)
	insertReviewWithPhotos(t, database, businessID, customerID, 3, []string{})

	// Insert a review with null photo_urls (directly via SQL)
	reviewID := generateUUID()
	now := time.Now()
	_, err := database.Exec(`
		INSERT INTO reviews (id, customer_id, business_id, rating, photo_urls, flagged, created_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, 0, ?, ?)
	`, reviewID, customerID, businessID, 2, now, now)
	if err != nil {
		t.Fatalf("Failed to insert review with null photos: %v", err)
	}

	// Fetch photos — should only get 1 photo from the first review
	req := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos", nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	var resp businessPhotosResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Pagination.Total != 1 {
		t.Errorf("Expected total 1 (only review with photos), got %d", resp.Pagination.Total)
	}
}

// TestGetBusinessPhotos_SortedNewestFirst verifies photos are returned
// with the newest review's photos first.
func TestGetBusinessPhotos_SortedNewestFirst(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	customerID := generateUUID()

	// Insert an older review
	olderReviewID := generateUUID()
	olderTime := time.Now().Add(-24 * time.Hour)
	olderPhotoJSON, _ := json.Marshal([]string{"https://cdn.example.com/old-photo.jpg"})
	_, err := database.Exec(`
		INSERT INTO reviews (id, customer_id, business_id, rating, photo_urls, flagged, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
	`, olderReviewID, customerID, businessID, 3, string(olderPhotoJSON), olderTime, olderTime)
	if err != nil {
		t.Fatalf("Failed to insert older review: %v", err)
	}

	// Insert a newer review
	newerReviewID := generateUUID()
	newerTime := time.Now()
	newerPhotoJSON, _ := json.Marshal([]string{"https://cdn.example.com/new-photo.jpg"})
	_, err = database.Exec(`
		INSERT INTO reviews (id, customer_id, business_id, rating, photo_urls, flagged, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
	`, newerReviewID, customerID, businessID, 5, string(newerPhotoJSON), newerTime, newerTime)
	if err != nil {
		t.Fatalf("Failed to insert newer review: %v", err)
	}

	// Fetch photos
	req := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos", nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	var resp businessPhotosResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 2 {
		t.Fatalf("Expected 2 photos, got %d", len(resp.Data))
	}

	// First photo should be from the newer review
	if resp.Data[0].ReviewID != newerReviewID {
		t.Errorf("Expected first photo from newer review %s, got %s", newerReviewID, resp.Data[0].ReviewID)
	}
	if resp.Data[0].URL != "https://cdn.example.com/new-photo.jpg" {
		t.Errorf("Expected newer photo URL, got %s", resp.Data[0].URL)
	}

	// Second photo should be from the older review
	if resp.Data[1].ReviewID != olderReviewID {
		t.Errorf("Expected second photo from older review %s, got %s", olderReviewID, resp.Data[1].ReviewID)
	}
}

// TestGetBusinessPhotos_CustomerNameFallback verifies that when a review's
// customer_id doesn't match any customer in the customers table, the name
// falls back to "Customer".
func TestGetBusinessPhotos_CustomerNameFallback(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	// Use a random UUID that doesn't exist in the customers table
	fakeCustomerID := generateUUID()
	insertReviewWithPhotos(t, database, businessID, fakeCustomerID, 4,
		[]string{"https://cdn.example.com/photo.jpg"})

	req := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos", nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	var resp businessPhotosResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 1 {
		t.Fatalf("Expected 1 photo, got %d", len(resp.Data))
	}

	// Should fall back to "Customer" since no matching customer row
	if resp.Data[0].CustomerName != "Customer" {
		t.Errorf("Expected fallback name 'Customer', got '%s'", resp.Data[0].CustomerName)
	}
}

// TestGetBusinessPhotos_InvalidLimit verifies that invalid limit values return 400.
func TestGetBusinessPhotos_InvalidLimit(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	tests := []struct {
		name  string
		query string
	}{
		{"limit too high", "?limit=200"},
		{"limit too low", "?limit=0"},
		{"limit non-numeric", "?limit=abc"},
		{"negative offset", "?offset=-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/business/"+businessID+"/photos"+tc.query, nil)
			req.SetPathValue("businessId", businessID)
			w := httptest.NewRecorder()

			handler.HandleGetBusinessPhotos(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

// TestGetBusinessPhotos_MissingBusinessID verifies that missing businessId returns 400.
func TestGetBusinessPhotos_MissingBusinessID(t *testing.T) {
	database := setupPhotosTestDB(t)
	handler := NewBusinessPhotosHandler(database)

	req := httptest.NewRequest("GET", "/api/business//photos", nil)
	// Don't set path value — simulates missing businessId
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestGetBusinessPhotos_NonexistentBusiness verifies that a valid but
// non-existent businessId returns an empty result (not an error).
func TestGetBusinessPhotos_NonexistentBusiness(t *testing.T) {
	database := setupPhotosTestDB(t)
	handler := NewBusinessPhotosHandler(database)

	fakeBusinessID := generateUUID()

	req := httptest.NewRequest("GET", "/api/business/"+fakeBusinessID+"/photos", nil)
	req.SetPathValue("businessId", fakeBusinessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp businessPhotosResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 0 {
		t.Errorf("Expected 0 photos for nonexistent business, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 0 {
		t.Errorf("Expected total 0, got %d", resp.Pagination.Total)
	}
}

// TestGetBusinessPhotos_OffsetBeyondTotal verifies that an offset beyond
// the total count returns empty data with the correct total.
func TestGetBusinessPhotos_OffsetBeyondTotal(t *testing.T) {
	database := setupPhotosTestDB(t)
	businessID := createTestBusinessForPhotos(t, database)
	handler := NewBusinessPhotosHandler(database)

	customerID := generateUUID()
	insertReviewWithPhotos(t, database, businessID, customerID, 5,
		[]string{"https://cdn.example.com/photo1.jpg"})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/business/%s/photos?offset=100", businessID), nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleGetBusinessPhotos(w, req)

	var resp businessPhotosResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 0 {
		t.Errorf("Expected 0 photos with offset beyond total, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 1 {
		t.Errorf("Expected total 1, got %d", resp.Pagination.Total)
	}
}

// TestAbbreviateCustomerName tests the name abbreviation helper function.
func TestAbbreviateCustomerName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alex", "Alex"},
		{"", "Customer"},
		{"Alex Martinez", "Alex M."},
		{"Customer", "Customer"},
		{"Mary Jane Watson", "Mary W."},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := abbreviateCustomerName(tc.input)
			if result != tc.expected {
				t.Errorf("abbreviateCustomerName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
