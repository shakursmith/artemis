package db

import (
	"database/sql"
	"fmt"
	"log"

	// Import the SQLite driver — the underscore import registers the driver
	// with database/sql so we can use "sqlite3" as the driver name.
	_ "github.com/mattn/go-sqlite3"
)

// InitDB creates (or opens) a SQLite database at the given path, enables
// WAL mode for better concurrent read performance, turns on foreign key
// enforcement, and runs all schema migrations.
//
// Pass ":memory:" for an ephemeral in-memory database (useful for tests).
// Returns the *sql.DB connection ready for use.
func InitDB(dbPath string) (*sql.DB, error) {
	// Open the SQLite database (creates the file if it doesn't exist)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	// Verify the connection is actually working
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL (Write-Ahead Logging) mode for better concurrent read performance.
	// WAL allows readers and writers to operate simultaneously without blocking each other.
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign key enforcement — SQLite has this OFF by default,
	// so we must explicitly enable it for CASCADE deletes etc. to work.
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Run schema migrations to create tables if they don't exist
	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("🗄️  Database initialized at %s", dbPath)
	return db, nil
}
