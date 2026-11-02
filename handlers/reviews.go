package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pantheon/artemis/db"
)

// ReviewHandler holds the database connection and provides HTTP handlers
// for review CRUD operations. Use NewReviewHandler to create one.
type ReviewHandler struct {
	DB *sql.DB
}

// NewReviewHandler creates a new ReviewHandler with the given database connection.
func NewReviewHandler(database *sql.DB) *ReviewHandler {
	return &ReviewHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createReviewRequest is the JSON body for POST /api/business/{businessId}/reviews
type createReviewRequest struct {
	Rating     int      `json:"rating"`
	ReviewText *string  `json:"reviewText,omitempty"`
	PhotoURLs  []string `json:"photoUrls,omitempty"`
}

// updateReviewRequest is the JSON body for PUT /api/review/{id}
// Rating cannot be changed after creation to prevent gaming.
// Only reviewText and photoUrls can be updated.
type updateReviewRequest struct {
	ReviewText string   `json:"reviewText"`
	PhotoURLs  []string `json:"photoUrls"`
}

// addBusinessReplyRequest is the JSON body for PUT /api/review/{id}/reply
type addBusinessReplyRequest struct {
	ReplyText string `json:"replyText"`
}

// flagReviewRequest is the JSON body for PUT /api/review/{id}/flag
type flagReviewRequest struct {
	Reason *string `json:"reason,omitempty"` // Optional reason for flagging (for internal tracking)
}

// reviewListResponse is the paginated response for review lists
type reviewListResponse struct {
	Data       []reviewResponse `json:"data"`
	Pagination paginationMeta   `json:"pagination"`
}

// paginationMeta contains pagination metadata
type paginationMeta struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

// reviewResponse is the formatted review response matching the contract shape
type reviewResponse struct {
	ID            string               `json:"id"`
	CustomerID    string               `json:"customerId"`
	BusinessID    string               `json:"businessId"`
	Rating        int                  `json:"rating"`
	ReviewText    *string              `json:"reviewText,omitempty"`
	PhotoURLs     []string             `json:"photoUrls,omitempty"`
	BusinessReply *businessReplyNested `json:"businessReply,omitempty"`
	Flagged       bool                 `json:"flagged"`
	FlaggedAt     *string              `json:"flaggedAt,omitempty"`
	CreatedAt     string               `json:"createdAt"`
	UpdatedAt     string               `json:"updatedAt"`
}

// businessReplyNested is the nested business reply object in review responses
type businessReplyNested struct {
	ReplyText string `json:"replyText"`
	RepliedAt string `json:"repliedAt"`
	RepliedBy string `json:"repliedBy"`
}

// imageUploadRequest is the JSON body for POST /api/reviews/image-upload
type imageUploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int    `json:"sizeBytes"`
}

// imageUploadResponse is the response for POST /api/reviews/image-upload
type imageUploadResponse struct {
	UploadURL string `json:"uploadUrl"`
	PublicURL string `json:"publicUrl"`
	ExpiresAt string `json:"expiresAt"`
}

// =============================================================================
// Helper Functions
// =============================================================================

// generateUUID creates a random UUID v4 string.
// Matches the implementation in db/repository.go to avoid pulling in a UUID library.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	// Set version (4) and variant (RFC 4122) bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// formatReview converts a db.Review to a reviewResponse with proper timestamp formatting
func formatReview(r *db.Review) reviewResponse {
	resp := reviewResponse{
		ID:         r.ID,
		CustomerID: r.CustomerID,
		BusinessID: r.BusinessID,
		Rating:     r.Rating,
		ReviewText: r.ReviewText,
		PhotoURLs:  r.PhotoURLs,
		Flagged:    r.Flagged,
		CreatedAt:  r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	// Format business reply if present
	if r.BusinessReply != nil {
		resp.BusinessReply = &businessReplyNested{
			ReplyText: r.BusinessReply.ReplyText,
			RepliedAt: r.BusinessReply.RepliedAt.Format("2006-01-02T15:04:05Z"),
			RepliedBy: r.BusinessReply.RepliedBy,
		}
	}

	// Format flaggedAt if present
	if r.FlaggedAt != nil {
		flaggedAtStr := r.FlaggedAt.Format("2006-01-02T15:04:05Z")
		resp.FlaggedAt = &flaggedAtStr
	}

	return resp
}

// =============================================================================
// Customer Review Endpoints
// =============================================================================

// HandleCreateReview creates a new review for a business.
// POST /api/business/{businessId}/reviews
// Request body: {"rating": 5, "reviewText": "Great food!", "photoUrls": ["https://..."]}
// Response (201): full review object
func (h *ReviewHandler) HandleCreateReview(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Parse request body
	var req createReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Review create: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate rating (1-5)
	if req.Rating < 1 || req.Rating > 5 {
		writeError(w, http.StatusBadRequest, "Rating must be between 1 and 5")
		return
	}

	// For now, use a stub customer ID since customer model doesn't exist yet
	// In a real implementation, this would come from the authenticated user's session
	customerID := generateUUID()

	// Create review
	reviewID := generateUUID()
	now := time.Now()

	// Serialize photo URLs as JSON
	photoURLsJSON := "[]"
	if len(req.PhotoURLs) > 0 {
		photoBytes, _ := json.Marshal(req.PhotoURLs)
		photoURLsJSON = string(photoBytes)
	}

	_, err := h.DB.Exec(`
		INSERT INTO reviews (id, customer_id, business_id, rating, review_text, photo_urls, flagged, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, reviewID, customerID, businessID, req.Rating, req.ReviewText, photoURLsJSON, now, now)
	if err != nil {
		log.Printf("❌ Review create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create review")
		return
	}

	// Fetch the created review
	review, err := getReviewByID(h.DB, reviewID)
	if err != nil {
		log.Printf("❌ Failed to fetch created review: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch created review")
		return
	}

	log.Printf("⭐ Created review: %s for business %s (rating: %d)", reviewID, businessID, req.Rating)
	writeJSON(w, http.StatusCreated, formatReview(review))
}

// HandleGetReview returns a single review by ID.
// GET /api/review/{reviewId}
// Response (200): full review object
func (h *ReviewHandler) HandleGetReview(w http.ResponseWriter, r *http.Request) {
	reviewID := r.PathValue("reviewId")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "Review ID is required")
		return
	}

	review, err := getReviewByID(h.DB, reviewID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Review not found")
			return
		}
		log.Printf("❌ Failed to fetch review: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch review")
		return
	}

	writeJSON(w, http.StatusOK, formatReview(review))
}

// HandleUpdateReview updates a customer's own review.
// PUT /api/review/{reviewId}
// Only reviewText and photoUrls can be updated. Rating cannot be changed after creation.
// Request body: {"reviewText": "Updated text", "photoUrls": ["https://..."]}
// Response (200): updated review object
func (h *ReviewHandler) HandleUpdateReview(w http.ResponseWriter, r *http.Request) {
	reviewID := r.PathValue("reviewId")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "Review ID is required")
		return
	}

	// Parse request body
	var req updateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Review update: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Serialize photo URLs as JSON
	photoURLsJSON := "[]"
	if len(req.PhotoURLs) > 0 {
		photoBytes, _ := json.Marshal(req.PhotoURLs)
		photoURLsJSON = string(photoBytes)
	}

	// Update review (rating cannot be changed)
	result, err := h.DB.Exec(`
		UPDATE reviews
		SET review_text = ?, photo_urls = ?, updated_at = ?
		WHERE id = ?
	`, req.ReviewText, photoURLsJSON, time.Now(), reviewID)
	if err != nil {
		log.Printf("❌ Review update failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update review")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Review not found")
		return
	}

	// Fetch the updated review
	review, err := getReviewByID(h.DB, reviewID)
	if err != nil {
		log.Printf("❌ Failed to fetch updated review: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch updated review")
		return
	}

	log.Printf("✏️ Updated review: %s", reviewID)
	writeJSON(w, http.StatusOK, formatReview(review))
}

// HandleDeleteReview deletes a customer's own review.
// DELETE /api/review/{reviewId}
// Response (204): no content
func (h *ReviewHandler) HandleDeleteReview(w http.ResponseWriter, r *http.Request) {
	reviewID := r.PathValue("reviewId")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "Review ID is required")
		return
	}

	result, err := h.DB.Exec("DELETE FROM reviews WHERE id = ?", reviewID)
	if err != nil {
		log.Printf("❌ Review delete failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete review")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Review not found")
		return
	}

	log.Printf("🗑️ Deleted review: %s", reviewID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleListCustomerReviews returns all reviews created by a customer.
// GET /api/customer/{customerId}/reviews
// Response (200): array of reviews (no pagination, direct array)
func (h *ReviewHandler) HandleListCustomerReviews(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, customer_id, business_id, rating, review_text, photo_urls,
		       business_reply_text, business_reply_at, business_replied_by,
		       flagged, flagged_at, created_at, updated_at
		FROM reviews
		WHERE customer_id = ?
		ORDER BY created_at DESC
	`, customerID)
	if err != nil {
		log.Printf("❌ Failed to list customer reviews: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list reviews")
		return
	}
	defer rows.Close()

	reviews := []reviewResponse{}
	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			log.Printf("❌ Failed to scan review: %v", err)
			continue
		}
		reviews = append(reviews, formatReview(review))
	}

	// Return empty array instead of null
	if reviews == nil {
		reviews = []reviewResponse{}
	}

	writeJSON(w, http.StatusOK, reviews)
}

// HandleListBusinessReviews returns paginated reviews for a business.
// GET /api/business/{businessId}/reviews?minRating=4&limit=50&offset=0
// Response (200): {"data": [...], "pagination": {...}}
func (h *ReviewHandler) HandleListBusinessReviews(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Parse query parameters
	minRating := 0
	if minRatingStr := r.URL.Query().Get("minRating"); minRatingStr != "" {
		parsed, err := strconv.Atoi(minRatingStr)
		if err != nil || parsed < 1 || parsed > 5 {
			writeError(w, http.StatusBadRequest, "minRating must be between 1 and 5")
			return
		}
		minRating = parsed
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		parsed, err := strconv.Atoi(offsetStr)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "offset must be >= 0")
			return
		}
		offset = parsed
	}

	// Build query with optional minRating filter
	baseQuery := `
		SELECT id, customer_id, business_id, rating, review_text, photo_urls,
		       business_reply_text, business_reply_at, business_replied_by,
		       flagged, flagged_at, created_at, updated_at
		FROM reviews
		WHERE business_id = ?
	`
	countQuery := "SELECT COUNT(*) FROM reviews WHERE business_id = ?"
	args := []interface{}{businessID}

	if minRating > 0 {
		baseQuery += " AND rating >= ?"
		countQuery += " AND rating >= ?"
		args = append(args, minRating)
	}

	// Get total count
	var total int
	err := h.DB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		log.Printf("❌ Failed to count reviews: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to count reviews")
		return
	}

	// Fetch paginated reviews
	baseQuery += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := h.DB.Query(baseQuery, args...)
	if err != nil {
		log.Printf("❌ Failed to list business reviews: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list reviews")
		return
	}
	defer rows.Close()

	reviews := []reviewResponse{}
	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			log.Printf("❌ Failed to scan review: %v", err)
			continue
		}
		reviews = append(reviews, formatReview(review))
	}

	// Return empty array instead of null
	if reviews == nil {
		reviews = []reviewResponse{}
	}

	response := reviewListResponse{
		Data: reviews,
		Pagination: paginationMeta{
			Offset: offset,
			Limit:  limit,
			Total:  total,
		},
	}

	writeJSON(w, http.StatusOK, response)
}

// =============================================================================
// Business Reply Endpoints
// =============================================================================

// HandleAddBusinessReply adds or updates a business reply to a review.
// PUT /api/review/{reviewId}/reply
// Request body: {"replyText": "Thank you for your feedback!"}
// Response (200): updated review object
func (h *ReviewHandler) HandleAddBusinessReply(w http.ResponseWriter, r *http.Request) {
	reviewID := r.PathValue("reviewId")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "Review ID is required")
		return
	}

	// Parse request body
	var req addBusinessReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Add business reply: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ReplyText == "" {
		writeError(w, http.StatusBadRequest, "replyText is required")
		return
	}

	// For now, use a stub admin ID since admin model doesn't exist yet
	// In a real implementation, this would come from the authenticated admin's session
	repliedBy := generateUUID()
	now := time.Now()

	// Update review with business reply
	result, err := h.DB.Exec(`
		UPDATE reviews
		SET business_reply_text = ?, business_reply_at = ?, business_replied_by = ?, updated_at = ?
		WHERE id = ?
	`, req.ReplyText, now, repliedBy, now, reviewID)
	if err != nil {
		log.Printf("❌ Add business reply failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to add business reply")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Review not found")
		return
	}

	// Fetch the updated review
	review, err := getReviewByID(h.DB, reviewID)
	if err != nil {
		log.Printf("❌ Failed to fetch updated review: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch updated review")
		return
	}

	log.Printf("💬 Added business reply to review: %s", reviewID)
	writeJSON(w, http.StatusOK, formatReview(review))
}

// HandleDeleteBusinessReply deletes a business reply from a review.
// DELETE /api/review/{reviewId}/reply
// Response (204): no content
func (h *ReviewHandler) HandleDeleteBusinessReply(w http.ResponseWriter, r *http.Request) {
	reviewID := r.PathValue("reviewId")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "Review ID is required")
		return
	}

	// Check if review exists and has a reply
	var hasReply bool
	err := h.DB.QueryRow("SELECT business_reply_text IS NOT NULL FROM reviews WHERE id = ?", reviewID).Scan(&hasReply)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Review not found")
			return
		}
		log.Printf("❌ Failed to check review reply: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to check review reply")
		return
	}

	if !hasReply {
		writeError(w, http.StatusNotFound, "Review or reply not found")
		return
	}

	// Delete business reply
	_, err = h.DB.Exec(`
		UPDATE reviews
		SET business_reply_text = NULL, business_reply_at = NULL, business_replied_by = NULL, updated_at = ?
		WHERE id = ?
	`, time.Now(), reviewID)
	if err != nil {
		log.Printf("❌ Delete business reply failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete business reply")
		return
	}

	log.Printf("🗑️ Deleted business reply from review: %s", reviewID)
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// Moderation Endpoints
// =============================================================================

// HandleFlagReview flags a review as inappropriate.
// PUT /api/review/{reviewId}/flag
// Request body: {"reason": "Spam or irrelevant content"} (reason is optional)
// Response (200): updated review object
func (h *ReviewHandler) HandleFlagReview(w http.ResponseWriter, r *http.Request) {
	reviewID := r.PathValue("reviewId")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "Review ID is required")
		return
	}

	// Parse request body (reason is optional)
	var req flagReviewRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	now := time.Now()

	// Flag the review
	result, err := h.DB.Exec(`
		UPDATE reviews
		SET flagged = 1, flagged_at = ?, updated_at = ?
		WHERE id = ?
	`, now, now, reviewID)
	if err != nil {
		log.Printf("❌ Flag review failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to flag review")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Review not found")
		return
	}

	// Fetch the updated review
	review, err := getReviewByID(h.DB, reviewID)
	if err != nil {
		log.Printf("❌ Failed to fetch flagged review: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch flagged review")
		return
	}

	log.Printf("🚩 Flagged review: %s", reviewID)
	writeJSON(w, http.StatusOK, formatReview(review))
}

// =============================================================================
// Image Upload Endpoint
// =============================================================================

// HandleImageUpload generates a signed URL for uploading review photos.
// POST /api/reviews/image-upload
// Request body: {"filename": "photo.jpg", "contentType": "image/jpeg", "sizeBytes": 524288}
// Response (200): {"uploadUrl": "...", "publicUrl": "...", "expiresAt": "..."}
func (h *ReviewHandler) HandleImageUpload(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req imageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Image upload: invalid request body: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Filename == "" || req.ContentType == "" || req.SizeBytes == 0 {
		writeError(w, http.StatusBadRequest, "filename, contentType, and sizeBytes are required")
		return
	}

	// For now, return a stub response since we don't have cloud storage configured yet
	// In a real implementation, this would generate a signed URL for S3, GCS, etc.
	uploadID := generateUUID()
	expiresAt := time.Now().Add(1 * time.Hour)

	response := imageUploadResponse{
		UploadURL: "https://storage.example.com/bucket/reviews/" + uploadID + "?signature=stub",
		PublicURL: "https://cdn.example.com/reviews/" + uploadID,
		ExpiresAt: expiresAt.Format("2006-01-02T15:04:05Z"),
	}

	log.Printf("📸 Generated image upload URL for: %s", req.Filename)
	writeJSON(w, http.StatusOK, response)
}

// =============================================================================
// Database Helper Functions
// =============================================================================

// getReviewByID fetches a single review by ID from the database
func getReviewByID(db *sql.DB, reviewID string) (*db.Review, error) {
	row := db.QueryRow(`
		SELECT id, customer_id, business_id, rating, review_text, photo_urls,
		       business_reply_text, business_reply_at, business_replied_by,
		       flagged, flagged_at, created_at, updated_at
		FROM reviews
		WHERE id = ?
	`, reviewID)

	return scanReview(row)
}

// scanReview scans a single review row into a db.Review struct
func scanReview(scanner interface {
	Scan(dest ...interface{}) error
}) (*db.Review, error) {
	var review db.Review
	var reviewText sql.NullString
	var photoURLsJSON sql.NullString
	var businessReplyText sql.NullString
	var businessReplyAt sql.NullTime
	var businessRepliedBy sql.NullString
	var flaggedAt sql.NullTime

	err := scanner.Scan(
		&review.ID,
		&review.CustomerID,
		&review.BusinessID,
		&review.Rating,
		&reviewText,
		&photoURLsJSON,
		&businessReplyText,
		&businessReplyAt,
		&businessRepliedBy,
		&review.Flagged,
		&flaggedAt,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &notFoundError{message: "Review not found"}
		}
		return nil, err
	}

	// Handle nullable fields
	if reviewText.Valid {
		review.ReviewText = &reviewText.String
	}

	// Parse photo URLs from JSON
	if photoURLsJSON.Valid && photoURLsJSON.String != "" && photoURLsJSON.String != "[]" {
		var photoURLs []string
		if err := json.Unmarshal([]byte(photoURLsJSON.String), &photoURLs); err == nil {
			review.PhotoURLs = photoURLs
		}
	}

	// Handle business reply
	if businessReplyText.Valid && businessReplyAt.Valid && businessRepliedBy.Valid {
		review.BusinessReply = &db.BusinessReply{
			ReplyText: businessReplyText.String,
			RepliedAt: businessReplyAt.Time,
			RepliedBy: businessRepliedBy.String,
		}
	}

	if flaggedAt.Valid {
		review.FlaggedAt = &flaggedAt.Time
	}

	return &review, nil
}

// notFoundError is a custom error type for not found errors
type notFoundError struct {
	message string
}

func (e *notFoundError) Error() string {
	return e.message
}
