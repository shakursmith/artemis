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
	if room.ProfileID == nil || *room.ProfileID != profile.ID {
		t.Errorf("expected profile_id '%s', got %v", profile.ID, room.ProfileID)
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

// =============================================================================
// Custom Template Tests
// =============================================================================

// TestSaveRoomTemplate verifies saving and retrieving a custom template via GetRoom.
func TestSaveRoomTemplate(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Office", "desktopcomputer")

	// Save a custom template.
	templateJSON := `{"id":"custom_office","name":"My Office","backgroundColor":"#AABB00","elements":[]}`
	err := SaveRoomTemplate(database, room.ID, templateJSON)
	if err != nil {
		t.Fatalf("expected no error saving template, got: %v", err)
	}

	// Fetch the room and check the custom template is populated.
	fetched, err := GetRoom(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error fetching room, got: %v", err)
	}
	if fetched.CustomTemplate == nil {
		t.Fatal("expected custom_template to be non-nil after save")
	}
	if *fetched.CustomTemplate != templateJSON {
		t.Errorf("expected template JSON to match, got '%s'", *fetched.CustomTemplate)
	}
}

// TestClearRoomTemplate verifies clearing a custom template sets it back to nil.
func TestClearRoomTemplate(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Bedroom", "bed.double")

	// Save then clear.
	SaveRoomTemplate(database, room.ID, `{"id":"custom","name":"Custom","backgroundColor":"#000","elements":[]}`)
	err := ClearRoomTemplate(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error clearing template, got: %v", err)
	}

	// Fetch and verify it's nil.
	fetched, _ := GetRoom(database, room.ID)
	if fetched.CustomTemplate != nil {
		t.Errorf("expected custom_template to be nil after clear, got '%s'", *fetched.CustomTemplate)
	}
}

// TestCustomTemplateInRoomList verifies ListRoomsByProfile includes custom_template.
func TestCustomTemplateInRoomList(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "sofa")
	templateJSON := `{"id":"custom_lr","name":"My Living Room","backgroundColor":"#FFF","elements":[]}`
	SaveRoomTemplate(database, room.ID, templateJSON)

	// Create a second room without a custom template.
	CreateRoom(database, profile.ID, "Garage", "car")

	rooms, err := ListRoomsByProfile(database, profile.ID)
	if err != nil {
		t.Fatalf("expected no error listing rooms, got: %v", err)
	}

	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(rooms))
	}

	// First room (Living Room) should have the custom template.
	if rooms[0].CustomTemplate == nil {
		t.Error("expected first room to have custom_template set")
	} else if *rooms[0].CustomTemplate != templateJSON {
		t.Errorf("expected custom template to match, got '%s'", *rooms[0].CustomTemplate)
	}

	// Second room (Garage) should have nil custom template.
	if rooms[1].CustomTemplate != nil {
		t.Error("expected second room to have nil custom_template")
	}
}

// =============================================================================
// Business Tests
// =============================================================================

func TestCreateBusiness(t *testing.T) {
	database := setupTestDB(t)

	ownerName := "Alice"
	business, err := CreateBusiness(database, "Yummy Cafe", &ownerName)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the returned business has all expected fields
	if business.ID == "" {
		t.Error("expected business ID to be set")
	}
	if business.Name != "Yummy Cafe" {
		t.Errorf("expected name 'Yummy Cafe', got '%s'", business.Name)
	}
	if business.OwnerName == nil || *business.OwnerName != "Alice" {
		t.Errorf("expected owner name 'Alice', got %v", business.OwnerName)
	}
	if business.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if business.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}
}

func TestCreateBusinessWithoutOwner(t *testing.T) {
	database := setupTestDB(t)

	business, err := CreateBusiness(database, "Coffee Shop", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if business.OwnerName != nil {
		t.Errorf("expected nil owner name, got %v", business.OwnerName)
	}
}

func TestGetBusiness(t *testing.T) {
	database := setupTestDB(t)

	ownerName := "Alice"
	created, _ := CreateBusiness(database, "Yummy Cafe", &ownerName)
	fetched, err := GetBusiness(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
	if fetched.Name != "Yummy Cafe" {
		t.Errorf("expected name 'Yummy Cafe', got '%s'", fetched.Name)
	}
}

func TestGetBusinessNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetBusiness(database, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent business, got nil")
	}
}

func TestListBusinesses(t *testing.T) {
	database := setupTestDB(t)

	// Start with empty list
	businesses, err := ListBusinesses(database)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(businesses) != 0 {
		t.Errorf("expected 0 businesses, got %d", len(businesses))
	}

	// Add two businesses and verify both are returned
	CreateBusiness(database, "Cafe A", nil)
	CreateBusiness(database, "Cafe B", nil)

	businesses, err = ListBusinesses(database)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(businesses) != 2 {
		t.Errorf("expected 2 businesses, got %d", len(businesses))
	}
}

func TestUpdateBusiness(t *testing.T) {
	database := setupTestDB(t)

	ownerName := "Alice"
	created, _ := CreateBusiness(database, "Old Cafe", &ownerName)

	newOwner := "Bob"
	updated, err := UpdateBusiness(database, created.ID, "New Cafe", &newOwner)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updated.Name != "New Cafe" {
		t.Errorf("expected name 'New Cafe', got '%s'", updated.Name)
	}
	if updated.OwnerName == nil || *updated.OwnerName != "Bob" {
		t.Errorf("expected owner name 'Bob', got %v", updated.OwnerName)
	}
	if updated.UpdatedAt.Before(created.CreatedAt) {
		t.Error("expected updated_at to be >= created_at")
	}
}

func TestUpdateBusinessNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UpdateBusiness(database, "nonexistent", "Name", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent business, got nil")
	}
}

func TestDeleteBusiness(t *testing.T) {
	database := setupTestDB(t)

	created, _ := CreateBusiness(database, "Yummy Cafe", nil)
	err := DeleteBusiness(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify it's actually deleted
	_, err = GetBusiness(database, created.ID)
	if err == nil {
		t.Fatal("expected error when fetching deleted business, got nil")
	}
}

func TestDeleteBusinessNotFound(t *testing.T) {
	database := setupTestDB(t)

	err := DeleteBusiness(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent business, got nil")
	}
}

// Test cascade behavior: deleting a business should delete all locations and rooms
func TestDeleteBusinessCascade(t *testing.T) {
	database := setupTestDB(t)

	// Create business → location → room
	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	location, _ := CreateLocation(database, business.ID, "Downtown Store", nil)
	room, _ := CreateRoomForLocation(database, location.ID, "Main Hall", "storefront")

	// Delete the business
	err := DeleteBusiness(database, business.ID)
	if err != nil {
		t.Fatalf("expected no error deleting business, got: %v", err)
	}

	// Verify location is deleted
	_, err = GetLocation(database, location.ID)
	if err == nil {
		t.Error("expected location to be deleted via cascade")
	}

	// Verify room is deleted
	_, err = GetRoom(database, room.ID)
	if err == nil {
		t.Error("expected room to be deleted via cascade")
	}
}

// =============================================================================
// Location Tests
// =============================================================================

func TestCreateLocation(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	address := "123 Main St"
	location, err := CreateLocation(database, business.ID, "Downtown Store", &address)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the returned location has all expected fields
	if location.ID == "" {
		t.Error("expected location ID to be set")
	}
	if location.BusinessID != business.ID {
		t.Errorf("expected business ID '%s', got '%s'", business.ID, location.BusinessID)
	}
	if location.Name != "Downtown Store" {
		t.Errorf("expected name 'Downtown Store', got '%s'", location.Name)
	}
	if location.Address == nil || *location.Address != "123 Main St" {
		t.Errorf("expected address '123 Main St', got %v", location.Address)
	}
	if location.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestCreateLocationWithoutAddress(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	location, err := CreateLocation(database, business.ID, "Airport Branch", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if location.Address != nil {
		t.Errorf("expected nil address, got %v", location.Address)
	}
}

func TestGetLocation(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	address := "456 Oak Ave"
	created, _ := CreateLocation(database, business.ID, "Westside Store", &address)

	fetched, err := GetLocation(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
	if fetched.Name != "Westside Store" {
		t.Errorf("expected name 'Westside Store', got '%s'", fetched.Name)
	}
}

func TestGetLocationNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetLocation(database, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent location, got nil")
	}
}

func TestListLocationsByBusiness(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)

	// Start with empty list
	locations, err := ListLocationsByBusiness(database, business.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(locations) != 0 {
		t.Errorf("expected 0 locations, got %d", len(locations))
	}

	// Add two locations
	CreateLocation(database, business.ID, "Location A", nil)
	CreateLocation(database, business.ID, "Location B", nil)

	locations, err = ListLocationsByBusiness(database, business.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(locations) != 2 {
		t.Errorf("expected 2 locations, got %d", len(locations))
	}
}

func TestUpdateLocation(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	address1 := "Old Address"
	created, _ := CreateLocation(database, business.ID, "Old Name", &address1)

	address2 := "New Address"
	updated, err := UpdateLocation(database, created.ID, "New Name", &address2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("expected name 'New Name', got '%s'", updated.Name)
	}
	if updated.Address == nil || *updated.Address != "New Address" {
		t.Errorf("expected address 'New Address', got %v", updated.Address)
	}
	if updated.UpdatedAt.Before(created.CreatedAt) {
		t.Error("expected updated_at to be >= created_at")
	}
}

func TestUpdateLocationNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UpdateLocation(database, "nonexistent", "Name", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent location, got nil")
	}
}

func TestDeleteLocation(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	created, _ := CreateLocation(database, business.ID, "Store", nil)

	err := DeleteLocation(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify it's actually deleted
	_, err = GetLocation(database, created.ID)
	if err == nil {
		t.Fatal("expected error when fetching deleted location, got nil")
	}
}

func TestDeleteLocationNotFound(t *testing.T) {
	database := setupTestDB(t)

	err := DeleteLocation(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent location, got nil")
	}
}

// Test cascade behavior: deleting a location should delete all rooms
func TestDeleteLocationCascade(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	location, _ := CreateLocation(database, business.ID, "Downtown", nil)
	room, _ := CreateRoomForLocation(database, location.ID, "Main Hall", "storefront")

	// Delete the location
	err := DeleteLocation(database, location.ID)
	if err != nil {
		t.Fatalf("expected no error deleting location, got: %v", err)
	}

	// Verify room is deleted
	_, err = GetRoom(database, room.ID)
	if err == nil {
		t.Error("expected room to be deleted via cascade")
	}
}

func TestListRoomsByLocation(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	location, _ := CreateLocation(database, business.ID, "Downtown", nil)

	// Start with empty list
	rooms, err := ListRoomsByLocation(database, location.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms, got %d", len(rooms))
	}

	// Add two rooms
	CreateRoomForLocation(database, location.ID, "Front", "storefront")
	CreateRoomForLocation(database, location.ID, "Back", "warehouse")

	rooms, err = ListRoomsByLocation(database, location.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}

	// Verify rooms have location_id set
	if rooms[0].LocationID == nil || *rooms[0].LocationID != location.ID {
		t.Errorf("expected location_id '%s', got %v", location.ID, rooms[0].LocationID)
	}
}

func TestCreateRoomForLocation(t *testing.T) {
	database := setupTestDB(t)

	business, _ := CreateBusiness(database, "Chain Cafe", nil)
	location, _ := CreateLocation(database, business.ID, "Downtown", nil)

	room, err := CreateRoomForLocation(database, location.ID, "Main Hall", "storefront")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if room.ID == "" {
		t.Error("expected room ID to be set")
	}
	if room.LocationID == nil || *room.LocationID != location.ID {
		t.Errorf("expected location_id '%s', got %v", location.ID, room.LocationID)
	}
	if room.ProfileID != nil {
		t.Errorf("expected nil profile_id for shop mode room, got %v", room.ProfileID)
	}
	if room.Name != "Main Hall" {
		t.Errorf("expected name 'Main Hall', got '%s'", room.Name)
	}
	if room.Icon != "storefront" {
		t.Errorf("expected icon 'storefront', got '%s'", room.Icon)
	}
}

// =============================================================================
// Room Trigger Tests
// =============================================================================

func TestCreateRoomTrigger(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "house")

	actionConfig := `{"title":"Welcome!","body":"You're home"}`
	trigger, err := CreateRoomTrigger(database, room.ID, "entry", "notification", actionConfig, nil, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the returned trigger has all expected fields
	if trigger.ID == "" {
		t.Error("expected trigger ID to be set")
	}
	if trigger.RoomID != room.ID {
		t.Errorf("expected room ID '%s', got '%s'", room.ID, trigger.RoomID)
	}
	if trigger.TriggerType != "entry" {
		t.Errorf("expected trigger type 'entry', got '%s'", trigger.TriggerType)
	}
	if trigger.ActionType != "notification" {
		t.Errorf("expected action type 'notification', got '%s'", trigger.ActionType)
	}
	if trigger.ActionConfig != actionConfig {
		t.Errorf("expected action config '%s', got '%s'", actionConfig, trigger.ActionConfig)
	}
	if !trigger.Enabled {
		t.Error("expected trigger to be enabled")
	}
	if trigger.DwellSeconds != nil {
		t.Errorf("expected nil dwell_seconds, got %v", trigger.DwellSeconds)
	}
}

func TestCreateRoomTriggerWithDwell(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "house")

	dwellSeconds := 30
	actionConfig := `{"title":"Still here?","body":"Enjoy!"}`
	trigger, err := CreateRoomTrigger(database, room.ID, "dwell", "notification", actionConfig, &dwellSeconds, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if trigger.DwellSeconds == nil || *trigger.DwellSeconds != 30 {
		t.Errorf("expected dwell_seconds 30, got %v", trigger.DwellSeconds)
	}
}

func TestGetRoomTrigger(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "house")
	actionConfig := `{"device_id":"123"}`

	created, _ := CreateRoomTrigger(database, room.ID, "exit", "device_control", actionConfig, nil, false)

	fetched, err := GetRoomTrigger(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
	if fetched.TriggerType != "exit" {
		t.Errorf("expected trigger type 'exit', got '%s'", fetched.TriggerType)
	}
	if fetched.Enabled {
		t.Error("expected trigger to be disabled")
	}
}

func TestGetRoomTriggerNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetRoomTrigger(database, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent trigger, got nil")
	}
}

func TestListTriggersByRoom(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "house")

	// Start with empty list
	triggers, err := ListTriggersByRoom(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(triggers) != 0 {
		t.Errorf("expected 0 triggers, got %d", len(triggers))
	}

	// Add three triggers
	CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Entry"}`, nil, true)
	CreateRoomTrigger(database, room.ID, "exit", "notification", `{"title":"Exit"}`, nil, true)
	dwellSeconds := 10
	CreateRoomTrigger(database, room.ID, "dwell", "device_control", `{"device_id":"123"}`, &dwellSeconds, true)

	triggers, err = ListTriggersByRoom(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(triggers) != 3 {
		t.Errorf("expected 3 triggers, got %d", len(triggers))
	}

	// Verify trigger types
	if triggers[0].TriggerType != "entry" {
		t.Errorf("expected first trigger type 'entry', got '%s'", triggers[0].TriggerType)
	}
	if triggers[1].TriggerType != "exit" {
		t.Errorf("expected second trigger type 'exit', got '%s'", triggers[1].TriggerType)
	}
	if triggers[2].TriggerType != "dwell" {
		t.Errorf("expected third trigger type 'dwell', got '%s'", triggers[2].TriggerType)
	}
	if triggers[2].DwellSeconds == nil || *triggers[2].DwellSeconds != 10 {
		t.Errorf("expected dwell trigger to have dwell_seconds 10, got %v", triggers[2].DwellSeconds)
	}
}

func TestUpdateRoomTrigger(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "house")

	created, _ := CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Old"}`, nil, true)

	newDwell := 20
	updated, err := UpdateRoomTrigger(database, created.ID, "dwell", "device_control", `{"device_id":"456"}`, &newDwell, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updated.TriggerType != "dwell" {
		t.Errorf("expected trigger type 'dwell', got '%s'", updated.TriggerType)
	}
	if updated.ActionType != "device_control" {
		t.Errorf("expected action type 'device_control', got '%s'", updated.ActionType)
	}
	if updated.ActionConfig != `{"device_id":"456"}` {
		t.Errorf("expected action config to be updated, got '%s'", updated.ActionConfig)
	}
	if updated.DwellSeconds == nil || *updated.DwellSeconds != 20 {
		t.Errorf("expected dwell_seconds 20, got %v", updated.DwellSeconds)
	}
	if updated.Enabled {
		t.Error("expected trigger to be disabled")
	}
	if updated.UpdatedAt.Before(created.CreatedAt) {
		t.Error("expected updated_at to be >= created_at")
	}
}

func TestUpdateRoomTriggerNotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := UpdateRoomTrigger(database, "nonexistent", "entry", "notification", `{}`, nil, true)
	if err == nil {
		t.Fatal("expected error for nonexistent trigger, got nil")
	}
}

func TestDeleteRoomTrigger(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "house")
	created, _ := CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Hi"}`, nil, true)

	err := DeleteRoomTrigger(database, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify it's actually deleted
	_, err = GetRoomTrigger(database, created.ID)
	if err == nil {
		t.Fatal("expected error when fetching deleted trigger, got nil")
	}
}

func TestDeleteRoomTriggerNotFound(t *testing.T) {
	database := setupTestDB(t)

	err := DeleteRoomTrigger(database, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent trigger, got nil")
	}
}

// Test cascade behavior: deleting a room should delete all triggers
func TestDeleteRoomCascadesTriggers(t *testing.T) {
	database := setupTestDB(t)

	profile, _ := CreateProfile(database, "Shakur")
	room, _ := CreateRoom(database, profile.ID, "Living Room", "house")
	trigger, _ := CreateRoomTrigger(database, room.ID, "entry", "notification", `{"title":"Hi"}`, nil, true)

	// Delete the room
	err := DeleteRoom(database, room.ID)
	if err != nil {
		t.Fatalf("expected no error deleting room, got: %v", err)
	}

	// Verify trigger is deleted
	_, err = GetRoomTrigger(database, trigger.ID)
	if err == nil {
		t.Error("expected trigger to be deleted via cascade")
	}
}
