package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// seedDevice holds the config for a device to be seeded into the database.
type seedDevice struct {
	ID         string
	Name       string
	DeviceType string
	RoomID     string // which room this device belongs to
}

// seedRoom holds the config for a room to be seeded, including its beacon config.
type seedRoom struct {
	ID          string
	Name        string
	Icon        string
	BeaconUUID  string
	BeaconMajor int
	BeaconMinor int
}

// DevSeedConfig defines the complete local development configuration.
// This captures Shakur's home setup so it can be recreated instantly
// after a fresh build instead of manually re-adding everything through the app.
//
// To update: just edit the structs below and restart with SEED_DEV_DATA=true.
var DevSeedConfig = struct {
	ProfileID   string
	ProfileName string
	Rooms       []seedRoom
	Devices     []seedDevice
}{
	// Fixed profile ID so the iOS app can store/reuse it across rebuilds
	ProfileID:   "f3417317-30ca-46b0-9ec4-19dc4d90509f",
	ProfileName: "Shakur",

	Rooms: []seedRoom{
		{
			ID:          "c377df21-5a7e-410d-b744-2488f686cd14",
			Name:        "Office",
			Icon:        "desktopcomputer",
			BeaconUUID:  "00000000-0000-0000-0000-000000000000",
			BeaconMajor: 1,
			BeaconMinor: 1,
		},
		{
			ID:          "11d48650-196a-494c-bb0b-932bf46c9373",
			Name:        "Bedroom",
			Icon:        "bed.double",
			BeaconUUID:  "00000000-0000-0000-0000-000000000000",
			BeaconMajor: 1,
			BeaconMinor: 2,
		},
		{
			ID:          "a6341cd0-72a5-494e-80e6-abe735ea40ef",
			Name:        "Living Room",
			Icon:        "sofa",
			BeaconUUID:  "00000000-0000-0000-0000-000000000000",
			BeaconMajor: 1,
			BeaconMinor: 3,
		},
	},

	Devices: []seedDevice{
		// Office devices
		{ID: "ff342a65-411e-4cc0-9c3f-b4aa12d3a2dc", Name: "Shakur's Desk", DeviceType: "govee_light", RoomID: "c377df21-5a7e-410d-b744-2488f686cd14"},
		{ID: "e56f0365-af78-4b02-9f80-ad3a73987463", Name: "SHAKUR's desk lamp", DeviceType: "govee_light", RoomID: "c377df21-5a7e-410d-b744-2488f686cd14"},
		{ID: "636618e1-0a19-470e-b22d-7645853cffc9", Name: "office lamp - bottom", DeviceType: "govee_light", RoomID: "c377df21-5a7e-410d-b744-2488f686cd14"},
		{ID: "d2069915-b2e2-4737-92bd-f569ed3ade55", Name: "office lamp - top", DeviceType: "govee_light", RoomID: "c377df21-5a7e-410d-b744-2488f686cd14"},
		{ID: "7ae7296b-988b-49e4-a67b-05da2a549ced", Name: "Najah Desk Light", DeviceType: "govee_light", RoomID: "c377df21-5a7e-410d-b744-2488f686cd14"},

		// Bedroom devices
		{ID: "5a5e6268-d5c8-4aa9-bef3-141d1e3639d3", Name: "Bedroom TV", DeviceType: "govee_light", RoomID: "11d48650-196a-494c-bb0b-932bf46c9373"},
		{ID: "060374e1-0de5-467d-b058-b06e6f8878f4", Name: "bedroom lamp 1/2", DeviceType: "govee_light", RoomID: "11d48650-196a-494c-bb0b-932bf46c9373"},
		{ID: "0809e05c-2f18-4667-a831-256cafd4665e", Name: "bedroom lamp 2/2", DeviceType: "govee_light", RoomID: "11d48650-196a-494c-bb0b-932bf46c9373"},

		// Living Room devices
		{ID: "6a5e5438-6c3d-41b0-ad54-b249d3a8a67c", Name: "living room ramp", DeviceType: "govee_light", RoomID: "a6341cd0-72a5-494e-80e6-abe735ea40ef"},
	},
}

// SeedDevData populates the database with the local development configuration.
// This is idempotent — if the profile already exists, it skips seeding entirely.
// Call this on startup when SEED_DEV_DATA=true to avoid manually re-creating
// your profile, rooms, devices, and beacon configs after every rebuild.
func SeedDevData(database *sql.DB) error {
	cfg := DevSeedConfig

	// Check if the dev profile already exists — if so, skip seeding
	var exists int
	err := database.QueryRow("SELECT COUNT(*) FROM profiles WHERE id = ?", cfg.ProfileID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check for existing seed data: %w", err)
	}
	if exists > 0 {
		log.Printf("🌱 Dev seed data already present (profile %s exists), skipping", cfg.ProfileID)
		return nil
	}

	// Use a transaction so everything succeeds or nothing does
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin seed transaction: %w", err)
	}
	defer tx.Rollback() // no-op if committed

	now := time.Now().UTC()

	// 1. Create the profile
	_, err = tx.Exec(
		"INSERT INTO profiles (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		cfg.ProfileID, cfg.ProfileName, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to seed profile: %w", err)
	}
	log.Printf("🌱 Seeded profile: %s (%s)", cfg.ProfileName, cfg.ProfileID)

	// 2. Create rooms with beacon configs
	for _, room := range cfg.Rooms {
		_, err = tx.Exec(
			"INSERT INTO rooms (id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			room.ID, cfg.ProfileID, room.Name, room.Icon,
			room.BeaconUUID, room.BeaconMajor, room.BeaconMinor,
			now, now,
		)
		if err != nil {
			return fmt.Errorf("failed to seed room %s: %w", room.Name, err)
		}
		log.Printf("🌱 Seeded room: %s (beacon minor=%d)", room.Name, room.BeaconMinor)
	}

	// 3. Create devices and assign them to rooms
	for _, device := range cfg.Devices {
		_, err = tx.Exec(
			"INSERT INTO devices (id, profile_id, room_id, name, device_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			device.ID, cfg.ProfileID, device.RoomID, device.Name, device.DeviceType,
			now, now,
		)
		if err != nil {
			return fmt.Errorf("failed to seed device %s: %w", device.Name, err)
		}
	}
	log.Printf("🌱 Seeded %d devices across %d rooms", len(cfg.Devices), len(cfg.Rooms))

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit seed transaction: %w", err)
	}

	log.Printf("🌱 Dev seed data loaded successfully!")
	return nil
}
