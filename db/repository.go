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
		ProfileID: &profileID,
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
		"SELECT id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, custom_template, location_id, created_at, updated_at FROM rooms WHERE id = ?", id,
	).Scan(&r.ID, &r.ProfileID, &r.Name, &r.Icon, &r.BeaconUUID, &r.BeaconMajor, &r.BeaconMinor, &r.CustomTemplate, &r.LocationID, &r.CreatedAt, &r.UpdatedAt)
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
		"SELECT id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, custom_template, location_id, created_at, updated_at FROM rooms WHERE profile_id = ? ORDER BY created_at ASC",
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list rooms: %w", err)
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.ProfileID, &r.Name, &r.Icon, &r.BeaconUUID, &r.BeaconMajor, &r.BeaconMinor, &r.CustomTemplate, &r.LocationID, &r.CreatedAt, &r.UpdatedAt); err != nil {
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

// SaveRoomTemplate saves a user-customized scene layout JSON for a room.
// The template JSON is stored as a TEXT blob in the custom_template column.
func SaveRoomTemplate(db *sql.DB, roomID string, templateJSON string) error {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE rooms SET custom_template = ?, updated_at = ? WHERE id = ?",
		templateJSON, now, roomID,
	)
	if err != nil {
		return fmt.Errorf("failed to save room template: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("room not found: %s", roomID)
	}
	return nil
}

// ClearRoomTemplate removes a user's custom template, reverting to the default.
// Sets custom_template to NULL so HandleGetRoomTemplate falls through to the default.
func ClearRoomTemplate(db *sql.DB, roomID string) error {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE rooms SET custom_template = NULL, updated_at = ? WHERE id = ?",
		now, roomID,
	)
	if err != nil {
		return fmt.Errorf("failed to clear room template: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("room not found: %s", roomID)
	}
	return nil
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
		"SELECT id, profile_id, room_id, name, device_type, external_id, model, metadata, business_id, created_at, updated_at FROM devices WHERE id = ?", id,
	).Scan(&d.ID, &d.ProfileID, &d.RoomID, &d.Name, &d.DeviceType, &d.ExternalID, &d.Model, &d.Metadata, &d.BusinessID, &d.CreatedAt, &d.UpdatedAt)
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
		"SELECT id, profile_id, room_id, name, device_type, external_id, model, metadata, business_id, created_at, updated_at FROM devices WHERE profile_id = ? ORDER BY created_at ASC",
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices by profile: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.ProfileID, &d.RoomID, &d.Name, &d.DeviceType, &d.ExternalID, &d.Model, &d.Metadata, &d.BusinessID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// ListDevicesByRoom returns all devices assigned to a specific room.
func ListDevicesByRoom(db *sql.DB, roomID string) ([]Device, error) {
	rows, err := db.Query(
		"SELECT id, profile_id, room_id, name, device_type, external_id, model, metadata, business_id, created_at, updated_at FROM devices WHERE room_id = ? ORDER BY created_at ASC",
		roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices by room: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.ProfileID, &d.RoomID, &d.Name, &d.DeviceType, &d.ExternalID, &d.Model, &d.Metadata, &d.BusinessID, &d.CreatedAt, &d.UpdatedAt); err != nil {
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

// =============================================================================
// Business Operations
// =============================================================================

// CreateBusiness inserts a new business with the given name and optional owner name.
// A UUID is auto-generated for the business ID.
func CreateBusiness(db *sql.DB, name string, ownerName *string) (*Business, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO businesses (id, name, owner_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		id, name, ownerName, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create business: %w", err)
	}

	return &Business{
		ID:        id,
		Name:      name,
		OwnerName: ownerName,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetBusiness retrieves a single business by its ID.
// Returns nil and an error if not found.
func GetBusiness(db *sql.DB, id string) (*Business, error) {
	var b Business
	err := db.QueryRow(
		"SELECT id, name, owner_name, created_at, updated_at FROM businesses WHERE id = ?", id,
	).Scan(&b.ID, &b.Name, &b.OwnerName, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("business not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get business: %w", err)
	}
	return &b, nil
}

// ListBusinesses returns all businesses in the database.
// Mainly useful for development and debugging.
func ListBusinesses(db *sql.DB) ([]Business, error) {
	rows, err := db.Query("SELECT id, name, owner_name, created_at, updated_at FROM businesses ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to list businesses: %w", err)
	}
	defer rows.Close()

	var businesses []Business
	for rows.Next() {
		var b Business
		if err := rows.Scan(&b.ID, &b.Name, &b.OwnerName, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan business row: %w", err)
		}
		businesses = append(businesses, b)
	}
	return businesses, rows.Err()
}

// UpdateBusiness changes the name and owner name of an existing business and bumps updated_at.
func UpdateBusiness(db *sql.DB, id string, name string, ownerName *string) (*Business, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE businesses SET name = ?, owner_name = ?, updated_at = ? WHERE id = ?",
		name, ownerName, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update business: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("business not found: %s", id)
	}

	return GetBusiness(db, id)
}

// DeleteBusiness removes a business and all associated locations/rooms (via CASCADE).
func DeleteBusiness(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM businesses WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete business: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("business not found: %s", id)
	}
	return nil
}

// =============================================================================
// Location Operations
// =============================================================================

// CreateLocation adds a new location under the given business.
func CreateLocation(db *sql.DB, businessID, name string, address *string) (*Location, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO locations (id, business_id, name, address, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, businessID, name, address, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create location: %w", err)
	}

	return &Location{
		ID:         id,
		BusinessID: businessID,
		Name:       name,
		Address:    address,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// GetLocation retrieves a single location by its ID.
func GetLocation(db *sql.DB, id string) (*Location, error) {
	var l Location
	err := db.QueryRow(
		"SELECT id, business_id, name, address, metadata, created_at, updated_at FROM locations WHERE id = ?", id,
	).Scan(&l.ID, &l.BusinessID, &l.Name, &l.Address, &l.Metadata, &l.CreatedAt, &l.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("location not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	return &l, nil
}

// ListLocationsByBusiness returns all locations belonging to a business, ordered by creation time.
func ListLocationsByBusiness(db *sql.DB, businessID string) ([]Location, error) {
	rows, err := db.Query(
		"SELECT id, business_id, name, address, metadata, created_at, updated_at FROM locations WHERE business_id = ? ORDER BY created_at ASC",
		businessID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
	}
	defer rows.Close()

	var locations []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.BusinessID, &l.Name, &l.Address, &l.Metadata, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan location row: %w", err)
		}
		locations = append(locations, l)
	}
	return locations, rows.Err()
}

// UpdateLocation changes a location's name and address, bumping updated_at.
func UpdateLocation(db *sql.DB, id, name string, address *string) (*Location, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE locations SET name = ?, address = ?, updated_at = ? WHERE id = ?",
		name, address, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("location not found: %s", id)
	}

	return GetLocation(db, id)
}

// DeleteLocation removes a location and all associated rooms (via CASCADE).
func DeleteLocation(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM locations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("location not found: %s", id)
	}
	return nil
}

// ListRoomsByLocation returns all rooms belonging to a location, ordered by creation time.
func ListRoomsByLocation(db *sql.DB, locationID string) ([]Room, error) {
	rows, err := db.Query(
		"SELECT id, profile_id, name, icon, beacon_uuid, beacon_major, beacon_minor, custom_template, location_id, created_at, updated_at FROM rooms WHERE location_id = ? ORDER BY created_at ASC",
		locationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list rooms by location: %w", err)
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.ProfileID, &r.Name, &r.Icon, &r.BeaconUUID, &r.BeaconMajor, &r.BeaconMinor, &r.CustomTemplate, &r.LocationID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan room row: %w", err)
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// CreateRoomForLocation adds a new room under the given location (shop mode).
func CreateRoomForLocation(db *sql.DB, locationID, name, icon string) (*Room, error) {
	id := generateUUID()
	now := time.Now().UTC()

	// Use NULL for profile_id since this is a shop mode room
	_, err := db.Exec(
		"INSERT INTO rooms (id, profile_id, location_id, name, icon, created_at, updated_at) VALUES (?, NULL, ?, ?, ?, ?, ?)",
		id, locationID, name, icon, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create room for location: %w", err)
	}

	return &Room{
		ID:         id,
		ProfileID:  nil,  // NULL for shop mode rooms
		LocationID: &locationID,
		Name:       name,
		Icon:       icon,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// =============================================================================
// Room Trigger Operations
// =============================================================================

// CreateRoomTrigger adds a new trigger for a room.
func CreateRoomTrigger(db *sql.DB, roomID, triggerType, actionType, actionConfig string, dwellSeconds *int, enabled bool) (*RoomTrigger, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO room_triggers (id, room_id, trigger_type, dwell_seconds, action_type, action_config, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, roomID, triggerType, dwellSeconds, actionType, actionConfig, enabled, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create room trigger: %w", err)
	}

	return &RoomTrigger{
		ID:           id,
		RoomID:       roomID,
		TriggerType:  triggerType,
		DwellSeconds: dwellSeconds,
		ActionType:   actionType,
		ActionConfig: actionConfig,
		Enabled:      enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// GetRoomTrigger retrieves a single trigger by its ID.
func GetRoomTrigger(db *sql.DB, id string) (*RoomTrigger, error) {
	var t RoomTrigger
	err := db.QueryRow(
		"SELECT id, room_id, trigger_type, dwell_seconds, action_type, action_config, enabled, created_at, updated_at FROM room_triggers WHERE id = ?", id,
	).Scan(&t.ID, &t.RoomID, &t.TriggerType, &t.DwellSeconds, &t.ActionType, &t.ActionConfig, &t.Enabled, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("room trigger not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room trigger: %w", err)
	}
	return &t, nil
}

// ListTriggersByRoom returns all triggers for a specific room, ordered by creation time.
func ListTriggersByRoom(db *sql.DB, roomID string) ([]RoomTrigger, error) {
	rows, err := db.Query(
		"SELECT id, room_id, trigger_type, dwell_seconds, action_type, action_config, enabled, created_at, updated_at FROM room_triggers WHERE room_id = ? ORDER BY created_at ASC",
		roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list triggers by room: %w", err)
	}
	defer rows.Close()

	var triggers []RoomTrigger
	for rows.Next() {
		var t RoomTrigger
		if err := rows.Scan(&t.ID, &t.RoomID, &t.TriggerType, &t.DwellSeconds, &t.ActionType, &t.ActionConfig, &t.Enabled, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan trigger row: %w", err)
		}
		triggers = append(triggers, t)
	}
	return triggers, rows.Err()
}

// UpdateRoomTrigger changes a trigger's configuration.
func UpdateRoomTrigger(db *sql.DB, id, triggerType, actionType, actionConfig string, dwellSeconds *int, enabled bool) (*RoomTrigger, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE room_triggers SET trigger_type = ?, dwell_seconds = ?, action_type = ?, action_config = ?, enabled = ?, updated_at = ? WHERE id = ?",
		triggerType, dwellSeconds, actionType, actionConfig, enabled, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update room trigger: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("room trigger not found: %s", id)
	}

	return GetRoomTrigger(db, id)
}

// DeleteRoomTrigger permanently removes a trigger record.
func DeleteRoomTrigger(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM room_triggers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete room trigger: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("room trigger not found: %s", id)
	}
	return nil
}
