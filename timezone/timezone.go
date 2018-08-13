// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package timezone

import (
	"bytes"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
)

// TimeZone represents the system time zone
type TimeZone struct {
	Code        string
	userDefined bool
}

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
	result := []*TimeZone{}

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

		result = append(result, tz)
	}

	return result, nil
}
