// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/clearlinux/clr-installer/user"
	"github.com/clearlinux/clr-installer/utils"
)

var (
	testsDir string
)

func init() {
	testsDir = os.Getenv("TESTS_DIR")
}

func TestLoadFile(t *testing.T) {
	tests := []struct {
		file  string
		valid bool
	}{
		{"basic-invalid-descriptor.yaml", false},
		{"basic-valid-descriptor.yaml", true},
		{"invalid-no-keyboard.yaml", false},
		{"invalid-no-language.yaml", false},
		{"malformed-descriptor.yaml", false},
		{"no-bootable-descriptor.yaml", false},
		{"no-root-partition-descriptor.yaml", false},
		{"no-telemetry.yaml", false},
		{"real-example.yaml", true},
		{"valid-network.yaml", true},
	}

	for _, curr := range tests {
		path := filepath.Join(testsDir, curr.file)
		model, err := LoadFile(path)

		if curr.valid && err != nil {
			t.Fatalf("%s is a valid tests and shouldn't return an error: %v", curr.file, err)
		}

		err = model.Validate()
		if curr.valid && err != nil {
			t.Fatalf("%s is a valid tests and shouldn't return an error: %v", curr.file, err)
		}
	}
}

func TestEnableTelemetry(t *testing.T) {
	si := &SystemInstall{}

	if si.IsTelemetryEnabled() == true {
		t.Fatal("Default value for telemetry should be false")
	}

	// should always succeed
	si.EnableTelemetry(true)
	if si.Telemetry == nil {
		t.Fatal("SystemInstall.EnableTelemetry() should allocate Telemetry object")
	}

	if si.IsTelemetryEnabled() == false {
		t.Fatal("Wrong Telemetry value set or returned")
	}
}

func TestUnreadable(t *testing.T) {
	file, err := ioutil.TempFile("", "test-")
	if err != nil {
		t.Fatal("Could not create a temp file")
	}
	defer func() {
		if err = file.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	if file.Chmod(0111) != nil {
		t.Fatal("Failed to change tmp file mod")
	}

	if utils.IsRoot() {
		t.Log("Not running as 'root', not checking read permission")
	} else {
		_, err = LoadFile(file.Name())
		if err == nil {
			t.Fatal("Should have failed to read")
		}
	}
	if os.Remove(file.Name()) != nil {
		t.Fatal("Failed to cleanup test file")
	}
}

func TestBundle(t *testing.T) {
	si := &SystemInstall{}

	if si.ContainsBundle("test-bundle") {
		t.Fatal("Should return false since test-bundle wasn't added to si")
	}

	si.AddBundle("test-bundle")
	si.AddBundle("test-bundle-2")
	if !si.ContainsBundle("test-bundle") {
		t.Fatal("Should return true since test-bundle was added to si")
	}

	si.RemoveBundle("test-bundle")
	if si.ContainsBundle("test-bundle") {
		t.Fatal("Should return false since test-bundle was removed from si")
	}

	si.RemoveBundle("test-bundle-2")

	// duplicated
	si.AddBundle("test-bundle")
	si.AddBundle("test-bundle")
	if len(si.Bundles) > 1 {
		t.Fatal("We should have handled the duplication")
	}
}

func TestAddTargetMedia(t *testing.T) {
	path := filepath.Join(testsDir, "basic-valid-descriptor.yaml")
	loaded, err := LoadFile(path)

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	nm := &SystemInstall{}
	nm.AddTargetMedia(loaded.TargetMedias[0])
	if len(nm.TargetMedias) != 1 {
		t.Fatal("Failed to add target media to model")
	}

	// the AddTargetMedia() interface must prevent duplication
	cl := len(nm.TargetMedias)
	nm.AddTargetMedia(loaded.TargetMedias[0])
	if len(nm.TargetMedias) != cl {
		t.Fatal("AddTargetMedia() must prevent duplication")
	}

	// AddTargetMedia() should always add non equal medias
	clone := loaded.TargetMedias[0].Clone()
	clone.Name = clone.Name + "-cloned"

	nm.AddTargetMedia(clone)
	if len(nm.TargetMedias) == cl {
		t.Fatal("AddTargetMedia() failed to add a cloned and modified target media")
	}
}

func TestAddNetworkInterface(t *testing.T) {
	path := filepath.Join(testsDir, "valid-network.yaml")
	loaded, err := LoadFile(path)

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	nm := &SystemInstall{}
	nm.AddNetworkInterface(loaded.NetworkInterfaces[0])
	if len(nm.NetworkInterfaces) != 1 {
		t.Fatal("Failed to add network interface to model")
	}
}

func TestUser(t *testing.T) {
	users := []*user.User{
		{Login: "login1", Password: "pwd1", Admin: false},
		{Login: "login2", Password: "pwd2", Admin: false},
		{Login: "login3", Password: "pwd3", Admin: false},
		{Login: "login4", Password: "pwd4", Admin: false},
	}

	si := &SystemInstall{}

	for i, curr := range users {
		si.AddUser(curr)

		if len(si.Users) != i+1 {
			t.Fatal("User wasn't added")
		}
	}

	cl := len(si.Users)

	// don't add same user twice
	si.AddUser(users[0])
	if len(si.Users) != cl {
		t.Fatal("The AddUser() interface should prevent user duplication")
	}

	si.RemoveAllUsers()
	if len(si.Users) != 0 {
		t.Fatal("User list should be empty")
	}

}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(testsDir, "basic-valid-descriptor.yaml")
	loaded, err := LoadFile(path)

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	tmpFile, err := ioutil.TempFile("", "test-")
	if err != nil {
		t.Fatal("Could not create a temp file")
	}
	path = tmpFile.Name()
	if err = tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := loaded.WriteFile(path); err != nil {
		t.Fatal("Failed to write descriptor, should be valid")
	}
}
