package handlers

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PaymentProvider defines the interface for payment processing.
// This abstraction allows swapping between different payment providers
// (Stripe, Square, stub) without changing the order creation logic.
//
// Future implementations will integrate with real payment APIs:
// - StripeProvider: uses Stripe Payment Intents API
// - SquareProvider: uses Square Payments API
//
// For now, StubProvider always succeeds for testing and development.
type PaymentProvider interface {
	// CreatePaymentIntent creates a payment intent and returns the intent ID.
	// In production, this calls the provider's API to authorize payment.
	// For the stub provider, this generates a fake intent ID.
	CreatePaymentIntent(amountMinorUnits int, currencyCode string, metadata map[string]string) (string, error)

	// CapturePayment captures a previously authorized payment.
	// In production, this moves money from the customer's account.
	// For the stub provider, this is a no-op that always succeeds.
	CapturePayment(paymentIntentID string) error

	// RefundPayment refunds a captured payment.
	// In production, this returns money to the customer's account.
	// For the stub provider, this is a no-op that always succeeds.
	RefundPayment(paymentIntentID string, amountMinorUnits int) error
}

// StubProvider is a stub payment provider that always succeeds.
// Used for testing and development before integrating real payment providers.
// All methods return success immediately without any external API calls.
type StubProvider struct{}

// NewStubProvider creates a new stub payment provider instance.
func NewStubProvider() *StubProvider {
	return &StubProvider{}
}

// CreatePaymentIntent generates a fake payment intent ID.
// Format: "pi_stub_<uuid>_<timestamp>"
// Always succeeds immediately.
func (p *StubProvider) CreatePaymentIntent(amountMinorUnits int, currencyCode string, metadata map[string]string) (string, error) {
	// Generate a fake payment intent ID that looks realistic
	intentID := fmt.Sprintf("pi_stub_%s_%d", uuid.NewString()[:8], time.Now().Unix())
	return intentID, nil
}

// CapturePayment simulates capturing a payment.
// Always succeeds immediately for any intent ID.
func (p *StubProvider) CapturePayment(paymentIntentID string) error {
	// Stub: always succeeds
	return nil
}

// RefundPayment simulates refunding a payment.
// Always succeeds immediately for any intent ID and amount.
func (p *StubProvider) RefundPayment(paymentIntentID string, amountMinorUnits int) error {
	// Stub: always succeeds
	return nil
}

// GetPaymentProvider returns the appropriate payment provider based on the provider name.
// Supported providers: "stub", "stripe" (future), "square" (future)
// Returns an error if the provider name is not recognized.
func GetPaymentProvider(providerName string) (PaymentProvider, error) {
	switch providerName {
	case "stub":
		return NewStubProvider(), nil
	case "stripe":
		// Future: return NewStripeProvider(apiKey), nil
		return nil, fmt.Errorf("stripe provider not implemented yet")
	case "square":
		// Future: return NewSquareProvider(apiKey), nil
		return nil, fmt.Errorf("square provider not implemented yet")
	default:
		return nil, fmt.Errorf("unknown payment provider: %s", providerName)
	}
}
