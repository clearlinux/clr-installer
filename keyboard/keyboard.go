// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package keyboard

import (
	"bytes"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
)

// Keymap represents a system' keymap
type Keymap struct {
	Code        string
	userDefined bool
}

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (k *Keymap) IsUserDefined() bool {
	return k.userDefined
}

// MarshalYAML marshals Keymap into YAML format
func (k *Keymap) MarshalYAML() (interface{}, error) {
	return k.Code, nil
}

// UnmarshalYAML unmarshals Keymap from YAML format
func (k *Keymap) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var code string

	if err := unmarshal(&code); err != nil {
		return err
	}

	k.Code = code
	k.userDefined = false
	return nil
}

// Equals compares tow Keymap instances
func (k *Keymap) Equals(comp *Keymap) bool {
	if comp == nil {
		return false
	}

	return k.Code == comp.Code
}

// LoadKeymaps loads the system's available keymaps
func LoadKeymaps() ([]*Keymap, error) {
	result := []*Keymap{}

	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "localectl", "list-keymaps", "--no-pager")
	if err != nil {
		return nil, err
	}

	tks := strings.Split(w.String(), "\n")
	for _, curr := range tks {
		if curr == "" {
			continue
		}

		result = append(result, &Keymap{Code: curr})
	}

	return result, nil
}

// Apply apply the k keymap to the running system
func Apply(k *Keymap) error {
	if err := cmd.RunAndLog("localectl", "set-keymap", k.Code); err != nil {
		return err
	}
	return nil
}
