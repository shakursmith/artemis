package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// migrations is the ordered list of SQL statements to run when initializing the database.
// Each migration creates a table if it doesn't already exist, making it safe to run
// on every startup. Foreign key constraints enforce referential integrity:
//   - Deleting a profile cascades to delete all its rooms and devices
//   - Deleting a room sets room_id to NULL on its devices (unassigns them)
var migrations = []string{
	// profiles table — the top-level entity, one per user
	`CREATE TABLE IF NOT EXISTS profiles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,

	// rooms table — physical spaces belonging to a profile OR location
	// profile_id is nullable to support shop mode (rooms belong to location instead)
	// beacon_uuid/major/minor are optional and used for BLE iBeacon proximity detection
	`CREATE TABLE IF NOT EXISTS rooms (
		id TEXT PRIMARY KEY,
		profile_id TEXT,
		name TEXT NOT NULL,
		icon TEXT DEFAULT 'house',
		beacon_uuid TEXT,
		beacon_major INTEGER,
		beacon_minor INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
	);`,

	// devices table — smart devices registered by the user
	// room_id is nullable so devices can exist without being assigned to a room yet
	// device_type maps to the integration handler (govee_light, fire_tv, wyze_camera, generic)
	// external_id links to the third-party service's identifier for this device
	// metadata stores extra JSON data specific to the device type
	`CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		profile_id TEXT NOT NULL,
		room_id TEXT,
		name TEXT NOT NULL,
		device_type TEXT NOT NULL,
		external_id TEXT,
		model TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
		FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE SET NULL
	);`,

	// businesses table — represents a business entity for shop mode
	// This is the profile equivalent for commercial/shop mode.
	// A business can have multiple locations (e.g. chain stores).
	`CREATE TABLE IF NOT EXISTS businesses (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		owner_name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,

	// locations table — physical locations belonging to a business
	// For multi-location chains (e.g. "Downtown Store", "Airport Branch").
	// Each location can have multiple rooms with beacons.
	`CREATE TABLE IF NOT EXISTS locations (
		id TEXT PRIMARY KEY,
		business_id TEXT NOT NULL,
		name TEXT NOT NULL,
		address TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE
	);`,

	// room_triggers table — automated workflows triggered by beacon events
	// Enables actions like notifications, menu display, or device control
	// when users enter/exit/dwell in a room.
	`CREATE TABLE IF NOT EXISTS room_triggers (
		id TEXT PRIMARY KEY,
		room_id TEXT NOT NULL,
		trigger_type TEXT NOT NULL,
		dwell_seconds INTEGER,
		action_type TEXT NOT NULL,
		action_config TEXT NOT NULL,
		enabled BOOLEAN DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE
	);`,

	// menu_option_templates table — global template library for menu options
	// System templates (wifi, url) are seeded on first migration.
	// Businesses can also create custom templates that other businesses can copy.
	`CREATE TABLE IF NOT EXISTS menu_option_templates (
		id TEXT PRIMARY KEY,
		created_by_business_id TEXT,
		option_type TEXT NOT NULL,
		default_title TEXT NOT NULL,
		default_icon TEXT NOT NULL,
		data_schema TEXT NOT NULL,
		description TEXT NOT NULL,
		is_system BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (created_by_business_id) REFERENCES businesses(id) ON DELETE CASCADE
	);`,

	// menu_options table — business-specific menu options (copied from templates)
	// Contains actual data like WiFi credentials or menu URLs.
	// option_type is immutable after creation to prevent data schema mismatches.
	`CREATE TABLE IF NOT EXISTS menu_options (
		id TEXT PRIMARY KEY,
		business_id TEXT NOT NULL,
		template_id TEXT,
		option_type TEXT NOT NULL,
		title TEXT NOT NULL,
		icon TEXT NOT NULL,
		data TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE,
		FOREIGN KEY (template_id) REFERENCES menu_option_templates(id) ON DELETE SET NULL
	);`,

	// menus table — one menu per room
	// One-to-one relationship: each room has exactly one menu.
	// is_enabled controls whether the menu is shown when entering the room.
	// Appearance fields control the visual customization of the menu.
	`CREATE TABLE IF NOT EXISTS menus (
		id TEXT PRIMARY KEY,
		room_id TEXT NOT NULL UNIQUE,
		is_enabled BOOLEAN DEFAULT 1,
		menu_color TEXT DEFAULT '#000000',
		menu_opacity REAL DEFAULT 0.3,
		text_color TEXT DEFAULT '#FFFFFF',
		icon_color TEXT DEFAULT '#FF9500',
		collapsed_icon TEXT DEFAULT 'storefront',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE
	);`,

	// menu_option_assignments table — junction table with ordering
	// Manages the many-to-many relationship between menus and menu options.
	// UNIQUE constraints enforce one assignment per option per menu and prevent position collisions.
	`CREATE TABLE IF NOT EXISTS menu_option_assignments (
		id TEXT PRIMARY KEY,
		menu_id TEXT NOT NULL,
		menu_option_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (menu_id) REFERENCES menus(id) ON DELETE CASCADE,
		FOREIGN KEY (menu_option_id) REFERENCES menu_options(id) ON DELETE CASCADE,
		UNIQUE (menu_id, menu_option_id),
		UNIQUE (menu_id, position)
	);`,

	// =============================================================================
	// Product Catalog Tables (Food/Drink Menu System)
	// =============================================================================
	//
	// The product catalog is the food and drink menu system for shop mode.
	// This is COMPLETELY SEPARATE from the shop menu system (menu_options, menus, etc.)
	//
	// Shop menu = environment-based actions (WiFi, URLs, room controls)
	// Product catalog = browsing and ordering food/drink items
	//
	// CRITICAL: These two systems must NEVER share table names, model names, or route paths.
	// Product catalog uses: product_menus, categories, items, modifier_groups, modifiers
	// Shop menu uses: menu_options, menus, menu_option_assignments

	// product_menus table — food/drink menu collections (e.g., "Breakfast", "Lunch")
	// A business can have multiple product menus activated at different times/locations.
	// This is separate from the menus table (which is for shop menu system).
	`CREATE TABLE IF NOT EXISTS product_menus (
		id TEXT PRIMARY KEY,
		business_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		is_active BOOLEAN DEFAULT 1,
		display_order INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE
	);`,

	// categories table — groups of related items within a product menu
	// (e.g., "Appetizers", "Entrees", "Desserts")
	`CREATE TABLE IF NOT EXISTS categories (
		id TEXT PRIMARY KEY,
		menu_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		is_available BOOLEAN DEFAULT 1,
		display_order INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (menu_id) REFERENCES product_menus(id) ON DELETE CASCADE
	);`,

	// items table — specific products (food/drink) that customers can order
	// Prices are stored in minor currency units (cents for USD) to avoid floating point issues
	// image_urls and dietary_tags are JSON arrays stored as TEXT
	`CREATE TABLE IF NOT EXISTS items (
		id TEXT PRIMARY KEY,
		category_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		price_minor_units INTEGER NOT NULL,
		currency_code TEXT NOT NULL DEFAULT 'USD',
		image_urls TEXT,
		dietary_tags TEXT,
		is_available BOOLEAN DEFAULT 1,
		is_sold_out BOOLEAN DEFAULT 0,
		display_order INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
	);`,

	// modifier_groups table — sets of options for customizing items
	// (e.g., "Size", "Add-ons", "Preparation")
	// min_selections and max_selections define the validation rules for customer selections
	`CREATE TABLE IF NOT EXISTS modifier_groups (
		id TEXT PRIMARY KEY,
		item_id TEXT NOT NULL,
		name TEXT NOT NULL,
		min_selections INTEGER NOT NULL,
		max_selections INTEGER NOT NULL,
		is_available BOOLEAN DEFAULT 1,
		display_order INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE
	);`,

	// modifiers table — specific options within a modifier group
	// (e.g., "Small", "Medium", "Large" for a Size group)
	// price_adjustment_minor_units can be negative (e.g., discounts)
	// is_default marks which modifier is selected by default in the UI
	`CREATE TABLE IF NOT EXISTS modifiers (
		id TEXT PRIMARY KEY,
		modifier_group_id TEXT NOT NULL,
		name TEXT NOT NULL,
		price_adjustment_minor_units INTEGER NOT NULL,
		is_available BOOLEAN DEFAULT 1,
		is_default BOOLEAN DEFAULT 0,
		display_order INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (modifier_group_id) REFERENCES modifier_groups(id) ON DELETE CASCADE
	);`,

	// reviews table — customer reviews of businesses
	// Reviews are business-level (not product-level) as this fits food trucks,
	// market stalls, and small food businesses where the overall experience matters.
	// Business replies are embedded in the same table (one reply per review).
	// Flagged reviews remain visible but are marked for moderation (no auto-hiding yet).
	// photo_urls is stored as JSON array.
	// For now, customer_id is a UUID string (customers table doesn't exist yet).
	`CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		business_id TEXT NOT NULL,
		rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
		review_text TEXT,
		photo_urls TEXT,
		business_reply_text TEXT,
		business_reply_at DATETIME,
		business_replied_by TEXT,
		flagged BOOLEAN DEFAULT 0,
		flagged_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE
	);`,

	// =============================================================================
	// Customer and Arrival Tables
	// =============================================================================

	// customers table — represents customers who interact with businesses
	// Each customer has privacy settings that control what businesses can see
	`CREATE TABLE IF NOT EXISTS customers (
		id TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,

	// customer_sharing_settings table — privacy controls for customer data
	// One-to-one relationship with customers table.
	// sharing_level determines what data is visible: minimal (name only), standard (name + history + loyalty), full (all fields)
	// blocked_businesses is a JSON array of business UUIDs where sharing is completely disabled
	`CREATE TABLE IF NOT EXISTS customer_sharing_settings (
		customer_id TEXT PRIMARY KEY,
		sharing_level TEXT NOT NULL DEFAULT 'minimal',
		blocked_businesses TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE
	);`,

	// arrival_events table — records customer arrivals at business locations
	// Events are detected via BLE beacon proximity and broadcast to kiosks via SSE
	// state: entered (first detection), dwelling (heartbeat), exited (lost signal)
	// proximity: immediate (0-0.5m), near (0.5-3m), far (3-10m, triggers exit)
	`CREATE TABLE IF NOT EXISTS arrival_events (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		business_id TEXT NOT NULL,
		location_id TEXT NOT NULL,
		beacon_uuid TEXT NOT NULL,
		beacon_major INTEGER NOT NULL,
		beacon_minor INTEGER NOT NULL,
		state TEXT NOT NULL,
		proximity TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE,
		FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE CASCADE
	);`,

	// =============================================================================
	// Loyalty and Favorites Tables
	// =============================================================================

	// loyalty_programs table — simple points-based loyalty program configuration
	// A business has at most one loyalty program. Loyalty is opt-in (no program = loyalty disabled).
	// Points are awarded automatically when orders transition to "completed" status.
	// Redemption is future work - this version is configuration-only.
	// UNIQUE constraint on business_id ensures one program per business.
	`CREATE TABLE IF NOT EXISTS loyalty_programs (
		id TEXT PRIMARY KEY,
		business_id TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		points_multiplier REAL NOT NULL,
		reward_threshold INTEGER NOT NULL,
		reward_description TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE
	);`,

	// loyalty_memberships table — customer membership in a business's loyalty program
	// Memberships are auto-created when a customer places their first order at a business
	// with an active loyalty program. Points are awarded on order completion.
	// UNIQUE constraint on (customer_id, business_id) ensures one membership per customer per business.
	`CREATE TABLE IF NOT EXISTS loyalty_memberships (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		business_id TEXT NOT NULL,
		points_balance INTEGER NOT NULL DEFAULT 0,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_activity_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE,
		UNIQUE (customer_id, business_id)
	);`,

	// favorites table — customer's bookmarked menu items for quick reordering
	// Favorites include full item details (name, price, availability) when retrieved.
	// business_id is denormalized for filtering (favorites per business).
	// favorited_at is updated if the customer re-adds the same item (idempotent).
	// UNIQUE constraint on (customer_id, item_id) ensures one favorite per item per customer.
	`CREATE TABLE IF NOT EXISTS favorites (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		item_id TEXT NOT NULL,
		business_id TEXT NOT NULL,
		favorited_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE,
		FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE,
		FOREIGN KEY (business_id) REFERENCES businesses(id) ON DELETE CASCADE,
		UNIQUE (customer_id, item_id)
	);`,
}

// RunMigrations executes all schema migrations against the given database connection.
// Safe to call on every startup since all statements use IF NOT EXISTS.
// Also runs ALTER TABLE migrations for adding new columns to existing tables.
func RunMigrations(db *sql.DB) error {
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return err
		}
	}

	// Create indexes for reviews table to optimize common queries
	// These are idempotent — CREATE INDEX IF NOT EXISTS is safe to re-run
	indexMigrations := []string{
		`CREATE INDEX IF NOT EXISTS idx_reviews_business_id ON reviews(business_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_customer_id ON reviews(customer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_created_at ON reviews(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_rating ON reviews(rating)`,
	}
	for _, idx := range indexMigrations {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Additive column migrations — safe to re-run because we ignore
	// "duplicate column name" errors from SQLite.
	alterMigrations := []string{
		// custom_template stores user-customized room scene layouts as JSON text.
		// Added in Prompt 12 for the scene editor feature.
		`ALTER TABLE rooms ADD COLUMN custom_template TEXT`,

		// location_id enables rooms to belong to either a profile (home mode) OR
		// a location (shop mode). Added in Prompt 14A for business/location support.
		`ALTER TABLE rooms ADD COLUMN location_id TEXT REFERENCES locations(id) ON DELETE CASCADE`,

		// business_id enables devices to belong to either a profile (home mode) OR
		// a business (shop mode). Added in Prompt 14A for business/location support.
		`ALTER TABLE devices ADD COLUMN business_id TEXT REFERENCES businesses(id) ON DELETE CASCADE`,

		// Menu appearance columns — added in shop menu backend system implementation.
		// These control the visual customization of the shop menu.
		`ALTER TABLE menus ADD COLUMN menu_color TEXT DEFAULT '#000000'`,
		`ALTER TABLE menus ADD COLUMN menu_opacity REAL DEFAULT 0.3`,
		`ALTER TABLE menus ADD COLUMN text_color TEXT DEFAULT '#FFFFFF'`,
		`ALTER TABLE menus ADD COLUMN icon_color TEXT DEFAULT '#FF9500'`,
		`ALTER TABLE menus ADD COLUMN collapsed_icon TEXT DEFAULT 'storefront'`,

		// Business branding fields — added in Ticket 008 for store info contract.
		// These fields are business-wide branding shared across all locations.
		`ALTER TABLE businesses ADD COLUMN welcome_message TEXT`,
		`ALTER TABLE businesses ADD COLUMN website TEXT`,
		`ALTER TABLE businesses ADD COLUMN phone TEXT`,
		`ALTER TABLE businesses ADD COLUMN logo_url TEXT`,
		`ALTER TABLE businesses ADD COLUMN accent_color TEXT`,
		`ALTER TABLE businesses ADD COLUMN hero_image_url TEXT`,

		// Location structured address and hours fields — added in Ticket 008 for store info contract.
		// These fields enable geocoding, timezone-aware hours, and customer-facing store info.
		`ALTER TABLE locations ADD COLUMN street TEXT`,
		`ALTER TABLE locations ADD COLUMN street2 TEXT`,
		`ALTER TABLE locations ADD COLUMN city TEXT`,
		`ALTER TABLE locations ADD COLUMN state TEXT`,
		`ALTER TABLE locations ADD COLUMN postal_code TEXT`,
		`ALTER TABLE locations ADD COLUMN country TEXT`,
		`ALTER TABLE locations ADD COLUMN latitude REAL`,
		`ALTER TABLE locations ADD COLUMN longitude REAL`,
		`ALTER TABLE locations ADD COLUMN timezone TEXT`,
		`ALTER TABLE locations ADD COLUMN opening_hours TEXT`,
		`ALTER TABLE locations ADD COLUMN special_hours TEXT`,
	}
	for _, alter := range alterMigrations {
		_, err := db.Exec(alter)
		if err != nil {
			// SQLite returns "duplicate column name" if the column already exists.
			// This is expected on subsequent startups — safe to ignore.
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("failed to run alter migration: %w", err)
			}
		}
	}

	// Fix profile_id constraint in rooms table — this migration makes profile_id nullable
	// so that rooms can belong to either a profile (home mode) OR a location (shop mode).
	// Added in Prompt 14A after discovering the old schema had NOT NULL on profile_id.
	if err := migrateRoomsProfileIDNullable(db); err != nil {
		return fmt.Errorf("failed to migrate rooms.profile_id to nullable: %w", err)
	}

	// Fix room_triggers foreign key to point to correct rooms table.
	// This fixes the issue where the FK was pointing to "rooms_old" after the migration.
	if err := migrateRoomTriggersForeignKey(db); err != nil {
		return fmt.Errorf("failed to migrate room_triggers foreign key: %w", err)
	}

	// Seed system menu option templates (wifi, url) if they don't exist.
	// These templates are available to all businesses to copy and customize.
	if err := seedSystemTemplates(db); err != nil {
		return fmt.Errorf("failed to seed system templates: %w", err)
	}

	return nil
}

// migrateRoomsProfileIDNullable fixes the profile_id column to be nullable.
// This migration is idempotent — it checks if the fix is already applied before running.
func migrateRoomsProfileIDNullable(db *sql.DB) error {
	// Check if profile_id is already nullable by examining the schema
	var schema string
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='rooms'").Scan(&schema)
	if err != nil {
		return fmt.Errorf("failed to check rooms table schema: %w", err)
	}

	// If profile_id is already nullable (doesn't contain "profile_id TEXT NOT NULL"),
	// skip this migration.
	if !strings.Contains(schema, "profile_id TEXT NOT NULL") {
		return nil // Already migrated
	}

	log.Printf("🔄 Migrating rooms table to make profile_id nullable (shop mode support)...")

	// SQLite doesn't support ALTER COLUMN, so we need to recreate the table.
	// Step 1: Rename the old table
	_, err = db.Exec("ALTER TABLE rooms RENAME TO rooms_old")
	if err != nil {
		return fmt.Errorf("failed to rename old rooms table: %w", err)
	}

	// Step 2: Create new table with correct schema (profile_id nullable)
	_, err = db.Exec(`
		CREATE TABLE rooms (
			id TEXT PRIMARY KEY,
			profile_id TEXT,
			name TEXT NOT NULL,
			icon TEXT DEFAULT 'house',
			beacon_uuid TEXT,
			beacon_major INTEGER,
			beacon_minor INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			custom_template TEXT,
			location_id TEXT,
			FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
			FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new rooms table: %w", err)
	}

	// Step 3: Copy data from old table to new table
	_, err = db.Exec(`
		INSERT INTO rooms (id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, created_at, updated_at, custom_template, location_id)
		SELECT id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, created_at, updated_at, custom_template, location_id
		FROM rooms_old
	`)
	if err != nil {
		return fmt.Errorf("failed to copy data to new rooms table: %w", err)
	}

	// Step 4: Drop the old table
	_, err = db.Exec("DROP TABLE rooms_old")
	if err != nil {
		return fmt.Errorf("failed to drop old rooms table: %w", err)
	}

	log.Printf("✅ Rooms table migrated successfully — profile_id is now nullable")
	return nil
}

// migrateRoomTriggersForeignKey fixes the room_id foreign key to point to "rooms" instead of "rooms_old".
// This migration is idempotent — it checks if the fix is already applied before running.
func migrateRoomTriggersForeignKey(db *sql.DB) error {
	// Check if foreign key already points to correct table
	var schema string
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='room_triggers'").Scan(&schema)
	if err != nil {
		return fmt.Errorf("failed to check room_triggers table schema: %w", err)
	}

	// If FK already points to "rooms" (not "rooms_old"), skip this migration
	if !strings.Contains(schema, `"rooms_old"`) {
		return nil // Already migrated
	}

	log.Printf("🔄 Migrating room_triggers table to fix foreign key reference...")

	// SQLite doesn't support ALTER TABLE for foreign keys, so we need to recreate the table.
	// Step 1: Rename the old table
	_, err = db.Exec("ALTER TABLE room_triggers RENAME TO room_triggers_old")
	if err != nil {
		return fmt.Errorf("failed to rename old room_triggers table: %w", err)
	}

	// Step 2: Create new table with correct foreign key
	_, err = db.Exec(`
		CREATE TABLE room_triggers (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			dwell_seconds INTEGER,
			action_type TEXT NOT NULL,
			action_config TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new room_triggers table: %w", err)
	}

	// Step 3: Copy data from old table to new table
	_, err = db.Exec(`
		INSERT INTO room_triggers (id, room_id, trigger_type, dwell_seconds, action_type, action_config, enabled, created_at, updated_at)
		SELECT id, room_id, trigger_type, dwell_seconds, action_type, action_config, enabled, created_at, updated_at
		FROM room_triggers_old
	`)
	if err != nil {
		return fmt.Errorf("failed to copy data to new room_triggers table: %w", err)
	}

	// Step 4: Drop the old table
	_, err = db.Exec("DROP TABLE room_triggers_old")
	if err != nil {
		return fmt.Errorf("failed to drop old room_triggers table: %w", err)
	}

	log.Printf("✅ room_triggers table migrated successfully — foreign key now points to rooms table")
	return nil
}

// seedSystemTemplates inserts the built-in system templates for menu options.
// This is idempotent — it checks if templates already exist before inserting.
func seedSystemTemplates(db *sql.DB) error {
	// Check if system templates already exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM menu_option_templates WHERE is_system = 1").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for existing system templates: %w", err)
	}

	// If system templates already exist, skip seeding
	if count > 0 {
		return nil
	}

	log.Printf("🌱 Seeding system menu option templates...")

	// WiFi template — allows customers to join the business's WiFi network
	wifiSchema := `{"type":"object","properties":{"ssid":{"type":"string"},"password":{"type":"string"}},"required":["ssid","password"]}`
	_, err = db.Exec(`
		INSERT INTO menu_option_templates (id, created_by_business_id, option_type, default_title, default_icon, data_schema, description, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, "tpl_wifi_system", "wifi", "Join WiFi", "wifi", wifiSchema, "Allow customers to connect to your WiFi network")
	if err != nil {
		return fmt.Errorf("failed to seed wifi template: %w", err)
	}

	// URL template — allows businesses to link to menus, websites, social media, etc.
	urlSchema := `{"type":"object","properties":{"url":{"type":"string"},"title":{"type":"string"}},"required":["url"]}`
	_, err = db.Exec(`
		INSERT INTO menu_option_templates (id, created_by_business_id, option_type, default_title, default_icon, data_schema, description, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, "tpl_url_system", "url", "View Menu", "link", urlSchema, "Link to your menu, website, or social media")
	if err != nil {
		return fmt.Errorf("failed to seed url template: %w", err)
	}

	log.Printf("✅ System templates seeded successfully")
	return nil
}
