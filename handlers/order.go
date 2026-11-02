package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pantheon/artemis/db"
)

// OrderHandler handles all order-related HTTP requests.
type OrderHandler struct {
	DB               *sql.DB
	SSEBroker        *SSEBroker
	PaymentProviders map[string]PaymentProvider
}

// NewOrderHandler creates a new order handler instance.
func NewOrderHandler(database *sql.DB) *OrderHandler {
	return &OrderHandler{
		DB:        database,
		SSEBroker: NewSSEBroker(),
		PaymentProviders: map[string]PaymentProvider{
			"stub": NewStubProvider(),
		},
	}
}

// orderWithItemsResponse is the API response shape for an order with nested line items.
type orderWithItemsResponse struct {
	ID                  string                `json:"id"`
	CustomerID          string                `json:"customerId"`
	BusinessID          string                `json:"businessId"`
	LocationID          string                `json:"locationId"`
	OrderType           string                `json:"orderType"`
	RequestedPickupTime *string               `json:"requestedPickupTime,omitempty"`
	Status              string                `json:"status"`
	SubtotalMinorUnits  int                   `json:"subtotalMinorUnits"`
	TaxMinorUnits       int                   `json:"taxMinorUnits"`
	TotalMinorUnits     int                   `json:"totalMinorUnits"`
	CurrencyCode        string                `json:"currencyCode"`
	PaymentProvider     string                `json:"paymentProvider"`
	PaymentIntentID     string                `json:"paymentIntentId"`
	PaymentStatus       string                `json:"paymentStatus"`
	CreatedAt           string                `json:"createdAt"`
	UpdatedAt           string                `json:"updatedAt"`
	LineItems           []orderLineItemResp   `json:"lineItems"`
}

// orderLineItemResp is the API response shape for an order line item.
type orderLineItemResp struct {
	ID                     string               `json:"id"`
	OrderID                string               `json:"orderId"`
	ItemID                 string               `json:"itemId"`
	ItemName               string               `json:"itemName"`
	ItemPriceMinorUnits    int                  `json:"itemPriceMinorUnits"`
	Quantity               int                  `json:"quantity"`
	Modifiers              []orderModifierResp  `json:"modifiers"`
	LineItemTotalMinorUnits int                 `json:"lineItemTotalMinorUnits"`
	CreatedAt              string               `json:"createdAt"`
}

// orderModifierResp is the API response shape for an order modifier.
type orderModifierResp struct {
	ModifierID                string `json:"modifierId"`
	ModifierName              string `json:"modifierName"`
	PriceAdjustmentMinorUnits int    `json:"priceAdjustmentMinorUnits"`
}

// HandleCreateOrder creates an order from a cart.
// POST /api/orders
func (h *OrderHandler) HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req struct {
		CartID              string  `json:"cartId"`
		LocationID          string  `json:"locationId"`
		OrderType           string  `json:"orderType"`
		RequestedPickupTime *string `json:"requestedPickupTime,omitempty"`
		PaymentProvider     string  `json:"paymentProvider"`
		PaymentIntentID     *string `json:"paymentIntentId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.CartID == "" || req.LocationID == "" || req.OrderType == "" || req.PaymentProvider == "" {
		writeError(w, http.StatusBadRequest, "cartId, locationId, orderType, and paymentProvider are required")
		return
	}

	if req.OrderType != "order-ahead" && req.OrderType != "asap" {
		writeError(w, http.StatusBadRequest, "orderType must be 'order-ahead' or 'asap'")
		return
	}

	if req.OrderType == "order-ahead" && req.RequestedPickupTime == nil {
		writeError(w, http.StatusBadRequest, "requestedPickupTime is required for order-ahead orders")
		return
	}

	// Get cart with line items
	cartHandler := NewCartHandler(h.DB)
	cart, err := cartHandler.getCartByID(req.CartID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Cart not found")
			return
		}
		log.Printf("❌ Failed to get cart: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get cart")
		return
	}

	// Get cart line items
	cartLineItems, err := cartHandler.getLineItemsByCartID(cart.ID)
	if err != nil {
		log.Printf("❌ Failed to get cart line items: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get cart line items")
		return
	}

	if len(cartLineItems) == 0 {
		writeError(w, http.StatusBadRequest, "Cart is empty")
		return
	}

	// Calculate order totals by fetching item prices and modifier adjustments
	subtotal := 0
	for _, lineItem := range cartLineItems {
		// Fetch item price from catalog
		var itemPrice int
		err := h.DB.QueryRow("SELECT price_minor_units FROM items WHERE id = ?", lineItem.ItemID).Scan(&itemPrice)
		if err != nil {
			log.Printf("❌ Failed to get item price: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to calculate order total")
			return
		}

		lineTotal := itemPrice

		// Add modifier adjustments
		for _, mod := range lineItem.Modifiers {
			var modPrice int
			err := h.DB.QueryRow("SELECT price_adjustment_minor_units FROM modifiers WHERE id = ?", mod.ModifierID).Scan(&modPrice)
			if err != nil {
				log.Printf("❌ Failed to get modifier price: %v", err)
				writeError(w, http.StatusInternalServerError, "Failed to calculate order total")
				return
			}
			lineTotal += modPrice
		}

		subtotal += lineTotal * lineItem.Quantity
	}

	// Calculate tax (simplified: 9% for now, should be configurable per location)
	tax := int(float64(subtotal) * 0.09)
	total := subtotal + tax

	// Get payment provider
	provider, err := GetPaymentProvider(req.PaymentProvider)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid payment provider: %v", err))
		return
	}

	// Create or use provided payment intent
	var paymentIntentID string
	if req.PaymentIntentID != nil && *req.PaymentIntentID != "" {
		paymentIntentID = *req.PaymentIntentID
	} else {
		metadata := map[string]string{
			"cart_id":     cart.ID,
			"customer_id": cart.CustomerID,
			"business_id": cart.BusinessID,
		}
		paymentIntentID, err = provider.CreatePaymentIntent(total, "USD", metadata)
		if err != nil {
			log.Printf("❌ Failed to create payment intent: %v", err)
			writeError(w, http.StatusBadRequest, "Payment failed")
			return
		}
	}

	// Capture payment
	err = provider.CapturePayment(paymentIntentID)
	if err != nil {
		log.Printf("❌ Failed to capture payment: %v", err)
		writeError(w, http.StatusBadRequest, "Payment failed")
		return
	}

	// Parse requested pickup time if provided
	var requestedPickupTime *time.Time
	if req.RequestedPickupTime != nil {
		t, err := time.Parse(time.RFC3339, *req.RequestedPickupTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid requestedPickupTime format, must be RFC3339")
			return
		}
		requestedPickupTime = &t
	}

	// Create order
	order := &db.Order{
		ID:                  uuid.NewString(),
		CustomerID:          cart.CustomerID,
		BusinessID:          cart.BusinessID,
		LocationID:          req.LocationID,
		OrderType:           req.OrderType,
		RequestedPickupTime: requestedPickupTime,
		Status:              "pending",
		SubtotalMinorUnits:  subtotal,
		TaxMinorUnits:       tax,
		TotalMinorUnits:     total,
		CurrencyCode:        "USD",
		PaymentProvider:     req.PaymentProvider,
		PaymentIntentID:     paymentIntentID,
		PaymentStatus:       "captured",
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	// Insert order
	orderQuery := `
		INSERT INTO orders (id, customer_id, business_id, location_id, order_type, requested_pickup_time, status,
			subtotal_minor_units, tax_minor_units, total_minor_units, currency_code,
			payment_provider, payment_intent_id, payment_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = h.DB.Exec(orderQuery, order.ID, order.CustomerID, order.BusinessID, order.LocationID,
		order.OrderType, order.RequestedPickupTime, order.Status, order.SubtotalMinorUnits,
		order.TaxMinorUnits, order.TotalMinorUnits, order.CurrencyCode, order.PaymentProvider,
		order.PaymentIntentID, order.PaymentStatus, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		log.Printf("❌ Failed to create order: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create order")
		return
	}

	// Create order line items (snapshot from cart)
	orderLineItems := []orderLineItemResp{}
	for _, cartLineItem := range cartLineItems {
		// Fetch item details from catalog
		var itemName string
		var itemPrice int
		err := h.DB.QueryRow("SELECT name, price_minor_units FROM items WHERE id = ?", cartLineItem.ItemID).Scan(&itemName, &itemPrice)
		if err != nil {
			log.Printf("❌ Failed to get item details: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to create order line items")
			return
		}

		lineTotal := itemPrice

		// Fetch and snapshot modifiers
		orderModifiers := []orderModifierResp{}
		for _, cartMod := range cartLineItem.Modifiers {
			var modName string
			var modPrice int
			err := h.DB.QueryRow("SELECT name, price_adjustment_minor_units FROM modifiers WHERE id = ?", cartMod.ModifierID).Scan(&modName, &modPrice)
			if err != nil {
				log.Printf("❌ Failed to get modifier details: %v", err)
				writeError(w, http.StatusInternalServerError, "Failed to create order line items")
				return
			}

			lineTotal += modPrice

			orderModifiers = append(orderModifiers, orderModifierResp{
				ModifierID:                cartMod.ModifierID,
				ModifierName:              modName,
				PriceAdjustmentMinorUnits: modPrice,
			})

			// Insert order line item modifier
			modID := uuid.NewString()
			modQuery := `
				INSERT INTO order_line_item_modifiers (id, order_line_item_id, modifier_id, modifier_name, price_adjustment_minor_units)
				VALUES (?, ?, ?, ?, ?)
			`
			// We'll need to insert this after we have the order line item ID
			defer func(modID, orderLineItemID, modifierID, modifierName string, priceAdjustment int) {
				_, err := h.DB.Exec(modQuery, modID, orderLineItemID, modifierID, modifierName, priceAdjustment)
				if err != nil {
					log.Printf("❌ Failed to insert order modifier: %v", err)
				}
			}(modID, "", cartMod.ModifierID, modName, modPrice)
		}

		lineTotalWithQty := lineTotal * cartLineItem.Quantity

		// Insert order line item
		orderLineItemID := uuid.NewString()
		lineItemQuery := `
			INSERT INTO order_line_items (id, order_id, item_id, item_name, item_price_minor_units, quantity, line_item_total_minor_units, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = h.DB.Exec(lineItemQuery, orderLineItemID, order.ID, cartLineItem.ItemID, itemName, itemPrice, cartLineItem.Quantity, lineTotalWithQty, time.Now().UTC())
		if err != nil {
			log.Printf("❌ Failed to insert order line item: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to create order line items")
			return
		}

		// Now insert the modifiers with the correct order_line_item_id
		for _, cartMod := range cartLineItem.Modifiers {
			var modName string
			var modPrice int
			err := h.DB.QueryRow("SELECT name, price_adjustment_minor_units FROM modifiers WHERE id = ?", cartMod.ModifierID).Scan(&modName, &modPrice)
			if err != nil {
				log.Printf("❌ Failed to get modifier details: %v", err)
				continue
			}

			modID := uuid.NewString()
			modQuery := `
				INSERT INTO order_line_item_modifiers (id, order_line_item_id, modifier_id, modifier_name, price_adjustment_minor_units)
				VALUES (?, ?, ?, ?, ?)
			`
			_, err = h.DB.Exec(modQuery, modID, orderLineItemID, cartMod.ModifierID, modName, modPrice)
			if err != nil {
				log.Printf("❌ Failed to insert order modifier: %v", err)
			}
		}

		orderLineItems = append(orderLineItems, orderLineItemResp{
			ID:                     orderLineItemID,
			OrderID:                order.ID,
			ItemID:                 cartLineItem.ItemID,
			ItemName:               itemName,
			ItemPriceMinorUnits:    itemPrice,
			Quantity:               cartLineItem.Quantity,
			Modifiers:              orderModifiers,
			LineItemTotalMinorUnits: lineTotalWithQty,
			CreatedAt:              time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	// Delete cart after order creation
	_, err = h.DB.Exec("DELETE FROM carts WHERE id = ?", cart.ID)
	if err != nil {
		log.Printf("⚠️ Failed to delete cart after order creation: %v", err)
		// Don't fail the request, just log the warning
	}

	// Build response
	resp := h.buildOrderWithItemsResponse(order, orderLineItems)

	// Broadcast new_order event to SSE clients
	h.broadcastOrderEvent(order.LocationID, "new_order", resp)

	writeJSON(w, http.StatusCreated, resp)
}

// HandleGetOrder returns an order by ID.
// GET /api/order/{orderId}
func (h *OrderHandler) HandleGetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")

	if orderID == "" {
		writeError(w, http.StatusBadRequest, "order ID is required")
		return
	}

	order, lineItems, err := h.getOrderByID(orderID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Order not found")
			return
		}
		log.Printf("❌ Failed to get order: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get order")
		return
	}

	resp := h.buildOrderWithItemsResponse(order, lineItems)
	writeJSON(w, http.StatusOK, resp)
}

// HandleListCustomerOrders lists all orders for a customer.
// GET /api/customer/{customerId}/orders
func (h *OrderHandler) HandleListCustomerOrders(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")

	if customerID == "" {
		writeError(w, http.StatusBadRequest, "customer ID is required")
		return
	}

	// Parse query parameters
	status := r.URL.Query().Get("status")
	businessID := r.URL.Query().Get("businessId")

	// Build query
	query := `
		SELECT id, customer_id, business_id, location_id, order_type, requested_pickup_time, status,
			subtotal_minor_units, tax_minor_units, total_minor_units, currency_code,
			payment_provider, payment_intent_id, payment_status, created_at, updated_at
		FROM orders
		WHERE customer_id = ?
	`
	args := []interface{}{customerID}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	if businessID != "" {
		query += " AND business_id = ?"
		args = append(args, businessID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		log.Printf("❌ Failed to query orders: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get orders")
		return
	}
	defer rows.Close()

	orders := []orderWithItemsResponse{}
	for rows.Next() {
		order, err := h.scanOrder(rows)
		if err != nil {
			log.Printf("❌ Failed to scan order: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to get orders")
			return
		}

		lineItems, err := h.getOrderLineItems(order.ID)
		if err != nil {
			log.Printf("❌ Failed to get order line items: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to get orders")
			return
		}

		orders = append(orders, h.buildOrderWithItemsResponse(order, lineItems))
	}

	if orders == nil {
		orders = []orderWithItemsResponse{}
	}

	writeJSON(w, http.StatusOK, orders)
}

// HandleListLocationOrders lists all orders for a location (kiosk view).
// GET /api/location/{locationId}/orders
func (h *OrderHandler) HandleListLocationOrders(w http.ResponseWriter, r *http.Request) {
	locationID := r.PathValue("locationId")

	if locationID == "" {
		writeError(w, http.StatusBadRequest, "location ID is required")
		return
	}

	// Parse query parameters
	status := r.URL.Query().Get("status")

	// Build query
	query := `
		SELECT id, customer_id, business_id, location_id, order_type, requested_pickup_time, status,
			subtotal_minor_units, tax_minor_units, total_minor_units, currency_code,
			payment_provider, payment_intent_id, payment_status, created_at, updated_at
		FROM orders
		WHERE location_id = ?
	`
	args := []interface{}{locationID}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		log.Printf("❌ Failed to query orders: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get orders")
		return
	}
	defer rows.Close()

	orders := []orderWithItemsResponse{}
	for rows.Next() {
		order, err := h.scanOrder(rows)
		if err != nil {
			log.Printf("❌ Failed to scan order: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to get orders")
			return
		}

		lineItems, err := h.getOrderLineItems(order.ID)
		if err != nil {
			log.Printf("❌ Failed to get order line items: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to get orders")
			return
		}

		orders = append(orders, h.buildOrderWithItemsResponse(order, lineItems))
	}

	if orders == nil {
		orders = []orderWithItemsResponse{}
	}

	writeJSON(w, http.StatusOK, orders)
}

// HandleCancelOrder cancels an order (customer action).
// PUT /api/order/{orderId}/cancel
func (h *OrderHandler) HandleCancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")

	if orderID == "" {
		writeError(w, http.StatusBadRequest, "order ID is required")
		return
	}

	// Get order
	order, lineItems, err := h.getOrderByID(orderID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Order not found")
			return
		}
		log.Printf("❌ Failed to get order: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get order")
		return
	}

	// Check if order can be cancelled (only pending or confirmed)
	if order.Status != "pending" && order.Status != "confirmed" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Cannot cancel order with status: %s", order.Status))
		return
	}

	// Refund payment if captured
	if order.PaymentStatus == "captured" {
		provider, err := GetPaymentProvider(order.PaymentProvider)
		if err != nil {
			log.Printf("❌ Failed to get payment provider: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to process refund")
			return
		}

		err = provider.RefundPayment(order.PaymentIntentID, order.TotalMinorUnits)
		if err != nil {
			log.Printf("❌ Failed to refund payment: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to process refund")
			return
		}

		order.PaymentStatus = "refunded"
	}

	// Update order status
	order.Status = "cancelled"
	order.UpdatedAt = time.Now().UTC()

	updateQuery := `UPDATE orders SET status = ?, payment_status = ?, updated_at = ? WHERE id = ?`
	_, err = h.DB.Exec(updateQuery, order.Status, order.PaymentStatus, order.UpdatedAt, order.ID)
	if err != nil {
		log.Printf("❌ Failed to update order status: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to cancel order")
		return
	}

	resp := h.buildOrderWithItemsResponse(order, lineItems)

	// Broadcast order_updated event to SSE clients
	h.broadcastOrderEvent(order.LocationID, "order_updated", resp)

	writeJSON(w, http.StatusOK, resp)
}

// HandleUpdateOrderStatus updates an order status (business action).
// PUT /api/order/{orderId}/status
func (h *OrderHandler) HandleUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")

	if orderID == "" {
		writeError(w, http.StatusBadRequest, "order ID is required")
		return
	}

	// Parse request body
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"confirmed": true,
		"preparing": true,
		"ready":     true,
		"completed": true,
		"cancelled": true,
	}
	if !validStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "Invalid status")
		return
	}

	// Get order
	order, lineItems, err := h.getOrderByID(orderID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "Order not found")
			return
		}
		log.Printf("❌ Failed to get order: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get order")
		return
	}

	// Validate status transition (simplified - could be more strict)
	if order.Status == "completed" {
		writeError(w, http.StatusBadRequest, "Cannot change status of completed order")
		return
	}

	// If cancelling and payment was captured, refund
	if req.Status == "cancelled" && order.PaymentStatus == "captured" {
		provider, err := GetPaymentProvider(order.PaymentProvider)
		if err != nil {
			log.Printf("❌ Failed to get payment provider: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to process refund")
			return
		}

		err = provider.RefundPayment(order.PaymentIntentID, order.TotalMinorUnits)
		if err != nil {
			log.Printf("❌ Failed to refund payment: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to process refund")
			return
		}

		order.PaymentStatus = "refunded"
	}

	// Update order status
	order.Status = req.Status
	order.UpdatedAt = time.Now().UTC()

	updateQuery := `UPDATE orders SET status = ?, payment_status = ?, updated_at = ? WHERE id = ?`
	_, err = h.DB.Exec(updateQuery, order.Status, order.PaymentStatus, order.UpdatedAt, order.ID)
	if err != nil {
		log.Printf("❌ Failed to update order status: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update order status")
		return
	}

	resp := h.buildOrderWithItemsResponse(order, lineItems)

	// Broadcast order_updated event to SSE clients
	h.broadcastOrderEvent(order.LocationID, "order_updated", resp)

	writeJSON(w, http.StatusOK, resp)
}

// HandleOrderEventsSSE streams order events to kiosks via Server-Sent Events.
// GET /api/location/{locationId}/orders/events
func (h *OrderHandler) HandleOrderEventsSSE(w http.ResponseWriter, r *http.Request) {
	locationID := r.PathValue("locationId")

	if locationID == "" {
		writeError(w, http.StatusBadRequest, "location ID is required")
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a new client channel
	clientChan := make(chan SSEEvent, 10)
	h.SSEBroker.AddClient(locationID, clientChan)
	defer h.SSEBroker.RemoveClient(locationID, clientChan)

	// Get Last-Event-ID header for reconnection
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID != "" {
		// Replay missed events from the last 10 minutes
		h.replayMissedEvents(w, locationID, lastEventID)
	}

	// Keep connection alive and send events
	for {
		select {
		case event := <-clientChan:
			// Format SSE event
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "id: %s\n", event.ID)
			fmt.Fprintf(w, "data: %s\n\n", event.Data)
			w.(http.Flusher).Flush()

		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}

// Helper methods

// getOrderByID fetches an order and its line items by order ID.
func (h *OrderHandler) getOrderByID(orderID string) (*db.Order, []orderLineItemResp, error) {
	query := `
		SELECT id, customer_id, business_id, location_id, order_type, requested_pickup_time, status,
			subtotal_minor_units, tax_minor_units, total_minor_units, currency_code,
			payment_provider, payment_intent_id, payment_status, created_at, updated_at
		FROM orders
		WHERE id = ?
	`
	row := h.DB.QueryRow(query, orderID)
	order, err := h.scanOrder(row)
	if err == sql.ErrNoRows {
		return nil, nil, &notFoundError{message: "order not found"}
	}
	if err != nil {
		return nil, nil, err
	}

	lineItems, err := h.getOrderLineItems(order.ID)
	if err != nil {
		return nil, nil, err
	}

	return order, lineItems, nil
}

// scanOrder scans an order from a database row.
func (h *OrderHandler) scanOrder(row interface {
	Scan(dest ...interface{}) error
}) (*db.Order, error) {
	var order db.Order
	var requestedPickupTime sql.NullTime

	err := row.Scan(
		&order.ID, &order.CustomerID, &order.BusinessID, &order.LocationID,
		&order.OrderType, &requestedPickupTime, &order.Status,
		&order.SubtotalMinorUnits, &order.TaxMinorUnits, &order.TotalMinorUnits,
		&order.CurrencyCode, &order.PaymentProvider, &order.PaymentIntentID,
		&order.PaymentStatus, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if requestedPickupTime.Valid {
		order.RequestedPickupTime = &requestedPickupTime.Time
	}

	return &order, nil
}

// getOrderLineItems fetches all line items for an order with modifiers.
func (h *OrderHandler) getOrderLineItems(orderID string) ([]orderLineItemResp, error) {
	query := `
		SELECT id, order_id, item_id, item_name, item_price_minor_units, quantity, line_item_total_minor_units, created_at
		FROM order_line_items
		WHERE order_id = ?
	`
	rows, err := h.DB.Query(query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lineItems := []orderLineItemResp{}
	for rows.Next() {
		var item db.OrderLineItem
		err := rows.Scan(&item.ID, &item.OrderID, &item.ItemID, &item.ItemName,
			&item.ItemPriceMinorUnits, &item.Quantity, &item.LineItemTotalMinorUnits, &item.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Fetch modifiers
		modifiers, err := h.getOrderLineItemModifiers(item.ID)
		if err != nil {
			return nil, err
		}

		lineItems = append(lineItems, orderLineItemResp{
			ID:                     item.ID,
			OrderID:                item.OrderID,
			ItemID:                 item.ItemID,
			ItemName:               item.ItemName,
			ItemPriceMinorUnits:    item.ItemPriceMinorUnits,
			Quantity:               item.Quantity,
			Modifiers:              modifiers,
			LineItemTotalMinorUnits: item.LineItemTotalMinorUnits,
			CreatedAt:              item.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return lineItems, nil
}

// getOrderLineItemModifiers fetches all modifiers for an order line item.
func (h *OrderHandler) getOrderLineItemModifiers(orderLineItemID string) ([]orderModifierResp, error) {
	query := `
		SELECT modifier_id, modifier_name, price_adjustment_minor_units
		FROM order_line_item_modifiers
		WHERE order_line_item_id = ?
	`
	rows, err := h.DB.Query(query, orderLineItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modifiers := []orderModifierResp{}
	for rows.Next() {
		var mod db.OrderModifier
		err := rows.Scan(&mod.ModifierID, &mod.ModifierName, &mod.PriceAdjustmentMinorUnits)
		if err != nil {
			return nil, err
		}
		modifiers = append(modifiers, orderModifierResp{
			ModifierID:                mod.ModifierID,
			ModifierName:              mod.ModifierName,
			PriceAdjustmentMinorUnits: mod.PriceAdjustmentMinorUnits,
		})
	}

	return modifiers, nil
}

// buildOrderWithItemsResponse builds the API response for an order with line items.
func (h *OrderHandler) buildOrderWithItemsResponse(order *db.Order, lineItems []orderLineItemResp) orderWithItemsResponse {
	var requestedPickupTime *string
	if order.RequestedPickupTime != nil {
		t := order.RequestedPickupTime.Format("2006-01-02T15:04:05Z")
		requestedPickupTime = &t
	}

	return orderWithItemsResponse{
		ID:                  order.ID,
		CustomerID:          order.CustomerID,
		BusinessID:          order.BusinessID,
		LocationID:          order.LocationID,
		OrderType:           order.OrderType,
		RequestedPickupTime: requestedPickupTime,
		Status:              order.Status,
		SubtotalMinorUnits:  order.SubtotalMinorUnits,
		TaxMinorUnits:       order.TaxMinorUnits,
		TotalMinorUnits:     order.TotalMinorUnits,
		CurrencyCode:        order.CurrencyCode,
		PaymentProvider:     order.PaymentProvider,
		PaymentIntentID:     order.PaymentIntentID,
		PaymentStatus:       order.PaymentStatus,
		CreatedAt:           order.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:           order.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		LineItems:           lineItems,
	}
}

// broadcastOrderEvent broadcasts an order event to all SSE clients for a location.
func (h *OrderHandler) broadcastOrderEvent(locationID, eventType string, order orderWithItemsResponse) {
	// Create event data
	eventData := map[string]interface{}{
		"eventType": eventType,
		"order":     order,
		"timestamp": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	dataJSON, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("❌ Failed to marshal event data: %v", err)
		return
	}

	// Store event in database for replay
	eventID := h.storeOrderEvent(locationID, eventType, order.ID, string(dataJSON))

	// Broadcast to SSE clients
	event := SSEEvent{
		ID:   eventID,
		Type: eventType,
		Data: string(dataJSON),
	}
	h.SSEBroker.Broadcast(locationID, event)
}

// storeOrderEvent stores an order event in the database for SSE replay.
// Returns the event ID.
func (h *OrderHandler) storeOrderEvent(locationID, eventType, orderID, eventData string) string {
	query := `
		INSERT INTO order_events (location_id, event_type, order_id, event_data, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := h.DB.Exec(query, locationID, eventType, orderID, eventData, time.Now().UTC())
	if err != nil {
		log.Printf("❌ Failed to store order event: %v", err)
		return "0"
	}

	id, _ := result.LastInsertId()
	return fmt.Sprintf("%d", id)
}

// replayMissedEvents replays events from the last 10 minutes for a reconnecting client.
func (h *OrderHandler) replayMissedEvents(w http.ResponseWriter, locationID, lastEventID string) {
	query := `
		SELECT id, event_type, event_data
		FROM order_events
		WHERE location_id = ? AND id > ? AND created_at > datetime('now', '-10 minutes')
		ORDER BY id ASC
	`
	rows, err := h.DB.Query(query, locationID, lastEventID)
	if err != nil {
		log.Printf("❌ Failed to query missed events: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var eventType, eventData string
		err := rows.Scan(&id, &eventType, &eventData)
		if err != nil {
			log.Printf("❌ Failed to scan event: %v", err)
			continue
		}

		// Send event to client
		fmt.Fprintf(w, "event: %s\n", eventType)
		fmt.Fprintf(w, "id: %d\n", id)
		fmt.Fprintf(w, "data: %s\n\n", eventData)
		w.(http.Flusher).Flush()
	}
}

// SSEBroker manages SSE client connections for real-time order updates.
type SSEBroker struct {
	clients map[string][]chan SSEEvent
	mu      sync.RWMutex
}

// SSEEvent represents a server-sent event.
type SSEEvent struct {
	ID   string
	Type string
	Data string
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[string][]chan SSEEvent),
	}
}

// AddClient adds a new SSE client for a location.
func (b *SSEBroker) AddClient(locationID string, clientChan chan SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.clients[locationID] == nil {
		b.clients[locationID] = []chan SSEEvent{}
	}
	b.clients[locationID] = append(b.clients[locationID], clientChan)
}

// RemoveClient removes an SSE client for a location.
func (b *SSEBroker) RemoveClient(locationID string, clientChan chan SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	clients := b.clients[locationID]
	for i, ch := range clients {
		if ch == clientChan {
			b.clients[locationID] = append(clients[:i], clients[i+1:]...)
			close(clientChan)
			break
		}
	}

	if len(b.clients[locationID]) == 0 {
		delete(b.clients, locationID)
	}
}

// Broadcast sends an event to all clients for a location.
func (b *SSEBroker) Broadcast(locationID string, event SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	clients := b.clients[locationID]
	for _, clientChan := range clients {
		select {
		case clientChan <- event:
		default:
			// Client channel is full, skip
		}
	}
}
