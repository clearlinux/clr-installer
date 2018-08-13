// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package swupd

import (
	"testing"

	"github.com/clearlinux/clr-installer/utils"
)

func TestGetHostMirror(t *testing.T) {
	if !utils.IsClearLinux() {
		t.Skip("Not running Clear Linux, skipping test")
	}

	if _, err := GetHostMirror(); err != nil {
		t.Fatal("Getting Host Mirror failed")
	}
}

func TestBadSetHostMirror(t *testing.T) {
	if !utils.IsClearLinux() {
		t.Skip("Not running Clear Linux, skipping test")
	}
	if !utils.IsRoot() {
		t.Skip("Not running as 'root', skipping test")
	}

	mirror := "http://www.google.com"
	if _, err := SetHostMirror(mirror); err == nil {
		t.Fatal("Setting Bad Host Mirror failed")
	}
}

func TestGoodSetHostMirror(t *testing.T) {
	if !utils.IsClearLinux() {
		t.Skip("Not running Clear Linux, skipping test")
	}
	if !utils.IsRoot() {
		t.Skip("Not running as 'root', skipping test")
	}

	mirror := "https://download.clearlinux.org/update/"
	//mirror := "http://linux-ftp.jf.intel.com/pub/mirrors/clearlinux/update/"
	if _, err := SetHostMirror(mirror); err != nil {
		t.Fatal("Setting Good Host Mirror failed")
	}

	// Remove the mirror
	if _, err := UnSetHostMirror(); err != nil {
		t.Fatal("Unsetting Good Host Mirror failed")
	}
}

func TestIsCoreBundle(t *testing.T) {
	tests := []struct {
		bundle string
		core   bool
	}{
		{"editors", false},
		{"go-basic", false},
		{"git", false},
		{"games", false},
		{"os-core", true},
		{"os-core-update", true},
	}

	for _, curr := range tests {
		res := IsCoreBundle(curr.bundle)

		if res != curr.core {
			t.Fatalf("IsCoreBundle() returned %v for %s, expected %v", res, curr.bundle, curr.core)
		}
	}
}
