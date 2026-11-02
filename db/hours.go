package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// IsOpenNow computes whether a location is currently open based on its timezone,
// regular opening hours, and special hours overrides.
//
// Algorithm:
// 1. Determine the current time in the location's timezone (defaults to server timezone if not set)
// 2. Check if there's a special hours override for today's date
// 3. If special hours exist for today, use those hours; otherwise use regular opening hours
// 4. Check if the current time falls within any of the day's time ranges
//
// Returns true if the location is open now, false otherwise.
// Returns false if opening hours are not configured or if there's an error parsing the data.
func IsOpenNow(location *Location) bool {
	// Determine the timezone for this location
	var loc *time.Location
	if location.Timezone != nil && *location.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(*location.Timezone)
		if err != nil {
			// Invalid timezone, fall back to UTC
			loc = time.UTC
		}
	} else {
		// No timezone set, use UTC
		loc = time.UTC
	}

	// Get current time in the location's timezone
	now := time.Now().In(loc)
	currentDate := now.Format("2006-01-02")
	currentWeekday := now.Weekday()
	currentTime := now.Format("15:04")

	// Check for special hours override for today
	var dayHours *DayHours
	foundSpecialHours := false
	if location.SpecialHours != nil && *location.SpecialHours != "" {
		var specialHours []SpecialHours
		if err := json.Unmarshal([]byte(*location.SpecialHours), &specialHours); err == nil {
			// Find special hours for today's date
			for _, sh := range specialHours {
				if sh.Date == currentDate {
					dayHours = sh.Hours
					foundSpecialHours = true
					break
				}
			}
		}
	}

	// If no special hours for today, use regular opening hours
	if !foundSpecialHours && location.OpeningHours != nil && *location.OpeningHours != "" {
		var openingHours OpeningHours
		if err := json.Unmarshal([]byte(*location.OpeningHours), &openingHours); err == nil {
			// Map weekday to the corresponding day's hours
			switch currentWeekday {
			case time.Monday:
				dayHours = openingHours.Monday
			case time.Tuesday:
				dayHours = openingHours.Tuesday
			case time.Wednesday:
				dayHours = openingHours.Wednesday
			case time.Thursday:
				dayHours = openingHours.Thursday
			case time.Friday:
				dayHours = openingHours.Friday
			case time.Saturday:
				dayHours = openingHours.Saturday
			case time.Sunday:
				dayHours = openingHours.Sunday
			}
		}
	}

	// If no hours configured or closed all day (nil or empty ranges), return false
	if dayHours == nil || len(dayHours.Ranges) == 0 {
		return false
	}

	// Check if current time falls within any of the day's time ranges
	for _, timeRange := range dayHours.Ranges {
		if isTimeInRange(currentTime, timeRange.Open, timeRange.Close) {
			return true
		}
	}

	return false
}

// isTimeInRange checks if the given time falls within the open-close range.
// All times are in HH:MM format (24-hour).
// Handles ranges that span midnight (e.g., 22:00 to 02:00).
func isTimeInRange(current, open, close string) bool {
	// Parse times as HH:MM
	currentMinutes, err := parseTimeToMinutes(current)
	if err != nil {
		return false
	}
	openMinutes, err := parseTimeToMinutes(open)
	if err != nil {
		return false
	}
	closeMinutes, err := parseTimeToMinutes(close)
	if err != nil {
		return false
	}

	// Handle normal range (open < close)
	if openMinutes < closeMinutes {
		return currentMinutes >= openMinutes && currentMinutes < closeMinutes
	}

	// Handle overnight range (close < open, spans midnight)
	// Example: 22:00 to 02:00 means open from 22:00-23:59 OR 00:00-02:00
	return currentMinutes >= openMinutes || currentMinutes < closeMinutes
}

// parseTimeToMinutes converts a time string in HH:MM format to minutes since midnight.
// Example: "09:30" -> 570 (9*60 + 30)
func parseTimeToMinutes(timeStr string) (int, error) {
	var hours, minutes int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &hours, &minutes)
	if err != nil {
		return 0, err
	}
	if hours < 0 || hours > 23 || minutes < 0 || minutes > 59 {
		return 0, fmt.Errorf("invalid time: %s", timeStr)
	}
	return hours*60 + minutes, nil
}
