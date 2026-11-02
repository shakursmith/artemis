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

// setupTestFavoritesHandler creates a FavoritesHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestFavoritesHandler(t *testing.T) (*FavoritesHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewFavoritesHandler(database), database
}

// seedProductCatalog seeds a business, product menu, category, and item for testing.
// Returns the item ID.
func seedProductCatalog(t *testing.T, database *sql.DB) (businessID, itemID string) {
	t.Helper()

	// Create business
	business, err := db.CreateBusiness(database, "Coffee Shop", nil)
	if err != nil {
		t.Fatalf("Failed to create business: %v", err)
	}

	// Create product menu
	_, err = database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order)
		VALUES ('menu1', ?, 'Breakfast', 1, 0)
	`, business.ID)
	if err != nil {
		t.Fatalf("Failed to create product menu: %v", err)
	}

	// Create category
	_, err = database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order)
		VALUES ('cat1', 'menu1', 'Drinks', 1, 0)
	`)
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Create item
	_, err = database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order)
		VALUES ('item1', 'cat1', 'Espresso', 350, 'USD', 1, 0, 0)
	`)
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	return business.ID, "item1"
}

// =============================================================================
// POST /api/customer/{customerId}/favorites — Add Favorite
// =============================================================================

func TestAddFavorite_Success(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")
	_, itemID := seedProductCatalog(t, database)

	body := `{"itemId": "` + itemID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/favorites", bytes.NewBufferString(body))
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleAddFavorite(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp favoriteItemResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ItemID != itemID {
		t.Errorf("expected itemId '%s', got '%s'", itemID, resp.ItemID)
	}
	if resp.Item.Name != "Espresso" {
		t.Errorf("expected item name 'Espresso', got '%s'", resp.Item.Name)
	}
	if resp.Item.PriceMinorUnits != 350 {
		t.Errorf("expected price 350, got %d", resp.Item.PriceMinorUnits)
	}
}

func TestAddFavorite_Idempotent(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")
	_, itemID := seedProductCatalog(t, database)

	body := `{"itemId": "` + itemID + `"}`

	// Add favorite first time
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/favorites", bytes.NewBufferString(body))
	req1.SetPathValue("customerId", customer.ID)
	w1 := httptest.NewRecorder()
	h.HandleAddFavorite(w1, req1)

	var resp1 favoriteItemResponse
	json.NewDecoder(w1.Body).Decode(&resp1)
	firstTimestamp := resp1.FavoritedAt

	// Add favorite second time (should update timestamp)
	req2 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/favorites", bytes.NewBufferString(body))
	req2.SetPathValue("customerId", customer.ID)
	w2 := httptest.NewRecorder()
	h.HandleAddFavorite(w2, req2)

	var resp2 favoriteItemResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	// Should still be 201 Created
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w2.Code)
	}

	// Timestamp should be updated (or equal if executed very quickly)
	if resp2.FavoritedAt < firstTimestamp {
		t.Error("expected favorited_at to be updated or equal")
	}

	// Verify only one favorite exists in DB
	var count int
	database.QueryRow("SELECT COUNT(*) FROM favorites WHERE customer_id = ? AND item_id = ?", customer.ID, itemID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 favorite, got %d", count)
	}
}

func TestAddFavorite_ItemNotFound(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")

	body := `{"itemId": "nonexistent-item"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/favorites", bytes.NewBufferString(body))
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleAddFavorite(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddFavorite_MissingItemId(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/favorites", bytes.NewBufferString(body))
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleAddFavorite(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// DELETE /api/customer/{customerId}/favorites/{itemId} — Remove Favorite
// =============================================================================

func TestRemoveFavorite_Success(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")
	businessID, itemID := seedProductCatalog(t, database)

	// Seed favorite
	_, err := database.Exec(`
		INSERT INTO favorites (id, customer_id, item_id, business_id)
		VALUES ('fav1', ?, ?, ?)
	`, customer.ID, itemID, businessID)
	if err != nil {
		t.Fatalf("Failed to seed favorite: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/customer/"+customer.ID+"/favorites/"+itemID, nil)
	req.SetPathValue("customerId", customer.ID)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	h.HandleRemoveFavorite(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify favorite was deleted
	var count int
	database.QueryRow("SELECT COUNT(*) FROM favorites WHERE customer_id = ? AND item_id = ?", customer.ID, itemID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 favorites, got %d", count)
	}
}

func TestRemoveFavorite_Idempotent(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")
	_, itemID := seedProductCatalog(t, database)

	// Remove favorite that doesn't exist (should still return 204)
	req := httptest.NewRequest(http.MethodDelete, "/api/customer/"+customer.ID+"/favorites/"+itemID, nil)
	req.SetPathValue("customerId", customer.ID)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	h.HandleRemoveFavorite(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /api/customer/{customerId}/favorites — Get Favorites
// =============================================================================

func TestGetFavorites_Success(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")
	businessID, itemID := seedProductCatalog(t, database)

	// Create second item
	_, err := database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order)
		VALUES ('item2', 'cat1', 'Cappuccino', 450, 'USD', 1, 0, 1)
	`)
	if err != nil {
		t.Fatalf("Failed to create item2: %v", err)
	}

	// Seed favorites
	_, err = database.Exec(`
		INSERT INTO favorites (id, customer_id, item_id, business_id)
		VALUES ('fav1', ?, ?, ?), ('fav2', ?, 'item2', ?)
	`, customer.ID, itemID, businessID, customer.ID, businessID)
	if err != nil {
		t.Fatalf("Failed to seed favorites: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/favorites", nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetFavorites(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []favoriteItemResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 favorites, got %d", len(resp))
	}

	// Verify item details are included
	if resp[0].Item.Name == "" {
		t.Error("expected item details to be populated")
	}
}

func TestGetFavorites_EmptyList(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/favorites", nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetFavorites(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []favoriteItemResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected 0 favorites, got %d", len(resp))
	}
}

func TestGetFavorites_FilterByBusiness(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")
	businessID1, itemID1 := seedProductCatalog(t, database)

	// Create second business with menu and item
	business2, _ := db.CreateBusiness(database, "Bakery", nil)
	_, err := database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order)
		VALUES ('menu2', ?, 'Pastries', 1, 0)
	`, business2.ID)
	if err != nil {
		t.Fatalf("Failed to create menu2: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order)
		VALUES ('cat2', 'menu2', 'Breads', 1, 0)
	`)
	if err != nil {
		t.Fatalf("Failed to create cat2: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order)
		VALUES ('item2', 'cat2', 'Croissant', 350, 'USD', 1, 0, 0)
	`)
	if err != nil {
		t.Fatalf("Failed to create item2: %v", err)
	}

	// Add favorites from both businesses
	_, err = database.Exec(`
		INSERT INTO favorites (id, customer_id, item_id, business_id)
		VALUES ('fav1', ?, ?, ?), ('fav2', ?, 'item2', ?)
	`, customer.ID, itemID1, businessID1, customer.ID, business2.ID)
	if err != nil {
		t.Fatalf("Failed to seed favorites: %v", err)
	}

	// Request favorites filtered by business1
	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/favorites?businessId="+businessID1, nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetFavorites(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []favoriteItemResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 favorite for business1, got %d", len(resp))
	}
	if len(resp) > 0 && resp[0].BusinessID != businessID1 {
		t.Errorf("expected business_id %s, got %s", businessID1, resp[0].BusinessID)
	}
}

func TestGetFavorites_IncludesFullItemDetails(t *testing.T) {
	h, database := setupTestFavoritesHandler(t)

	customer, _ := db.CreateCustomer(database, "Alex")
	businessID, _ := seedProductCatalog(t, database)

	// Create item with full details (image URLs, dietary tags, description)
	_, err := database.Exec(`
		INSERT INTO items (id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order)
		VALUES ('item_detailed', 'cat1', 'Caesar Salad', 'Crisp romaine, parmesan, croutons', 1295, 'USD', '["https://cdn.example.com/salad.jpg"]', '["vegetarian"]', 1, 0, 2)
	`)
	if err != nil {
		t.Fatalf("Failed to create detailed item: %v", err)
	}

	// Add favorite
	_, err = database.Exec(`
		INSERT INTO favorites (id, customer_id, item_id, business_id)
		VALUES ('fav1', ?, 'item_detailed', ?)
	`, customer.ID, businessID)
	if err != nil {
		t.Fatalf("Failed to seed favorite: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/"+customer.ID+"/favorites", nil)
	req.SetPathValue("customerId", customer.ID)
	w := httptest.NewRecorder()

	h.HandleGetFavorites(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []favoriteItemResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 favorite, got %d", len(resp))
	}

	item := resp[0].Item
	if item.Name != "Caesar Salad" {
		t.Errorf("expected name 'Caesar Salad', got '%s'", item.Name)
	}
	if item.Description == nil || *item.Description != "Crisp romaine, parmesan, croutons" {
		t.Errorf("expected description to be set")
	}
	if item.PriceMinorUnits != 1295 {
		t.Errorf("expected price 1295, got %d", item.PriceMinorUnits)
	}
	if len(item.ImageURLs) != 1 || item.ImageURLs[0] != "https://cdn.example.com/salad.jpg" {
		t.Errorf("expected image URLs to be parsed")
	}
	if len(item.DietaryTags) != 1 || item.DietaryTags[0] != "vegetarian" {
		t.Errorf("expected dietary tags to be parsed")
	}
	if !item.IsAvailable {
		t.Error("expected item to be available")
	}
	if item.IsSoldOut {
		t.Error("expected item not to be sold out")
	}
}
