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

// setupTestProductCatalogHandler creates a ProductCatalogHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestProductCatalogHandler(t *testing.T) (*ProductCatalogHandler, *sql.DB) {
	t.Helper()
	// Use file:memdb?mode=memory&cache=shared for shared in-memory database
	// This ensures all connections see the same data
	database, err := db.InitDB("file:memdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	handler := NewProductCatalogHandler(database)
	return handler, database
}

// =============================================================================
// Product Menu Tests
// =============================================================================

func TestCreateProductMenu_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create a business first
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)

	body := `{"name": "Breakfast Menu", "description": "Served 7am-11am", "isActive": true, "displayOrder": 0}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/product-menus/create", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleCreateProductMenu(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.ProductMenu
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Breakfast Menu" {
		t.Errorf("expected name 'Breakfast Menu', got '%s'", resp.Name)
	}
	if !resp.IsActive {
		t.Error("expected isActive to be true")
	}
}

func TestGetFullMenu_WithNestedData(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business
	business, err := db.CreateBusiness(database, "Test Restaurant", nil)
	if err != nil {
		t.Fatalf("failed to create business: %v", err)
	}

	// Create menu
	menuID := "menu-1"
	_, err = database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)
	if err != nil {
		t.Fatalf("failed to create menu: %v", err)
	}

	// Create category
	categoryID := "cat-1"
	_, err = database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Entrees', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, categoryID, menuID)
	if err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	// Create item
	itemID := "item-1"
	_, err = database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, 'Caesar Salad', 1295, 'USD', 1, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, itemID, categoryID)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Create modifier group
	groupID := "group-1"
	_, err = database.Exec(`
		INSERT INTO modifier_groups (id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Size', 1, 1, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, groupID, itemID)
	if err != nil {
		t.Fatalf("failed to create modifier group: %v", err)
	}

	// Create modifiers
	_, err = database.Exec(`
		INSERT INTO modifiers (id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at)
		VALUES ('mod-1', ?, 'Small', 0, 1, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, groupID)
	if err != nil {
		t.Fatalf("failed to create modifier 1: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO modifiers (id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at)
		VALUES ('mod-2', ?, 'Large', 200, 1, 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, groupID)
	if err != nil {
		t.Fatalf("failed to create modifier 2: %v", err)
	}

	// Test GET full menu
	req := httptest.NewRequest(http.MethodGet, "/api/product-menu/"+menuID+"/full", nil)
	req.SetPathValue("menuId", menuID)
	w := httptest.NewRecorder()

	h.HandleGetFullMenu(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fullMenuResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "Lunch Menu" {
		t.Errorf("expected menu name 'Lunch Menu', got '%s'", resp.Name)
	}
	if len(resp.Categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(resp.Categories))
	}
	if resp.Categories[0].Name != "Entrees" {
		t.Errorf("expected category name 'Entrees', got '%s'", resp.Categories[0].Name)
	}
	if len(resp.Categories[0].Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Categories[0].Items))
	}
	if resp.Categories[0].Items[0].Name != "Caesar Salad" {
		t.Errorf("expected item name 'Caesar Salad', got '%s'", resp.Categories[0].Items[0].Name)
	}
	if len(resp.Categories[0].Items[0].ModifierGroups) != 1 {
		t.Fatalf("expected 1 modifier group, got %d", len(resp.Categories[0].Items[0].ModifierGroups))
	}
	if len(resp.Categories[0].Items[0].ModifierGroups[0].Modifiers) != 2 {
		t.Fatalf("expected 2 modifiers, got %d", len(resp.Categories[0].Items[0].ModifierGroups[0].Modifiers))
	}
}

func TestReorderProductMenus_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)

	// Create menus
	menu1ID := "menu-1"
	menu2ID := "menu-2"
	menu3ID := "menu-3"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Breakfast', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menu1ID, business.ID)
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menu2ID, business.ID)
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Dinner', 1, 2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menu3ID, business.ID)

	// Reorder: move Dinner to first position
	body := `{"orderedIds": ["` + menu3ID + `", "` + menu1ID + `", "` + menu2ID + `"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/business/"+business.ID+"/product-menus/reorder", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleReorderProductMenus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []db.ProductMenu
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp) != 3 {
		t.Fatalf("expected 3 menus, got %d", len(resp))
	}
	if resp[0].Name != "Dinner" || resp[0].DisplayOrder != 0 {
		t.Errorf("expected first menu to be Dinner with order 0, got %s with order %d", resp[0].Name, resp[0].DisplayOrder)
	}
	if resp[1].Name != "Breakfast" || resp[1].DisplayOrder != 1 {
		t.Errorf("expected second menu to be Breakfast with order 1, got %s with order %d", resp[1].Name, resp[1].DisplayOrder)
	}
}

// =============================================================================
// Category Tests
// =============================================================================

func TestCreateCategory_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business and menu
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	menuID := "menu-1"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)

	body := `{"name": "Appetizers", "description": "Start your meal", "isAvailable": true, "displayOrder": 0}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-menu/"+menuID+"/categories", bytes.NewBufferString(body))
	req.SetPathValue("menuId", menuID)
	w := httptest.NewRecorder()

	h.HandleCreateCategory(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Category
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Appetizers" {
		t.Errorf("expected name 'Appetizers', got '%s'", resp.Name)
	}
}

func TestUpdateCategory_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business, menu, and category
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	menuID := "menu-1"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)

	categoryID := "cat-1"
	database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Appetizers', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, categoryID, menuID)

	body := `{"name": "Starters", "description": "Begin your meal", "isAvailable": true, "displayOrder": 0}`
	req := httptest.NewRequest(http.MethodPut, "/api/category/"+categoryID, bytes.NewBufferString(body))
	req.SetPathValue("categoryId", categoryID)
	w := httptest.NewRecorder()

	h.HandleUpdateCategory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Category
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Starters" {
		t.Errorf("expected name 'Starters', got '%s'", resp.Name)
	}
}

// =============================================================================
// Item Tests
// =============================================================================

func TestCreateItem_WithImageURLsAndDietaryTags(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business, menu, and category
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	menuID := "menu-1"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)

	categoryID := "cat-1"
	database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Entrees', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, categoryID, menuID)

	body := `{
		"name": "Grilled Salmon",
		"description": "Fresh Atlantic salmon",
		"priceMinorUnits": 2495,
		"currencyCode": "USD",
		"imageUrls": ["https://cdn.example.com/salmon.jpg"],
		"dietaryTags": ["gluten-free", "dairy-free"],
		"isAvailable": true,
		"isSoldOut": false,
		"displayOrder": 0
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/category/"+categoryID+"/items", bytes.NewBufferString(body))
	req.SetPathValue("categoryId", categoryID)
	w := httptest.NewRecorder()

	h.HandleCreateItem(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Item
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Grilled Salmon" {
		t.Errorf("expected name 'Grilled Salmon', got '%s'", resp.Name)
	}
	if resp.PriceMinorUnits != 2495 {
		t.Errorf("expected price 2495, got %d", resp.PriceMinorUnits)
	}
}

func TestToggleSoldOut_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business, menu, category, and item
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	menuID := "menu-1"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)

	categoryID := "cat-1"
	database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Entrees', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, categoryID, menuID)

	itemID := "item-1"
	database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, 'Salmon', 2495, 'USD', 1, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, itemID, categoryID)

	// Toggle sold out to true
	body := `{"isSoldOut": true}`
	req := httptest.NewRequest(http.MethodPut, "/api/item/"+itemID+"/sold-out", bytes.NewBufferString(body))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	h.HandleToggleSoldOut(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Item
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.IsSoldOut {
		t.Error("expected isSoldOut to be true")
	}
}

func TestReorderItems_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business, menu, and category
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	menuID := "menu-1"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)

	categoryID := "cat-1"
	database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Entrees', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, categoryID, menuID)

	// Create items
	item1ID := "item-1"
	item2ID := "item-2"
	item3ID := "item-3"
	database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, 'Salad', 1295, 'USD', 1, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, item1ID, categoryID)
	database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, 'Salmon', 2495, 'USD', 1, 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, item2ID, categoryID)
	database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, 'Steak', 3495, 'USD', 1, 0, 2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, item3ID, categoryID)

	// Reorder: move Steak to first position
	body := `{"orderedIds": ["` + item3ID + `", "` + item1ID + `", "` + item2ID + `"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/category/"+categoryID+"/items/reorder", bytes.NewBufferString(body))
	req.SetPathValue("categoryId", categoryID)
	w := httptest.NewRecorder()

	h.HandleReorderItems(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []db.Item
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp))
	}
	if resp[0].Name != "Steak" || resp[0].DisplayOrder != 0 {
		t.Errorf("expected first item to be Steak with order 0, got %s with order %d", resp[0].Name, resp[0].DisplayOrder)
	}
}

// =============================================================================
// Modifier Group Tests
// =============================================================================

func TestCreateModifierGroup_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business, menu, category, and item
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	menuID := "menu-1"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)

	categoryID := "cat-1"
	database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Entrees', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, categoryID, menuID)

	itemID := "item-1"
	database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, 'Burger', 1295, 'USD', 1, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, itemID, categoryID)

	body := `{"name": "Size", "minSelections": 1, "maxSelections": 1, "isAvailable": true, "displayOrder": 0}`
	req := httptest.NewRequest(http.MethodPost, "/api/item/"+itemID+"/modifier-groups", bytes.NewBufferString(body))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	h.HandleCreateModifierGroup(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.ModifierGroup
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Size" {
		t.Errorf("expected name 'Size', got '%s'", resp.Name)
	}
	if resp.MinSelections != 1 {
		t.Errorf("expected minSelections 1, got %d", resp.MinSelections)
	}
}

// =============================================================================
// Modifier Tests
// =============================================================================

func TestCreateModifier_Success(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business, menu, category, item, and modifier group
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)
	menuID := "menu-1"
	database.Exec(`
		INSERT INTO product_menus (id, business_id, name, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, 'Lunch Menu', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, menuID, business.ID)

	categoryID := "cat-1"
	database.Exec(`
		INSERT INTO categories (id, menu_id, name, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Entrees', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, categoryID, menuID)

	itemID := "item-1"
	database.Exec(`
		INSERT INTO items (id, category_id, name, price_minor_units, currency_code, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, 'Burger', 1295, 'USD', 1, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, itemID, categoryID)

	groupID := "group-1"
	database.Exec(`
		INSERT INTO modifier_groups (id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, 'Size', 1, 1, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, groupID, itemID)

	body := `{"name": "Large", "priceAdjustmentMinorUnits": 150, "isAvailable": true, "isDefault": false, "displayOrder": 1}`
	req := httptest.NewRequest(http.MethodPost, "/api/modifier-group/"+groupID+"/modifiers", bytes.NewBufferString(body))
	req.SetPathValue("modifierGroupId", groupID)
	w := httptest.NewRecorder()

	h.HandleCreateModifier(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Modifier
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Large" {
		t.Errorf("expected name 'Large', got '%s'", resp.Name)
	}
	if resp.PriceAdjustmentMinorUnits != 150 {
		t.Errorf("expected price adjustment 150, got %d", resp.PriceAdjustmentMinorUnits)
	}
}

// =============================================================================
// Image Upload Tests
// =============================================================================

func TestImageUpload_Success(t *testing.T) {
	h, _ := setupTestProductCatalogHandler(t)

	body := `{"filename": "salad.jpg", "contentType": "image/jpeg", "sizeBytes": 524288}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-catalog/image-upload", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.HandleImageUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp imageUploadResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.UploadURL == "" {
		t.Error("expected non-empty uploadUrl")
	}
	if resp.PublicURL == "" {
		t.Error("expected non-empty publicUrl")
	}
	if resp.ExpiresAt == "" {
		t.Error("expected non-empty expiresAt")
	}
}

// =============================================================================
// Bulk Import Tests
// =============================================================================

func TestBulkMenuImport_DryRunMode(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)

	body := `{
		"menuName": "Test Menu",
		"menuIsActive": true,
		"menuDisplayOrder": 0,
		"dryRun": true,
		"rows": [
			{
				"categoryName": "Appetizers",
				"categoryDisplayOrder": 0,
				"itemName": "Caesar Salad",
				"itemPriceMinorUnits": 1295,
				"itemCurrencyCode": "USD",
				"itemDisplayOrder": 0
			},
			{
				"categoryName": "Entrees",
				"categoryDisplayOrder": 1,
				"itemName": "Grilled Salmon",
				"itemPriceMinorUnits": 2495,
				"itemCurrencyCode": "USD",
				"itemDisplayOrder": 0,
				"modifierGroupName": "Size",
				"modifierGroupMinSelections": 1,
				"modifierGroupMaxSelections": 1,
				"modifierGroupDisplayOrder": 0,
				"modifierName": "Large",
				"modifierPriceAdjustment": 150,
				"modifierIsDefault": false,
				"modifierDisplayOrder": 1
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/product-menus/bulk-import", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleBulkMenuImport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp bulkMenuImportResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.MenuID != nil {
		t.Error("expected menuId to be nil in dry-run mode")
	}
	if resp.CategoriesCreated != 2 {
		t.Errorf("expected 2 categories, got %d", resp.CategoriesCreated)
	}
	if resp.ItemsCreated != 2 {
		t.Errorf("expected 2 items, got %d", resp.ItemsCreated)
	}
}

func TestBulkMenuImport_ValidationErrors(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)

	body := `{
		"menuName": "Test Menu",
		"menuIsActive": true,
		"menuDisplayOrder": 0,
		"dryRun": true,
		"rows": [
			{
				"categoryName": "",
				"categoryDisplayOrder": 0,
				"itemName": "Caesar Salad",
				"itemPriceMinorUnits": 0,
				"itemCurrencyCode": "",
				"itemDisplayOrder": 0
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/product-menus/bulk-import", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleBulkMenuImport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp bulkMenuImportResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success to be false")
	}
	if len(resp.Errors) == 0 {
		t.Error("expected validation errors")
	}
}

func TestBulkMenuImport_ActualImport(t *testing.T) {
	h, database := setupTestProductCatalogHandler(t)

	// Create business
	business, _ := db.CreateBusiness(database, "Test Restaurant", nil)

	body := `{
		"menuName": "Imported Menu",
		"menuDescription": "From spreadsheet",
		"menuIsActive": true,
		"menuDisplayOrder": 0,
		"dryRun": false,
		"rows": [
			{
				"categoryName": "Appetizers",
				"categoryDescription": "Start your meal",
				"categoryDisplayOrder": 0,
				"itemName": "Caesar Salad",
				"itemDescription": "Romaine, parmesan, croutons",
				"itemPriceMinorUnits": 1295,
				"itemCurrencyCode": "USD",
				"itemImageUrls": ["https://cdn.example.com/salad.jpg"],
				"itemDietaryTags": ["vegetarian"],
				"itemDisplayOrder": 0,
				"modifierGroupName": "Size",
				"modifierGroupMinSelections": 1,
				"modifierGroupMaxSelections": 1,
				"modifierGroupDisplayOrder": 0,
				"modifierName": "Small",
				"modifierPriceAdjustment": 0,
				"modifierIsDefault": true,
				"modifierDisplayOrder": 0
			},
			{
				"categoryName": "Appetizers",
				"categoryDescription": "Start your meal",
				"categoryDisplayOrder": 0,
				"itemName": "Caesar Salad",
				"itemDescription": "Romaine, parmesan, croutons",
				"itemPriceMinorUnits": 1295,
				"itemCurrencyCode": "USD",
				"itemDisplayOrder": 0,
				"modifierGroupName": "Size",
				"modifierGroupMinSelections": 1,
				"modifierGroupMaxSelections": 1,
				"modifierGroupDisplayOrder": 0,
				"modifierName": "Large",
				"modifierPriceAdjustment": 200,
				"modifierIsDefault": false,
				"modifierDisplayOrder": 1
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/business/"+business.ID+"/product-menus/bulk-import", bytes.NewBufferString(body))
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleBulkMenuImport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp bulkMenuImportResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Errorf("expected success to be true, got errors: %v", resp.Errors)
	}
	if resp.MenuID == nil {
		t.Error("expected menuId to be set in actual import mode")
	}
	if resp.CategoriesCreated != 1 {
		t.Errorf("expected 1 category, got %d", resp.CategoriesCreated)
	}
	if resp.ItemsCreated != 2 {
		t.Errorf("expected 2 items, got %d", resp.ItemsCreated)
	}
	// Note: We create 2 modifier groups because each row creates a new item,
	// and each item gets its own modifier group (even with the same name)
	if resp.ModifierGroupsCreated != 2 {
		t.Errorf("expected 2 modifier groups, got %d", resp.ModifierGroupsCreated)
	}
	if resp.ModifiersCreated != 2 {
		t.Errorf("expected 2 modifiers, got %d", resp.ModifiersCreated)
	}

	// Verify data was actually created by fetching the full menu
	req2 := httptest.NewRequest(http.MethodGet, "/api/product-menu/"+*resp.MenuID+"/full", nil)
	req2.SetPathValue("menuId", *resp.MenuID)
	w2 := httptest.NewRecorder()

	h.HandleGetFullMenu(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 when fetching imported menu, got %d: %s", w2.Code, w2.Body.String())
	}

	var fullMenu fullMenuResponse
	json.NewDecoder(w2.Body).Decode(&fullMenu)
	if fullMenu.Name != "Imported Menu" {
		t.Errorf("expected menu name 'Imported Menu', got '%s'", fullMenu.Name)
	}
	if len(fullMenu.Categories) != 1 {
		t.Fatalf("expected 1 category in imported menu, got %d", len(fullMenu.Categories))
	}
	if len(fullMenu.Categories[0].Items) != 2 {
		t.Fatalf("expected 2 items in category, got %d", len(fullMenu.Categories[0].Items))
	}
}
