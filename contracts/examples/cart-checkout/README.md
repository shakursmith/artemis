# Cart and Checkout Examples

This directory contains example requests and responses for the cart and checkout API.

## Cart Examples

### cart-with-line-items.json
Example of a cart with two line items. One item has a modifier (Large) and the other has no modifiers.

## Order Creation Requests

### create-order-ahead-request.json
Request to create an order-ahead order with a requested pickup time. This is for customers who want to place an order in advance and pick it up at a specific time.

### create-asap-order-request.json
Request to create an ASAP order for customers who are already at the business and want immediate fulfillment. Note that this request does not include a `requestedPickupTime` field.

## Order Status Examples

The following examples show an order progressing through each status in the lifecycle:

### order-pending.json
Order just created, awaiting business confirmation. Payment is already captured.

### order-confirmed.json
Business accepted the order and will fulfill it. Updated timestamp reflects when status changed.

### order-preparing.json
Business is actively preparing the order.

### order-ready.json
Order is ready for customer pickup.

### order-completed.json
Customer picked up the order. This is the final successful state.

### order-cancelled-with-refund.json
Order was cancelled after payment was captured. Payment status is "refunded" to indicate a full refund was issued.

## Kiosk (Business) Operations

### update-status-request.json
Example request for a business to update order status (e.g., from pending to confirmed).

## Server-Sent Events (SSE) Examples

### sse-new-order-event.json
Event payload sent to kiosk when a new order arrives. The event type is "new_order" and includes the complete order with line items.

### sse-order-updated-event.json
Event payload sent to kiosk when an existing order is updated (e.g., customer cancelled). The event type is "order_updated" and includes the updated order state.

## Status Transitions

Valid status transitions are:

- pending → confirmed (business accepts)
- pending → cancelled (business or customer cancels)
- confirmed → preparing (business starts preparation)
- confirmed → cancelled (business or customer cancels)
- preparing → ready (business marks ready)
- preparing → cancelled (business cancels)
- ready → completed (customer picks up)
- ready → cancelled (business cancels)

## Cancellation and Refund Rules

- **Customer can cancel:** pending, confirmed
- **Business can cancel:** any status except completed
- **Refund trigger:** Cancellation after payment status is "captured" changes payment status to "refunded"
