package db

import (
	"testing"
)

// TestSeedDevData verifies that the seed function creates all expected data.
func TestSeedDevData(t *testing.T) {
	database := setupTestDB(t)

	// Run seed
	err := SeedDevData(database)
	if err != nil {
		t.Fatalf("SeedDevData failed: %v", err)
	}

	// Verify profile was created
	profile, err := GetProfile(database, DevSeedConfig.ProfileID)
	if err != nil {
		t.Fatalf("Expected seeded profile to exist: %v", err)
	}
	if profile.Name != DevSeedConfig.ProfileName {
		t.Errorf("Expected profile name %q, got %q", DevSeedConfig.ProfileName, profile.Name)
	}

	// Verify rooms were created with correct beacon configs
	rooms, err := ListRoomsByProfile(database, DevSeedConfig.ProfileID)
	if err != nil {
		t.Fatalf("Failed to list rooms: %v", err)
	}
	if len(rooms) != len(DevSeedConfig.Rooms) {
		t.Fatalf("Expected %d rooms, got %d", len(DevSeedConfig.Rooms), len(rooms))
	}

	// Build a map of seeded rooms by ID for easy lookup
	seedRoomsByID := make(map[string]seedRoom)
	for _, r := range DevSeedConfig.Rooms {
		seedRoomsByID[r.ID] = r
	}

	for _, room := range rooms {
		expected, ok := seedRoomsByID[room.ID]
		if !ok {
			t.Errorf("Unexpected room ID: %s", room.ID)
			continue
		}
		if room.Name != expected.Name {
			t.Errorf("Room %s: expected name %q, got %q", room.ID, expected.Name, room.Name)
		}
		if room.BeaconUUID == nil || *room.BeaconUUID != expected.BeaconUUID {
			t.Errorf("Room %s: expected beacon UUID %q", room.ID, expected.BeaconUUID)
		}
		if room.BeaconMajor == nil || *room.BeaconMajor != expected.BeaconMajor {
			t.Errorf("Room %s: expected beacon major %d", room.ID, expected.BeaconMajor)
		}
		if room.BeaconMinor == nil || *room.BeaconMinor != expected.BeaconMinor {
			t.Errorf("Room %s: expected beacon minor %d", room.ID, expected.BeaconMinor)
		}
	}

	// Verify devices were created and assigned to correct rooms
	devices, err := ListDevicesByProfile(database, DevSeedConfig.ProfileID)
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}
	if len(devices) != len(DevSeedConfig.Devices) {
		t.Fatalf("Expected %d devices, got %d", len(DevSeedConfig.Devices), len(devices))
	}

	// Build a map of seeded devices by ID for easy lookup
	seedDevicesByID := make(map[string]seedDevice)
	for _, d := range DevSeedConfig.Devices {
		seedDevicesByID[d.ID] = d
	}

	for _, device := range devices {
		expected, ok := seedDevicesByID[device.ID]
		if !ok {
			t.Errorf("Unexpected device ID: %s", device.ID)
			continue
		}
		if device.Name != expected.Name {
			t.Errorf("Device %s: expected name %q, got %q", device.ID, expected.Name, device.Name)
		}
		if device.DeviceType != expected.DeviceType {
			t.Errorf("Device %s: expected type %q, got %q", device.ID, expected.DeviceType, device.DeviceType)
		}
		if device.RoomID == nil || *device.RoomID != expected.RoomID {
			t.Errorf("Device %s (%s): expected room %q, got %v", device.ID, device.Name, expected.RoomID, device.RoomID)
		}
	}
}

// TestSeedDevDataIdempotent verifies that running seed twice doesn't duplicate data.
func TestSeedDevDataIdempotent(t *testing.T) {
	database := setupTestDB(t)

	// Run seed twice
	if err := SeedDevData(database); err != nil {
		t.Fatalf("First SeedDevData failed: %v", err)
	}
	if err := SeedDevData(database); err != nil {
		t.Fatalf("Second SeedDevData failed: %v", err)
	}

	// Verify only one profile exists
	profiles, err := ListProfiles(database)
	if err != nil {
		t.Fatalf("Failed to list profiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Errorf("Expected 1 profile after double seed, got %d", len(profiles))
	}

	// Verify correct number of rooms (not doubled)
	rooms, err := ListRoomsByProfile(database, DevSeedConfig.ProfileID)
	if err != nil {
		t.Fatalf("Failed to list rooms: %v", err)
	}
	if len(rooms) != len(DevSeedConfig.Rooms) {
		t.Errorf("Expected %d rooms after double seed, got %d", len(DevSeedConfig.Rooms), len(rooms))
	}

	// Verify correct number of devices (not doubled)
	devices, err := ListDevicesByProfile(database, DevSeedConfig.ProfileID)
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}
	if len(devices) != len(DevSeedConfig.Devices) {
		t.Errorf("Expected %d devices after double seed, got %d", len(DevSeedConfig.Devices), len(devices))
	}
}
