// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package hostname

import (
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/utils"
)

var (
	startsWithExp = regexp.MustCompile(`^[0-9A-Za-z]`)
	hostnameExp   = regexp.MustCompile(`^[0-9A-Za-z]+[0-9A-Za-z-]*$`)
)

const (
	// MaxHostnameLength is the longest possible username
	MaxHostnameLength = 63
)

// IsValidHostname returns error message or nil if is valid
// https://en.wikipedia.org/wiki/Hostname
func IsValidHostname(hostname string) string {
	if !startsWithExp.MatchString(hostname) {
		return utils.Locale.Get("Hostname can only start with alphanumeric")
	}
	if !hostnameExp.MatchString(hostname) {
		return utils.Locale.Get("Hostname can only contain alphanumeric and hyphen")
	}
	if len(hostname) > MaxHostnameLength {
		return utils.Locale.Get("Hostname can only have a maximum of %d characters", MaxHostnameLength)
	}

	return ""
}

// SetTargetHostname set the new installation target's hostname
func SetTargetHostname(rootDir string, hostname string) error {
	hostDir := filepath.Join(rootDir, "etc")

	if err := utils.MkdirAll(hostDir, 0755); err != nil {
		// Fallback in the unlikely case we can't use root's home
		return errors.Errorf("Failed to create directory (%v) %q", err, hostDir)
	}

	hostFile := filepath.Join(hostDir, "hostname")

	hostBytes := []byte(hostname)

	var err error
	if err = ioutil.WriteFile(hostFile, hostBytes, 0644); err != nil {
		log.Error("Failed to create hostname file (%v) %q", err, hostFile)
	}

	log.Debug("Set Installation Target (%q) hostname to %q", hostFile, hostname)

	return err
}
