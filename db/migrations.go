package db

import (
	"database/sql"
	"fmt"
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

	return nil
}
