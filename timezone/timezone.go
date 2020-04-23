// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package timezone

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/utils"
)

// TimeZone represents the system time zone
type TimeZone struct {
	Code        string
	userDefined bool
}

const (
	// DefaultTimezone is the default timezone string
	// This is what is set in os-core
	DefaultTimezone = "UTC"

	// RequiredBundle the bundle needed to set timezone other than the default
	RequiredBundle = "tzdata"
)

// validTimezones stores the list of all valid, known timezones
var validTimezones []*TimeZone

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (tz *TimeZone) IsUserDefined() bool {
	return tz.userDefined
}

// MarshalYAML marshals TimeZone into YAML format
func (tz *TimeZone) MarshalYAML() (interface{}, error) {
	return tz.Code, nil
}

// UnmarshalYAML unmarshals TimeZone from YAML format
func (tz *TimeZone) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var code string

	if err := unmarshal(&code); err != nil {
		return err
	}

	tz.Code = code
	tz.userDefined = false
	return nil
}

// Equals compares tow Timezone instances
func (tz *TimeZone) Equals(comp *TimeZone) bool {
	if comp == nil {
		return false
	}

	return tz.Code == comp.Code
}

// Load uses timedatectl to load the currently available timezones
func Load() ([]*TimeZone, error) {
	if validTimezones != nil {
		return validTimezones, nil
	}
	validTimezones = []*TimeZone{}

	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "timedatectl", "list-timezones")
	if err != nil {
		return nil, err
	}

	tks := strings.Split(w.String(), "\n")
	for _, curr := range tks {
		if curr == "" {
			continue
		}

		tz := &TimeZone{
			Code: curr,
		}

		validTimezones = append(validTimezones, tz)
	}

	return validTimezones, nil
}

// IsValidTimezone verifies if the given keyboard is valid
func IsValidTimezone(t *TimeZone) bool {
	var result = false

	tzs, err := Load()
	if err != nil {
		return result
	}

	for _, curr := range tzs {
		if curr.Equals(t) {
			result = true
		}
	}

	return result
}

// SetTargetTimezone uses creates a symlink to set the timezone on the target
func SetTargetTimezone(rootDir string, timezone string) error {
	tzFile := filepath.Join("/usr/share/zoneinfo", timezone)
	targetTzFile := filepath.Join(rootDir, tzFile)

	if ok, err := utils.FileExists(targetTzFile); err != nil || !ok {
		return fmt.Errorf("Target timezone file missing")
	}

	args := []string{
		"chroot",
		rootDir,
		"ln",
		"-s",
		"-r",
		tzFile,
		"/etc/localtime",
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}
