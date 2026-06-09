package github

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

// PersonalDigestConfig is the top-level structure of the personal digest config file.
type PersonalDigestConfig struct {
	Users []PersonalDigestUserEntry `yaml:"users"`
}

// ParsePersonalDigestConfig reads and validates the personal digest config file at path.
func ParsePersonalDigestConfig(path string) (*PersonalDigestConfig, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg PersonalDigestConfig
	if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decoding personal digest config: %w", err)
	}

	if len(cfg.Users) == 0 {
		return nil, fmt.Errorf("personal digest: users list is empty")
	}

	for i := range cfg.Users {
		if err := applyPersonalDigestDefaults(&cfg.Users[i]); err != nil {
			return nil, fmt.Errorf("personal digest: user %q: %w", cfg.Users[i].Login, err)
		}
	}

	return &cfg, nil
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
