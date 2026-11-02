package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

// parseJSON parses a JSON string into the given destination.
func parseJSON(data string, dest interface{}) error {
	return json.Unmarshal([]byte(data), dest)
}

// serializeJSON serializes a value to a JSON string.
func serializeJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
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
// Branding fields (logo, color, hero image) are nullable and can be set later via UpdateBusiness.
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
		"SELECT id, name, owner_name, welcome_message, website, phone, logo_url, accent_color, hero_image_url, created_at, updated_at FROM businesses WHERE id = ?", id,
	).Scan(&b.ID, &b.Name, &b.OwnerName, &b.WelcomeMessage, &b.Website, &b.Phone, &b.LogoURL, &b.AccentColor, &b.HeroImageURL, &b.CreatedAt, &b.UpdatedAt)
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
	rows, err := db.Query("SELECT id, name, owner_name, welcome_message, website, phone, logo_url, accent_color, hero_image_url, created_at, updated_at FROM businesses ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to list businesses: %w", err)
	}
	defer rows.Close()

	var businesses []Business
	for rows.Next() {
		var b Business
		if err := rows.Scan(&b.ID, &b.Name, &b.OwnerName, &b.WelcomeMessage, &b.Website, &b.Phone, &b.LogoURL, &b.AccentColor, &b.HeroImageURL, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan business row: %w", err)
		}
		businesses = append(businesses, b)
	}
	return businesses, rows.Err()
}

// UpdateBusiness changes the name, owner name, and branding fields of an existing business.
// All fields are updated together; use nil for optional fields to leave them unchanged would require
// a different approach, but for simplicity we update all fields as provided in the contract.
func UpdateBusiness(db *sql.DB, id string, name string, ownerName *string, welcomeMessage *string, website *string, phone *string, logoURL *string, accentColor *string, heroImageURL *string) (*Business, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE businesses SET name = ?, owner_name = ?, welcome_message = ?, website = ?, phone = ?, logo_url = ?, accent_color = ?, hero_image_url = ?, updated_at = ? WHERE id = ?",
		name, ownerName, welcomeMessage, website, phone, logoURL, accentColor, heroImageURL, now, id,
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
		"SELECT id, business_id, name, address, metadata, street, street2, city, state, postal_code, country, latitude, longitude, timezone, opening_hours, special_hours, created_at, updated_at FROM locations WHERE id = ?", id,
	).Scan(&l.ID, &l.BusinessID, &l.Name, &l.Address, &l.Metadata, &l.Street, &l.Street2, &l.City, &l.State, &l.PostalCode, &l.Country, &l.Latitude, &l.Longitude, &l.Timezone, &l.OpeningHours, &l.SpecialHours, &l.CreatedAt, &l.UpdatedAt)
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
		"SELECT id, business_id, name, address, metadata, street, street2, city, state, postal_code, country, latitude, longitude, timezone, opening_hours, special_hours, created_at, updated_at FROM locations WHERE business_id = ? ORDER BY created_at ASC",
		businessID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
	}
	defer rows.Close()

	var locations []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.BusinessID, &l.Name, &l.Address, &l.Metadata, &l.Street, &l.Street2, &l.City, &l.State, &l.PostalCode, &l.Country, &l.Latitude, &l.Longitude, &l.Timezone, &l.OpeningHours, &l.SpecialHours, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan location row: %w", err)
		}
		locations = append(locations, l)
	}
	return locations, rows.Err()
}

// UpdateLocation changes a location's name, structured address fields, timezone, and hours.
// All fields are updated together as provided in the contract.
func UpdateLocation(db *sql.DB, id, name string, street, street2, city, state, postalCode, country *string, latitude, longitude *float64, timezone, openingHours, specialHours *string) (*Location, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE locations SET name = ?, street = ?, street2 = ?, city = ?, state = ?, postal_code = ?, country = ?, latitude = ?, longitude = ?, timezone = ?, opening_hours = ?, special_hours = ?, updated_at = ? WHERE id = ?",
		name, street, street2, city, state, postalCode, country, latitude, longitude, timezone, openingHours, specialHours, now, id,
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

// =============================================================================
// Menu Option Template Operations
// =============================================================================

// CreateMenuOptionTemplate creates a new menu option template.
// Business-created templates are visible to all businesses.
// System templates (is_system=true) can only be created during seeding.
func CreateMenuOptionTemplate(db *sql.DB, createdByBusinessID *string, optionType, defaultTitle, defaultIcon, dataSchema, description string) (*MenuOptionTemplate, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO menu_option_templates (id, created_by_business_id, option_type, default_title, default_icon, data_schema, description, is_system, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)",
		id, createdByBusinessID, optionType, defaultTitle, defaultIcon, dataSchema, description, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create menu option template: %w", err)
	}

	return &MenuOptionTemplate{
		ID:                  id,
		CreatedByBusinessID: createdByBusinessID,
		OptionType:          optionType,
		DefaultTitle:        defaultTitle,
		DefaultIcon:         defaultIcon,
		DataSchema:          dataSchema,
		Description:         description,
		IsSystem:            false,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// GetMenuOptionTemplate retrieves a single template by its ID.
func GetMenuOptionTemplate(db *sql.DB, id string) (*MenuOptionTemplate, error) {
	var t MenuOptionTemplate
	err := db.QueryRow(
		"SELECT id, created_by_business_id, option_type, default_title, default_icon, data_schema, description, is_system, created_at, updated_at FROM menu_option_templates WHERE id = ?", id,
	).Scan(&t.ID, &t.CreatedByBusinessID, &t.OptionType, &t.DefaultTitle, &t.DefaultIcon, &t.DataSchema, &t.Description, &t.IsSystem, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("menu option template not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get menu option template: %w", err)
	}
	return &t, nil
}

// ListMenuOptionTemplates returns all menu option templates (system + business-created).
// Ordered by system templates first, then by creation time.
func ListMenuOptionTemplates(db *sql.DB) ([]MenuOptionTemplate, error) {
	rows, err := db.Query(
		"SELECT id, created_by_business_id, option_type, default_title, default_icon, data_schema, description, is_system, created_at, updated_at FROM menu_option_templates ORDER BY is_system DESC, created_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list menu option templates: %w", err)
	}
	defer rows.Close()

	var templates []MenuOptionTemplate
	for rows.Next() {
		var t MenuOptionTemplate
		if err := rows.Scan(&t.ID, &t.CreatedByBusinessID, &t.OptionType, &t.DefaultTitle, &t.DefaultIcon, &t.DataSchema, &t.Description, &t.IsSystem, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan template row: %w", err)
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// UpdateMenuOptionTemplate updates a template's fields.
// System templates cannot be edited (returns error).
func UpdateMenuOptionTemplate(db *sql.DB, id, defaultTitle, defaultIcon, dataSchema, description string) (*MenuOptionTemplate, error) {
	// Check if this is a system template
	template, err := GetMenuOptionTemplate(db, id)
	if err != nil {
		return nil, err
	}
	if template.IsSystem {
		return nil, fmt.Errorf("cannot edit system template: %s", id)
	}

	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE menu_option_templates SET default_title = ?, default_icon = ?, data_schema = ?, description = ?, updated_at = ? WHERE id = ?",
		defaultTitle, defaultIcon, dataSchema, description, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update menu option template: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("menu option template not found: %s", id)
	}

	return GetMenuOptionTemplate(db, id)
}

// DeleteMenuOptionTemplate removes a template.
// System templates cannot be deleted (returns error).
func DeleteMenuOptionTemplate(db *sql.DB, id string) error {
	// Check if this is a system template
	template, err := GetMenuOptionTemplate(db, id)
	if err != nil {
		return err
	}
	if template.IsSystem {
		return fmt.Errorf("cannot delete system template: %s", id)
	}

	result, err := db.Exec("DELETE FROM menu_option_templates WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete menu option template: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("menu option template not found: %s", id)
	}
	return nil
}

// =============================================================================
// Menu Option Operations
// =============================================================================

// CreateMenuOption creates a new menu option for a business.
// Can optionally be linked to a template via templateID.
func CreateMenuOption(db *sql.DB, businessID string, templateID *string, optionType, title, icon, data string) (*MenuOption, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO menu_options (id, business_id, template_id, option_type, title, icon, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, businessID, templateID, optionType, title, icon, data, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create menu option: %w", err)
	}

	return &MenuOption{
		ID:         id,
		BusinessID: businessID,
		TemplateID: templateID,
		OptionType: optionType,
		Title:      title,
		Icon:       icon,
		Data:       data,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// CopyMenuOptionFromTemplate copies a template to create a business-specific menu option.
// The data parameter must conform to the template's data schema (validation should be done by caller).
func CopyMenuOptionFromTemplate(db *sql.DB, businessID, templateID, data string) (*MenuOption, error) {
	// Get the template to copy its default values
	template, err := GetMenuOptionTemplate(db, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Create the menu option with template's defaults
	return CreateMenuOption(db, businessID, &templateID, template.OptionType, template.DefaultTitle, template.DefaultIcon, data)
}

// GetMenuOption retrieves a single menu option by its ID.
func GetMenuOption(db *sql.DB, id string) (*MenuOption, error) {
	var o MenuOption
	err := db.QueryRow(
		"SELECT id, business_id, template_id, option_type, title, icon, data, created_at, updated_at FROM menu_options WHERE id = ?", id,
	).Scan(&o.ID, &o.BusinessID, &o.TemplateID, &o.OptionType, &o.Title, &o.Icon, &o.Data, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("menu option not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get menu option: %w", err)
	}
	return &o, nil
}

// ListMenuOptionsByBusiness returns all menu options for a specific business.
func ListMenuOptionsByBusiness(db *sql.DB, businessID string) ([]MenuOption, error) {
	rows, err := db.Query(
		"SELECT id, business_id, template_id, option_type, title, icon, data, created_at, updated_at FROM menu_options WHERE business_id = ? ORDER BY created_at ASC",
		businessID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list menu options by business: %w", err)
	}
	defer rows.Close()

	var options []MenuOption
	for rows.Next() {
		var o MenuOption
		if err := rows.Scan(&o.ID, &o.BusinessID, &o.TemplateID, &o.OptionType, &o.Title, &o.Icon, &o.Data, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan menu option row: %w", err)
		}
		options = append(options, o)
	}
	return options, rows.Err()
}

// UpdateMenuOption updates a menu option's title, icon, and data.
// option_type cannot be changed after creation.
func UpdateMenuOption(db *sql.DB, id, title, icon, data string) (*MenuOption, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE menu_options SET title = ?, icon = ?, data = ?, updated_at = ? WHERE id = ?",
		title, icon, data, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update menu option: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("menu option not found: %s", id)
	}

	return GetMenuOption(db, id)
}

// DeleteMenuOption removes a menu option.
// This will also remove all assignments of this option to menus (via CASCADE).
func DeleteMenuOption(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM menu_options WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete menu option: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("menu option not found: %s", id)
	}
	return nil
}

// =============================================================================
// Menu Operations
// =============================================================================

// GetOrCreateMenu retrieves the menu for a room, creating it if it doesn't exist.
// Each room has exactly one menu (one-to-one relationship).
func GetOrCreateMenu(db *sql.DB, roomID string) (*Menu, error) {
	// Try to get existing menu
	var m Menu
	err := db.QueryRow(
		"SELECT id, room_id, is_enabled, menu_color, menu_opacity, text_color, icon_color, collapsed_icon, created_at, updated_at FROM menus WHERE room_id = ?", roomID,
	).Scan(&m.ID, &m.RoomID, &m.IsEnabled, &m.MenuColor, &m.MenuOpacity, &m.TextColor, &m.IconColor, &m.CollapsedIcon, &m.CreatedAt, &m.UpdatedAt)

	if err == nil {
		// Menu exists, return it
		return &m, nil
	}

	if err != sql.ErrNoRows {
		// Unexpected error
		return nil, fmt.Errorf("failed to get menu: %w", err)
	}

	// Menu doesn't exist, create it with default appearance values
	id := generateUUID()
	now := time.Now().UTC()

	_, err = db.Exec(
		"INSERT INTO menus (id, room_id, is_enabled, menu_color, menu_opacity, text_color, icon_color, collapsed_icon, created_at, updated_at) VALUES (?, ?, 1, '#000000', 0.3, '#FFFFFF', '#FF9500', 'storefront', ?, ?)",
		id, roomID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create menu: %w", err)
	}

	return &Menu{
		ID:            id,
		RoomID:        roomID,
		IsEnabled:     true,
		MenuColor:     "#000000",
		MenuOpacity:   0.3,
		TextColor:     "#FFFFFF",
		IconColor:     "#FF9500",
		CollapsedIcon: "storefront",
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// UpdateMenuEnabled updates the is_enabled flag for a menu.
func UpdateMenuEnabled(db *sql.DB, roomID string, enabled bool) (*Menu, error) {
	// Ensure menu exists first
	menu, err := GetOrCreateMenu(db, roomID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	_, err = db.Exec(
		"UPDATE menus SET is_enabled = ?, updated_at = ? WHERE id = ?",
		enabled, now, menu.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update menu enabled: %w", err)
	}

	menu.IsEnabled = enabled
	menu.UpdatedAt = now
	return menu, nil
}

// UpdateMenuAppearance updates the appearance customization for a menu.
func UpdateMenuAppearance(db *sql.DB, roomID string, menuColor string, menuOpacity float64, textColor string, iconColor string, collapsedIcon string) (*Menu, error) {
	// Ensure menu exists first
	menu, err := GetOrCreateMenu(db, roomID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	_, err = db.Exec(
		"UPDATE menus SET menu_color = ?, menu_opacity = ?, text_color = ?, icon_color = ?, collapsed_icon = ?, updated_at = ? WHERE id = ?",
		menuColor, menuOpacity, textColor, iconColor, collapsedIcon, now, menu.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update menu appearance: %w", err)
	}

	menu.MenuColor = menuColor
	menu.MenuOpacity = menuOpacity
	menu.TextColor = textColor
	menu.IconColor = iconColor
	menu.CollapsedIcon = collapsedIcon
	menu.UpdatedAt = now
	return menu, nil
}

// =============================================================================
// Menu Option Assignment Operations
// =============================================================================

// AssignMenuOption assigns a menu option to a room's menu at the specified position.
// If the position is already taken, this will fail due to UNIQUE constraint.
func AssignMenuOption(db *sql.DB, menuID, menuOptionID string, position int) (*MenuOptionAssignment, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO menu_option_assignments (id, menu_id, menu_option_id, position, created_at) VALUES (?, ?, ?, ?, ?)",
		id, menuID, menuOptionID, position, now,
	)
	if err != nil {
		// Check for UNIQUE constraint violation
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("menu option already assigned or position already taken")
		}
		return nil, fmt.Errorf("failed to assign menu option: %w", err)
	}

	return &MenuOptionAssignment{
		ID:           id,
		MenuID:       menuID,
		MenuOptionID: menuOptionID,
		Position:     position,
		CreatedAt:    now,
	}, nil
}

// UnassignMenuOption removes a menu option assignment from a menu.
func UnassignMenuOption(db *sql.DB, menuID, menuOptionID string) error {
	result, err := db.Exec(
		"DELETE FROM menu_option_assignments WHERE menu_id = ? AND menu_option_id = ?",
		menuID, menuOptionID,
	)
	if err != nil {
		return fmt.Errorf("failed to unassign menu option: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("menu option assignment not found")
	}
	return nil
}

// ReorderMenuOptions updates the positions of menu options in a menu.
// menuOptionIDs should be ordered by desired position (index 0 = position 0, etc.)
// This is transaction-based to ensure atomicity.
func ReorderMenuOptions(db *sql.DB, menuID string, menuOptionIDs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update each assignment with its new position
	for i, menuOptionID := range menuOptionIDs {
		result, err := tx.Exec(
			"UPDATE menu_option_assignments SET position = ? WHERE menu_id = ? AND menu_option_id = ?",
			i, menuID, menuOptionID,
		)
		if err != nil {
			return fmt.Errorf("failed to update position for option %s: %w", menuOptionID, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return fmt.Errorf("menu option assignment not found: %s", menuOptionID)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit reorder transaction: %w", err)
	}
	return nil
}

// GetMenuWithOptions retrieves a menu with all its assigned options, ordered by position.
// Returns the menu and a slice of menu options.
func GetMenuWithOptions(db *sql.DB, roomID string) (*Menu, []MenuOption, error) {
	// Get or create the menu
	menu, err := GetOrCreateMenu(db, roomID)
	if err != nil {
		return nil, nil, err
	}

	// Get all assigned menu options, ordered by position
	rows, err := db.Query(`
		SELECT mo.id, mo.business_id, mo.template_id, mo.option_type, mo.title, mo.icon, mo.data, mo.created_at, mo.updated_at
		FROM menu_options mo
		INNER JOIN menu_option_assignments moa ON moa.menu_option_id = mo.id
		WHERE moa.menu_id = ?
		ORDER BY moa.position ASC
	`, menu.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get menu options: %w", err)
	}
	defer rows.Close()

	var options []MenuOption
	for rows.Next() {
		var o MenuOption
		if err := rows.Scan(&o.ID, &o.BusinessID, &o.TemplateID, &o.OptionType, &o.Title, &o.Icon, &o.Data, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("failed to scan menu option row: %w", err)
		}
		options = append(options, o)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return menu, options, nil
}

// =============================================================================
// Customer Operations
// =============================================================================

// CreateCustomer creates a new customer with the given first name.
// Automatically creates default sharing settings (minimal level).
func CreateCustomer(db *sql.DB, firstName string) (*Customer, error) {
	id := generateUUID()
	now := time.Now().UTC()

	// Start a transaction to create customer and settings atomically
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create customer
	_, err = tx.Exec(
		"INSERT INTO customers (id, first_name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		id, firstName, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	// Create default sharing settings (minimal level)
	_, err = tx.Exec(
		"INSERT INTO customer_sharing_settings (customer_id, sharing_level, blocked_businesses, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		id, "minimal", "[]", now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer sharing settings: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &Customer{
		ID:        id,
		FirstName: firstName,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetCustomer retrieves a single customer by ID.
func GetCustomer(db *sql.DB, id string) (*Customer, error) {
	var c Customer
	err := db.QueryRow(
		"SELECT id, first_name, created_at, updated_at FROM customers WHERE id = ?", id,
	).Scan(&c.ID, &c.FirstName, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}
	return &c, nil
}

// UpdateCustomer changes the first name of an existing customer.
func UpdateCustomer(db *sql.DB, id string, firstName string) (*Customer, error) {
	now := time.Now().UTC()
	result, err := db.Exec(
		"UPDATE customers SET first_name = ?, updated_at = ? WHERE id = ?",
		firstName, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("customer not found: %s", id)
	}

	return GetCustomer(db, id)
}

// GetCustomerSharingSettings retrieves sharing settings for a customer.
func GetCustomerSharingSettings(db *sql.DB, customerID string) (*CustomerSharingSettings, error) {
	var settings CustomerSharingSettings
	var blockedBusinessesJSON string

	err := db.QueryRow(
		"SELECT customer_id, sharing_level, blocked_businesses, created_at, updated_at FROM customer_sharing_settings WHERE customer_id = ?",
		customerID,
	).Scan(&settings.CustomerID, &settings.SharingLevel, &blockedBusinessesJSON, &settings.CreatedAt, &settings.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer sharing settings not found: %s", customerID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer sharing settings: %w", err)
	}

	// Parse blocked businesses JSON array
	settings.BlockedBusinesses = []string{}
	if blockedBusinessesJSON != "[]" && blockedBusinessesJSON != "" {
		if err := parseJSON(blockedBusinessesJSON, &settings.BlockedBusinesses); err != nil {
			return nil, fmt.Errorf("failed to parse blocked businesses: %w", err)
		}
	}

	return &settings, nil
}

// UpdateCustomerSharingSettings updates a customer's sharing settings.
func UpdateCustomerSharingSettings(db *sql.DB, customerID string, sharingLevel string, blockedBusinesses []string) (*CustomerSharingSettings, error) {
	now := time.Now().UTC()

	// Ensure blockedBusinesses is not nil
	if blockedBusinesses == nil {
		blockedBusinesses = []string{}
	}

	// Serialize blocked businesses to JSON
	blockedBusinessesJSON, err := serializeJSON(blockedBusinesses)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize blocked businesses: %w", err)
	}

	result, err := db.Exec(
		"UPDATE customer_sharing_settings SET sharing_level = ?, blocked_businesses = ?, updated_at = ? WHERE customer_id = ?",
		sharingLevel, blockedBusinessesJSON, now, customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update customer sharing settings: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("customer sharing settings not found: %s", customerID)
	}

	return GetCustomerSharingSettings(db, customerID)
}

// =============================================================================
// Arrival Event Operations
// =============================================================================

// CreateArrivalEvent records a customer arrival event at a location.
// The business_id and location_id are resolved from the beacon configuration.
func CreateArrivalEvent(db *sql.DB, customerID, businessID, locationID, beaconUUID string, beaconMajor, beaconMinor int, state, proximity string, timestamp time.Time) (*ArrivalEvent, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		`INSERT INTO arrival_events
		(id, customer_id, business_id, location_id, beacon_uuid, beacon_major, beacon_minor, state, proximity, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, customerID, businessID, locationID, beaconUUID, beaconMajor, beaconMinor, state, proximity, timestamp, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create arrival event: %w", err)
	}

	return &ArrivalEvent{
		ID:          id,
		CustomerID:  customerID,
		BusinessID:  businessID,
		LocationID:  locationID,
		BeaconUUID:  beaconUUID,
		BeaconMajor: beaconMajor,
		BeaconMinor: beaconMinor,
		State:       state,
		Proximity:   proximity,
		Timestamp:   timestamp,
		CreatedAt:   now,
	}, nil
}

// GetRecentArrivalsByLocation retrieves recent arrival events for a location.
// Returns events ordered by timestamp descending (most recent first).
func GetRecentArrivalsByLocation(db *sql.DB, locationID string, limit int) ([]ArrivalEvent, error) {
	rows, err := db.Query(`
		SELECT id, customer_id, business_id, location_id, beacon_uuid, beacon_major, beacon_minor, state, proximity, timestamp, created_at
		FROM arrival_events
		WHERE location_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, locationID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent arrivals: %w", err)
	}
	defer rows.Close()

	var events []ArrivalEvent
	for rows.Next() {
		var e ArrivalEvent
		if err := rows.Scan(&e.ID, &e.CustomerID, &e.BusinessID, &e.LocationID, &e.BeaconUUID, &e.BeaconMajor, &e.BeaconMinor, &e.State, &e.Proximity, &e.Timestamp, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan arrival event row: %w", err)
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

// FindRoomByBeacon looks up a room by its beacon configuration.
// Returns the room and its associated business_id and location_id.
func FindRoomByBeacon(db *sql.DB, beaconUUID string, beaconMajor, beaconMinor int) (roomID, businessID, locationID string, err error) {
	err = db.QueryRow(`
		SELECT r.id, r.location_id, l.business_id
		FROM rooms r
		INNER JOIN locations l ON l.id = r.location_id
		WHERE r.beacon_uuid = ? AND r.beacon_major = ? AND r.beacon_minor = ?
		AND r.location_id IS NOT NULL
	`, beaconUUID, beaconMajor, beaconMinor).Scan(&roomID, &locationID, &businessID)

	if err == sql.ErrNoRows {
		return "", "", "", fmt.Errorf("beacon not found: no room configured with UUID %s, major %d, minor %d", beaconUUID, beaconMajor, beaconMinor)
	}
	if err != nil {
		return "", "", "", fmt.Errorf("failed to find room by beacon: %w", err)
	}

	return roomID, businessID, locationID, nil
}

// =============================================================================
// Product Catalog Operations (helper functions for tests)
// =============================================================================

// CreateProductMenu creates a new product menu for a business.
func CreateProductMenu(db *sql.DB, businessID, name string, description *string, isActive bool, displayOrder int) (*ProductMenu, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO product_menus (id, business_id, name, description, is_active, display_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, businessID, name, description, isActive, displayOrder, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create product menu: %w", err)
	}

	return &ProductMenu{
		ID:           id,
		BusinessID:   businessID,
		Name:         name,
		Description:  description,
		IsActive:     isActive,
		DisplayOrder: displayOrder,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// CreateCategory creates a new category for a product menu.
func CreateCategory(db *sql.DB, menuID, name string, description *string, isAvailable bool, displayOrder int) (*Category, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO categories (id, menu_id, name, description, is_available, display_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, menuID, name, description, isAvailable, displayOrder, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	return &Category{
		ID:           id,
		MenuID:       menuID,
		Name:         name,
		Description:  description,
		IsAvailable:  isAvailable,
		DisplayOrder: displayOrder,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// CreateItem creates a new item for a category.
func CreateItem(db *sql.DB, categoryID, name string, description *string, priceMinorUnits int, currencyCode string, imageURLs, dietaryTags *string, isAvailable, isSoldOut bool, displayOrder int) (*Item, error) {
	id := generateUUID()
	now := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO items (id, category_id, name, description, price_minor_units, currency_code, image_urls, dietary_tags, is_available, is_sold_out, display_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, categoryID, name, description, priceMinorUnits, currencyCode, imageURLs, dietaryTags, isAvailable, isSoldOut, displayOrder, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	return &Item{
		ID:              id,
		CategoryID:      categoryID,
		Name:            name,
		Description:     description,
		PriceMinorUnits: priceMinorUnits,
		CurrencyCode:    currencyCode,
		ImageURLs:       imageURLs,
		DietaryTags:     dietaryTags,
		IsAvailable:     isAvailable,
		IsSoldOut:       isSoldOut,
		DisplayOrder:    displayOrder,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}
