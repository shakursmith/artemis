package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pantheon/artemis/db"
)

// =============================================================================
// Test Helpers (prefixed with "sc" to avoid collisions with reviews_test.go)
// =============================================================================

// setupTestShopConfigHandler creates an in-memory database with migrations,
// returns a ShopConfigHandler and the raw database connection for inserting
// test data directly.
func setupTestShopConfigHandler(t *testing.T) (*ShopConfigHandler, *sql.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewShopConfigHandler(database), database
}

// scBusinessOpts holds optional fields for creating test businesses.
type scBusinessOpts struct {
	accentColor  *string
	heroImageURL *string
	phone        *string
	website      *string
	featuresJSON *string
	extrasJSON   *string
}

func scWithAccentColor(c string) func(*scBusinessOpts) {
	return func(o *scBusinessOpts) { o.accentColor = &c }
}
func scWithHeroImageURL(u string) func(*scBusinessOpts) {
	return func(o *scBusinessOpts) { o.heroImageURL = &u }
}
func scWithPhone(p string) func(*scBusinessOpts) {
	return func(o *scBusinessOpts) { o.phone = &p }
}
func scWithWebsite(w string) func(*scBusinessOpts) {
	return func(o *scBusinessOpts) { o.website = &w }
}
func scWithFeaturesJSON(f string) func(*scBusinessOpts) {
	return func(o *scBusinessOpts) { o.featuresJSON = &f }
}
func scWithExtrasJSON(e string) func(*scBusinessOpts) {
	return func(o *scBusinessOpts) { o.extrasJSON = &e }
}

// scCreateBusiness inserts a business row with branding and feature config.
// Returns the business ID.
func scCreateBusiness(t *testing.T, database *sql.DB, name string, opts ...func(*scBusinessOpts)) string {
	t.Helper()
	o := &scBusinessOpts{}
	for _, fn := range opts {
		fn(o)
	}
	b, err := db.CreateBusiness(database, name, nil)
	if err != nil {
		t.Fatalf("Failed to create business: %v", err)
	}
	// Update branding and feature fields via raw SQL
	now := time.Now().UTC()
	_, err = database.Exec(
		"UPDATE businesses SET accent_color = ?, hero_image_url = ?, phone = ?, website = ?, features_json = ?, extras_json = ?, updated_at = ? WHERE id = ?",
		o.accentColor, o.heroImageURL, o.phone, o.website, o.featuresJSON, o.extrasJSON, now, b.ID,
	)
	if err != nil {
		t.Fatalf("Failed to update business: %v", err)
	}
	return b.ID
}

// scLocationOpts holds optional fields for creating test locations.
type scLocationOpts struct {
	street, street2, city, state, postalCode, country *string
	latitude, longitude                               *float64
	timezone, openingHours, specialHours              *string
	orderModesJSON                                    *string
}

func scWithAddress(street, city, state, postalCode, country string) func(*scLocationOpts) {
	return func(o *scLocationOpts) {
		o.street = &street
		o.city = &city
		o.state = &state
		o.postalCode = &postalCode
		o.country = &country
	}
}
func scWithTimezone(tz string) func(*scLocationOpts) {
	return func(o *scLocationOpts) { o.timezone = &tz }
}
func scWithOpeningHours(h string) func(*scLocationOpts) {
	return func(o *scLocationOpts) { o.openingHours = &h }
}
func scWithOrderModesJSON(m string) func(*scLocationOpts) {
	return func(o *scLocationOpts) { o.orderModesJSON = &m }
}

// scCreateLocation inserts a location with structured address and hours.
// Returns the location ID.
func scCreateLocation(t *testing.T, database *sql.DB, businessID, name string, opts ...func(*scLocationOpts)) string {
	t.Helper()
	o := &scLocationOpts{}
	for _, fn := range opts {
		fn(o)
	}
	loc, err := db.CreateLocation(database, businessID, name, nil)
	if err != nil {
		t.Fatalf("Failed to create location: %v", err)
	}
	now := time.Now().UTC()
	_, err = database.Exec(
		`UPDATE locations SET street = ?, street2 = ?, city = ?, state = ?, postal_code = ?, country = ?,
		latitude = ?, longitude = ?, timezone = ?, opening_hours = ?, special_hours = ?, order_modes_json = ?, updated_at = ?
		WHERE id = ?`,
		o.street, o.street2, o.city, o.state, o.postalCode, o.country,
		o.latitude, o.longitude, o.timezone, o.openingHours, o.specialHours, o.orderModesJSON, now, loc.ID,
	)
	if err != nil {
		t.Fatalf("Failed to update location: %v", err)
	}
	return loc.ID
}

// scCreateReview inserts a review directly into the database.
func scCreateReview(t *testing.T, database *sql.DB, customerID, businessID string, rating int) {
	t.Helper()
	now := time.Now().UTC()
	_, err := database.Exec(
		"INSERT INTO reviews (id, customer_id, business_id, rating, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		fmt.Sprintf("review-%d-%d", rating, now.UnixNano()), customerID, businessID, rating, now, now,
	)
	if err != nil {
		t.Fatalf("Failed to create review: %v", err)
	}
}

// scCreateLoyaltyProgram inserts a loyalty program directly into the database.
func scCreateLoyaltyProgram(t *testing.T, database *sql.DB, businessID string, rewardThreshold int) string {
	t.Helper()
	id := fmt.Sprintf("lp-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	_, err := database.Exec(
		"INSERT INTO loyalty_programs (id, business_id, name, points_multiplier, reward_threshold, reward_description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, businessID, "Test Rewards", 1.0, rewardThreshold, "Free item", now, now,
	)
	if err != nil {
		t.Fatalf("Failed to create loyalty program: %v", err)
	}
	return id
}

// scCreateLoyaltyMembership inserts a loyalty membership directly.
func scCreateLoyaltyMembership(t *testing.T, database *sql.DB, customerID, businessID string, points int) {
	t.Helper()
	id := fmt.Sprintf("lm-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	_, err := database.Exec(
		"INSERT INTO loyalty_memberships (id, customer_id, business_id, points_balance, joined_at, last_activity_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, customerID, businessID, points, now, now,
	)
	if err != nil {
		t.Fatalf("Failed to create loyalty membership: %v", err)
	}
}

// scCreateCustomer inserts a customer directly into the database.
func scCreateCustomer(t *testing.T, database *sql.DB, firstName string) string {
	t.Helper()
	c, err := db.CreateCustomer(database, firstName)
	if err != nil {
		t.Fatalf("Failed to create customer: %v", err)
	}
	return c.ID
}

// scBuildOpenHours builds opening hours JSON where the location is open all day
// on every day of the week. Used to guarantee isOpen=true in tests.
func scBuildOpenHours() string {
	return `{
		"monday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"tuesday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"wednesday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"thursday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"friday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"saturday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"sunday": {"ranges": [{"open": "00:00", "close": "23:59"}]}
	}`
}

// scBuildClosedHours builds opening hours JSON where the location is closed
// all day on every day of the week (empty ranges = closed).
func scBuildClosedHours() string {
	return `{
		"monday": {"ranges": []},
		"tuesday": {"ranges": []},
		"wednesday": {"ranges": []},
		"thursday": {"ranges": []},
		"friday": {"ranges": []},
		"saturday": {"ranges": []},
		"sunday": {"ranges": []}
	}`
}

// =============================================================================
// Tests
// =============================================================================

// TestGetShopConfig_FullyConfigured tests a shop with all data present:
// branding, address, reviews, loyalty program, and customer membership.
func TestGetShopConfig_FullyConfigured(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	// Set up a fully configured business with branding and extras
	businessID := scCreateBusiness(t, database, "Brew & Bytes Coffee",
		scWithAccentColor("#8B4513"),
		scWithHeroImageURL("https://cdn.example.com/hero.jpg"),
		scWithPhone("+14155551234"),
		scWithWebsite("https://brewandbytes.com"),
		scWithFeaturesJSON(`{"menuEnabled":true,"rewardsEnabled":true,"reviewsEnabled":true,"cartEnabled":true}`),
		scWithExtrasJSON(`[{"type":"wifi","label":"Free WiFi","enabled":true},{"type":"parking","label":"Street Parking","enabled":true}]`),
	)

	// Create location with address, timezone, and hours that make it currently open
	openHours := scBuildOpenHours()
	locationID := scCreateLocation(t, database, businessID, "Downtown",
		scWithAddress("123 Main St", "San Francisco", "CA", "94102", "US"),
		scWithTimezone("America/Los_Angeles"),
		scWithOpeningHours(openHours),
		scWithOrderModesJSON(`["pickup","dineIn"]`),
	)

	// Add some reviews (5 + 4 + 4 = avg 4.3)
	customerID := scCreateCustomer(t, database, "Alice")
	scCreateReview(t, database, customerID, businessID, 5)
	scCreateReview(t, database, customerID, businessID, 4)
	scCreateReview(t, database, customerID, businessID, 4)

	// Set up loyalty program and membership
	scCreateLoyaltyProgram(t, database, businessID, 10)
	scCreateLoyaltyMembership(t, database, customerID, businessID, 7)

	// Make the request with customerId to get loyalty stamps
	req := httptest.NewRequest(http.MethodGet,
		"/api/shop-config/"+locationID+"?customerId="+customerID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var config db.ShopConfig
	if err := json.NewDecoder(w.Body).Decode(&config); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify identity fields
	if config.ShopID != locationID {
		t.Errorf("expected shopId %s, got %s", locationID, config.ShopID)
	}
	if config.BusinessID != businessID {
		t.Errorf("expected businessId %s, got %s", businessID, config.BusinessID)
	}
	if config.BusinessName != "Brew & Bytes Coffee" {
		t.Errorf("expected businessName 'Brew & Bytes Coffee', got '%s'", config.BusinessName)
	}

	// Verify branding
	if config.AccentColor == nil || *config.AccentColor != "#8B4513" {
		t.Errorf("expected accentColor '#8B4513', got %v", config.AccentColor)
	}
	if config.HeroImageURL == nil || *config.HeroImageURL != "https://cdn.example.com/hero.jpg" {
		t.Errorf("expected heroImageUrl set, got %v", config.HeroImageURL)
	}

	// Verify open status — location should be open because we set 24h ranges
	if !config.IsOpen {
		t.Error("expected isOpen to be true")
	}
	if config.ClosesAt == nil {
		t.Error("expected closesAt to be non-nil when location is open")
	}

	// Verify address
	if config.Address == nil {
		t.Fatal("expected address to be non-nil")
	}
	if config.Address.Street != "123 Main St" {
		t.Errorf("expected street '123 Main St', got '%s'", config.Address.Street)
	}
	if config.Address.City != "San Francisco" {
		t.Errorf("expected city 'San Francisco', got '%s'", config.Address.City)
	}

	// Verify contact
	if config.Phone == nil || *config.Phone != "+14155551234" {
		t.Errorf("expected phone '+14155551234', got %v", config.Phone)
	}
	if config.Website == nil || *config.Website != "https://brewandbytes.com" {
		t.Errorf("expected website set, got %v", config.Website)
	}

	// Verify opening hours are present
	if config.OpeningHours == nil {
		t.Error("expected openingHours to be non-nil")
	}

	// Verify order modes
	if len(config.OrderModes) != 2 || config.OrderModes[0] != "pickup" || config.OrderModes[1] != "dineIn" {
		t.Errorf("expected orderModes [pickup, dineIn], got %v", config.OrderModes)
	}

	// Verify features
	if !config.Features.MenuEnabled {
		t.Error("expected menuEnabled to be true")
	}
	if !config.Features.RewardsEnabled {
		t.Error("expected rewardsEnabled to be true")
	}

	// Verify extras
	if len(config.Extras) != 2 {
		t.Fatalf("expected 2 extras, got %d", len(config.Extras))
	}
	if config.Extras[0].Type != "wifi" {
		t.Errorf("expected first extra type 'wifi', got '%s'", config.Extras[0].Type)
	}

	// Verify review summary
	if config.ReviewSummary == nil {
		t.Fatal("expected reviewSummary to be non-nil")
	}
	if config.ReviewSummary.TotalReviews != 3 {
		t.Errorf("expected 3 total reviews, got %d", config.ReviewSummary.TotalReviews)
	}
	// Average of 5+4+4 = 13/3 = 4.3333... rounded to 4.3
	if config.ReviewSummary.AverageRating < 4.2 || config.ReviewSummary.AverageRating > 4.4 {
		t.Errorf("expected averageRating ~4.3, got %f", config.ReviewSummary.AverageRating)
	}

	// Verify loyalty summary with customer stamps
	if config.LoyaltySummary == nil {
		t.Fatal("expected loyaltySummary to be non-nil")
	}
	if !config.LoyaltySummary.ProgramEnabled {
		t.Error("expected programEnabled to be true")
	}
	if config.LoyaltySummary.StampsRequired == nil || *config.LoyaltySummary.StampsRequired != 10 {
		t.Errorf("expected stampsRequired 10, got %v", config.LoyaltySummary.StampsRequired)
	}
	if config.LoyaltySummary.CurrentStamps == nil || *config.LoyaltySummary.CurrentStamps != 7 {
		t.Errorf("expected currentStamps 7, got %v", config.LoyaltySummary.CurrentStamps)
	}
}

// TestGetShopConfig_MinimalDefaults tests a bare-minimum shop with no reviews,
// no loyalty, and default feature flags and extras.
func TestGetShopConfig_MinimalDefaults(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	// Create a bare-minimum business (no branding, no extras set)
	businessID := scCreateBusiness(t, database, "Corner Taco Stand")

	// Create location with minimal config — relies on column defaults
	locationID := scCreateLocation(t, database, businessID, "Main Location",
		scWithAddress("456 Oak Ave", "Austin", "TX", "78701", "US"),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/shop-config/"+locationID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var config db.ShopConfig
	if err := json.NewDecoder(w.Body).Decode(&config); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Identity
	if config.ShopID != locationID {
		t.Errorf("expected shopId %s, got %s", locationID, config.ShopID)
	}
	if config.BusinessName != "Corner Taco Stand" {
		t.Errorf("expected businessName 'Corner Taco Stand', got '%s'", config.BusinessName)
	}

	// Branding should be null
	if config.AccentColor != nil {
		t.Errorf("expected null accentColor, got %v", config.AccentColor)
	}
	if config.HeroImageURL != nil {
		t.Errorf("expected null heroImageUrl, got %v", config.HeroImageURL)
	}

	// Features should default to all-enabled
	if !config.Features.MenuEnabled || !config.Features.RewardsEnabled || !config.Features.ReviewsEnabled || !config.Features.CartEnabled {
		t.Errorf("expected all features enabled by default, got %+v", config.Features)
	}

	// Extras should default to empty array
	if len(config.Extras) != 0 {
		t.Errorf("expected empty extras array, got %v", config.Extras)
	}

	// Order modes should default to ["pickup", "dineIn"]
	if len(config.OrderModes) != 2 {
		t.Errorf("expected 2 default order modes, got %v", config.OrderModes)
	}

	// No reviews or loyalty
	if config.ReviewSummary != nil {
		t.Errorf("expected null reviewSummary, got %v", config.ReviewSummary)
	}
	if config.LoyaltySummary != nil {
		t.Errorf("expected null loyaltySummary, got %v", config.LoyaltySummary)
	}
}

// TestGetShopConfig_LocationOpen tests that isOpen and closesAt are computed
// correctly when the location is currently open.
func TestGetShopConfig_LocationOpen(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	businessID := scCreateBusiness(t, database, "Open Shop")
	openHours := scBuildOpenHours()
	locationID := scCreateLocation(t, database, businessID, "Always Open",
		scWithAddress("1 Test St", "Test City", "TS", "00000", "US"),
		scWithTimezone("UTC"),
		scWithOpeningHours(openHours),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/shop-config/"+locationID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var config db.ShopConfig
	json.NewDecoder(w.Body).Decode(&config)

	if !config.IsOpen {
		t.Error("expected isOpen to be true for a 24-hour location")
	}
	if config.ClosesAt == nil {
		t.Error("expected closesAt to be non-nil when location is open")
	} else if *config.ClosesAt != "11:59 PM" {
		t.Errorf("expected closesAt '11:59 PM', got '%s'", *config.ClosesAt)
	}
}

// TestGetShopConfig_LocationClosed tests that isOpen=false and closesAt=null
// when the location is currently closed.
func TestGetShopConfig_LocationClosed(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	businessID := scCreateBusiness(t, database, "Closed Shop")
	closedHours := scBuildClosedHours()
	locationID := scCreateLocation(t, database, businessID, "Always Closed",
		scWithAddress("2 Test St", "Test City", "TS", "00000", "US"),
		scWithTimezone("UTC"),
		scWithOpeningHours(closedHours),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/shop-config/"+locationID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var config db.ShopConfig
	json.NewDecoder(w.Body).Decode(&config)

	if config.IsOpen {
		t.Error("expected isOpen to be false for a closed location")
	}
	if config.ClosesAt != nil {
		t.Errorf("expected closesAt to be null when closed, got '%s'", *config.ClosesAt)
	}
}

// TestGetShopConfig_LoyaltyStampsWithCustomerId tests that currentStamps
// is populated when the customerId query param is provided.
func TestGetShopConfig_LoyaltyStampsWithCustomerId(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	businessID := scCreateBusiness(t, database, "Stamp Shop")
	locationID := scCreateLocation(t, database, businessID, "Main",
		scWithAddress("3 Test St", "Test City", "TS", "00000", "US"),
	)
	customerID := scCreateCustomer(t, database, "Bob")
	scCreateLoyaltyProgram(t, database, businessID, 8)
	scCreateLoyaltyMembership(t, database, customerID, businessID, 5)

	req := httptest.NewRequest(http.MethodGet,
		"/api/shop-config/"+locationID+"?customerId="+customerID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var config db.ShopConfig
	json.NewDecoder(w.Body).Decode(&config)

	if config.LoyaltySummary == nil {
		t.Fatal("expected loyaltySummary to be non-nil")
	}
	if config.LoyaltySummary.CurrentStamps == nil || *config.LoyaltySummary.CurrentStamps != 5 {
		t.Errorf("expected currentStamps 5, got %v", config.LoyaltySummary.CurrentStamps)
	}
}

// TestGetShopConfig_LoyaltyStampsWithoutCustomerId tests that currentStamps
// is null when no customerId is provided, even with a loyalty program.
func TestGetShopConfig_LoyaltyStampsWithoutCustomerId(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	businessID := scCreateBusiness(t, database, "Stamp Shop 2")
	locationID := scCreateLocation(t, database, businessID, "Main",
		scWithAddress("4 Test St", "Test City", "TS", "00000", "US"),
	)
	scCreateLoyaltyProgram(t, database, businessID, 8)

	// No customerId query param
	req := httptest.NewRequest(http.MethodGet, "/api/shop-config/"+locationID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var config db.ShopConfig
	json.NewDecoder(w.Body).Decode(&config)

	if config.LoyaltySummary == nil {
		t.Fatal("expected loyaltySummary to be non-nil (program exists)")
	}
	if !config.LoyaltySummary.ProgramEnabled {
		t.Error("expected programEnabled to be true")
	}
	if config.LoyaltySummary.StampsRequired == nil || *config.LoyaltySummary.StampsRequired != 8 {
		t.Errorf("expected stampsRequired 8, got %v", config.LoyaltySummary.StampsRequired)
	}
	if config.LoyaltySummary.CurrentStamps != nil {
		t.Errorf("expected currentStamps to be null without customerId, got %v", config.LoyaltySummary.CurrentStamps)
	}
}

// TestGetShopConfig_NotFound tests that 404 is returned for an unknown locationId.
func TestGetShopConfig_NotFound(t *testing.T) {
	h, _ := setupTestShopConfigHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/shop-config/nonexistent-id", nil)
	req.SetPathValue("locationId", "nonexistent-id")
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "Location not found" {
		t.Errorf("expected error 'Location not found', got '%s'", errResp["error"])
	}
}

// TestGetShopConfig_CustomFeatures tests that custom feature flags are returned
// when explicitly set on the business.
func TestGetShopConfig_CustomFeatures(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	// Only menu is enabled, everything else disabled
	businessID := scCreateBusiness(t, database, "Menu Only Shop",
		scWithFeaturesJSON(`{"menuEnabled":true,"rewardsEnabled":false,"reviewsEnabled":false,"cartEnabled":false}`),
	)
	locationID := scCreateLocation(t, database, businessID, "Main",
		scWithAddress("5 Test St", "Test City", "TS", "00000", "US"),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/shop-config/"+locationID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var config db.ShopConfig
	json.NewDecoder(w.Body).Decode(&config)

	if !config.Features.MenuEnabled {
		t.Error("expected menuEnabled true")
	}
	if config.Features.RewardsEnabled {
		t.Error("expected rewardsEnabled false")
	}
	if config.Features.ReviewsEnabled {
		t.Error("expected reviewsEnabled false")
	}
	if config.Features.CartEnabled {
		t.Error("expected cartEnabled false")
	}
}

// TestGetShopConfig_ReviewSummary tests that the review summary accurately
// reflects the average and count from the reviews table.
func TestGetShopConfig_ReviewSummary(t *testing.T) {
	h, database := setupTestShopConfigHandler(t)

	businessID := scCreateBusiness(t, database, "Reviewed Shop")
	locationID := scCreateLocation(t, database, businessID, "Main",
		scWithAddress("6 Test St", "Test City", "TS", "00000", "US"),
	)

	customerID := scCreateCustomer(t, database, "Charlie")
	// Create reviews: 5, 3, 4 → average = 4.0
	scCreateReview(t, database, customerID, businessID, 5)
	scCreateReview(t, database, customerID, businessID, 3)
	scCreateReview(t, database, customerID, businessID, 4)

	req := httptest.NewRequest(http.MethodGet, "/api/shop-config/"+locationID, nil)
	req.SetPathValue("locationId", locationID)
	w := httptest.NewRecorder()

	h.HandleGetShopConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var config db.ShopConfig
	json.NewDecoder(w.Body).Decode(&config)

	if config.ReviewSummary == nil {
		t.Fatal("expected reviewSummary to be non-nil")
	}
	if config.ReviewSummary.TotalReviews != 3 {
		t.Errorf("expected 3 total reviews, got %d", config.ReviewSummary.TotalReviews)
	}
	if config.ReviewSummary.AverageRating != 4.0 {
		t.Errorf("expected average rating 4.0, got %f", config.ReviewSummary.AverageRating)
	}
}
