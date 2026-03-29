package db

import (
	"database/sql"
	"testing"
)

// setupTestDB creates a fresh in-memory SQLite database with all migrations applied.
// Each test gets its own isolated database so tests don't interfere with each other.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// =============================================================================
// Profile Tests
// =============================================================================

func TestCreateProfile(t *testing.T) {
	database := setupTestDB(t)

	profile, err := CreateProfile(database, "Shakur")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the returned profile has all expected fields
	if profile.ID == "" {
		t.Error("expected profile ID to be set")
	}
	if profile.Name != "Shakur" {
		t.Errorf("expected name 'Shakur', got '%s'", profile.Name)
	}
	if profile.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if profile.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}
}

func TestGetProfile(t *testing.T) {
	database := setupTestDB(t)

	// Create a profile, then retrieve it by ID
	created, _ := CreateProfile(database, "Shakur")
	fetched, err := GetProfile(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
	if fetched.Name != "Shakur" {
		t.Errorf("expected name 'Shakur', got '%s'", fetched.Name)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetProfile(database, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent profile, got nil")
	}
}

func TestGetProfileByName(t *testing.T) {
	database := setupTestDB(t)

	CreateProfile(database, "Shakur")
	profile, err := GetProfileByName(database, "Shakur")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if profile.Name != "Shakur" {
		t.Errorf("expected name 'Shakur', got '%s'", profile.Name)
	}
}

func TestGetProfileByNameNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetProfileByName(database, "Nobody")
	if err == nil {
		t.Fatal("expected error for nonexistent profile name, got nil")
	}
}

func TestListProfiles(t *testing.T) {
	database := setupTestDB(t)

	// Start with empty list
	profiles, err := ListProfiles(database)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}

	// Add two profiles and verify both are returned
	CreateProfile(database, "Alice")
	CreateProfile(database, "Bob")

	profiles, err = ListProfiles(database)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestUpdateProfile(t *testing.T) {
	database := setupTestDB(t)

	created, _ := CreateProfile(database, "OldName")
	updated, err := UpdateProfile(database, created.ID, "NewName")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updated.Name != "NewName" {
		t.Errorf("expected name 'NewName', got '%s'", updated.Name)
	}
	// updated_at should be bumped (at least not before created_at)
	if updated.UpdatedAt.Before(created.CreatedAt) {
		t.Error("expected updated_at to be >= created_at")
	}
}

func TestUpdateProfileNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UpdateProfile(database, "nonexistent", "Name")
	if err == nil {
		t.Fatal("expected error for nonexistent profile, got nil")
	}
}

func TestDeleteProfile(t *testing.T) {
	database := setupTestDB(t)

	created, _ := CreateProfile(database, "ToDelete")
	err := DeleteProfile(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify it's actually gone
	_, err = GetProfile(database, created.ID)
	if err == nil {
		t.Fatal("expected error fetching deleted profile, got nil")
	}
}

func TestDeleteProfileNotFound(t *testing.T) {
	database := setupTestDB(t)

	err := DeleteProfile(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile, got nil")
	}
}

// =============================================================================
// Room Tests
// =============================================================================

func TestCreateRoom(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, err := CreateRoom(database, profile.ID, "Living Room", "sofa")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if room.ID == "" {
		t.Error("expected room ID to be set")
	}
	if room.ProfileID != profile.ID {
		t.Errorf("expected profile_id '%s', got '%s'", profile.ID, room.ProfileID)
	}
	if room.Name != "Living Room" {
		t.Errorf("expected name 'Living Room', got '%s'", room.Name)
	}
	if room.Icon != "sofa" {
		t.Errorf("expected icon 'sofa', got '%s'", room.Icon)
	}
	// Beacon fields should be nil initially
	if room.BeaconUUID != nil {
		t.Error("expected beacon_uuid to be nil initially")
	}
}

func TestGetRoom(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	created, _ := CreateRoom(database, profile.ID, "Office", "desktopcomputer")
	fetched, err := GetRoom(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fetched.Name != "Office" {
		t.Errorf("expected name 'Office', got '%s'", fetched.Name)
	}
}

func TestGetRoomNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetRoom(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room, got nil")
	}
}

func TestListRoomsByProfile(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	CreateRoom(database, profile.ID, "Living Room", "sofa")
	CreateRoom(database, profile.ID, "Office", "desktopcomputer")
	CreateRoom(database, profile.ID, "Bedroom", "bed.double")

	rooms, err := ListRoomsByProfile(database, profile.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(rooms) != 3 {
		t.Errorf("expected 3 rooms, got %d", len(rooms))
	}
}

func TestListRoomsByProfileEmpty(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	rooms, err := ListRoomsByProfile(database, profile.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms, got %d", len(rooms))
	}
}

func TestUpdateRoom(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "OldRoom", "house")
	updated, err := UpdateRoom(database, room.ID, "NewRoom", "star")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updated.Name != "NewRoom" {
		t.Errorf("expected name 'NewRoom', got '%s'", updated.Name)
	}
	if updated.Icon != "star" {
		t.Errorf("expected icon 'star', got '%s'", updated.Icon)
	}
}

func TestUpdateRoomNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UpdateRoom(database, "nonexistent", "Name", "icon")
	if err == nil {
		t.Fatal("expected error for nonexistent room, got nil")
	}
}

func TestUpdateRoomBeacon(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "sofa")

	// Set beacon configuration
	beaconUUID := "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0"
	updated, err := UpdateRoomBeacon(database, room.ID, beaconUUID, 1, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updated.BeaconUUID == nil || *updated.BeaconUUID != beaconUUID {
		t.Errorf("expected beacon_uuid '%s', got '%v'", beaconUUID, updated.BeaconUUID)
	}
	if updated.BeaconMajor == nil || *updated.BeaconMajor != 1 {
		t.Errorf("expected beacon_major 1, got '%v'", updated.BeaconMajor)
	}
	if updated.BeaconMinor == nil || *updated.BeaconMinor != 100 {
		t.Errorf("expected beacon_minor 100, got '%v'", updated.BeaconMinor)
	}
}

func TestUpdateRoomBeaconNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UpdateRoomBeacon(database, "nonexistent", "uuid", 1, 1)
	if err == nil {
		t.Fatal("expected error for nonexistent room, got nil")
	}
}

func TestDeleteRoom(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "ToDelete", "trash")

	err := DeleteRoom(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, err = GetRoom(database, room.ID)
	if err == nil {
		t.Fatal("expected error fetching deleted room, got nil")
	}
}

func TestDeleteRoomNotFound(t *testing.T) {
	database := setupTestDB(t)

	err := DeleteRoom(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room, got nil")
	}
}

// =============================================================================
// Device Tests
// =============================================================================

func TestCreateDevice(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	extID := "govee-abc-123"
	model := "H6160"
	device, err := CreateDevice(database, profile.ID, "Desk Lamp", "govee_light", &extID, &model)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if device.ID == "" {
		t.Error("expected device ID to be set")
	}
	if device.Name != "Desk Lamp" {
		t.Errorf("expected name 'Desk Lamp', got '%s'", device.Name)
	}
	if device.DeviceType != "govee_light" {
		t.Errorf("expected device_type 'govee_light', got '%s'", device.DeviceType)
	}
	if device.ExternalID == nil || *device.ExternalID != extID {
		t.Errorf("expected external_id '%s', got '%v'", extID, device.ExternalID)
	}
	if device.Model == nil || *device.Model != model {
		t.Errorf("expected model '%s', got '%v'", model, device.Model)
	}
	// Should start unassigned (no room)
	if device.RoomID != nil {
		t.Error("expected room_id to be nil initially")
	}
}

func TestCreateDeviceWithNilOptionals(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	device, err := CreateDevice(database, profile.ID, "Generic Sensor", "generic", nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if device.ExternalID != nil {
		t.Error("expected external_id to be nil")
	}
	if device.Model != nil {
		t.Error("expected model to be nil")
	}
}

func TestGetDevice(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	created, _ := CreateDevice(database, profile.ID, "TV", "fire_tv", nil, nil)
	fetched, err := GetDevice(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fetched.Name != "TV" {
		t.Errorf("expected name 'TV', got '%s'", fetched.Name)
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetDevice(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent device, got nil")
	}
}

func TestListDevicesByProfile(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)
	CreateDevice(database, profile.ID, "TV", "fire_tv", nil, nil)
	CreateDevice(database, profile.ID, "Camera", "wyze_camera", nil, nil)

	devices, err := ListDevicesByProfile(database, profile.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(devices))
	}
}

func TestListDevicesByRoom(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "sofa")

	// Create 2 devices and assign them to the room
	d1, _ := CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)
	d2, _ := CreateDevice(database, profile.ID, "TV", "fire_tv", nil, nil)
	CreateDevice(database, profile.ID, "Unassigned", "generic", nil, nil) // not assigned

	AssignDeviceToRoom(database, d1.ID, room.ID)
	AssignDeviceToRoom(database, d2.ID, room.ID)

	devices, err := ListDevicesByRoom(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("expected 2 devices in room, got %d", len(devices))
	}
}

func TestAssignDeviceToRoom(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Office", "desktopcomputer")
	device, _ := CreateDevice(database, profile.ID, "Monitor", "fire_tv", nil, nil)

	// Assign the device to the room
	assigned, err := AssignDeviceToRoom(database, device.ID, room.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if assigned.RoomID == nil || *assigned.RoomID != room.ID {
		t.Errorf("expected room_id '%s', got '%v'", room.ID, assigned.RoomID)
	}
}

func TestAssignDeviceNotFound(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Room", "house")

	_, err := AssignDeviceToRoom(database, "nonexistent", room.ID)
	if err == nil {
		t.Fatal("expected error for nonexistent device, got nil")
	}
}

func TestUnassignDevice(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Office", "desktopcomputer")
	device, _ := CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	// Assign then unassign
	AssignDeviceToRoom(database, device.ID, room.ID)
	unassigned, err := UnassignDevice(database, device.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if unassigned.RoomID != nil {
		t.Errorf("expected room_id to be nil after unassign, got '%v'", unassigned.RoomID)
	}
}

func TestUnassignDeviceNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UnassignDevice(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent device, got nil")
	}
}

func TestUpdateDevice(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	device, _ := CreateDevice(database, profile.ID, "OldName", "govee_light", nil, nil)

	updated, err := UpdateDevice(database, device.ID, "NewName")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updated.Name != "NewName" {
		t.Errorf("expected name 'NewName', got '%s'", updated.Name)
	}
}

func TestUpdateDeviceNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UpdateDevice(database, "nonexistent", "Name")
	if err == nil {
		t.Fatal("expected error for nonexistent device, got nil")
	}
}

func TestDeleteDevice(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	device, _ := CreateDevice(database, profile.ID, "ToDelete", "generic", nil, nil)

	err := DeleteDevice(database, device.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, err = GetDevice(database, device.ID)
	if err == nil {
		t.Fatal("expected error fetching deleted device, got nil")
	}
}

func TestDeleteDeviceNotFound(t *testing.T) {
	database := setupTestDB(t)

	err := DeleteDevice(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent device, got nil")
	}
}

// =============================================================================
// Foreign Key Constraint / Cascade Tests
// =============================================================================

// When a profile is deleted, all its rooms should be cascade-deleted too.
func TestDeleteProfileCascadesToRooms(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room1, _ := CreateRoom(database, profile.ID, "Living Room", "sofa")
	room2, _ := CreateRoom(database, profile.ID, "Office", "desktopcomputer")

	// Delete the profile
	DeleteProfile(database, profile.ID)

	// Both rooms should be gone
	_, err := GetRoom(database, room1.ID)
	if err == nil {
		t.Error("expected room1 to be cascade-deleted with profile")
	}
	_, err = GetRoom(database, room2.ID)
	if err == nil {
		t.Error("expected room2 to be cascade-deleted with profile")
	}
}

// When a profile is deleted, all its devices should be cascade-deleted too.
func TestDeleteProfileCascadesToDevices(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	device, _ := CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	DeleteProfile(database, profile.ID)

	_, err := GetDevice(database, device.ID)
	if err == nil {
		t.Error("expected device to be cascade-deleted with profile")
	}
}

// When a room is deleted, devices assigned to it should become unassigned (room_id = NULL).
func TestDeleteRoomUnassignsDevices(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Office", "desktopcomputer")
	device, _ := CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)

	// Assign device to the room, then delete the room
	AssignDeviceToRoom(database, device.ID, room.ID)
	DeleteRoom(database, room.ID)

	// Device should still exist but with room_id = NULL
	fetched, err := GetDevice(database, device.ID)
	if err != nil {
		t.Fatalf("expected device to still exist after room deletion, got: %v", err)
	}
	if fetched.RoomID != nil {
		t.Errorf("expected room_id to be NULL after room deletion, got '%v'", fetched.RoomID)
	}
}

// Full cascade chain: profile deletion removes rooms AND devices.
func TestDeleteProfileFullCascade(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "sofa")
	device, _ := CreateDevice(database, profile.ID, "Lamp", "govee_light", nil, nil)
	AssignDeviceToRoom(database, device.ID, room.ID)

	// Nuke the profile — everything should be gone
	DeleteProfile(database, profile.ID)

	_, err := GetRoom(database, room.ID)
	if err == nil {
		t.Error("expected room to be cascade-deleted")
	}
	_, err = GetDevice(database, device.ID)
	if err == nil {
		t.Error("expected device to be cascade-deleted")
	}
}

// =============================================================================
// Cross-cutting / Integration Tests
// =============================================================================

// Test the full onboarding flow: create profile -> add rooms -> add devices -> assign devices -> set beacons
func TestFullOnboardingFlow(t *testing.T) {
	database := setupTestDB(t)

	// Step 1: Create profile
	profile, err := CreateProfile(database, "Shakur")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	// Step 2: Create 3 rooms
	livingRoom, err := CreateRoom(database, profile.ID, "Living Room", "sofa")
	if err != nil {
		t.Fatalf("create living room: %v", err)
	}
	office, err := CreateRoom(database, profile.ID, "Office", "desktopcomputer")
	if err != nil {
		t.Fatalf("create office: %v", err)
	}
	bedroom, err := CreateRoom(database, profile.ID, "Bedroom", "bed.double")
	if err != nil {
		t.Fatalf("create bedroom: %v", err)
	}

	// Step 3: Create devices
	extID1 := "govee-living-lamp"
	lamp, err := CreateDevice(database, profile.ID, "Living Room Lamp", "govee_light", &extID1, nil)
	if err != nil {
		t.Fatalf("create lamp: %v", err)
	}
	tv, err := CreateDevice(database, profile.ID, "Fire TV", "fire_tv", nil, nil)
	if err != nil {
		t.Fatalf("create tv: %v", err)
	}
	cam, err := CreateDevice(database, profile.ID, "Hallway Cam", "wyze_camera", nil, nil)
	if err != nil {
		t.Fatalf("create camera: %v", err)
	}

	// Step 4: Assign devices to rooms
	AssignDeviceToRoom(database, lamp.ID, livingRoom.ID)
	AssignDeviceToRoom(database, tv.ID, livingRoom.ID)
	AssignDeviceToRoom(database, cam.ID, bedroom.ID)

	// Step 5: Configure beacons
	beaconUUID := "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0"
	UpdateRoomBeacon(database, livingRoom.ID, beaconUUID, 1, 1)
	UpdateRoomBeacon(database, office.ID, beaconUUID, 1, 2)
	UpdateRoomBeacon(database, bedroom.ID, beaconUUID, 1, 3)

	// Verify: list rooms and check beacon config
	rooms, _ := ListRoomsByProfile(database, profile.ID)
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms, got %d", len(rooms))
	}
	for _, r := range rooms {
		if r.BeaconUUID == nil {
			t.Errorf("room '%s' should have beacon_uuid set", r.Name)
		}
	}

	// Verify: living room has 2 devices
	livingRoomDevices, _ := ListDevicesByRoom(database, livingRoom.ID)
	if len(livingRoomDevices) != 2 {
		t.Errorf("expected 2 devices in living room, got %d", len(livingRoomDevices))
	}

	// Verify: bedroom has 1 device
	bedroomDevices, _ := ListDevicesByRoom(database, bedroom.ID)
	if len(bedroomDevices) != 1 {
		t.Errorf("expected 1 device in bedroom, got %d", len(bedroomDevices))
	}

	// Verify: office has 0 devices
	officeDevices, _ := ListDevicesByRoom(database, office.ID)
	if len(officeDevices) != 0 {
		t.Errorf("expected 0 devices in office, got %d", len(officeDevices))
	}

	// Verify: total devices for profile
	allDevices, _ := ListDevicesByProfile(database, profile.ID)
	if len(allDevices) != 3 {
		t.Errorf("expected 3 total devices, got %d", len(allDevices))
	}
}

// Test that multiple profiles are fully isolated from each other.
func TestProfileIsolation(t *testing.T) {
	database := setupTestDB(t)

	// Create two separate profiles
	alice, _ := CreateProfile(database, "Alice")
	bob, _ := CreateProfile(database, "Bob")

	// Each creates their own rooms
	CreateRoom(database, alice.ID, "Alice's Office", "desktopcomputer")
	CreateRoom(database, bob.ID, "Bob's Garage", "car")
	CreateRoom(database, bob.ID, "Bob's Kitchen", "fork.knife")

	// Verify room counts are isolated
	aliceRooms, _ := ListRoomsByProfile(database, alice.ID)
	bobRooms, _ := ListRoomsByProfile(database, bob.ID)

	if len(aliceRooms) != 1 {
		t.Errorf("expected Alice to have 1 room, got %d", len(aliceRooms))
	}
	if len(bobRooms) != 2 {
		t.Errorf("expected Bob to have 2 rooms, got %d", len(bobRooms))
	}

	// Deleting Alice should not affect Bob
	DeleteProfile(database, alice.ID)
	bobRoomsAfter, _ := ListRoomsByProfile(database, bob.ID)
	if len(bobRoomsAfter) != 2 {
		t.Errorf("expected Bob to still have 2 rooms after Alice deletion, got %d", len(bobRoomsAfter))
	}
}

// Test reassigning a device from one room to another.
func TestDeviceReassignment(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room1, _ := CreateRoom(database, profile.ID, "Room 1", "1.circle")
	room2, _ := CreateRoom(database, profile.ID, "Room 2", "2.circle")
	device, _ := CreateDevice(database, profile.ID, "Portable Speaker", "generic", nil, nil)

	// Assign to room 1
	assigned, _ := AssignDeviceToRoom(database, device.ID, room1.ID)
	if *assigned.RoomID != room1.ID {
		t.Errorf("expected device in room1, got room '%s'", *assigned.RoomID)
	}

	// Reassign directly to room 2 (no need to unassign first)
	reassigned, _ := AssignDeviceToRoom(database, device.ID, room2.ID)
	if *reassigned.RoomID != room2.ID {
		t.Errorf("expected device in room2 after reassignment, got room '%s'", *reassigned.RoomID)
	}

	// Room 1 should now have 0 devices, room 2 should have 1
	r1Devices, _ := ListDevicesByRoom(database, room1.ID)
	r2Devices, _ := ListDevicesByRoom(database, room2.ID)
	if len(r1Devices) != 0 {
		t.Errorf("expected 0 devices in room1, got %d", len(r1Devices))
	}
	if len(r2Devices) != 1 {
		t.Errorf("expected 1 device in room2, got %d", len(r2Devices))
	}
}
