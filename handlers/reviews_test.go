package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pantheon/artemis/db"
)

// setupReviewTestDB creates an in-memory database for testing reviews
func setupReviewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// createTestBusiness creates a test business for review testing
func createTestBusiness(t *testing.T, database *sql.DB) string {
	business, err := db.CreateBusiness(database, "Test Food Truck", nil)
	if err != nil {
		t.Fatalf("Failed to create test business: %v", err)
	}
	return business.ID
}

// TestCreateReview tests creating a review with photos
func TestCreateReview(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Test creating a review with photos
	reqBody := map[string]interface{}{
		"rating":     5,
		"reviewText": "Amazing tacos and great service!",
		"photoUrls":  []string{"https://cdn.example.com/reviews/photo1.jpg", "https://cdn.example.com/reviews/photo2.jpg"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(body))
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleCreateReview(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp reviewResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Rating != 5 {
		t.Errorf("Expected rating 5, got %d", resp.Rating)
	}
	if *resp.ReviewText != "Amazing tacos and great service!" {
		t.Errorf("Expected review text to match, got %s", *resp.ReviewText)
	}
	if len(resp.PhotoURLs) != 2 {
		t.Errorf("Expected 2 photo URLs, got %d", len(resp.PhotoURLs))
	}
	if resp.Flagged != false {
		t.Errorf("Expected flagged to be false, got %v", resp.Flagged)
	}
	if resp.BusinessReply != nil {
		t.Errorf("Expected no business reply, got %v", resp.BusinessReply)
	}
}

// TestCreateReviewInvalidRating tests creating a review with invalid rating
func TestCreateReviewInvalidRating(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Test rating too low
	reqBody := map[string]interface{}{
		"rating": 0,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(body))
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleCreateReview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Test rating too high
	reqBody["rating"] = 6
	body, _ = json.Marshal(reqBody)

	req = httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(body))
	req.SetPathValue("businessId", businessID)
	w = httptest.NewRecorder()

	handler.HandleCreateReview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateReview tests updating review text and photos
func TestUpdateReview(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review first
	createReqBody := map[string]interface{}{
		"rating":     4,
		"reviewText": "Good food",
		"photoUrls":  []string{"https://cdn.example.com/reviews/photo1.jpg"},
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	reviewID := createResp.ID

	// Update the review
	updateReqBody := map[string]interface{}{
		"reviewText": "Great food and atmosphere!",
		"photoUrls":  []string{"https://cdn.example.com/reviews/photo1.jpg", "https://cdn.example.com/reviews/photo2.jpg", "https://cdn.example.com/reviews/photo3.jpg"},
	}
	updateBody, _ := json.Marshal(updateReqBody)

	updateReq := httptest.NewRequest("PUT", "/api/review/"+reviewID, bytes.NewReader(updateBody))
	updateReq.SetPathValue("reviewId", reviewID)
	updateW := httptest.NewRecorder()

	handler.HandleUpdateReview(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	var updateResp reviewResponse
	json.NewDecoder(updateW.Body).Decode(&updateResp)

	// Rating should remain unchanged
	if updateResp.Rating != 4 {
		t.Errorf("Expected rating to remain 4, got %d", updateResp.Rating)
	}
	if *updateResp.ReviewText != "Great food and atmosphere!" {
		t.Errorf("Expected updated review text, got %s", *updateResp.ReviewText)
	}
	if len(updateResp.PhotoURLs) != 3 {
		t.Errorf("Expected 3 photo URLs, got %d", len(updateResp.PhotoURLs))
	}
}

// TestDeleteReview tests deleting a review
func TestDeleteReview(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review first
	createReqBody := map[string]interface{}{
		"rating": 3,
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	reviewID := createResp.ID

	// Delete the review
	deleteReq := httptest.NewRequest("DELETE", "/api/review/"+reviewID, nil)
	deleteReq.SetPathValue("reviewId", reviewID)
	deleteW := httptest.NewRecorder()

	handler.HandleDeleteReview(deleteW, deleteReq)

	if deleteW.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", deleteW.Code)
	}

	// Verify review is deleted
	getReq := httptest.NewRequest("GET", "/api/review/"+reviewID, nil)
	getReq.SetPathValue("reviewId", reviewID)
	getW := httptest.NewRecorder()

	handler.HandleGetReview(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 after delete, got %d", getW.Code)
	}
}

// TestAddBusinessReply tests adding a business reply to a review
func TestAddBusinessReply(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review first
	createReqBody := map[string]interface{}{
		"rating":     5,
		"reviewText": "Best tacos ever!",
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	reviewID := createResp.ID

	// Add a business reply
	replyReqBody := map[string]interface{}{
		"replyText": "Thank you for your kind words!",
	}
	replyBody, _ := json.Marshal(replyReqBody)

	replyReq := httptest.NewRequest("PUT", "/api/review/"+reviewID+"/reply", bytes.NewReader(replyBody))
	replyReq.SetPathValue("reviewId", reviewID)
	replyW := httptest.NewRecorder()

	handler.HandleAddBusinessReply(replyW, replyReq)

	if replyW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", replyW.Code, replyW.Body.String())
	}

	var replyResp reviewResponse
	json.NewDecoder(replyW.Body).Decode(&replyResp)

	if replyResp.BusinessReply == nil {
		t.Fatal("Expected business reply to be present")
	}
	if replyResp.BusinessReply.ReplyText != "Thank you for your kind words!" {
		t.Errorf("Expected reply text to match, got %s", replyResp.BusinessReply.ReplyText)
	}
	if replyResp.BusinessReply.RepliedBy == "" {
		t.Error("Expected repliedBy to be set")
	}
	if replyResp.BusinessReply.RepliedAt == "" {
		t.Error("Expected repliedAt to be set")
	}
}

// TestUpdateBusinessReply tests updating an existing business reply
func TestUpdateBusinessReply(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review
	createReqBody := map[string]interface{}{
		"rating": 4,
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	reviewID := createResp.ID

	// Add initial reply
	replyReqBody := map[string]interface{}{
		"replyText": "Thank you!",
	}
	replyBody, _ := json.Marshal(replyReqBody)

	replyReq := httptest.NewRequest("PUT", "/api/review/"+reviewID+"/reply", bytes.NewReader(replyBody))
	replyReq.SetPathValue("reviewId", reviewID)
	replyW := httptest.NewRecorder()

	handler.HandleAddBusinessReply(replyW, replyReq)

	// Update the reply
	updateReplyReqBody := map[string]interface{}{
		"replyText": "Thank you so much for your feedback!",
	}
	updateReplyBody, _ := json.Marshal(updateReplyReqBody)

	updateReplyReq := httptest.NewRequest("PUT", "/api/review/"+reviewID+"/reply", bytes.NewReader(updateReplyBody))
	updateReplyReq.SetPathValue("reviewId", reviewID)
	updateReplyW := httptest.NewRecorder()

	handler.HandleAddBusinessReply(updateReplyW, updateReplyReq)

	if updateReplyW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", updateReplyW.Code)
	}

	var updateReplyResp reviewResponse
	json.NewDecoder(updateReplyW.Body).Decode(&updateReplyResp)

	if updateReplyResp.BusinessReply.ReplyText != "Thank you so much for your feedback!" {
		t.Errorf("Expected updated reply text, got %s", updateReplyResp.BusinessReply.ReplyText)
	}
}

// TestDeleteBusinessReply tests deleting a business reply
func TestDeleteBusinessReply(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review with a reply
	createReqBody := map[string]interface{}{
		"rating": 5,
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	reviewID := createResp.ID

	// Add a reply
	replyReqBody := map[string]interface{}{
		"replyText": "Thank you!",
	}
	replyBody, _ := json.Marshal(replyReqBody)

	replyReq := httptest.NewRequest("PUT", "/api/review/"+reviewID+"/reply", bytes.NewReader(replyBody))
	replyReq.SetPathValue("reviewId", reviewID)
	replyW := httptest.NewRecorder()

	handler.HandleAddBusinessReply(replyW, replyReq)

	// Delete the reply
	deleteReplyReq := httptest.NewRequest("DELETE", "/api/review/"+reviewID+"/reply", nil)
	deleteReplyReq.SetPathValue("reviewId", reviewID)
	deleteReplyW := httptest.NewRecorder()

	handler.HandleDeleteBusinessReply(deleteReplyW, deleteReplyReq)

	if deleteReplyW.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", deleteReplyW.Code)
	}

	// Verify reply is deleted
	getReq := httptest.NewRequest("GET", "/api/review/"+reviewID, nil)
	getReq.SetPathValue("reviewId", reviewID)
	getW := httptest.NewRecorder()

	handler.HandleGetReview(getW, getReq)

	var getResp reviewResponse
	json.NewDecoder(getW.Body).Decode(&getResp)

	if getResp.BusinessReply != nil {
		t.Errorf("Expected business reply to be nil after delete, got %v", getResp.BusinessReply)
	}
}

// TestFlagReview tests flagging a review as inappropriate
func TestFlagReview(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review
	createReqBody := map[string]interface{}{
		"rating":     1,
		"reviewText": "Terrible service!",
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	reviewID := createResp.ID

	// Flag the review
	flagReqBody := map[string]interface{}{
		"reason": "Spam or inappropriate content",
	}
	flagBody, _ := json.Marshal(flagReqBody)

	flagReq := httptest.NewRequest("PUT", "/api/review/"+reviewID+"/flag", bytes.NewReader(flagBody))
	flagReq.SetPathValue("reviewId", reviewID)
	flagW := httptest.NewRecorder()

	handler.HandleFlagReview(flagW, flagReq)

	if flagW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", flagW.Code, flagW.Body.String())
	}

	var flagResp reviewResponse
	json.NewDecoder(flagW.Body).Decode(&flagResp)

	if !flagResp.Flagged {
		t.Error("Expected review to be flagged")
	}
	if flagResp.FlaggedAt == nil || *flagResp.FlaggedAt == "" {
		t.Error("Expected flaggedAt to be set")
	}
}

// TestListBusinessReviewsWithPagination tests listing reviews with pagination and filtering
func TestListBusinessReviewsWithPagination(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create multiple reviews with different ratings
	ratings := []int{5, 4, 3, 5, 2, 4, 5, 1, 3, 4}
	for _, rating := range ratings {
		reqBody := map[string]interface{}{
			"rating": rating,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(body))
		req.SetPathValue("businessId", businessID)
		w := httptest.NewRecorder()

		handler.HandleCreateReview(w, req)
	}

	// Test pagination with limit and offset
	req := httptest.NewRequest("GET", "/api/business/"+businessID+"/reviews?limit=5&offset=0", nil)
	req.SetPathValue("businessId", businessID)
	w := httptest.NewRecorder()

	handler.HandleListBusinessReviews(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp reviewListResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 5 {
		t.Errorf("Expected 5 reviews in first page, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 10 {
		t.Errorf("Expected total 10 reviews, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 5 {
		t.Errorf("Expected limit 5, got %d", resp.Pagination.Limit)
	}
	if resp.Pagination.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", resp.Pagination.Offset)
	}

	// Test filtering by minimum rating (4-5 stars only)
	req = httptest.NewRequest("GET", "/api/business/"+businessID+"/reviews?minRating=4", nil)
	req.SetPathValue("businessId", businessID)
	w = httptest.NewRecorder()

	handler.HandleListBusinessReviews(w, req)

	var filteredResp reviewListResponse
	json.NewDecoder(w.Body).Decode(&filteredResp)

	// Should have 6 reviews with rating >= 4 (three 5s, three 4s)
	if filteredResp.Pagination.Total != 6 {
		t.Errorf("Expected 6 reviews with rating >= 4, got %d", filteredResp.Pagination.Total)
	}

	// Verify all returned reviews have rating >= 4
	for _, review := range filteredResp.Data {
		if review.Rating < 4 {
			t.Errorf("Expected all reviews to have rating >= 4, got %d", review.Rating)
		}
	}
}

// TestListCustomerReviews tests listing all reviews by a customer
func TestListCustomerReviews(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review to get the customer ID
	createReqBody := map[string]interface{}{
		"rating": 5,
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	customerID := createResp.CustomerID

	// List reviews for this customer
	req := httptest.NewRequest("GET", "/api/customer/"+customerID+"/reviews", nil)
	req.SetPathValue("customerId", customerID)
	w := httptest.NewRecorder()

	handler.HandleListCustomerReviews(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var reviews []reviewResponse
	json.NewDecoder(w.Body).Decode(&reviews)

	// Should have at least 1 review
	if len(reviews) < 1 {
		t.Errorf("Expected at least 1 review, got %d", len(reviews))
	}

	// Verify all reviews belong to this customer
	for _, review := range reviews {
		if review.CustomerID != customerID {
			t.Errorf("Expected customer ID %s, got %s", customerID, review.CustomerID)
		}
	}
}

// TestImageUpload tests requesting a signed URL for image upload
func TestImageUpload(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	handler := NewReviewHandler(database)

	reqBody := map[string]interface{}{
		"filename":    "taco-photo.jpg",
		"contentType": "image/jpeg",
		"sizeBytes":   524288,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/reviews/image-upload", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleImageUpload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp imageUploadResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.UploadURL == "" {
		t.Error("Expected uploadUrl to be set")
	}
	if resp.PublicURL == "" {
		t.Error("Expected publicUrl to be set")
	}
	if resp.ExpiresAt == "" {
		t.Error("Expected expiresAt to be set")
	}
	if !strings.Contains(resp.UploadURL, "storage.example.com") {
		t.Errorf("Expected uploadUrl to contain storage domain, got %s", resp.UploadURL)
	}
	if !strings.Contains(resp.PublicURL, "cdn.example.com") {
		t.Errorf("Expected publicUrl to contain CDN domain, got %s", resp.PublicURL)
	}
}

// TestGetReview tests fetching a single review by ID
func TestGetReview(t *testing.T) {
	database := setupReviewTestDB(t)
	defer database.Close()

	businessID := createTestBusiness(t, database)
	handler := NewReviewHandler(database)

	// Create a review
	createReqBody := map[string]interface{}{
		"rating":     4,
		"reviewText": "Good experience",
	}
	createBody, _ := json.Marshal(createReqBody)

	createReq := httptest.NewRequest("POST", "/api/business/"+businessID+"/reviews", bytes.NewReader(createBody))
	createReq.SetPathValue("businessId", businessID)
	createW := httptest.NewRecorder()

	handler.HandleCreateReview(createW, createReq)

	var createResp reviewResponse
	json.NewDecoder(createW.Body).Decode(&createResp)
	reviewID := createResp.ID

	// Get the review
	getReq := httptest.NewRequest("GET", "/api/review/"+reviewID, nil)
	getReq.SetPathValue("reviewId", reviewID)
	getW := httptest.NewRecorder()

	handler.HandleGetReview(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", getW.Code)
	}

	var getResp reviewResponse
	json.NewDecoder(getW.Body).Decode(&getResp)

	if getResp.ID != reviewID {
		t.Errorf("Expected review ID %s, got %s", reviewID, getResp.ID)
	}
	if getResp.Rating != 4 {
		t.Errorf("Expected rating 4, got %d", getResp.Rating)
	}
	if *getResp.ReviewText != "Good experience" {
		t.Errorf("Expected review text to match, got %s", *getResp.ReviewText)
	}
}
