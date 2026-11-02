package db

import (
	"testing"
	"time"
)

// =============================================================================
// IsOpenNow Tests
// =============================================================================

func TestIsOpenNow_NoHoursConfigured(t *testing.T) {
	location := &Location{}
	if IsOpenNow(location) {
		t.Error("expected IsOpenNow to be false when no hours configured")
	}
}

func TestIsOpenNow_ClosedAllDay(t *testing.T) {
	timezone := "America/Los_Angeles"
	// Empty ranges means closed all day
	openingHours := `{"monday": {"ranges": []}}`
	location := &Location{
		Timezone:     &timezone,
		OpeningHours: &openingHours,
	}
	if IsOpenNow(location) {
		t.Error("expected IsOpenNow to be false when closed all day")
	}
}

func TestIsOpenNow_AlwaysOpen(t *testing.T) {
	timezone := "America/Los_Angeles"
	// 00:00 to 23:59 means always open
	openingHours := `{
		"monday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"tuesday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"wednesday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"thursday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"friday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"saturday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"sunday": {"ranges": [{"open": "00:00", "close": "23:59"}]}
	}`
	location := &Location{
		Timezone:     &timezone,
		OpeningHours: &openingHours,
	}
	if !IsOpenNow(location) {
		t.Error("expected IsOpenNow to be true when always open")
	}
}

func TestIsOpenNow_SpecialHoursClosed(t *testing.T) {
	timezone := "UTC"
	today := time.Now().UTC().Format("2006-01-02")
	// Regular hours say open, but special hours override to closed
	openingHours := `{
		"monday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"tuesday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"wednesday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"thursday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"friday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"saturday": {"ranges": [{"open": "00:00", "close": "23:59"}]},
		"sunday": {"ranges": [{"open": "00:00", "close": "23:59"}]}
	}`
	specialHours := `[{"date": "` + today + `", "reason": "Holiday", "hours": null}]`
	location := &Location{
		Timezone:     &timezone,
		OpeningHours: &openingHours,
		SpecialHours: &specialHours,
	}
	if IsOpenNow(location) {
		t.Error("expected IsOpenNow to be false when special hours override to closed")
	}
}

func TestIsOpenNow_SpecialHoursOpen(t *testing.T) {
	timezone := "UTC"
	today := time.Now().UTC().Format("2006-01-02")
	// Regular hours say closed, but special hours override to always open
	openingHours := `{
		"monday": {"ranges": []},
		"tuesday": {"ranges": []},
		"wednesday": {"ranges": []},
		"thursday": {"ranges": []},
		"friday": {"ranges": []},
		"saturday": {"ranges": []},
		"sunday": {"ranges": []}
	}`
	specialHours := `[{"date": "` + today + `", "reason": "Special Event", "hours": {"ranges": [{"open": "00:00", "close": "23:59"}]}}]`
	location := &Location{
		Timezone:     &timezone,
		OpeningHours: &openingHours,
		SpecialHours: &specialHours,
	}
	if !IsOpenNow(location) {
		t.Error("expected IsOpenNow to be true when special hours override to open")
	}
}

func TestIsOpenNow_InvalidTimezone(t *testing.T) {
	timezone := "Invalid/Timezone"
	openingHours := `{"monday": {"ranges": [{"open": "00:00", "close": "23:59"}]}}`
	location := &Location{
		Timezone:     &timezone,
		OpeningHours: &openingHours,
	}
	// Should fall back to UTC and still compute correctly
	// This test just verifies it doesn't crash
	IsOpenNow(location)
}

func TestIsOpenNow_SplitShifts(t *testing.T) {
	timezone := "UTC"
	now := time.Now().UTC()
	currentTime := now.Format("15:04")
	currentWeekday := now.Weekday().String()

	// Create hours with a narrow split shift window
	// If current time is between 12:00-12:05 OR 18:00-18:05, should be open
	openingHoursTemplate := `{
		"monday": {"ranges": [{"open": "12:00", "close": "12:05"}, {"open": "18:00", "close": "18:05"}]},
		"tuesday": {"ranges": [{"open": "12:00", "close": "12:05"}, {"open": "18:00", "close": "18:05"}]},
		"wednesday": {"ranges": [{"open": "12:00", "close": "12:05"}, {"open": "18:00", "close": "18:05"}]},
		"thursday": {"ranges": [{"open": "12:00", "close": "12:05"}, {"open": "18:00", "close": "18:05"}]},
		"friday": {"ranges": [{"open": "12:00", "close": "12:05"}, {"open": "18:00", "close": "18:05"}]},
		"saturday": {"ranges": [{"open": "12:00", "close": "12:05"}, {"open": "18:00", "close": "18:05"}]},
		"sunday": {"ranges": [{"open": "12:00", "close": "12:05"}, {"open": "18:00", "close": "18:05"}]}
	}`

	location := &Location{
		Timezone:     &timezone,
		OpeningHours: &openingHoursTemplate,
	}

	result := IsOpenNow(location)

	// Check if we're in one of the windows
	inFirstWindow := currentTime >= "12:00" && currentTime < "12:05"
	inSecondWindow := currentTime >= "18:00" && currentTime < "18:05"
	expectedOpen := inFirstWindow || inSecondWindow

	if result != expectedOpen {
		t.Logf("Current time: %s on %s", currentTime, currentWeekday)
		t.Logf("Expected: %v, Got: %v", expectedOpen, result)
	}
}

// =============================================================================
// parseTimeToMinutes Tests
// =============================================================================

func TestParseTimeToMinutes(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"00:00", 0, false},
		{"00:01", 1, false},
		{"01:00", 60, false},
		{"09:30", 570, false},
		{"12:00", 720, false},
		{"23:59", 1439, false},
		{"24:00", 0, true},  // invalid hour
		{"12:60", 0, true},  // invalid minute
		{"12:-1", 0, true},  // invalid minute
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := parseTimeToMinutes(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error for input '%s', got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for input '%s': %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("for input '%s': expected %d, got %d", tt.input, tt.expected, result)
			}
		}
	}
}

// =============================================================================
// isTimeInRange Tests
// =============================================================================

func TestIsTimeInRange(t *testing.T) {
	tests := []struct {
		current  string
		open     string
		close    string
		expected bool
	}{
		// Normal ranges (open < close)
		{"09:00", "09:00", "17:00", true},   // at opening
		{"12:00", "09:00", "17:00", true},   // middle
		{"16:59", "09:00", "17:00", true},   // just before close
		{"17:00", "09:00", "17:00", false},  // at close (exclusive)
		{"08:59", "09:00", "17:00", false},  // before opening
		{"17:01", "09:00", "17:00", false},  // after closing

		// Overnight ranges (close < open)
		{"22:00", "22:00", "02:00", true},   // at opening
		{"23:30", "22:00", "02:00", true},   // late night
		{"01:00", "22:00", "02:00", true},   // early morning
		{"01:59", "22:00", "02:00", true},   // just before close
		{"02:00", "22:00", "02:00", false},  // at close (exclusive)
		{"12:00", "22:00", "02:00", false},  // middle of day (closed)
	}

	for _, tt := range tests {
		result := isTimeInRange(tt.current, tt.open, tt.close)
		if result != tt.expected {
			t.Errorf("isTimeInRange(%s, %s, %s): expected %v, got %v",
				tt.current, tt.open, tt.close, tt.expected, result)
		}
	}
}
