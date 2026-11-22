package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// BusinessPhotosHandler holds the database connection and provides HTTP handlers
// for the business photos aggregation endpoint. This endpoint flattens the
// photo_urls JSON arrays from individual reviews into a single paginated list
// of photos with attribution context (customer name, rating, date).
type BusinessPhotosHandler struct {
	DB *sql.DB
}

// NewBusinessPhotosHandler creates a new BusinessPhotosHandler with the given database connection.
func NewBusinessPhotosHandler(database *sql.DB) *BusinessPhotosHandler {
	return &BusinessPhotosHandler{DB: database}
}

// =============================================================================
// Response Types
// =============================================================================

// businessPhotoEntry represents a single photo extracted from a review,
// enriched with the review context (who posted it, their rating, when).
type businessPhotoEntry struct {
	URL          string `json:"url"`
	ReviewID     string `json:"reviewId"`
	CustomerName string `json:"customerName"`
	Rating       int    `json:"rating"`
	CreatedAt    string `json:"createdAt"`
}

// businessPhotosResponse is the paginated response from GET /api/business/{businessId}/photos
type businessPhotosResponse struct {
	Data       []businessPhotoEntry `json:"data"`
	Pagination paginationMeta       `json:"pagination"`
}

// =============================================================================
// Handler
// =============================================================================

// HandleGetBusinessPhotos returns a paginated list of review photos for a business.
// Photos are aggregated from all reviews that have non-empty photo_urls arrays,
// flattened into individual photo entries, and sorted newest-first by the review's
// created_at timestamp.
//
// GET /api/business/{businessId}/photos?limit=20&offset=0
//
// Query parameters:
//   - limit: max photos to return (1-100, default 20)
//   - offset: number of photos to skip (default 0)
//
// Response (200): {"data": [...], "pagination": {...}}
//
// Each photo entry includes the original review's context so the frontend
// can optionally show attribution (customer name, star rating, date).
func (h *BusinessPhotosHandler) HandleGetBusinessPhotos(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	// Parse pagination query parameters
	limit := 20
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

	// Query reviews that have photos for this business.
	// We LEFT JOIN with customers to get the customer's first name for attribution.
	// Reviews with stub customer IDs (no matching customer row) will show "Customer"
	// as the name fallback.
	// Only include reviews where photo_urls is not null/empty.
	rows, err := h.DB.Query(`
		SELECT r.id, r.customer_id, r.rating, r.photo_urls, r.created_at,
		       COALESCE(c.first_name, 'Customer') AS customer_name
		FROM reviews r
		LEFT JOIN customers c ON r.customer_id = c.id
		WHERE r.business_id = ?
		  AND r.photo_urls IS NOT NULL
		  AND r.photo_urls != '[]'
		  AND r.photo_urls != ''
		ORDER BY r.created_at DESC
	`, businessID)
	if err != nil {
		log.Printf("❌ Failed to query business photos: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch business photos")
		return
	}
	defer rows.Close()

	// Flatten all photo URLs from all reviews into a single list.
	// Each review may have multiple photos; we expand them into individual entries
	// that all share the same review context (customer name, rating, date).
	var allPhotos []businessPhotoEntry
	for rows.Next() {
		var (
			reviewID      string
			customerID    string
			rating        int
			photoURLsJSON string
			createdAt     time.Time
			customerName  string
		)

		if err := rows.Scan(&reviewID, &customerID, &rating, &photoURLsJSON, &createdAt, &customerName); err != nil {
			log.Printf("❌ Failed to scan review row for photos: %v", err)
			continue
		}

		// Parse the JSON array of photo URLs from this review
		var photoURLs []string
		if err := json.Unmarshal([]byte(photoURLsJSON), &photoURLs); err != nil {
			log.Printf("❌ Failed to parse photo_urls JSON for review %s: %v", reviewID, err)
			continue
		}

		// Abbreviate customer name for privacy: "Alex" -> "Alex", "Alexander" -> "Alexander"
		// The first_name field is typically a first name only, which is fine for display.
		// Add first initial + "." if the name is longer for a more Yelp-like display.
		displayName := abbreviateCustomerName(customerName)

		// Create a photo entry for each URL in this review
		formattedTime := createdAt.Format("2006-01-02T15:04:05Z")
		for _, url := range photoURLs {
			allPhotos = append(allPhotos, businessPhotoEntry{
				URL:          url,
				ReviewID:     reviewID,
				CustomerName: displayName,
				Rating:       rating,
				CreatedAt:    formattedTime,
			})
		}
	}

	if err := rows.Err(); err != nil {
		log.Printf("❌ Error iterating review photo rows: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch business photos")
		return
	}

	// Calculate total count before pagination
	total := len(allPhotos)

	// Apply pagination (offset + limit) to the flattened photo list
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	paginatedPhotos := allPhotos[start:end]

	// Return empty array instead of null for consistency
	if paginatedPhotos == nil {
		paginatedPhotos = []businessPhotoEntry{}
	}

	response := businessPhotosResponse{
		Data: paginatedPhotos,
		Pagination: paginationMeta{
			Offset: offset,
			Limit:  limit,
			Total:  total,
		},
	}

	log.Printf("📸 Returned %d photos for business %s (total: %d)", len(paginatedPhotos), businessID, total)
	writeJSON(w, http.StatusOK, response)
}

// abbreviateCustomerName formats a customer name for display in photo attribution.
// Shortens to first name + last initial if there are multiple parts:
// "Alex" -> "Alex", "Alex Martinez" -> "Alex M."
// Falls back to the full name if it's a single word.
func abbreviateCustomerName(name string) string {
	if name == "" {
		return "Customer"
	}
	// The Customer model only stores first_name, so typically this is a single name.
	// But handle multi-word names gracefully just in case.
	parts := splitName(name)
	if len(parts) <= 1 {
		return name
	}
	// First name + last initial with period
	return parts[0] + " " + string(parts[len(parts)-1][0]) + "."
}

// splitName splits a name string by spaces, filtering out empty parts.
func splitName(name string) []string {
	var parts []string
	current := ""
	for _, ch := range name {
		if ch == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
