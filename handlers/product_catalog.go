package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pantheon/artemis/db"
)

// ProductCatalogHandler holds the database connection and provides HTTP handlers
// for product catalog operations (menus, categories, items, modifiers).
// Use NewProductCatalogHandler to create one.
type ProductCatalogHandler struct {
	DB *sql.DB
}

// NewProductCatalogHandler creates a new ProductCatalogHandler with the given database connection.
func NewProductCatalogHandler(database *sql.DB) *ProductCatalogHandler {
	return &ProductCatalogHandler{DB: database}
}

// =============================================================================
// Request / Response Types
// =============================================================================

// createMenuRequest is the JSON body for POST /api/business/{businessId}/product-menus/create
type createMenuRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	IsActive     bool    `json:"isActive"`
	DisplayOrder int     `json:"displayOrder"`
}

// updateMenuRequest is the JSON body for PUT /api/product-menu/{menuId}
type updateMenuRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	IsActive     bool    `json:"isActive"`
	DisplayOrder int     `json:"displayOrder"`
}

// createCategoryRequest is the JSON body for POST /api/product-menu/{menuId}/categories
type createCategoryRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	IsAvailable  bool    `json:"isAvailable"`
	DisplayOrder int     `json:"displayOrder"`
}

// updateCategoryRequest is the JSON body for PUT /api/category/{categoryId}
type updateCategoryRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	IsAvailable  bool    `json:"isAvailable"`
	DisplayOrder int     `json:"displayOrder"`
}

// createItemRequest is the JSON body for POST /api/category/{categoryId}/items
type createItemRequest struct {
	Name            string   `json:"name"`
	Description     *string  `json:"description,omitempty"`
	PriceMinorUnits int      `json:"priceMinorUnits"`
	CurrencyCode    string   `json:"currencyCode"`
	ImageURLs       []string `json:"imageUrls,omitempty"`
	DietaryTags     []string `json:"dietaryTags,omitempty"`
	IsAvailable     bool     `json:"isAvailable"`
	IsSoldOut       bool     `json:"isSoldOut"`
	DisplayOrder    int      `json:"displayOrder"`
}

// updateItemRequest is the JSON body for PUT /api/item/{itemId}
type updateItemRequest struct {
	Name            string   `json:"name"`
	Description     *string  `json:"description,omitempty"`
	PriceMinorUnits int      `json:"priceMinorUnits"`
	CurrencyCode    string   `json:"currencyCode"`
	ImageURLs       []string `json:"imageUrls,omitempty"`
	DietaryTags     []string `json:"dietaryTags,omitempty"`
	IsAvailable     bool     `json:"isAvailable"`
	IsSoldOut       bool     `json:"isSoldOut"`
	DisplayOrder    int      `json:"displayOrder"`
}

// toggleSoldOutRequest is the JSON body for PUT /api/item/{itemId}/sold-out
type toggleSoldOutRequest struct {
	IsSoldOut bool `json:"isSoldOut"`
}

// createModifierGroupRequest is the JSON body for POST /api/item/{itemId}/modifier-groups
type createModifierGroupRequest struct {
	Name          string `json:"name"`
	MinSelections int    `json:"minSelections"`
	MaxSelections int    `json:"maxSelections"`
	IsAvailable   bool   `json:"isAvailable"`
	DisplayOrder  int    `json:"displayOrder"`
}

// updateModifierGroupRequest is the JSON body for PUT /api/modifier-group/{modifierGroupId}
type updateModifierGroupRequest struct {
	Name          string `json:"name"`
	MinSelections int    `json:"minSelections"`
	MaxSelections int    `json:"maxSelections"`
	IsAvailable   bool   `json:"isAvailable"`
	DisplayOrder  int    `json:"displayOrder"`
}

// createModifierRequest is the JSON body for POST /api/modifier-group/{modifierGroupId}/modifiers
type createModifierRequest struct {
	Name                      string `json:"name"`
	PriceAdjustmentMinorUnits int    `json:"priceAdjustmentMinorUnits"`
	IsAvailable               bool   `json:"isAvailable"`
	IsDefault                 bool   `json:"isDefault"`
	DisplayOrder              int    `json:"displayOrder"`
}

// updateModifierRequest is the JSON body for PUT /api/modifier/{modifierId}
type updateModifierRequest struct {
	Name                      string `json:"name"`
	PriceAdjustmentMinorUnits int    `json:"priceAdjustmentMinorUnits"`
	IsAvailable               bool   `json:"isAvailable"`
	IsDefault                 bool   `json:"isDefault"`
	DisplayOrder              int    `json:"displayOrder"`
}

// reorderRequest is the JSON body for all reorder endpoints
type reorderRequest struct {
	OrderedIDs []string `json:"orderedIds"`
}

// imageUploadRequest is the JSON body for POST /api/product-catalog/image-upload
type imageUploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int    `json:"sizeBytes"`
}

// imageUploadResponse is the response for POST /api/product-catalog/image-upload
type imageUploadResponse struct {
	UploadURL string `json:"uploadUrl"`
	PublicURL string `json:"publicUrl"`
	ExpiresAt string `json:"expiresAt"`
}

// bulkMenuImportRow represents a single row in the bulk import
type bulkMenuImportRow struct {
	CategoryName               string   `json:"categoryName"`
	CategoryDescription        *string  `json:"categoryDescription,omitempty"`
	CategoryDisplayOrder       int      `json:"categoryDisplayOrder"`
	ItemName                   string   `json:"itemName"`
	ItemDescription            *string  `json:"itemDescription,omitempty"`
	ItemPriceMinorUnits        int      `json:"itemPriceMinorUnits"`
	ItemCurrencyCode           string   `json:"itemCurrencyCode"`
	ItemImageURLs              []string `json:"itemImageUrls,omitempty"`
	ItemDietaryTags            []string `json:"itemDietaryTags,omitempty"`
	ItemDisplayOrder           int      `json:"itemDisplayOrder"`
	ModifierGroupName          *string  `json:"modifierGroupName,omitempty"`
	ModifierGroupMinSelections *int     `json:"modifierGroupMinSelections,omitempty"`
	ModifierGroupMaxSelections *int     `json:"modifierGroupMaxSelections,omitempty"`
	ModifierGroupDisplayOrder  *int     `json:"modifierGroupDisplayOrder,omitempty"`
	ModifierName               *string  `json:"modifierName,omitempty"`
	ModifierPriceAdjustment    *int     `json:"modifierPriceAdjustment,omitempty"`
	ModifierIsDefault          *bool    `json:"modifierIsDefault,omitempty"`
	ModifierDisplayOrder       *int     `json:"modifierDisplayOrder,omitempty"`
}

// bulkMenuImportRequest is the JSON body for POST /api/business/{businessId}/product-menus/bulk-import
type bulkMenuImportRequest struct {
	MenuName         string              `json:"menuName"`
	MenuDescription  *string             `json:"menuDescription,omitempty"`
	MenuIsActive     bool                `json:"menuIsActive"`
	MenuDisplayOrder int                 `json:"menuDisplayOrder"`
	DryRun           bool                `json:"dryRun"`
	Rows             []bulkMenuImportRow `json:"rows"`
}

// bulkMenuImportRowError represents a validation error for a specific row
type bulkMenuImportRowError struct {
	RowIndex int    `json:"rowIndex"`
	Field    string `json:"field"`
	Error    string `json:"error"`
}

// bulkMenuImportResponse is the response for bulk import
type bulkMenuImportResponse struct {
	Success              bool                     `json:"success"`
	MenuID               *string                  `json:"menuId,omitempty"`
	CategoriesCreated    int                      `json:"categoriesCreated"`
	ItemsCreated         int                      `json:"itemsCreated"`
	ModifierGroupsCreated int                     `json:"modifierGroupsCreated"`
	ModifiersCreated     int                      `json:"modifiersCreated"`
	Errors               []bulkMenuImportRowError `json:"errors,omitempty"`
}

// Full menu response types for nested data
type modifierResponse struct {
	ID                        string `json:"id"`
	ModifierGroupID           string `json:"modifierGroupId"`
	Name                      string `json:"name"`
	PriceAdjustmentMinorUnits int    `json:"priceAdjustmentMinorUnits"`
	IsAvailable               bool   `json:"isAvailable"`
	IsDefault                 bool   `json:"isDefault"`
	DisplayOrder              int    `json:"displayOrder"`
	CreatedAt                 string `json:"createdAt"`
	UpdatedAt                 string `json:"updatedAt"`
}

type modifierGroupResponse struct {
	ID            string             `json:"id"`
	ItemID        string             `json:"itemId"`
	Name          string             `json:"name"`
	MinSelections int                `json:"minSelections"`
	MaxSelections int                `json:"maxSelections"`
	IsAvailable   bool               `json:"isAvailable"`
	DisplayOrder  int                `json:"displayOrder"`
	CreatedAt     string             `json:"createdAt"`
	UpdatedAt     string             `json:"updatedAt"`
	Modifiers     []modifierResponse `json:"modifiers,omitempty"`
}

type itemResponse struct {
	ID              string                  `json:"id"`
	CategoryID      string                  `json:"categoryId"`
	Name            string                  `json:"name"`
	Description     *string                 `json:"description,omitempty"`
	PriceMinorUnits int                     `json:"priceMinorUnits"`
	CurrencyCode    string                  `json:"currencyCode"`
	ImageURLs       []string                `json:"imageUrls,omitempty"`
	DietaryTags     []string                `json:"dietaryTags,omitempty"`
	IsAvailable     bool                    `json:"isAvailable"`
	IsSoldOut       bool                    `json:"isSoldOut"`
	DisplayOrder    int                     `json:"displayOrder"`
	CreatedAt       string                  `json:"createdAt"`
	UpdatedAt       string                  `json:"updatedAt"`
	ModifierGroups  []modifierGroupResponse `json:"modifierGroups,omitempty"`
}

type categoryResponse struct {
	ID           string         `json:"id"`
	MenuID       string         `json:"menuId"`
	Name         string         `json:"name"`
	Description  *string        `json:"description,omitempty"`
	IsAvailable  bool           `json:"isAvailable"`
	DisplayOrder int            `json:"displayOrder"`
	CreatedAt    string         `json:"createdAt"`
	UpdatedAt    string         `json:"updatedAt"`
	Items        []itemResponse `json:"items,omitempty"`
}

type fullMenuResponse struct {
	ID           string             `json:"id"`
	BusinessID   string             `json:"businessId"`
	Name         string             `json:"name"`
	Description  *string            `json:"description,omitempty"`
	IsActive     bool               `json:"isActive"`
	DisplayOrder int                `json:"displayOrder"`
	CreatedAt    string             `json:"createdAt"`
	UpdatedAt    string             `json:"updatedAt"`
	Categories   []categoryResponse `json:"categories,omitempty"`
}

// =============================================================================
// Menu Handlers
// =============================================================================

// HandleListProductMenus returns all product menus for a business.
// GET /api/business/{businessId}/product-menus
func (h *ProductCatalogHandler) HandleListProductMenus(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, business_id, name, description, is_active, display_order, created_at, updated_at
		FROM product_menus
		WHERE business_id = ?
		ORDER BY display_order ASC
	`, businessID)
	if err != nil {
		log.Printf("❌ Failed to list product menus: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list product menus")
		return
	}
	defer rows.Close()

	menus := []db.ProductMenu{}
	for rows.Next() {
		var menu db.ProductMenu
		if err := rows.Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Description, &menu.IsActive, &menu.DisplayOrder, &menu.CreatedAt, &menu.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan product menu row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan product menu")
			return
		}
		menus = append(menus, menu)
	}

	if menus == nil {
		menus = []db.ProductMenu{}
	}

	writeJSON(w, http.StatusOK, menus)
}

// HandleCreateProductMenu creates a new product menu for a business.
// POST /api/business/{businessId}/product-menus/create
func (h *ProductCatalogHandler) HandleCreateProductMenu(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	var req createMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	id := uuid.NewString()
	now := time.Now()

	_, err := h.DB.Exec(`
		INSERT INTO product_menus (id, business_id, name, description, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, businessID, req.Name, req.Description, req.IsActive, req.DisplayOrder, now, now)
	if err != nil {
		log.Printf("❌ Failed to create product menu: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create product menu")
		return
	}

	menu := db.ProductMenu{
		ID:           id,
		BusinessID:   businessID,
		Name:         req.Name,
		Description:  req.Description,
		IsActive:     req.IsActive,
		DisplayOrder: req.DisplayOrder,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	log.Printf("📋 Created product menu: %s (id: %s)", menu.Name, menu.ID)
	writeJSON(w, http.StatusCreated, menu)
}

// HandleGetProductMenu returns a single product menu by ID.
// GET /api/product-menu/{menuId}
func (h *ProductCatalogHandler) HandleGetProductMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuId")
	if menuID == "" {
		writeError(w, http.StatusBadRequest, "Menu ID is required")
		return
	}

	var menu db.ProductMenu
	err := h.DB.QueryRow(`
		SELECT id, business_id, name, description, is_active, display_order, created_at, updated_at
		FROM product_menus
		WHERE id = ?
	`, menuID).Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Description, &menu.IsActive, &menu.DisplayOrder, &menu.CreatedAt, &menu.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Menu not found")
		return
	}
	if err != nil {
		log.Printf("❌ Failed to get product menu: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get product menu")
		return
	}

	writeJSON(w, http.StatusOK, menu)
}

// HandleGetFullMenu returns a complete menu with all nested categories, items, and modifiers.
// GET /api/product-menu/{menuId}/full
func (h *ProductCatalogHandler) HandleGetFullMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuId")
	if menuID == "" {
		writeError(w, http.StatusBadRequest, "Menu ID is required")
		return
	}

	// Fetch the menu
	var menu db.ProductMenu
	err := h.DB.QueryRow(`
		SELECT id, business_id, name, description, is_active, display_order, created_at, updated_at
		FROM product_menus
		WHERE id = ?
	`, menuID).Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Description, &menu.IsActive, &menu.DisplayOrder, &menu.CreatedAt, &menu.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Menu not found")
		return
	}
	if err != nil {
		log.Printf("❌ Failed to get product menu: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get product menu")
		return
	}

	// Fetch categories
	categories, err := h.getCategoriesForMenu(menuID)
	if err != nil {
		log.Printf("❌ Failed to get categories for menu: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get categories")
		return
	}

	// Build response
	resp := fullMenuResponse{
		ID:           menu.ID,
		BusinessID:   menu.BusinessID,
		Name:         menu.Name,
		Description:  menu.Description,
		IsActive:     menu.IsActive,
		DisplayOrder: menu.DisplayOrder,
		CreatedAt:    menu.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    menu.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Categories:   categories,
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleUpdateProductMenu updates a product menu.
// PUT /api/product-menu/{menuId}
func (h *ProductCatalogHandler) HandleUpdateProductMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuId")
	if menuID == "" {
		writeError(w, http.StatusBadRequest, "Menu ID is required")
		return
	}

	var req updateMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	now := time.Now()
	result, err := h.DB.Exec(`
		UPDATE product_menus
		SET name = ?, description = ?, is_active = ?, display_order = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.Description, req.IsActive, req.DisplayOrder, now, menuID)
	if err != nil {
		log.Printf("❌ Failed to update product menu: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update product menu")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Menu not found")
		return
	}

	// Fetch updated menu
	var menu db.ProductMenu
	err = h.DB.QueryRow(`
		SELECT id, business_id, name, description, is_active, display_order, created_at, updated_at
		FROM product_menus
		WHERE id = ?
	`, menuID).Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Description, &menu.IsActive, &menu.DisplayOrder, &menu.CreatedAt, &menu.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to get updated product menu: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get updated product menu")
		return
	}

	log.Printf("📋 Updated product menu: %s (id: %s)", menu.Name, menu.ID)
	writeJSON(w, http.StatusOK, menu)
}

// HandleDeleteProductMenu deletes a product menu and all its categories, items, and modifiers.
// DELETE /api/product-menu/{menuId}
func (h *ProductCatalogHandler) HandleDeleteProductMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuId")
	if menuID == "" {
		writeError(w, http.StatusBadRequest, "Menu ID is required")
		return
	}

	result, err := h.DB.Exec(`DELETE FROM product_menus WHERE id = ?`, menuID)
	if err != nil {
		log.Printf("❌ Failed to delete product menu: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete product menu")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Menu not found")
		return
	}

	log.Printf("🗑️ Deleted product menu: %s", menuID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderProductMenus reorders all menus for a business.
// PUT /api/business/{businessId}/product-menus/reorder
func (h *ProductCatalogHandler) HandleReorderProductMenus(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.OrderedIDs) == 0 {
		writeError(w, http.StatusBadRequest, "orderedIds is required")
		return
	}

	// Update display_order for each menu
	for i, menuID := range req.OrderedIDs {
		_, err := h.DB.Exec(`
			UPDATE product_menus
			SET display_order = ?, updated_at = ?
			WHERE id = ? AND business_id = ?
		`, i, time.Now(), menuID, businessID)
		if err != nil {
			log.Printf("❌ Failed to reorder product menu: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to reorder product menus")
			return
		}
	}

	// Fetch reordered menus
	rows, err := h.DB.Query(`
		SELECT id, business_id, name, description, is_active, display_order, created_at, updated_at
		FROM product_menus
		WHERE business_id = ?
		ORDER BY display_order ASC
	`, businessID)
	if err != nil {
		log.Printf("❌ Failed to fetch reordered menus: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch reordered menus")
		return
	}
	defer rows.Close()

	menus := []db.ProductMenu{}
	for rows.Next() {
		var menu db.ProductMenu
		if err := rows.Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Description, &menu.IsActive, &menu.DisplayOrder, &menu.CreatedAt, &menu.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan menu row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan menu")
			return
		}
		menus = append(menus, menu)
	}

	if menus == nil {
		menus = []db.ProductMenu{}
	}

	writeJSON(w, http.StatusOK, menus)
}

// =============================================================================
// Category Handlers
// =============================================================================

// HandleListCategories returns all categories for a menu.
// GET /api/product-menu/{menuId}/categories
func (h *ProductCatalogHandler) HandleListCategories(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuId")
	if menuID == "" {
		writeError(w, http.StatusBadRequest, "Menu ID is required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, menu_id, name, description, is_available, display_order, created_at, updated_at
		FROM categories
		WHERE menu_id = ?
		ORDER BY display_order ASC
	`, menuID)
	if err != nil {
		log.Printf("❌ Failed to list categories: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list categories")
		return
	}
	defer rows.Close()

	categories := []db.Category{}
	for rows.Next() {
		var category db.Category
		if err := rows.Scan(&category.ID, &category.MenuID, &category.Name, &category.Description, &category.IsAvailable, &category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan category row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan category")
			return
		}
		categories = append(categories, category)
	}

	if categories == nil {
		categories = []db.Category{}
	}

	writeJSON(w, http.StatusOK, categories)
}

// HandleCreateCategory creates a new category within a menu.
// POST /api/product-menu/{menuId}/categories
func (h *ProductCatalogHandler) HandleCreateCategory(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuId")
	if menuID == "" {
		writeError(w, http.StatusBadRequest, "Menu ID is required")
		return
	}

	var req createCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	id := uuid.NewString()
	now := time.Now()

	_, err := h.DB.Exec(`
		INSERT INTO categories (id, menu_id, name, description, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, menuID, req.Name, req.Description, req.IsAvailable, req.DisplayOrder, now, now)
	if err != nil {
		log.Printf("❌ Failed to create category: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create category")
		return
	}

	category := db.Category{
		ID:           id,
		MenuID:       menuID,
		Name:         req.Name,
		Description:  req.Description,
		IsAvailable:  req.IsAvailable,
		DisplayOrder: req.DisplayOrder,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	log.Printf("📁 Created category: %s (id: %s)", category.Name, category.ID)
	writeJSON(w, http.StatusCreated, category)
}

// HandleGetCategory returns a single category by ID.
// GET /api/category/{categoryId}
func (h *ProductCatalogHandler) HandleGetCategory(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("categoryId")
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	var category db.Category
	err := h.DB.QueryRow(`
		SELECT id, menu_id, name, description, is_available, display_order, created_at, updated_at
		FROM categories
		WHERE id = ?
	`, categoryID).Scan(&category.ID, &category.MenuID, &category.Name, &category.Description, &category.IsAvailable, &category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}
	if err != nil {
		log.Printf("❌ Failed to get category: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get category")
		return
	}

	writeJSON(w, http.StatusOK, category)
}

// HandleUpdateCategory updates a category.
// PUT /api/category/{categoryId}
func (h *ProductCatalogHandler) HandleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("categoryId")
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	var req updateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	now := time.Now()
	result, err := h.DB.Exec(`
		UPDATE categories
		SET name = ?, description = ?, is_available = ?, display_order = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.Description, req.IsAvailable, req.DisplayOrder, now, categoryID)
	if err != nil {
		log.Printf("❌ Failed to update category: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update category")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}

	// Fetch updated category
	var category db.Category
	err = h.DB.QueryRow(`
		SELECT id, menu_id, name, description, is_available, display_order, created_at, updated_at
		FROM categories
		WHERE id = ?
	`, categoryID).Scan(&category.ID, &category.MenuID, &category.Name, &category.Description, &category.IsAvailable, &category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to get updated category: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get updated category")
		return
	}

	log.Printf("📁 Updated category: %s (id: %s)", category.Name, category.ID)
	writeJSON(w, http.StatusOK, category)
}

// HandleDeleteCategory deletes a category and all its items.
// DELETE /api/category/{categoryId}
func (h *ProductCatalogHandler) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("categoryId")
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	result, err := h.DB.Exec(`DELETE FROM categories WHERE id = ?`, categoryID)
	if err != nil {
		log.Printf("❌ Failed to delete category: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete category")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}

	log.Printf("🗑️ Deleted category: %s", categoryID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderCategories reorders all categories within a menu.
// PUT /api/product-menu/{menuId}/categories/reorder
func (h *ProductCatalogHandler) HandleReorderCategories(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuId")
	if menuID == "" {
		writeError(w, http.StatusBadRequest, "Menu ID is required")
		return
	}

	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.OrderedIDs) == 0 {
		writeError(w, http.StatusBadRequest, "orderedIds is required")
		return
	}

	// Update display_order for each category
	for i, categoryID := range req.OrderedIDs {
		_, err := h.DB.Exec(`
			UPDATE categories
			SET display_order = ?, updated_at = ?
			WHERE id = ? AND menu_id = ?
		`, i, time.Now(), categoryID, menuID)
		if err != nil {
			log.Printf("❌ Failed to reorder category: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to reorder categories")
			return
		}
	}

	// Fetch reordered categories
	rows, err := h.DB.Query(`
		SELECT id, menu_id, name, description, is_available, display_order, created_at, updated_at
		FROM categories
		WHERE menu_id = ?
		ORDER BY display_order ASC
	`, menuID)
	if err != nil {
		log.Printf("❌ Failed to fetch reordered categories: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch reordered categories")
		return
	}
	defer rows.Close()

	categories := []db.Category{}
	for rows.Next() {
		var category db.Category
		if err := rows.Scan(&category.ID, &category.MenuID, &category.Name, &category.Description, &category.IsAvailable, &category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan category row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan category")
			return
		}
		categories = append(categories, category)
	}

	if categories == nil {
		categories = []db.Category{}
	}

	writeJSON(w, http.StatusOK, categories)
}

// =============================================================================
// Item Handlers
// =============================================================================

// HandleListItems returns all items for a category.
// GET /api/category/{categoryId}/items
func (h *ProductCatalogHandler) HandleListItems(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("categoryId")
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	items, err := h.getItemsForCategory(categoryID)
	if err != nil {
		log.Printf("❌ Failed to list items: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list items")
		return
	}

	// Convert to response format without nested data
	resp := []map[string]interface{}{}
	for _, item := range items {
		resp = append(resp, map[string]interface{}{
			"id":              item.ID,
			"categoryId":      item.CategoryID,
			"name":            item.Name,
			"description":     item.Description,
			"priceMinorUnits": item.PriceMinorUnits,
			"currencyCode":    item.CurrencyCode,
			"imageUrls":       item.ImageURLs,
			"dietaryTags":     item.DietaryTags,
			"isAvailable":     item.IsAvailable,
			"isSoldOut":       item.IsSoldOut,
			"displayOrder":    item.DisplayOrder,
			"createdAt":       item.CreatedAt,
			"updatedAt":       item.UpdatedAt,
		})
	}

	if resp == nil {
		resp = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleCreateItem creates a new item within a category.
// POST /api/category/{categoryId}/items
func (h *ProductCatalogHandler) HandleCreateItem(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("categoryId")
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	var req createItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Serialize arrays to JSON
	imageURLsJSON, _ := json.Marshal(req.ImageURLs)
	dietaryTagsJSON, _ := json.Marshal(req.DietaryTags)

	var imageURLsStr *string
	var dietaryTagsStr *string
	if len(req.ImageURLs) > 0 {
		s := string(imageURLsJSON)
		imageURLsStr = &s
	}
	if len(req.DietaryTags) > 0 {
		s := string(dietaryTagsJSON)
		dietaryTagsStr = &s
	}

	id := uuid.NewString()
	now := time.Now()

	_, err := h.DB.Exec(`
		INSERT INTO items (id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, categoryID, req.Name, req.Description, req.PriceMinorUnits, req.CurrencyCode, imageURLsStr, dietaryTagsStr, req.IsAvailable, req.IsSoldOut, req.DisplayOrder, now, now)
	if err != nil {
		log.Printf("❌ Failed to create item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create item")
		return
	}

	item := db.Item{
		ID:              id,
		CategoryID:      categoryID,
		Name:            req.Name,
		Description:     req.Description,
		PriceMinorUnits: req.PriceMinorUnits,
		CurrencyCode:    req.CurrencyCode,
		ImageURLs:       imageURLsStr,
		DietaryTags:     dietaryTagsStr,
		IsAvailable:     req.IsAvailable,
		IsSoldOut:       req.IsSoldOut,
		DisplayOrder:    req.DisplayOrder,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	log.Printf("🍽️ Created item: %s (id: %s)", item.Name, item.ID)
	writeJSON(w, http.StatusCreated, item)
}

// HandleGetItem returns a single item by ID.
// GET /api/item/{itemId}
func (h *ProductCatalogHandler) HandleGetItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	var item db.Item
	err := h.DB.QueryRow(`
		SELECT id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at
		FROM items
		WHERE id = ?
	`, itemID).Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceMinorUnits, &item.CurrencyCode, &item.ImageURLs, &item.DietaryTags, &item.IsAvailable, &item.IsSoldOut, &item.DisplayOrder, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}
	if err != nil {
		log.Printf("❌ Failed to get item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get item")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// HandleUpdateItem updates an item.
// PUT /api/item/{itemId}
func (h *ProductCatalogHandler) HandleUpdateItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	var req updateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Serialize arrays to JSON
	imageURLsJSON, _ := json.Marshal(req.ImageURLs)
	dietaryTagsJSON, _ := json.Marshal(req.DietaryTags)

	var imageURLsStr *string
	var dietaryTagsStr *string
	if len(req.ImageURLs) > 0 {
		s := string(imageURLsJSON)
		imageURLsStr = &s
	}
	if len(req.DietaryTags) > 0 {
		s := string(dietaryTagsJSON)
		dietaryTagsStr = &s
	}

	now := time.Now()
	result, err := h.DB.Exec(`
		UPDATE items
		SET name = ?, description = ?, price_minor_units = ?, currency_code = ?, image_urls = ?, dietary_tags = ?, is_available = ?, is_sold_out = ?, display_order = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.Description, req.PriceMinorUnits, req.CurrencyCode, imageURLsStr, dietaryTagsStr, req.IsAvailable, req.IsSoldOut, req.DisplayOrder, now, itemID)
	if err != nil {
		log.Printf("❌ Failed to update item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update item")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}

	// Fetch updated item
	var item db.Item
	err = h.DB.QueryRow(`
		SELECT id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at
		FROM items
		WHERE id = ?
	`, itemID).Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceMinorUnits, &item.CurrencyCode, &item.ImageURLs, &item.DietaryTags, &item.IsAvailable, &item.IsSoldOut, &item.DisplayOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to get updated item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get updated item")
		return
	}

	log.Printf("🍽️ Updated item: %s (id: %s)", item.Name, item.ID)
	writeJSON(w, http.StatusOK, item)
}

// HandleDeleteItem deletes an item and all its modifier groups.
// DELETE /api/item/{itemId}
func (h *ProductCatalogHandler) HandleDeleteItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	result, err := h.DB.Exec(`DELETE FROM items WHERE id = ?`, itemID)
	if err != nil {
		log.Printf("❌ Failed to delete item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete item")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}

	log.Printf("🗑️ Deleted item: %s", itemID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleToggleSoldOut toggles the sold-out status for an item.
// PUT /api/item/{itemId}/sold-out
func (h *ProductCatalogHandler) HandleToggleSoldOut(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	var req toggleSoldOutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	now := time.Now()
	result, err := h.DB.Exec(`
		UPDATE items
		SET is_sold_out = ?, updated_at = ?
		WHERE id = ?
	`, req.IsSoldOut, now, itemID)
	if err != nil {
		log.Printf("❌ Failed to toggle sold-out status: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to toggle sold-out status")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}

	// Fetch updated item
	var item db.Item
	err = h.DB.QueryRow(`
		SELECT id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at
		FROM items
		WHERE id = ?
	`, itemID).Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceMinorUnits, &item.CurrencyCode, &item.ImageURLs, &item.DietaryTags, &item.IsAvailable, &item.IsSoldOut, &item.DisplayOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to get updated item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get updated item")
		return
	}

	log.Printf("🍽️ Toggled sold-out status for item: %s (id: %s, sold out: %v)", item.Name, item.ID, item.IsSoldOut)
	writeJSON(w, http.StatusOK, item)
}

// HandleReorderItems reorders all items within a category.
// PUT /api/category/{categoryId}/items/reorder
func (h *ProductCatalogHandler) HandleReorderItems(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("categoryId")
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.OrderedIDs) == 0 {
		writeError(w, http.StatusBadRequest, "orderedIds is required")
		return
	}

	// Update display_order for each item
	for i, itemID := range req.OrderedIDs {
		_, err := h.DB.Exec(`
			UPDATE items
			SET display_order = ?, updated_at = ?
			WHERE id = ? AND category_id = ?
		`, i, time.Now(), itemID, categoryID)
		if err != nil {
			log.Printf("❌ Failed to reorder item: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to reorder items")
			return
		}
	}

	// Fetch reordered items
	rows, err := h.DB.Query(`
		SELECT id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at
		FROM items
		WHERE category_id = ?
		ORDER BY display_order ASC
	`, categoryID)
	if err != nil {
		log.Printf("❌ Failed to fetch reordered items: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch reordered items")
		return
	}
	defer rows.Close()

	items := []db.Item{}
	for rows.Next() {
		var item db.Item
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceMinorUnits, &item.CurrencyCode, &item.ImageURLs, &item.DietaryTags, &item.IsAvailable, &item.IsSoldOut, &item.DisplayOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan item row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan item")
			return
		}
		items = append(items, item)
	}

	if items == nil {
		items = []db.Item{}
	}

	writeJSON(w, http.StatusOK, items)
}

// =============================================================================
// Modifier Group Handlers
// =============================================================================

// HandleListModifierGroups returns all modifier groups for an item.
// GET /api/item/{itemId}/modifier-groups
func (h *ProductCatalogHandler) HandleListModifierGroups(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at
		FROM modifier_groups
		WHERE item_id = ?
		ORDER BY display_order ASC
	`, itemID)
	if err != nil {
		log.Printf("❌ Failed to list modifier groups: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list modifier groups")
		return
	}
	defer rows.Close()

	groups := []db.ModifierGroup{}
	for rows.Next() {
		var group db.ModifierGroup
		if err := rows.Scan(&group.ID, &group.ItemID, &group.Name, &group.MinSelections, &group.MaxSelections, &group.IsAvailable, &group.DisplayOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan modifier group row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan modifier group")
			return
		}
		groups = append(groups, group)
	}

	if groups == nil {
		groups = []db.ModifierGroup{}
	}

	writeJSON(w, http.StatusOK, groups)
}

// HandleCreateModifierGroup creates a new modifier group for an item.
// POST /api/item/{itemId}/modifier-groups
func (h *ProductCatalogHandler) HandleCreateModifierGroup(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	var req createModifierGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	id := uuid.NewString()
	now := time.Now()

	_, err := h.DB.Exec(`
		INSERT INTO modifier_groups (id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, itemID, req.Name, req.MinSelections, req.MaxSelections, req.IsAvailable, req.DisplayOrder, now, now)
	if err != nil {
		log.Printf("❌ Failed to create modifier group: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create modifier group")
		return
	}

	group := db.ModifierGroup{
		ID:            id,
		ItemID:        itemID,
		Name:          req.Name,
		MinSelections: req.MinSelections,
		MaxSelections: req.MaxSelections,
		IsAvailable:   req.IsAvailable,
		DisplayOrder:  req.DisplayOrder,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	log.Printf("🎛️ Created modifier group: %s (id: %s)", group.Name, group.ID)
	writeJSON(w, http.StatusCreated, group)
}

// HandleGetModifierGroup returns a single modifier group by ID.
// GET /api/modifier-group/{modifierGroupId}
func (h *ProductCatalogHandler) HandleGetModifierGroup(w http.ResponseWriter, r *http.Request) {
	modifierGroupID := r.PathValue("modifierGroupId")
	if modifierGroupID == "" {
		writeError(w, http.StatusBadRequest, "Modifier group ID is required")
		return
	}

	var group db.ModifierGroup
	err := h.DB.QueryRow(`
		SELECT id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at
		FROM modifier_groups
		WHERE id = ?
	`, modifierGroupID).Scan(&group.ID, &group.ItemID, &group.Name, &group.MinSelections, &group.MaxSelections, &group.IsAvailable, &group.DisplayOrder, &group.CreatedAt, &group.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Modifier group not found")
		return
	}
	if err != nil {
		log.Printf("❌ Failed to get modifier group: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get modifier group")
		return
	}

	writeJSON(w, http.StatusOK, group)
}

// HandleUpdateModifierGroup updates a modifier group.
// PUT /api/modifier-group/{modifierGroupId}
func (h *ProductCatalogHandler) HandleUpdateModifierGroup(w http.ResponseWriter, r *http.Request) {
	modifierGroupID := r.PathValue("modifierGroupId")
	if modifierGroupID == "" {
		writeError(w, http.StatusBadRequest, "Modifier group ID is required")
		return
	}

	var req updateModifierGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	now := time.Now()
	result, err := h.DB.Exec(`
		UPDATE modifier_groups
		SET name = ?, min_selections = ?, max_selections = ?, is_available = ?, display_order = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.MinSelections, req.MaxSelections, req.IsAvailable, req.DisplayOrder, now, modifierGroupID)
	if err != nil {
		log.Printf("❌ Failed to update modifier group: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update modifier group")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Modifier group not found")
		return
	}

	// Fetch updated modifier group
	var group db.ModifierGroup
	err = h.DB.QueryRow(`
		SELECT id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at
		FROM modifier_groups
		WHERE id = ?
	`, modifierGroupID).Scan(&group.ID, &group.ItemID, &group.Name, &group.MinSelections, &group.MaxSelections, &group.IsAvailable, &group.DisplayOrder, &group.CreatedAt, &group.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to get updated modifier group: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get updated modifier group")
		return
	}

	log.Printf("🎛️ Updated modifier group: %s (id: %s)", group.Name, group.ID)
	writeJSON(w, http.StatusOK, group)
}

// HandleDeleteModifierGroup deletes a modifier group and all its modifiers.
// DELETE /api/modifier-group/{modifierGroupId}
func (h *ProductCatalogHandler) HandleDeleteModifierGroup(w http.ResponseWriter, r *http.Request) {
	modifierGroupID := r.PathValue("modifierGroupId")
	if modifierGroupID == "" {
		writeError(w, http.StatusBadRequest, "Modifier group ID is required")
		return
	}

	result, err := h.DB.Exec(`DELETE FROM modifier_groups WHERE id = ?`, modifierGroupID)
	if err != nil {
		log.Printf("❌ Failed to delete modifier group: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete modifier group")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Modifier group not found")
		return
	}

	log.Printf("🗑️ Deleted modifier group: %s", modifierGroupID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderModifierGroups reorders all modifier groups within an item.
// PUT /api/item/{itemId}/modifier-groups/reorder
func (h *ProductCatalogHandler) HandleReorderModifierGroups(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.OrderedIDs) == 0 {
		writeError(w, http.StatusBadRequest, "orderedIds is required")
		return
	}

	// Update display_order for each modifier group
	for i, groupID := range req.OrderedIDs {
		_, err := h.DB.Exec(`
			UPDATE modifier_groups
			SET display_order = ?, updated_at = ?
			WHERE id = ? AND item_id = ?
		`, i, time.Now(), groupID, itemID)
		if err != nil {
			log.Printf("❌ Failed to reorder modifier group: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to reorder modifier groups")
			return
		}
	}

	// Fetch reordered modifier groups
	rows, err := h.DB.Query(`
		SELECT id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at
		FROM modifier_groups
		WHERE item_id = ?
		ORDER BY display_order ASC
	`, itemID)
	if err != nil {
		log.Printf("❌ Failed to fetch reordered modifier groups: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch reordered modifier groups")
		return
	}
	defer rows.Close()

	groups := []db.ModifierGroup{}
	for rows.Next() {
		var group db.ModifierGroup
		if err := rows.Scan(&group.ID, &group.ItemID, &group.Name, &group.MinSelections, &group.MaxSelections, &group.IsAvailable, &group.DisplayOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan modifier group row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan modifier group")
			return
		}
		groups = append(groups, group)
	}

	if groups == nil {
		groups = []db.ModifierGroup{}
	}

	writeJSON(w, http.StatusOK, groups)
}

// =============================================================================
// Modifier Handlers
// =============================================================================

// HandleListModifiers returns all modifiers for a modifier group.
// GET /api/modifier-group/{modifierGroupId}/modifiers
func (h *ProductCatalogHandler) HandleListModifiers(w http.ResponseWriter, r *http.Request) {
	modifierGroupID := r.PathValue("modifierGroupId")
	if modifierGroupID == "" {
		writeError(w, http.StatusBadRequest, "Modifier group ID is required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at
		FROM modifiers
		WHERE modifier_group_id = ?
		ORDER BY display_order ASC
	`, modifierGroupID)
	if err != nil {
		log.Printf("❌ Failed to list modifiers: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list modifiers")
		return
	}
	defer rows.Close()

	modifiers := []db.Modifier{}
	for rows.Next() {
		var modifier db.Modifier
		if err := rows.Scan(&modifier.ID, &modifier.ModifierGroupID, &modifier.Name, &modifier.PriceAdjustmentMinorUnits, &modifier.IsAvailable, &modifier.IsDefault, &modifier.DisplayOrder, &modifier.CreatedAt, &modifier.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan modifier row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan modifier")
			return
		}
		modifiers = append(modifiers, modifier)
	}

	if modifiers == nil {
		modifiers = []db.Modifier{}
	}

	writeJSON(w, http.StatusOK, modifiers)
}

// HandleCreateModifier creates a new modifier within a modifier group.
// POST /api/modifier-group/{modifierGroupId}/modifiers
func (h *ProductCatalogHandler) HandleCreateModifier(w http.ResponseWriter, r *http.Request) {
	modifierGroupID := r.PathValue("modifierGroupId")
	if modifierGroupID == "" {
		writeError(w, http.StatusBadRequest, "Modifier group ID is required")
		return
	}

	var req createModifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	id := uuid.NewString()
	now := time.Now()

	_, err := h.DB.Exec(`
		INSERT INTO modifiers (id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, modifierGroupID, req.Name, req.PriceAdjustmentMinorUnits, req.IsAvailable, req.IsDefault, req.DisplayOrder, now, now)
	if err != nil {
		log.Printf("❌ Failed to create modifier: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create modifier")
		return
	}

	modifier := db.Modifier{
		ID:                        id,
		ModifierGroupID:           modifierGroupID,
		Name:                      req.Name,
		PriceAdjustmentMinorUnits: req.PriceAdjustmentMinorUnits,
		IsAvailable:               req.IsAvailable,
		IsDefault:                 req.IsDefault,
		DisplayOrder:              req.DisplayOrder,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	log.Printf("⚙️ Created modifier: %s (id: %s)", modifier.Name, modifier.ID)
	writeJSON(w, http.StatusCreated, modifier)
}

// HandleGetModifier returns a single modifier by ID.
// GET /api/modifier/{modifierId}
func (h *ProductCatalogHandler) HandleGetModifier(w http.ResponseWriter, r *http.Request) {
	modifierID := r.PathValue("modifierId")
	if modifierID == "" {
		writeError(w, http.StatusBadRequest, "Modifier ID is required")
		return
	}

	var modifier db.Modifier
	err := h.DB.QueryRow(`
		SELECT id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at
		FROM modifiers
		WHERE id = ?
	`, modifierID).Scan(&modifier.ID, &modifier.ModifierGroupID, &modifier.Name, &modifier.PriceAdjustmentMinorUnits, &modifier.IsAvailable, &modifier.IsDefault, &modifier.DisplayOrder, &modifier.CreatedAt, &modifier.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Modifier not found")
		return
	}
	if err != nil {
		log.Printf("❌ Failed to get modifier: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get modifier")
		return
	}

	writeJSON(w, http.StatusOK, modifier)
}

// HandleUpdateModifier updates a modifier.
// PUT /api/modifier/{modifierId}
func (h *ProductCatalogHandler) HandleUpdateModifier(w http.ResponseWriter, r *http.Request) {
	modifierID := r.PathValue("modifierId")
	if modifierID == "" {
		writeError(w, http.StatusBadRequest, "Modifier ID is required")
		return
	}

	var req updateModifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	now := time.Now()
	result, err := h.DB.Exec(`
		UPDATE modifiers
		SET name = ?, price_adjustment_minor_units = ?, is_available = ?, is_default = ?, display_order = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.PriceAdjustmentMinorUnits, req.IsAvailable, req.IsDefault, req.DisplayOrder, now, modifierID)
	if err != nil {
		log.Printf("❌ Failed to update modifier: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update modifier")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Modifier not found")
		return
	}

	// Fetch updated modifier
	var modifier db.Modifier
	err = h.DB.QueryRow(`
		SELECT id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at
		FROM modifiers
		WHERE id = ?
	`, modifierID).Scan(&modifier.ID, &modifier.ModifierGroupID, &modifier.Name, &modifier.PriceAdjustmentMinorUnits, &modifier.IsAvailable, &modifier.IsDefault, &modifier.DisplayOrder, &modifier.CreatedAt, &modifier.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to get updated modifier: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get updated modifier")
		return
	}

	log.Printf("⚙️ Updated modifier: %s (id: %s)", modifier.Name, modifier.ID)
	writeJSON(w, http.StatusOK, modifier)
}

// HandleDeleteModifier deletes a modifier.
// DELETE /api/modifier/{modifierId}
func (h *ProductCatalogHandler) HandleDeleteModifier(w http.ResponseWriter, r *http.Request) {
	modifierID := r.PathValue("modifierId")
	if modifierID == "" {
		writeError(w, http.StatusBadRequest, "Modifier ID is required")
		return
	}

	result, err := h.DB.Exec(`DELETE FROM modifiers WHERE id = ?`, modifierID)
	if err != nil {
		log.Printf("❌ Failed to delete modifier: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete modifier")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Modifier not found")
		return
	}

	log.Printf("🗑️ Deleted modifier: %s", modifierID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleReorderModifiers reorders all modifiers within a modifier group.
// PUT /api/modifier-group/{modifierGroupId}/modifiers/reorder
func (h *ProductCatalogHandler) HandleReorderModifiers(w http.ResponseWriter, r *http.Request) {
	modifierGroupID := r.PathValue("modifierGroupId")
	if modifierGroupID == "" {
		writeError(w, http.StatusBadRequest, "Modifier group ID is required")
		return
	}

	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.OrderedIDs) == 0 {
		writeError(w, http.StatusBadRequest, "orderedIds is required")
		return
	}

	// Update display_order for each modifier
	for i, modifierID := range req.OrderedIDs {
		_, err := h.DB.Exec(`
			UPDATE modifiers
			SET display_order = ?, updated_at = ?
			WHERE id = ? AND modifier_group_id = ?
		`, i, time.Now(), modifierID, modifierGroupID)
		if err != nil {
			log.Printf("❌ Failed to reorder modifier: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to reorder modifiers")
			return
		}
	}

	// Fetch reordered modifiers
	rows, err := h.DB.Query(`
		SELECT id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at
		FROM modifiers
		WHERE modifier_group_id = ?
		ORDER BY display_order ASC
	`, modifierGroupID)
	if err != nil {
		log.Printf("❌ Failed to fetch reordered modifiers: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch reordered modifiers")
		return
	}
	defer rows.Close()

	modifiers := []db.Modifier{}
	for rows.Next() {
		var modifier db.Modifier
		if err := rows.Scan(&modifier.ID, &modifier.ModifierGroupID, &modifier.Name, &modifier.PriceAdjustmentMinorUnits, &modifier.IsAvailable, &modifier.IsDefault, &modifier.DisplayOrder, &modifier.CreatedAt, &modifier.UpdatedAt); err != nil {
			log.Printf("❌ Failed to scan modifier row: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to scan modifier")
			return
		}
		modifiers = append(modifiers, modifier)
	}

	if modifiers == nil {
		modifiers = []db.Modifier{}
	}

	writeJSON(w, http.StatusOK, modifiers)
}

// =============================================================================
// Image Upload Handler
// =============================================================================

// HandleImageUpload generates a signed URL for image upload.
// POST /api/product-catalog/image-upload
//
// This is a placeholder implementation that returns fake signed URLs.
// In production, this would integrate with S3, GCS, or another cloud storage service
// to generate real pre-signed URLs for direct client uploads.
func (h *ProductCatalogHandler) HandleImageUpload(w http.ResponseWriter, r *http.Request) {
	var req imageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Filename == "" {
		writeError(w, http.StatusBadRequest, "Filename is required")
		return
	}
	if req.ContentType == "" {
		writeError(w, http.StatusBadRequest, "ContentType is required")
		return
	}
	if req.SizeBytes == 0 {
		writeError(w, http.StatusBadRequest, "SizeBytes is required")
		return
	}

	// Generate a fake UUID for the image
	imageID := uuid.NewString()

	// TODO: Replace this with real S3/GCS signed URL generation
	// For now, return placeholder URLs that follow the expected format
	uploadURL := "https://storage.example.com/bucket/images/" + imageID + ".jpg?signature=fake-signature"
	publicURL := "https://cdn.example.com/images/" + imageID + ".jpg"
	expiresAt := time.Now().Add(1 * time.Hour).Format("2006-01-02T15:04:05Z")

	resp := imageUploadResponse{
		UploadURL: uploadURL,
		PublicURL: publicURL,
		ExpiresAt: expiresAt,
	}

	log.Printf("🖼️ Generated image upload URL for %s (placeholder implementation)", req.Filename)
	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
// Bulk Import Handler
// =============================================================================

// HandleBulkMenuImport imports a complete menu with categories, items, and modifiers in one call.
// POST /api/business/{businessId}/product-menus/bulk-import
//
// Supports dry-run mode for validation without saving. Returns validation errors per row if any data is invalid.
// This endpoint only handles structured JSON input — it does NOT parse PDFs, photos, or other file formats.
func (h *ProductCatalogHandler) HandleBulkMenuImport(w http.ResponseWriter, r *http.Request) {
	businessID := r.PathValue("businessId")
	if businessID == "" {
		writeError(w, http.StatusBadRequest, "Business ID is required")
		return
	}

	var req bulkMenuImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.MenuName == "" {
		writeError(w, http.StatusBadRequest, "MenuName is required")
		return
	}
	if len(req.Rows) == 0 {
		writeError(w, http.StatusBadRequest, "Rows are required")
		return
	}

	// Validate each row
	errors := []bulkMenuImportRowError{}
	for i, row := range req.Rows {
		if row.CategoryName == "" {
			errors = append(errors, bulkMenuImportRowError{
				RowIndex: i,
				Field:    "categoryName",
				Error:    "Category name is required",
			})
		}
		if row.ItemName == "" {
			errors = append(errors, bulkMenuImportRowError{
				RowIndex: i,
				Field:    "itemName",
				Error:    "Item name is required",
			})
		}
		if row.ItemPriceMinorUnits <= 0 {
			errors = append(errors, bulkMenuImportRowError{
				RowIndex: i,
				Field:    "itemPriceMinorUnits",
				Error:    "Price must be greater than zero",
			})
		}
		if row.ItemCurrencyCode == "" {
			errors = append(errors, bulkMenuImportRowError{
				RowIndex: i,
				Field:    "itemCurrencyCode",
				Error:    "Currency code is required",
			})
		}
	}

	// If there are validation errors, return them
	if len(errors) > 0 {
		resp := bulkMenuImportResponse{
			Success: false,
			Errors:  errors,
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// If dry-run mode, return success without saving
	if req.DryRun {
		// Count unique categories, items, etc. for the response
		categoryNames := make(map[string]bool)
		itemCount := len(req.Rows)
		modifierGroupCount := 0
		modifierCount := 0

		for _, row := range req.Rows {
			categoryNames[row.CategoryName] = true
			if row.ModifierGroupName != nil && *row.ModifierGroupName != "" {
				modifierGroupCount++
			}
			if row.ModifierName != nil && *row.ModifierName != "" {
				modifierCount++
			}
		}

		resp := bulkMenuImportResponse{
			Success:               true,
			CategoriesCreated:     len(categoryNames),
			ItemsCreated:          itemCount,
			ModifierGroupsCreated: modifierGroupCount,
			ModifiersCreated:      modifierCount,
		}

		log.Printf("📋 Dry-run bulk import validated: %d categories, %d items", len(categoryNames), itemCount)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Create the menu
	menuID := uuid.NewString()
	now := time.Now()

	_, err := h.DB.Exec(`
		INSERT INTO product_menus (id, business_id, name, description, is_active, display_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, menuID, businessID, req.MenuName, req.MenuDescription, req.MenuIsActive, req.MenuDisplayOrder, now, now)
	if err != nil {
		log.Printf("❌ Failed to create menu during bulk import: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create menu")
		return
	}

	// Track created entities
	categoryMap := make(map[string]string) // categoryName -> categoryID
	itemMap := make(map[string]string)     // itemName -> itemID
	modifierGroupMap := make(map[string]string) // itemID:groupName -> groupID

	categoriesCreated := 0
	itemsCreated := 0
	modifierGroupsCreated := 0
	modifiersCreated := 0

	// Process rows
	for _, row := range req.Rows {
		// Create category if it doesn't exist
		var categoryID string
		if existingID, exists := categoryMap[row.CategoryName]; exists {
			categoryID = existingID
		} else {
			categoryID = uuid.NewString()
			_, err := h.DB.Exec(`
				INSERT INTO categories (id, menu_id, name, description, is_available, display_order, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, categoryID, menuID, row.CategoryName, row.CategoryDescription, true, row.CategoryDisplayOrder, now, now)
			if err != nil {
				log.Printf("❌ Failed to create category during bulk import: %v", err)
				writeError(w, http.StatusInternalServerError, "Failed to create category")
				return
			}
			categoryMap[row.CategoryName] = categoryID
			categoriesCreated++
		}

		// Create item
		itemID := uuid.NewString()
		imageURLsJSON, _ := json.Marshal(row.ItemImageURLs)
		dietaryTagsJSON, _ := json.Marshal(row.ItemDietaryTags)

		var imageURLsStr *string
		var dietaryTagsStr *string
		if len(row.ItemImageURLs) > 0 {
			s := string(imageURLsJSON)
			imageURLsStr = &s
		}
		if len(row.ItemDietaryTags) > 0 {
			s := string(dietaryTagsJSON)
			dietaryTagsStr = &s
		}

		_, err = h.DB.Exec(`
			INSERT INTO items (id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, itemID, categoryID, row.ItemName, row.ItemDescription, row.ItemPriceMinorUnits, row.ItemCurrencyCode, imageURLsStr, dietaryTagsStr, true, false, row.ItemDisplayOrder, now, now)
		if err != nil {
			log.Printf("❌ Failed to create item during bulk import: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to create item")
			return
		}
		itemMap[row.ItemName] = itemID
		itemsCreated++

		// Create modifier group if specified
		if row.ModifierGroupName != nil && *row.ModifierGroupName != "" {
			groupKey := itemID + ":" + *row.ModifierGroupName
			var modifierGroupID string
			if existingID, exists := modifierGroupMap[groupKey]; exists {
				modifierGroupID = existingID
			} else {
				modifierGroupID = uuid.NewString()
				minSel := 0
				maxSel := 1
				displayOrder := 0
				if row.ModifierGroupMinSelections != nil {
					minSel = *row.ModifierGroupMinSelections
				}
				if row.ModifierGroupMaxSelections != nil {
					maxSel = *row.ModifierGroupMaxSelections
				}
				if row.ModifierGroupDisplayOrder != nil {
					displayOrder = *row.ModifierGroupDisplayOrder
				}

				_, err := h.DB.Exec(`
					INSERT INTO modifier_groups (id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, modifierGroupID, itemID, *row.ModifierGroupName, minSel, maxSel, true, displayOrder, now, now)
				if err != nil {
					log.Printf("❌ Failed to create modifier group during bulk import: %v", err)
					writeError(w, http.StatusInternalServerError, "Failed to create modifier group")
					return
				}
				modifierGroupMap[groupKey] = modifierGroupID
				modifierGroupsCreated++
			}

			// Create modifier if specified
			if row.ModifierName != nil && *row.ModifierName != "" {
				modifierID := uuid.NewString()
				priceAdj := 0
				isDefault := false
				displayOrder := 0
				if row.ModifierPriceAdjustment != nil {
					priceAdj = *row.ModifierPriceAdjustment
				}
				if row.ModifierIsDefault != nil {
					isDefault = *row.ModifierIsDefault
				}
				if row.ModifierDisplayOrder != nil {
					displayOrder = *row.ModifierDisplayOrder
				}

				_, err := h.DB.Exec(`
					INSERT INTO modifiers (id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, modifierID, modifierGroupID, *row.ModifierName, priceAdj, true, isDefault, displayOrder, now, now)
				if err != nil {
					log.Printf("❌ Failed to create modifier during bulk import: %v", err)
					writeError(w, http.StatusInternalServerError, "Failed to create modifier")
					return
				}
				modifiersCreated++
			}
		}
	}

	resp := bulkMenuImportResponse{
		Success:               true,
		MenuID:                &menuID,
		CategoriesCreated:     categoriesCreated,
		ItemsCreated:          itemsCreated,
		ModifierGroupsCreated: modifierGroupsCreated,
		ModifiersCreated:      modifiersCreated,
	}

	log.Printf("📋 Bulk import completed: menu=%s, categories=%d, items=%d, groups=%d, modifiers=%d",
		menuID, categoriesCreated, itemsCreated, modifierGroupsCreated, modifiersCreated)
	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
// Helper Methods for Nested Data Fetching
// =============================================================================

// getCategoriesForMenu fetches all categories for a menu with nested items and modifiers.
func (h *ProductCatalogHandler) getCategoriesForMenu(menuID string) ([]categoryResponse, error) {
	rows, err := h.DB.Query(`
		SELECT id, menu_id, name, description, is_available, display_order, created_at, updated_at
		FROM categories
		WHERE menu_id = ?
		ORDER BY display_order ASC
	`, menuID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []categoryResponse{}
	for rows.Next() {
		var cat db.Category
		if err := rows.Scan(&cat.ID, &cat.MenuID, &cat.Name, &cat.Description, &cat.IsAvailable, &cat.DisplayOrder, &cat.CreatedAt, &cat.UpdatedAt); err != nil {
			return nil, err
		}

		// Fetch items for this category
		items, err := h.getItemsForCategory(cat.ID)
		if err != nil {
			return nil, err
		}

		categories = append(categories, categoryResponse{
			ID:           cat.ID,
			MenuID:       cat.MenuID,
			Name:         cat.Name,
			Description:  cat.Description,
			IsAvailable:  cat.IsAvailable,
			DisplayOrder: cat.DisplayOrder,
			CreatedAt:    cat.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    cat.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Items:        items,
		})
	}

	return categories, nil
}

// getItemsForCategory fetches all items for a category with nested modifier groups.
func (h *ProductCatalogHandler) getItemsForCategory(categoryID string) ([]itemResponse, error) {
	rows, err := h.DB.Query(`
		SELECT id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at
		FROM items
		WHERE category_id = ?
		ORDER BY display_order ASC
	`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []itemResponse{}
	for rows.Next() {
		var item db.Item
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceMinorUnits, &item.CurrencyCode, &item.ImageURLs, &item.DietaryTags, &item.IsAvailable, &item.IsSoldOut, &item.DisplayOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}

		// Deserialize JSON arrays
		var imageURLs []string
		var dietaryTags []string
		if item.ImageURLs != nil {
			json.Unmarshal([]byte(*item.ImageURLs), &imageURLs)
		}
		if item.DietaryTags != nil {
			json.Unmarshal([]byte(*item.DietaryTags), &dietaryTags)
		}

		// Fetch modifier groups for this item
		modifierGroups, err := h.getModifierGroupsForItem(item.ID)
		if err != nil {
			return nil, err
		}

		items = append(items, itemResponse{
			ID:              item.ID,
			CategoryID:      item.CategoryID,
			Name:            item.Name,
			Description:     item.Description,
			PriceMinorUnits: item.PriceMinorUnits,
			CurrencyCode:    item.CurrencyCode,
			ImageURLs:       imageURLs,
			DietaryTags:     dietaryTags,
			IsAvailable:     item.IsAvailable,
			IsSoldOut:       item.IsSoldOut,
			DisplayOrder:    item.DisplayOrder,
			CreatedAt:       item.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:       item.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			ModifierGroups:  modifierGroups,
		})
	}

	return items, nil
}

// getModifierGroupsForItem fetches all modifier groups for an item with nested modifiers.
func (h *ProductCatalogHandler) getModifierGroupsForItem(itemID string) ([]modifierGroupResponse, error) {
	rows, err := h.DB.Query(`
		SELECT id, item_id, name, min_selections, max_selections, is_available, display_order, created_at, updated_at
		FROM modifier_groups
		WHERE item_id = ?
		ORDER BY display_order ASC
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []modifierGroupResponse{}
	for rows.Next() {
		var group db.ModifierGroup
		if err := rows.Scan(&group.ID, &group.ItemID, &group.Name, &group.MinSelections, &group.MaxSelections, &group.IsAvailable, &group.DisplayOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}

		// Fetch modifiers for this group
		modifiers, err := h.getModifiersForGroup(group.ID)
		if err != nil {
			return nil, err
		}

		groups = append(groups, modifierGroupResponse{
			ID:            group.ID,
			ItemID:        group.ItemID,
			Name:          group.Name,
			MinSelections: group.MinSelections,
			MaxSelections: group.MaxSelections,
			IsAvailable:   group.IsAvailable,
			DisplayOrder:  group.DisplayOrder,
			CreatedAt:     group.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:     group.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Modifiers:     modifiers,
		})
	}

	return groups, nil
}

// getModifiersForGroup fetches all modifiers for a modifier group.
func (h *ProductCatalogHandler) getModifiersForGroup(modifierGroupID string) ([]modifierResponse, error) {
	rows, err := h.DB.Query(`
		SELECT id, modifier_group_id, name, price_adjustment_minor_units, is_available, is_default, display_order, created_at, updated_at
		FROM modifiers
		WHERE modifier_group_id = ?
		ORDER BY display_order ASC
	`, modifierGroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modifiers := []modifierResponse{}
	for rows.Next() {
		var modifier db.Modifier
		if err := rows.Scan(&modifier.ID, &modifier.ModifierGroupID, &modifier.Name, &modifier.PriceAdjustmentMinorUnits, &modifier.IsAvailable, &modifier.IsDefault, &modifier.DisplayOrder, &modifier.CreatedAt, &modifier.UpdatedAt); err != nil {
			return nil, err
		}

		modifiers = append(modifiers, modifierResponse{
			ID:                        modifier.ID,
			ModifierGroupID:           modifier.ModifierGroupID,
			Name:                      modifier.Name,
			PriceAdjustmentMinorUnits: modifier.PriceAdjustmentMinorUnits,
			IsAvailable:               modifier.IsAvailable,
			IsDefault:                 modifier.IsDefault,
			DisplayOrder:              modifier.DisplayOrder,
			CreatedAt:                 modifier.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:                 modifier.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return modifiers, nil
}
