package db

import (
	"database/sql"
	"fmt"
	"time"

	"crypto/rand"
	"encoding/hex"
)

// generateUUID creates a random UUID v4 string.
// We roll our own here to avoid pulling in a UUID library for this single use case.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	// Set version (4) and variant (RFC 4122) bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// =============================================================================
// Profile Operations
// =============================================================================

// CreateProfile inserts a new profile with the given name and returns it.
// A UUID is auto-generated for the profile ID.
func CreateProfile(db *sql.DB, name string) (*Profile, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO profiles (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		id, name, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	return &Profile{
		ID:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetProfile retrieves a single profile by its ID.
// Returns nil and an error if not found.
func GetProfile(db *sql.DB, id string) (*Profile, error) {
	var p Profile
	err := db.QueryRow(
		"SELECT id, name, created_at, updated_at FROM profiles WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("profile not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}
	return &p, nil
}

// GetProfileByName finds a profile by its name.
// Useful during onboarding to check if a profile already exists.
func GetProfileByName(db *sql.DB, name string) (*Profile, error) {
	var p Profile
	err := db.QueryRow(
		"SELECT id, name, created_at, updated_at FROM profiles WHERE name = ?", name,
	).Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("profile not found with name: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile by name: %w", err)
	}
	return &p, nil
}

// ListProfiles returns all profiles in the database.
// Mainly useful for development and debugging.
func ListProfiles(db *sql.DB) ([]Profile, error) {
	rows, err := db.Query("SELECT id, name, created_at, updated_at FROM profiles ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}
	defer rows.Close()

	var profiles []Profile
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan profile row: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// UpdateProfile changes the name of an existing profile and bumps updated_at.
func UpdateProfile(db *sql.DB, id string, name string) (*Profile, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE profiles SET name = ?, updated_at = ? WHERE id = ?",
		name, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("profile not found: %s", id)
	}

	return GetProfile(db, id)
}

// DeleteProfile removes a profile and all associated rooms/devices (via CASCADE).
func DeleteProfile(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM profiles WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("profile not found: %s", id)
	}
	return nil
}

// =============================================================================
// Room Operations
// =============================================================================

// CreateRoom adds a new room under the given profile.
// Beacon configuration is not set here — use UpdateRoomBeacon for that.
func CreateRoom(db *sql.DB, profileID, name, icon string) (*Room, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO rooms (id, profile_id, name, icon, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, profileID, name, icon, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	return &Room{
		ID:        id,
		ProfileID: profileID,
		Name:      name,
		Icon:      icon,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetRoom retrieves a single room by its ID.
func GetRoom(db *sql.DB, id string) (*Room, error) {
	var r Room
	err := db.QueryRow(
		"SELECT id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, created_at, updated_at FROM rooms WHERE id = ?", id,
	).Scan(&r.ID, &r.ProfileID, &r.Name, &r.Icon, &r.BeaconUUID, &r.BeaconMajor, &r.BeaconMinor, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("room not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}
	return &r, nil
}

// ListRoomsByProfile returns all rooms belonging to a profile, ordered by creation time.
func ListRoomsByProfile(db *sql.DB, profileID string) ([]Room, error) {
	rows, err := db.Query(
		"SELECT id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, created_at, updated_at FROM rooms WHERE profile_id = ? ORDER BY created_at ASC",
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list rooms: %w", err)
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.ProfileID, &r.Name, &r.Icon, &r.BeaconUUID, &r.BeaconMajor, &r.BeaconMinor, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan room row: %w", err)
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// UpdateRoom changes a room's name and icon, bumping updated_at.
func UpdateRoom(db *sql.DB, id, name, icon string) (*Room, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE rooms SET name = ?, icon = ?, updated_at = ? WHERE id = ?",
		name, icon, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update room: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("room not found: %s", id)
	}

	return GetRoom(db, id)
}

// UpdateRoomBeacon sets the iBeacon configuration for a room.
// This links the room to a physical BLE beacon for proximity detection.
// The uuid/major/minor combo should be unique across all rooms.
func UpdateRoomBeacon(db *sql.DB, id string, uuid string, major, minor int) (*Room, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE rooms SET beacon_uuid = ?, beacon_major = ?, beacon_minor = ?, updated_at = ? WHERE id = ?",
		uuid, major, minor, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update room beacon: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("room not found: %s", id)
	}

	return GetRoom(db, id)
}

// DeleteRoom removes a room. Devices assigned to this room will have
// their room_id set to NULL (via ON DELETE SET NULL), making them unassigned.
func DeleteRoom(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM rooms WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete room: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("room not found: %s", id)
	}
	return nil
}

// =============================================================================
// Device Operations
// =============================================================================

// CreateDevice registers a new device under the given profile.
// The device starts unassigned (no room) — use AssignDeviceToRoom to place it.
func CreateDevice(db *sql.DB, profileID string, name, deviceType string, externalID, model *string) (*Device, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO devices (id, profile_id, name, device_type, external_id, model, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, profileID, name, deviceType, externalID, model, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	return &Device{
		ID:         id,
		ProfileID:  profileID,
		Name:       name,
		DeviceType: deviceType,
		ExternalID: externalID,
		Model:      model,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// GetDevice retrieves a single device by its ID.
func GetDevice(db *sql.DB, id string) (*Device, error) {
	var d Device
	err := db.QueryRow(
		"SELECT id, profile_id, room_id, name, device_type, external_id, model, metadata, created_at, updated_at FROM devices WHERE id = ?", id,
	).Scan(&d.ID, &d.ProfileID, &d.RoomID, &d.Name, &d.DeviceType, &d.ExternalID, &d.Model, &d.Metadata, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	return &d, nil
}

// ListDevicesByProfile returns all devices belonging to a profile.
func ListDevicesByProfile(db *sql.DB, profileID string) ([]Device, error) {
	rows, err := db.Query(
		"SELECT id, profile_id, room_id, name, device_type, external_id, model, metadata, created_at, updated_at FROM devices WHERE profile_id = ? ORDER BY created_at ASC",
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices by profile: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.ProfileID, &d.RoomID, &d.Name, &d.DeviceType, &d.ExternalID, &d.Model, &d.Metadata, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// ListDevicesByRoom returns all devices assigned to a specific room.
func ListDevicesByRoom(db *sql.DB, roomID string) ([]Device, error) {
	rows, err := db.Query(
		"SELECT id, profile_id, room_id, name, device_type, external_id, model, metadata, created_at, updated_at FROM devices WHERE room_id = ? ORDER BY created_at ASC",
		roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices by room: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.ProfileID, &d.RoomID, &d.Name, &d.DeviceType, &d.ExternalID, &d.Model, &d.Metadata, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// AssignDeviceToRoom places a device into a room.
// The device must belong to the same profile as the room (not enforced here,
// but the API layer should validate this).
func AssignDeviceToRoom(db *sql.DB, deviceID, roomID string) (*Device, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE devices SET room_id = ?, updated_at = ? WHERE id = ?",
		roomID, now, deviceID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to assign device to room: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	return GetDevice(db, deviceID)
}

// UnassignDevice removes a device from its room (sets room_id to NULL).
func UnassignDevice(db *sql.DB, deviceID string) (*Device, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE devices SET room_id = NULL, updated_at = ? WHERE id = ?",
		now, deviceID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to unassign device: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	return GetDevice(db, deviceID)
}

// UpdateDevice changes a device's friendly name.
func UpdateDevice(db *sql.DB, id, name string) (*Device, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE devices SET name = ?, updated_at = ? WHERE id = ?",
		name, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("device not found: %s", id)
	}

	return GetDevice(db, id)
}

// DeleteDevice permanently removes a device record.
func DeleteDevice(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("device not found: %s", id)
	}
	return nil
}
