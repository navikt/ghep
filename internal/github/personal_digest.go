package github

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	personalDigestDefaultDay      = "friday"
	personalDigestDefaultTime     = "14:00"
	personalDigestDefaultTimezone = "Europe/Oslo"
)

// PersonalDigestUserEntry holds the schedule for a single user's personal digest.
type PersonalDigestUserEntry struct {
	Login    string `yaml:"login"`
	Day      string `yaml:"day"`
	Time     string `yaml:"time"`
	Timezone string `yaml:"timezone"`
}

// applyPersonalDigestDefaults fills in missing fields with defaults and validates the entry.
func applyPersonalDigestDefaults(e *PersonalDigestUserEntry) error {
	if e.Login == "" {
		return fmt.Errorf("login is required")
	}
	if e.Day == "" {
		e.Day = personalDigestDefaultDay
	}
	if e.Time == "" {
		e.Time = personalDigestDefaultTime
	}
	if e.Timezone == "" {
		e.Timezone = personalDigestDefaultTimezone
	}

	if !slices.Contains(validWeekdays, strings.ToLower(e.Day)) {
		return fmt.Errorf("day %q is not a valid weekday", e.Day)
	}
	if _, err := time.Parse("15:04", e.Time); err != nil {
		return fmt.Errorf("time %q must be in HH:MM format", e.Time)
	}
	if _, err := time.LoadLocation(e.Timezone); err != nil {
		return fmt.Errorf("timezone %q is not a valid IANA timezone", e.Timezone)
	}
	return nil
}
