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

// CartHandler handles all cart-related HTTP requests.
type CartHandler struct {
	DB *sql.DB
}

// NewCartHandler creates a new cart handler instance.
func NewCartHandler(database *sql.DB) *CartHandler {
	return &CartHandler{DB: database}
}

// cartWithItemsResponse is the API response shape for a cart with nested line items.
// This matches the CartWithItems schema in the contract.
type cartWithItemsResponse struct {
	ID         string             `json:"id"`
	CustomerID string             `json:"customerId"`
	BusinessID string             `json:"businessId"`
	CreatedAt  string             `json:"createdAt"`
	UpdatedAt  string             `json:"updatedAt"`
	LineItems  []cartLineItemResp `json:"lineItems"`
}

// cartLineItemResp is the API response shape for a cart line item with modifiers.
// This matches the CartLineItem schema in the contract.
type cartLineItemResp struct {
	ID        string                 `json:"id"`
	CartID    string                 `json:"cartId"`
	ItemID    string                 `json:"itemId"`
	Quantity  int                    `json:"quantity"`
	Modifiers []selectedModifierResp `json:"modifiers"`
	CreatedAt string                 `json:"createdAt"`
	UpdatedAt string                 `json:"updatedAt"`
}

// selectedModifierResp is the API response shape for a selected modifier.
type selectedModifierResp struct {
	ModifierID string `json:"modifierId"`
}

// HandleCreateOrGetCart creates a new cart or returns the existing cart for a customer and business.
// POST /api/customer/{customerId}/business/{businessId}/cart
// Returns 201 if a new cart was created, 200 if an existing cart was returned.
func (h *CartHandler) HandleCreateOrGetCart(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	businessID := r.PathValue("businessId")

	if customerID == "" || businessID == "" {
		writeError(w, http.StatusBadRequest, "customer ID and business ID are required")
		return
	}

	// Check if cart already exists
	cart, lineItems, err := h.getCartByCustomerAndBusiness(customerID, businessID)
	if err == nil {
		// Cart exists, return it
		resp := h.buildCartWithItemsResponse(cart, lineItems)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Cart doesn't exist, create a new one
	cart = &db.Cart{
		ID:         uuid.NewString(),
		CustomerID: customerID,
		BusinessID: businessID,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	query := `
		INSERT INTO carts (id, customer_id, business_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = h.DB.Exec(query, cart.ID, cart.CustomerID, cart.BusinessID, cart.CreatedAt, cart.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to create cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create cart")
		return
	}

	resp := h.buildCartWithItemsResponse(cart, []cartLineItemResp{})
	writeJSON(w, http.StatusCreated, resp)
}

// HandleGetCart returns the active cart for a customer and business.
// GET /api/customer/{customerId}/business/{businessId}/cart
func (h *CartHandler) HandleGetCart(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	businessID := r.PathValue("businessId")

	if customerID == "" || businessID == "" {
		writeError(w, http.StatusBadRequest, "customer ID and business ID are required")
		return
	}

	cart, lineItems, err := h.getCartByCustomerAndBusiness(customerID, businessID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Cart not found")
			return
		}
		log.Printf("❌ Failed to get cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get cart")
		return
	}

	resp := h.buildCartWithItemsResponse(cart, lineItems)
	writeJSON(w, http.StatusOK, resp)
}

// HandleDeleteCart deletes the active cart for a customer and business.
// DELETE /api/customer/{customerId}/business/{businessId}/cart
func (h *CartHandler) HandleDeleteCart(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	businessID := r.PathValue("businessId")

	if customerID == "" || businessID == "" {
		writeError(w, http.StatusBadRequest, "customer ID and business ID are required")
		return
	}

	// Check if cart exists
	cart, _, err := h.getCartByCustomerAndBusiness(customerID, businessID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Cart not found")
			return
		}
		log.Printf("❌ Failed to get cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get cart")
		return
	}

	// Delete the cart (cascade will delete line items and modifiers)
	query := `DELETE FROM carts WHERE id = ?`
	_, err = h.DB.Exec(query, cart.ID)
	if err != nil {
		log.Printf("❌ Failed to delete cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete cart")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleClearCart removes all line items from a cart without deleting the cart itself.
// PUT /api/cart/{cartId}/clear
func (h *CartHandler) HandleClearCart(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")

	if cartID == "" {
		writeError(w, http.StatusBadRequest, "cart ID is required")
		return
	}

	// Check if cart exists
	cart, err := h.getCartByID(cartID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Cart not found")
			return
		}
		log.Printf("❌ Failed to get cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get cart")
		return
	}

	// Delete all line items (cascade will delete modifiers)
	query := `DELETE FROM cart_line_items WHERE cart_id = ?`
	_, err = h.DB.Exec(query, cartID)
	if err != nil {
		log.Printf("❌ Failed to clear cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to clear cart")
		return
	}

	// Update cart's updated_at timestamp
	updateQuery := `UPDATE carts SET updated_at = ? WHERE id = ?`
	_, err = h.DB.Exec(updateQuery, time.Now().UTC(), cartID)
	if err != nil {
		log.Printf("❌ Failed to update cart timestamp: %v", err)
	}

	resp := h.buildCartWithItemsResponse(cart, []cartLineItemResp{})
	writeJSON(w, http.StatusOK, resp)
}

// HandleAddLineItem adds a new line item to the cart.
// POST /api/cart/{cartId}/line-items
func (h *CartHandler) HandleAddLineItem(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")

	if cartID == "" {
		writeError(w, http.StatusBadRequest, "cart ID is required")
		return
	}

	// Parse request body
	var req struct {
		ItemID    string `json:"itemId"`
		Quantity  int    `json:"quantity"`
		Modifiers []struct {
			ModifierID string `json:"modifierId"`
		} `json:"modifiers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ItemID == "" || req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "itemId and quantity (> 0) are required")
		return
	}

	// Check if cart exists
	_, err := h.getCartByID(cartID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Cart not found")
			return
		}
		log.Printf("❌ Failed to get cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get cart")
		return
	}

	// Create line item
	lineItem := &db.CartLineItem{
		ID:        uuid.NewString(),
		CartID:    cartID,
		ItemID:    req.ItemID,
		Quantity:  req.Quantity,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	query := `
		INSERT INTO cart_line_items (id, cart_id, item_id, quantity, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err = h.DB.Exec(query, lineItem.ID, lineItem.CartID, lineItem.ItemID, lineItem.Quantity, lineItem.CreatedAt, lineItem.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to create line item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to add line item")
		return
	}

	// Add modifiers if any
	for _, mod := range req.Modifiers {
		modID := uuid.NewString()
		modQuery := `
			INSERT INTO cart_line_item_modifiers (id, cart_line_item_id, modifier_id)
			VALUES (?, ?, ?)
		`
		_, err = h.DB.Exec(modQuery, modID, lineItem.ID, mod.ModifierID)
		if err != nil {
			log.Printf("❌ Failed to add modifier: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to add modifier")
			return
		}
	}

	// Update cart's updated_at timestamp
	updateQuery := `UPDATE carts SET updated_at = ? WHERE id = ?`
	_, err = h.DB.Exec(updateQuery, time.Now().UTC(), cartID)
	if err != nil {
		log.Printf("❌ Failed to update cart timestamp: %v", err)
	}

	// Fetch modifiers for response
	modifiers := []selectedModifierResp{}
	for _, mod := range req.Modifiers {
		modifiers = append(modifiers, selectedModifierResp{ModifierID: mod.ModifierID})
	}

	resp := cartLineItemResp{
		ID:        lineItem.ID,
		CartID:    lineItem.CartID,
		ItemID:    lineItem.ItemID,
		Quantity:  lineItem.Quantity,
		Modifiers: modifiers,
		CreatedAt: lineItem.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: lineItem.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, http.StatusCreated, resp)
}

// HandleUpdateLineItem updates the quantity or modifiers of a cart line item.
// PUT /api/cart/{cartId}/line-item/{lineItemId}
func (h *CartHandler) HandleUpdateLineItem(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")
	lineItemID := r.PathValue("lineItemId")

	if cartID == "" || lineItemID == "" {
		writeError(w, http.StatusBadRequest, "cart ID and line item ID are required")
		return
	}

	// Parse request body
	var req struct {
		Quantity  int `json:"quantity"`
		Modifiers []struct {
			ModifierID string `json:"modifierId"`
		} `json:"modifiers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "quantity must be greater than 0")
		return
	}

	// Check if line item exists and belongs to this cart
	lineItem, err := h.getLineItemByID(lineItemID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Line item not found")
			return
		}
		log.Printf("❌ Failed to get line item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get line item")
		return
	}

	if lineItem.CartID != cartID {
		writeError(w, http.StatusNotFound, "Line item not found")
		return
	}

	// Update line item quantity
	updateQuery := `UPDATE cart_line_items SET quantity = ?, updated_at = ? WHERE id = ?`
	_, err = h.DB.Exec(updateQuery, req.Quantity, time.Now().UTC(), lineItemID)
	if err != nil {
		log.Printf("❌ Failed to update line item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update line item")
		return
	}

	// Delete existing modifiers
	deleteModsQuery := `DELETE FROM cart_line_item_modifiers WHERE cart_line_item_id = ?`
	_, err = h.DB.Exec(deleteModsQuery, lineItemID)
	if err != nil {
		log.Printf("❌ Failed to delete old modifiers: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update modifiers")
		return
	}

	// Add new modifiers
	for _, mod := range req.Modifiers {
		modID := uuid.NewString()
		modQuery := `
			INSERT INTO cart_line_item_modifiers (id, cart_line_item_id, modifier_id)
			VALUES (?, ?, ?)
		`
		_, err = h.DB.Exec(modQuery, modID, lineItemID, mod.ModifierID)
		if err != nil {
			log.Printf("❌ Failed to add modifier: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to add modifier")
			return
		}
	}

	// Update cart's updated_at timestamp
	updateCartQuery := `UPDATE carts SET updated_at = ? WHERE id = ?`
	_, err = h.DB.Exec(updateCartQuery, time.Now().UTC(), cartID)
	if err != nil {
		log.Printf("❌ Failed to update cart timestamp: %v", err)
	}

	// Fetch updated line item
	lineItem, err = h.getLineItemByID(lineItemID)
	if err != nil {
		log.Printf("❌ Failed to get updated line item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get updated line item")
		return
	}

	// Fetch modifiers for response
	modifiers := []selectedModifierResp{}
	for _, mod := range req.Modifiers {
		modifiers = append(modifiers, selectedModifierResp{ModifierID: mod.ModifierID})
	}

	resp := cartLineItemResp{
		ID:        lineItem.ID,
		CartID:    lineItem.CartID,
		ItemID:    lineItem.ItemID,
		Quantity:  req.Quantity,
		Modifiers: modifiers,
		CreatedAt: lineItem.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleDeleteLineItem removes a line item from the cart.
// DELETE /api/cart/{cartId}/line-item/{lineItemId}
func (h *CartHandler) HandleDeleteLineItem(w http.ResponseWriter, r *http.Request) {
	cartID := r.PathValue("cartId")
	lineItemID := r.PathValue("lineItemId")

	if cartID == "" || lineItemID == "" {
		writeError(w, http.StatusBadRequest, "cart ID and line item ID are required")
		return
	}

	// Check if line item exists and belongs to this cart
	lineItem, err := h.getLineItemByID(lineItemID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Line item not found")
			return
		}
		log.Printf("❌ Failed to get line item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get line item")
		return
	}

	if lineItem.CartID != cartID {
		writeError(w, http.StatusNotFound, "Line item not found")
		return
	}

	// Delete line item (cascade will delete modifiers)
	deleteQuery := `DELETE FROM cart_line_items WHERE id = ?`
	_, err = h.DB.Exec(deleteQuery, lineItemID)
	if err != nil {
		log.Printf("❌ Failed to delete line item: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete line item")
		return
	}

	// Update cart's updated_at timestamp
	updateCartQuery := `UPDATE carts SET updated_at = ? WHERE id = ?`
	_, err = h.DB.Exec(updateCartQuery, time.Now().UTC(), cartID)
	if err != nil {
		log.Printf("❌ Failed to update cart timestamp: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// getCartByCustomerAndBusiness fetches a cart by customer ID and business ID.
// Returns the cart and its line items with modifiers.
func (h *CartHandler) getCartByCustomerAndBusiness(customerID, businessID string) (*db.Cart, []cartLineItemResp, error) {
	query := `
		SELECT id, customer_id, business_id, created_at, updated_at
		FROM carts
		WHERE customer_id = ? AND business_id = ?
	`
	var cart db.Cart
	err := h.DB.QueryRow(query, customerID, businessID).Scan(
		&cart.ID, &cart.CustomerID, &cart.BusinessID, &cart.CreatedAt, &cart.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, &notFoundError{message: "cart not found"}
	}
	if err != nil {
		return nil, nil, err
	}

	lineItems, err := h.getLineItemsByCartID(cart.ID)
	if err != nil {
		return nil, nil, err
	}

	return &cart, lineItems, nil
}

// getCartByID fetches a cart by ID.
func (h *CartHandler) getCartByID(cartID string) (*db.Cart, error) {
	query := `
		SELECT id, customer_id, business_id, created_at, updated_at
		FROM carts
		WHERE id = ?
	`
	var cart db.Cart
	err := h.DB.QueryRow(query, cartID).Scan(
		&cart.ID, &cart.CustomerID, &cart.BusinessID, &cart.CreatedAt, &cart.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, &notFoundError{message: "cart not found"}
	}
	if err != nil {
		return nil, err
	}

	return &cart, nil
}

// getLineItemsByCartID fetches all line items for a cart with their modifiers.
func (h *CartHandler) getLineItemsByCartID(cartID string) ([]cartLineItemResp, error) {
	query := `
		SELECT id, cart_id, item_id, quantity, created_at, updated_at
		FROM cart_line_items
		WHERE cart_id = ?
	`
	rows, err := h.DB.Query(query, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lineItems := []cartLineItemResp{}
	for rows.Next() {
		var item db.CartLineItem
		err := rows.Scan(&item.ID, &item.CartID, &item.ItemID, &item.Quantity, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Fetch modifiers for this line item
		modifiers, err := h.getModifiersByLineItemID(item.ID)
		if err != nil {
			return nil, err
		}

		lineItems = append(lineItems, cartLineItemResp{
			ID:        item.ID,
			CartID:    item.CartID,
			ItemID:    item.ItemID,
			Quantity:  item.Quantity,
			Modifiers: modifiers,
			CreatedAt: item.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: item.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return lineItems, nil
}

// getLineItemByID fetches a single line item by ID.
func (h *CartHandler) getLineItemByID(lineItemID string) (*db.CartLineItem, error) {
	query := `
		SELECT id, cart_id, item_id, quantity, created_at, updated_at
		FROM cart_line_items
		WHERE id = ?
	`
	var item db.CartLineItem
	err := h.DB.QueryRow(query, lineItemID).Scan(
		&item.ID, &item.CartID, &item.ItemID, &item.Quantity, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, &notFoundError{message: "line item not found"}
	}
	if err != nil {
		return nil, err
	}

	return &item, nil
}

// getModifiersByLineItemID fetches all modifiers for a cart line item.
func (h *CartHandler) getModifiersByLineItemID(lineItemID string) ([]selectedModifierResp, error) {
	query := `
		SELECT modifier_id
		FROM cart_line_item_modifiers
		WHERE cart_line_item_id = ?
	`
	rows, err := h.DB.Query(query, lineItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modifiers := []selectedModifierResp{}
	for rows.Next() {
		var modifierID string
		err := rows.Scan(&modifierID)
		if err != nil {
			return nil, err
		}
		modifiers = append(modifiers, selectedModifierResp{ModifierID: modifierID})
	}

	return modifiers, nil
}

// buildCartWithItemsResponse builds the API response for a cart with line items.
func (h *CartHandler) buildCartWithItemsResponse(cart *db.Cart, lineItems []cartLineItemResp) cartWithItemsResponse {
	return cartWithItemsResponse{
		ID:         cart.ID,
		CustomerID: cart.CustomerID,
		BusinessID: cart.BusinessID,
		CreatedAt:  cart.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  cart.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		LineItems:  lineItems,
	}
}


