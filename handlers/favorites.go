package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pantheon/artemis/db"
)

// FavoritesHandler handles customer favorites endpoints.
// Favorites allow customers to bookmark menu items for quick reordering.
// The favorites list includes full item details (name, price, availability)
// from the product catalog, so the list is immediately useful.
type FavoritesHandler struct {
	db *sql.DB
}

// NewFavoritesHandler creates a new favorites handler with the given database connection.
func NewFavoritesHandler(database *sql.DB) *FavoritesHandler {
	return &FavoritesHandler{db: database}
}

// =============================================================================
// Request/Response Models
// =============================================================================

// addFavoriteRequest represents the request body for adding an item to favorites.
type addFavoriteRequest struct {
	ItemID string `json:"itemId"`
}

// favoriteItemResponse formats a favorite item for JSON responses with full item details.
// This includes denormalized item data from the product catalog for convenience.
type favoriteItemResponse struct {
	ID          string           `json:"id"`
	CustomerID  string           `json:"customerId"`
	ItemID      string           `json:"itemId"`
	BusinessID  string           `json:"businessId"`
	FavoritedAt string           `json:"favoritedAt"`
	Item        itemDetailResponse `json:"item"`
}

// itemDetailResponse represents denormalized item details included in favorite responses.
// This matches the Item schema from product-catalog.yaml.
type itemDetailResponse struct {
	ID               string   `json:"id"`
	CategoryID       string   `json:"categoryId"`
	Name             string   `json:"name"`
	Description      *string  `json:"description,omitempty"`
	PriceMinorUnits  int      `json:"priceMinorUnits"`
	CurrencyCode     string   `json:"currencyCode"`
	ImageURLs        []string `json:"imageUrls,omitempty"`
	DietaryTags      []string `json:"dietaryTags,omitempty"`
	IsAvailable      bool     `json:"isAvailable"`
	IsSoldOut        bool     `json:"isSoldOut"`
}

// =============================================================================
// Favorites Endpoints (Customer)
// =============================================================================

// HandleAddFavorite adds a menu item to the customer's favorites.
// POST /api/customer/{customerId}/favorites
// Idempotent: re-adding the same item updates the favoritedAt timestamp.
func (h *FavoritesHandler) HandleAddFavorite(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")

	var req addFavoriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ItemID == "" {
		writeError(w, http.StatusBadRequest, "itemId is required")
		return
	}

	// Verify item exists and get its business_id from the category -> menu -> business chain
	var businessID string
	err := h.db.QueryRow(`
		SELECT pm.business_id
		FROM items i
		JOIN categories c ON i.category_id = c.id
		JOIN product_menus pm ON c.menu_id = pm.id
		WHERE i.id = ?
	`, req.ItemID).Scan(&businessID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusBadRequest, "Item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Check if favorite already exists
	var existingID string
	err = h.db.QueryRow("SELECT id FROM favorites WHERE customer_id = ? AND item_id = ?", customerID, req.ItemID).Scan(&existingID)

	now := time.Now().UTC()

	if err == sql.ErrNoRows {
		// Create new favorite
		favoriteID := uuid.NewString()
		_, err = h.db.Exec(`
			INSERT INTO favorites (id, customer_id, item_id, business_id, favorited_at)
			VALUES (?, ?, ?, ?, ?)
		`, favoriteID, customerID, req.ItemID, businessID, now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to add favorite")
			return
		}
		existingID = favoriteID
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	} else {
		// Update existing favorite timestamp (idempotent behavior)
		_, err = h.db.Exec(`
			UPDATE favorites
			SET favorited_at = ?
			WHERE id = ?
		`, now, existingID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update favorite")
			return
		}
	}

	// Fetch the full favorite with item details
	favorite, err := h.getFavoriteWithDetails(existingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve favorite")
		return
	}

	writeJSON(w, http.StatusCreated, favorite)
}

// HandleRemoveFavorite removes a menu item from the customer's favorites.
// DELETE /api/customer/{customerId}/favorites/{itemId}
// Idempotent: returns 204 even if the item wasn't favorited.
func (h *FavoritesHandler) HandleRemoveFavorite(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	itemID := r.PathValue("itemId")

	// Delete favorite (idempotent - no error if not found)
	_, err := h.db.Exec("DELETE FROM favorites WHERE customer_id = ? AND item_id = ?", customerID, itemID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to remove favorite")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetFavorites retrieves all favorite items for a customer with full item details.
// GET /api/customer/{customerId}/favorites?businessId={businessId}
// Supports optional filtering by business.
func (h *FavoritesHandler) HandleGetFavorites(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	businessID := r.URL.Query().Get("businessId")

	var rows *sql.Rows
	var err error

	if businessID != "" {
		// Filter by business
		rows, err = h.db.Query(`
			SELECT f.id, f.customer_id, f.item_id, f.business_id, f.favorited_at,
				i.id, i.category_id, i.name, i.description, i.price_minor_units, i.currency_code, i.image_urls, i.dietary_tags, i.is_available, i.is_sold_out
			FROM favorites f
			JOIN items i ON f.item_id = i.id
			WHERE f.customer_id = ? AND f.business_id = ?
			ORDER BY f.favorited_at DESC
		`, customerID, businessID)
	} else {
		// No filter - return all favorites
		rows, err = h.db.Query(`
			SELECT f.id, f.customer_id, f.item_id, f.business_id, f.favorited_at,
				i.id, i.category_id, i.name, i.description, i.price_minor_units, i.currency_code, i.image_urls, i.dietary_tags, i.is_available, i.is_sold_out
			FROM favorites f
			JOIN items i ON f.item_id = i.id
			WHERE f.customer_id = ?
			ORDER BY f.favorited_at DESC
		`, customerID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	favorites := []favoriteItemResponse{}
	for rows.Next() {
		var f db.Favorite
		var item db.Item
		var imageURLsJSON, dietaryTagsJSON sql.NullString

		err := rows.Scan(
			&f.ID, &f.CustomerID, &f.ItemID, &f.BusinessID, &f.FavoritedAt,
			&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceMinorUnits, &item.CurrencyCode, &imageURLsJSON, &dietaryTagsJSON, &item.IsAvailable, &item.IsSoldOut,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Database error")
			return
		}

		// Parse JSON arrays for image URLs and dietary tags
		var imageURLs []string
		var dietaryTags []string
		if imageURLsJSON.Valid && imageURLsJSON.String != "" {
			json.Unmarshal([]byte(imageURLsJSON.String), &imageURLs)
		}
		if dietaryTagsJSON.Valid && dietaryTagsJSON.String != "" {
			json.Unmarshal([]byte(dietaryTagsJSON.String), &dietaryTags)
		}

		itemDetail := itemDetailResponse{
			ID:               item.ID,
			CategoryID:       item.CategoryID,
			Name:             item.Name,
			Description:      item.Description,
			PriceMinorUnits:  item.PriceMinorUnits,
			CurrencyCode:     item.CurrencyCode,
			ImageURLs:        imageURLs,
			DietaryTags:      dietaryTags,
			IsAvailable:      item.IsAvailable,
			IsSoldOut:        item.IsSoldOut,
		}

		favorites = append(favorites, favoriteItemResponse{
			ID:          f.ID,
			CustomerID:  f.CustomerID,
			ItemID:      f.ItemID,
			BusinessID:  f.BusinessID,
			FavoritedAt: f.FavoritedAt.Format("2006-01-02T15:04:05Z"),
			Item:        itemDetail,
		})
	}

	writeJSON(w, http.StatusOK, favorites)
}

// =============================================================================
// Helper Functions
// =============================================================================

// getFavoriteWithDetails retrieves a favorite by ID with full item details.
func (h *FavoritesHandler) getFavoriteWithDetails(favoriteID string) (*favoriteItemResponse, error) {
	var f db.Favorite
	var item db.Item
	var imageURLsJSON, dietaryTagsJSON sql.NullString

	err := h.db.QueryRow(`
		SELECT f.id, f.customer_id, f.item_id, f.business_id, f.favorited_at,
			i.id, i.category_id, i.name, i.description, i.price_minor_units, i.currency_code, i.image_urls, i.dietary_tags, i.is_available, i.is_sold_out
		FROM favorites f
		JOIN items i ON f.item_id = i.id
		WHERE f.id = ?
	`, favoriteID).Scan(
		&f.ID, &f.CustomerID, &f.ItemID, &f.BusinessID, &f.FavoritedAt,
		&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceMinorUnits, &item.CurrencyCode, &imageURLsJSON, &dietaryTagsJSON, &item.IsAvailable, &item.IsSoldOut,
	)
	if err != nil {
		return nil, err
	}

	// Parse JSON arrays for image URLs and dietary tags
	var imageURLs []string
	var dietaryTags []string
	if imageURLsJSON.Valid && imageURLsJSON.String != "" {
		json.Unmarshal([]byte(imageURLsJSON.String), &imageURLs)
	}
	if dietaryTagsJSON.Valid && dietaryTagsJSON.String != "" {
		json.Unmarshal([]byte(dietaryTagsJSON.String), &dietaryTags)
	}

	itemDetail := itemDetailResponse{
		ID:               item.ID,
		CategoryID:       item.CategoryID,
		Name:             item.Name,
		Description:      item.Description,
		PriceMinorUnits:  item.PriceMinorUnits,
		CurrencyCode:     item.CurrencyCode,
		ImageURLs:        imageURLs,
		DietaryTags:      dietaryTags,
		IsAvailable:      item.IsAvailable,
		IsSoldOut:        item.IsSoldOut,
	}

	return &favoriteItemResponse{
		ID:          f.ID,
		CustomerID:  f.CustomerID,
		ItemID:      f.ItemID,
		BusinessID:  f.BusinessID,
		FavoritedAt: f.FavoritedAt.Format("2006-01-02T15:04:05Z"),
		Item:        itemDetail,
	}, nil
}
