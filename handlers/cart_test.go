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

// setupTestCartHandler creates a CartHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestCartHandler(t *testing.T) (*CartHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewCartHandler(database), database
}

// =============================================================================
// POST /api/customer/{customerId}/business/{businessId}/cart — Create/Get Cart
// =============================================================================

func TestCreateOrGetCart_NewCart(t *testing.T) {
	h, database := setupTestCartHandler(t)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Shakur")
	business, _ := db.CreateBusiness(database, "Test Coffee Shop", nil)

	// Create cart
	req := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req.SetPathValue("customerId", customer.ID)
	req.SetPathValue("businessId", business.ID)
	w := httptest.NewRecorder()

	h.HandleCreateOrGetCart(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp cartWithItemsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.CustomerID != customer.ID {
		t.Errorf("expected customerId %s, got %s", customer.ID, resp.CustomerID)
	}
	if resp.BusinessID != business.ID {
		t.Errorf("expected businessId %s, got %s", business.ID, resp.BusinessID)
	}
	if len(resp.LineItems) != 0 {
		t.Errorf("expected empty line items, got %d", len(resp.LineItems))
	}
}

func TestCreateOrGetCart_ExistingCart(t *testing.T) {
	h, database := setupTestCartHandler(t)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Jordan")
	business, _ := db.CreateBusiness(database, "Test Bakery", nil)

	// Create cart first
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	h.HandleCreateOrGetCart(w1, req1)

	var cart1 cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart1)

	// Request cart again (should return existing)
	req2 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req2.SetPathValue("customerId", customer.ID)
	req2.SetPathValue("businessId", business.ID)
	w2 := httptest.NewRecorder()
	h.HandleCreateOrGetCart(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var cart2 cartWithItemsResponse
	json.NewDecoder(w2.Body).Decode(&cart2)

	if cart1.ID != cart2.ID {
		t.Errorf("expected same cart ID, got %s and %s", cart1.ID, cart2.ID)
	}
}

// =============================================================================
// POST /api/cart/{cartId}/line-items — Add Line Item
// =============================================================================

func TestAddLineItem_Success(t *testing.T) {
	h, database := setupTestCartHandler(t)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Alex")
	business, _ := db.CreateBusiness(database, "Test Deli", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Lunch Menu", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Sandwiches", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "BLT", nil, 895, "USD", nil, nil, true, false, 0)

	// Create cart
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	h.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	// Add line item
	body := `{
		"itemId": "` + item.ID + `",
		"quantity": 2,
		"modifiers": []
	}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(body))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()

	h.HandleAddLineItem(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w2.Code, w2.Body.String())
	}

	var lineItem cartLineItemResp
	json.NewDecoder(w2.Body).Decode(&lineItem)

	if lineItem.ItemID != item.ID {
		t.Errorf("expected itemId %s, got %s", item.ID, lineItem.ItemID)
	}
	if lineItem.Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", lineItem.Quantity)
	}
}

// =============================================================================
// PUT /api/cart/{cartId}/line-item/{lineItemId} — Update Line Item
// =============================================================================

func TestUpdateLineItem_Success(t *testing.T) {
	h, database := setupTestCartHandler(t)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Taylor")
	business, _ := db.CreateBusiness(database, "Test Cafe", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Breakfast", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Beverages", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "Coffee", nil, 395, "USD", nil, nil, true, false, 0)

	// Create cart and add line item
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	h.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 1, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	h.HandleAddLineItem(w2, req2)

	var lineItem cartLineItemResp
	json.NewDecoder(w2.Body).Decode(&lineItem)

	// Update line item quantity
	updateBody := `{"quantity": 3, "modifiers": []}`
	req3 := httptest.NewRequest(http.MethodPut, "/api/cart/"+cart.ID+"/line-item/"+lineItem.ID, bytes.NewBufferString(updateBody))
	req3.SetPathValue("cartId", cart.ID)
	req3.SetPathValue("lineItemId", lineItem.ID)
	w3 := httptest.NewRecorder()

	h.HandleUpdateLineItem(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var updated cartLineItemResp
	json.NewDecoder(w3.Body).Decode(&updated)

	if updated.Quantity != 3 {
		t.Errorf("expected quantity 3, got %d", updated.Quantity)
	}
}

// =============================================================================
// DELETE /api/cart/{cartId}/line-item/{lineItemId} — Remove Line Item
// =============================================================================

func TestDeleteLineItem_Success(t *testing.T) {
	h, database := setupTestCartHandler(t)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Morgan")
	business, _ := db.CreateBusiness(database, "Test Pizzeria", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Dinner", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Pizza", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "Margherita", nil, 1295, "USD", nil, nil, true, false, 0)

	// Create cart and add line item
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	h.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 1, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	h.HandleAddLineItem(w2, req2)

	var lineItem cartLineItemResp
	json.NewDecoder(w2.Body).Decode(&lineItem)

	// Delete line item
	req3 := httptest.NewRequest(http.MethodDelete, "/api/cart/"+cart.ID+"/line-item/"+lineItem.ID, nil)
	req3.SetPathValue("cartId", cart.ID)
	req3.SetPathValue("lineItemId", lineItem.ID)
	w3 := httptest.NewRecorder()

	h.HandleDeleteLineItem(w3, req3)

	if w3.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", w3.Code, w3.Body.String())
	}
}

// =============================================================================
// PUT /api/cart/{cartId}/clear — Clear Cart
// =============================================================================

func TestClearCart_Success(t *testing.T) {
	h, database := setupTestCartHandler(t)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Casey")
	business, _ := db.CreateBusiness(database, "Test Market", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Groceries", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Produce", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "Apples", nil, 399, "USD", nil, nil, true, false, 0)

	// Create cart and add line item
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	h.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 5, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	h.HandleAddLineItem(w2, req2)

	// Clear cart
	req3 := httptest.NewRequest(http.MethodPut, "/api/cart/"+cart.ID+"/clear", nil)
	req3.SetPathValue("cartId", cart.ID)
	w3 := httptest.NewRecorder()

	h.HandleClearCart(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var cleared cartWithItemsResponse
	json.NewDecoder(w3.Body).Decode(&cleared)

	if len(cleared.LineItems) != 0 {
		t.Errorf("expected empty line items after clear, got %d", len(cleared.LineItems))
	}
}
