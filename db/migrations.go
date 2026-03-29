package db

import "database/sql"

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

	// rooms table — physical spaces belonging to a profile
	// beacon_uuid/major/minor are optional and used for BLE iBeacon proximity detection
	`CREATE TABLE IF NOT EXISTS rooms (
		id TEXT PRIMARY KEY,
		profile_id TEXT NOT NULL,
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
}

// RunMigrations executes all schema migrations against the given database connection.
// Safe to call on every startup since all statements use IF NOT EXISTS.
func RunMigrations(db *sql.DB) error {
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return err
		}
	}
	return nil
}
