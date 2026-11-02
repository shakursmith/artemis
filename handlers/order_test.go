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

// setupTestOrderHandler creates an OrderHandler backed by an in-memory SQLite DB.
// Returns the handler and the underlying *sql.DB for seeding test data.
func setupTestOrderHandler(t *testing.T) (*OrderHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewOrderHandler(database), database
}

// =============================================================================
// POST /api/orders — Create Order from Cart
// =============================================================================

func TestCreateOrder_ASAPOrder(t *testing.T) {
	h, database := setupTestOrderHandler(t)
	cartHandler := NewCartHandler(database)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Quinn")
	business, _ := db.CreateBusiness(database, "Test Burger Joint", nil)
	location, _ := db.CreateLocation(database, business.ID, "Main Street", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Lunch Menu", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Burgers", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "Classic Burger", nil, 995, "USD", nil, nil, true, false, 0)

	// Create cart and add item
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	cartHandler.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 1, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	cartHandler.HandleAddLineItem(w2, req2)

	// Create order
	orderBody := `{
		"cartId": "` + cart.ID + `",
		"locationId": "` + location.ID + `",
		"orderType": "asap",
		"paymentProvider": "stub"
	}`
	req3 := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(orderBody))
	w3 := httptest.NewRecorder()

	h.HandleCreateOrder(w3, req3)

	if w3.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w3.Code, w3.Body.String())
	}

	var order orderWithItemsResponse
	json.NewDecoder(w3.Body).Decode(&order)

	if order.CustomerID != customer.ID {
		t.Errorf("expected customerId %s, got %s", customer.ID, order.CustomerID)
	}
	if order.BusinessID != business.ID {
		t.Errorf("expected businessId %s, got %s", business.ID, order.BusinessID)
	}
	if order.LocationID != location.ID {
		t.Errorf("expected locationId %s, got %s", location.ID, order.LocationID)
	}
	if order.OrderType != "asap" {
		t.Errorf("expected orderType 'asap', got %s", order.OrderType)
	}
	if order.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", order.Status)
	}
	if order.PaymentStatus != "captured" {
		t.Errorf("expected paymentStatus 'captured', got %s", order.PaymentStatus)
	}
	if len(order.LineItems) != 1 {
		t.Errorf("expected 1 line item, got %d", len(order.LineItems))
	}
}

func TestCreateOrder_OrderAhead(t *testing.T) {
	h, database := setupTestOrderHandler(t)
	cartHandler := NewCartHandler(database)

	// Seed test data
	customer, _ := db.CreateCustomer(database, "Riley")
	business, _ := db.CreateBusiness(database, "Test Sushi Bar", nil)
	location, _ := db.CreateLocation(database, business.ID, "Downtown", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Dinner", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Rolls", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "California Roll", nil, 1295, "USD", nil, nil, true, false, 0)

	// Create cart and add item
	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	cartHandler.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 2, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	cartHandler.HandleAddLineItem(w2, req2)

	// Create order-ahead order
	orderBody := `{
		"cartId": "` + cart.ID + `",
		"locationId": "` + location.ID + `",
		"orderType": "order-ahead",
		"requestedPickupTime": "2026-11-01T19:30:00Z",
		"paymentProvider": "stub"
	}`
	req3 := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(orderBody))
	w3 := httptest.NewRecorder()

	h.HandleCreateOrder(w3, req3)

	if w3.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w3.Code, w3.Body.String())
	}

	var order orderWithItemsResponse
	json.NewDecoder(w3.Body).Decode(&order)

	if order.OrderType != "order-ahead" {
		t.Errorf("expected orderType 'order-ahead', got %s", order.OrderType)
	}
	if order.RequestedPickupTime == nil {
		t.Error("expected requestedPickupTime to be set")
	}
}

// =============================================================================
// GET /api/order/{orderId} — Get Order
// =============================================================================

func TestGetOrder_Success(t *testing.T) {
	h, database := setupTestOrderHandler(t)
	cartHandler := NewCartHandler(database)

	// Seed test data and create order
	customer, _ := db.CreateCustomer(database, "Sam")
	business, _ := db.CreateBusiness(database, "Test Taco Stand", nil)
	location, _ := db.CreateLocation(database, business.ID, "Food Court", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Menu", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Tacos", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "Fish Taco", nil, 495, "USD", nil, nil, true, false, 0)

	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	cartHandler.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 3, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	cartHandler.HandleAddLineItem(w2, req2)

	orderBody := `{
		"cartId": "` + cart.ID + `",
		"locationId": "` + location.ID + `",
		"orderType": "asap",
		"paymentProvider": "stub"
	}`
	req3 := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(orderBody))
	w3 := httptest.NewRecorder()
	h.HandleCreateOrder(w3, req3)

	var createdOrder orderWithItemsResponse
	json.NewDecoder(w3.Body).Decode(&createdOrder)

	// Get order
	req4 := httptest.NewRequest(http.MethodGet, "/api/order/"+createdOrder.ID, nil)
	req4.SetPathValue("orderId", createdOrder.ID)
	w4 := httptest.NewRecorder()

	h.HandleGetOrder(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w4.Code, w4.Body.String())
	}

	var fetchedOrder orderWithItemsResponse
	json.NewDecoder(w4.Body).Decode(&fetchedOrder)

	if fetchedOrder.ID != createdOrder.ID {
		t.Errorf("expected order ID %s, got %s", createdOrder.ID, fetchedOrder.ID)
	}
}

// =============================================================================
// PUT /api/order/{orderId}/status — Update Order Status
// =============================================================================

func TestUpdateOrderStatus_Success(t *testing.T) {
	h, database := setupTestOrderHandler(t)
	cartHandler := NewCartHandler(database)

	// Seed test data and create order
	customer, _ := db.CreateCustomer(database, "Jamie")
	business, _ := db.CreateBusiness(database, "Test Diner", nil)
	location, _ := db.CreateLocation(database, business.ID, "Highway 101", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Breakfast", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Pancakes", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "Buttermilk Stack", nil, 795, "USD", nil, nil, true, false, 0)

	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	cartHandler.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 1, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	cartHandler.HandleAddLineItem(w2, req2)

	orderBody := `{
		"cartId": "` + cart.ID + `",
		"locationId": "` + location.ID + `",
		"orderType": "asap",
		"paymentProvider": "stub"
	}`
	req3 := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(orderBody))
	w3 := httptest.NewRecorder()
	h.HandleCreateOrder(w3, req3)

	var order orderWithItemsResponse
	json.NewDecoder(w3.Body).Decode(&order)

	// Update order status to confirmed
	updateBody := `{"status": "confirmed"}`
	req4 := httptest.NewRequest(http.MethodPut, "/api/order/"+order.ID+"/status", bytes.NewBufferString(updateBody))
	req4.SetPathValue("orderId", order.ID)
	w4 := httptest.NewRecorder()

	h.HandleUpdateOrderStatus(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w4.Code, w4.Body.String())
	}

	var updatedOrder orderWithItemsResponse
	json.NewDecoder(w4.Body).Decode(&updatedOrder)

	if updatedOrder.Status != "confirmed" {
		t.Errorf("expected status 'confirmed', got %s", updatedOrder.Status)
	}
}

// =============================================================================
// PUT /api/order/{orderId}/cancel — Cancel Order
// =============================================================================

func TestCancelOrder_Success(t *testing.T) {
	h, database := setupTestOrderHandler(t)
	cartHandler := NewCartHandler(database)

	// Seed test data and create order
	customer, _ := db.CreateCustomer(database, "Drew")
	business, _ := db.CreateBusiness(database, "Test Food Truck", nil)
	location, _ := db.CreateLocation(database, business.ID, "Park Location", nil)
	menu, _ := db.CreateProductMenu(database, business.ID, "Street Food", nil, true, 0)
	category, _ := db.CreateCategory(database, menu.ID, "Sandwiches", nil, true, 0)
	item, _ := db.CreateItem(database, category.ID, "Pulled Pork", nil, 1095, "USD", nil, nil, true, false, 0)

	req1 := httptest.NewRequest(http.MethodPost, "/api/customer/"+customer.ID+"/business/"+business.ID+"/cart", nil)
	req1.SetPathValue("customerId", customer.ID)
	req1.SetPathValue("businessId", business.ID)
	w1 := httptest.NewRecorder()
	cartHandler.HandleCreateOrGetCart(w1, req1)

	var cart cartWithItemsResponse
	json.NewDecoder(w1.Body).Decode(&cart)

	addBody := `{"itemId": "` + item.ID + `", "quantity": 1, "modifiers": []}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/cart/"+cart.ID+"/line-items", bytes.NewBufferString(addBody))
	req2.SetPathValue("cartId", cart.ID)
	w2 := httptest.NewRecorder()
	cartHandler.HandleAddLineItem(w2, req2)

	orderBody := `{
		"cartId": "` + cart.ID + `",
		"locationId": "` + location.ID + `",
		"orderType": "asap",
		"paymentProvider": "stub"
	}`
	req3 := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(orderBody))
	w3 := httptest.NewRecorder()
	h.HandleCreateOrder(w3, req3)

	var order orderWithItemsResponse
	json.NewDecoder(w3.Body).Decode(&order)

	// Cancel order
	req4 := httptest.NewRequest(http.MethodPut, "/api/order/"+order.ID+"/cancel", nil)
	req4.SetPathValue("orderId", order.ID)
	w4 := httptest.NewRecorder()

	h.HandleCancelOrder(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w4.Code, w4.Body.String())
	}

	var cancelledOrder orderWithItemsResponse
	json.NewDecoder(w4.Body).Decode(&cancelledOrder)

	if cancelledOrder.Status != "cancelled" {
		t.Errorf("expected status 'cancelled', got %s", cancelledOrder.Status)
	}
	if cancelledOrder.PaymentStatus != "refunded" {
		t.Errorf("expected paymentStatus 'refunded', got %s", cancelledOrder.PaymentStatus)
	}
}
