// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package keyboard

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
)

// Keymap represents a system' keymap
type Keymap struct {
	Code        string
	userDefined bool
}

const (
	// DefaultKeyboard is the default keyboard string
	// This is what is set in os-core
	DefaultKeyboard = "us"

	// RequiredBundle the bundle needed to set keyboard other than the default
	RequiredBundle = "kbd"
)

// validKeyboards stores the list of all valid, known keyboards
var validKeyboards []*Keymap

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
	if validKeyboards != nil {
		return validKeyboards, nil
	}
	validKeyboards = []*Keymap{}

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

		validKeyboards = append(validKeyboards, &Keymap{Code: curr})
	}

	return validKeyboards, nil
}

// Apply apply the k keymap to the running system
func Apply(k *Keymap) error {
	if err := cmd.RunAndLog("localectl", "set-keymap", k.Code); err != nil {
		return err
	}
	return nil
}

// IsValidKeyboard verifies if the given keyboard is valid
func IsValidKeyboard(k *Keymap) bool {
	var result = false

	kmaps, err := LoadKeymaps()
	if err != nil {
		return result
	}

	for _, curr := range kmaps {
		if curr.Equals(k) {
			result = true
		}
	}

	return result
}

// SetTargetKeyboard creates a keyboard vconsole.conf on the target
func SetTargetKeyboard(rootDir string, keyboard string) error {
	targetKeyboardFile := filepath.Join(rootDir, "/etc/vconsole.conf")

	filehandle, err := os.OpenFile(targetKeyboardFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("Could not create keyboard file")
	}

	defer func() {
		_ = filehandle.Close()
	}()

	if _, err := filehandle.Write([]byte("KEYMAP=" + keyboard + "\n")); err != nil {
		return fmt.Errorf("Could not write keyboard file")
	}

	return nil
}
