// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/user"
	"github.com/clearlinux/clr-installer/utils"
)

func init() {
	utils.SetLocale("en_US.UTF-8")
}

var (
	testsDir string
)

func init() {
	testsDir = os.Getenv("TESTS_DIR")
	testAlias = append(testAlias, "/dev/sda", "/dev/sdb")
}

func TestLoadFile(t *testing.T) {
	tests := []struct {
		file  string
		valid bool
	}{
		{"baseline.yaml", true},
		{"image-generation.yaml", true},
		{"basic-invalid-descriptor.yaml", false},
		{"basic-valid-descriptor.yaml", true},
		{"invalid-no-keyboard.yaml", false},
		{"invalid-no-language.yaml", false},
		{"malformed-descriptor.yaml", false},
		{"no-bootable-descriptor.yaml", false},
		{"no-root-partition-descriptor.yaml", false},
		{"no-telemetry.yaml", false},
		{"invalid-no-kernel.yaml", false},
		{"block-device-image.yaml", true},
		{"block-devices-alias.yaml", true},
		{"mixed-block-device.yaml", true},
		{"real-example.yaml", true},
		{"user-sshkeys.yaml", true},
		{"valid-minimal.yaml", true},
		{"valid-network.yaml", true},
		{"valid-with-pre-post-hooks.yaml", true},
		{"valid-with-version.yaml", true},
		{"iso-bad.yaml", false},
		{"iso-good.yaml", true},
		{"iso-desktop.yaml", true},
		{"azure-config.json", true},
		{"azure-docker-config.json", true},
		{"azure-machine-learning-config.json", true},
		{"ciao-networking-config.json", true},
		{"cloud-config.json", true},
		{"cloud-docker-config.json", true},
		{"gce-config.json", true},
		{"hyperv-config.json", true},
		{"hyperv-mini-config.json", true},
		{"hyperv-test-config.json", true},
		{"kvm-config.json", true},
		{"legacy-kvm-config.json", true},
		{"live-config.json", true},
		{"live-docker-config.json", true},
		{"provision-config.json", true},
		{"vmware-config.json", true},
		{"mbr.json", true},
		{"min-good.json", false},
		{"release-image-config.json", true},
		{"full-good.json", true},
		{"installer-config.json", true},
		{"installer-config-vm.json", true},
		{"ister.json", true},
		{"kernels.json", true},
		{"valid-ister-full-virtual.json", true},
		{"valid-ister-full-physical.json", true},
		{"invalid-ister-basic-descriptor.json", false},
		{"invalid-ister-no-kernel.json", false},
		{"invalid-ister-malformed-descriptor.json", false},
		{"invalid-ister-dt.json", false},
		{"invalid-ister-missing-pl.json", false},
		{"invalid-ister-duplicate-pl.json", false},
		{"invalid-ister-disk-ft.json", false},
		{"invalid-ister-partition-ft.json", false},
		{"invalid-ister-disk-pmp.json", false},
		{"invalid-ister-partition-pmp.json", false},
	}

	for _, curr := range tests {
		path := filepath.Join(testsDir, curr.file)
		if filepath.Ext(curr.file) == ".json" {
			md, err := JSONtoYAMLConfig(path)
			if err == nil {
				path, err = md.WriteYAMLConfig(path)
				defer func() {
					_ = os.Remove(path)
				}()
			}

			if curr.valid && err != nil {
				t.Fatalf("%s is a valid test and shouldn't return an error: %v", curr.file, err)
			}
		}

		model, err := LoadFile(path, args.Args{})

		if curr.valid && err != nil {
			t.Fatalf("%s is a valid tests and shouldn't return an error: %v", curr.file, err)
		}

		err = model.Validate()
		if curr.valid && err != nil {
			t.Fatalf("%s is a valid tests and shouldn't return an error: %v", curr.file, err)
		}
	}
}

func TestIsTestAlias(t *testing.T) {
	testAlias = []string{}

	if isTestAlias("/dev/sda") {
		t.Fatalf("Should have returned false for invalid alias")
	}

	testAlias = append(testAlias, "/dev/sda")
	if !isTestAlias("/dev/sda") {
		t.Fatalf("Should have returned true for valid alias")
	}

	testAlias = append(testAlias, "/dev/sdb")
	if !isTestAlias("/dev/sdb") {
		t.Fatalf("Should have returned true for valid alias")
	}
}

func TestBlockDevicesAlias(t *testing.T) {
	path := filepath.Join(testsDir, "block-devices-alias.yaml")
	model, err := LoadFile(path, args.Args{})

	if err != nil {
		t.Fatalf("Failed to load yaml file: %s", err)
	}

	tm := model.TargetMedias[0]

	if tm.Name != "sda" {
		t.Fatalf("Failed to expand Name variable, value: %s, expected: sda", tm.Name)
	}

	if tm.GetDeviceFile() != "/dev/sda" {
		t.Fatalf("Invalid device name value: %s, expected: /dev/sda", tm.GetDeviceFile())
	}

	for i, bd := range tm.Children {
		expected := fmt.Sprintf("sda%d", i+1)
		expectedFile := filepath.Join("/dev/", expected)

		if bd.Name != expected {
			t.Fatalf("Failed to expand Name variable, value: %s, expected: %s", bd.Name, expected)
		}

		if bd.GetDeviceFile() != expectedFile {
			t.Fatalf("Invalid device name value: %s, expected: %s", bd.GetDeviceFile(), expectedFile)
		}
	}
}

func TestBlockDevicesAliasOverwrite(t *testing.T) {
	path := filepath.Join(testsDir, "block-devices-alias.yaml")
	options := args.Args{BlockDevices: []string{"target:/dev/sdb"}}

	model, err := LoadFile(path, options)

	if err != nil {
		t.Fatalf("Failed to load yaml file: %s", err)
	}

	tm := model.TargetMedias[0]

	if tm.Name != "sdb" {
		t.Fatalf("Failed to expand Name variable, value: %s, expected: sdb", tm.Name)
	}

	if tm.GetDeviceFile() != "/dev/sdb" {
		t.Fatalf("Invalid device name value: %s, expected: /dev/sdb", tm.GetDeviceFile())
	}

	for i, bd := range tm.Children {
		expected := fmt.Sprintf("sdb%d", i+1)
		expectedFile := filepath.Join("/dev/", expected)

		if bd.Name != expected {
			t.Fatalf("Failed to expand Name variable, value: %s, expected: %s", bd.Name, expected)
		}

		if bd.GetDeviceFile() != expectedFile {
			t.Fatalf("Invalid device name value: %s, expected: %s", bd.GetDeviceFile(), expectedFile)
		}
	}
}

func TestInvalidBlockDeviceArgument(t *testing.T) {
	path := filepath.Join(testsDir, "block-devices-alias.yaml")
	options := args.Args{BlockDevices: []string{"invalid"}}

	model, err := LoadFile(path, options)

	if err != nil {
		t.Fatalf("Failed to load yaml file: %s", err)
	}

	if len(model.StorageAlias) != 1 {
		t.Fatalf("The model should contain only 2 storage aliases")
	}

	for _, curr := range model.StorageAlias {
		if curr.Name == "invalid" {
			t.Fatalf("The \"invalid\" block-device argument shouldn't be added to the model")
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
		_ = os.Remove(file.Name())
	}()

	if file.Chmod(0111) != nil {
		t.Fatal("Failed to change tmp file mod")
	}

	if utils.IsRoot() {
		t.Log("Not running as 'root', not checking read permission")
	} else {
		_, err = LoadFile(file.Name(), args.Args{})
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

func TestUserBundle(t *testing.T) {
	si := &SystemInstall{}

	if si.ContainsUserBundle("test-ubundle") {
		t.Fatal("Should return false since test-ubundle wasn't added to si")
	}

	si.AddUserBundle("test-ubundle")
	si.AddUserBundle("test-ubundle-2")
	if !si.ContainsUserBundle("test-ubundle") {
		t.Fatal("Should return true since test-ubundle was added to si")
	}

	si.RemoveUserBundle("test-ubundle")
	if si.ContainsUserBundle("test-ubundle") {
		t.Fatal("Should return false since test-ubundle was removed from si")
	}

	si.RemoveUserBundle("test-ubundle-2")

	// duplicated
	si.AddUserBundle("test-ubundle")
	si.AddUserBundle("test-ubundle")
	if len(si.UserBundles) > 1 {
		t.Fatal("We should have handled the duplication")
	}
}

func TestAddTargetMedia(t *testing.T) {
	path := filepath.Join(testsDir, "basic-valid-descriptor.yaml")
	loaded, err := LoadFile(path, args.Args{})

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

	// Check for encryption passphrase needed; should not
	if nm.EncryptionRequiresPassphrase() {
		t.Fatal("nm.EncryptionRequiresPassphrase() should NOT be true")
	}

	nm.AddTargetMedia(clone)
	if len(nm.TargetMedias) == cl {
		t.Fatal("AddTargetMedia() failed to add a cloned and modified target media")
	}
}

func TestAddNetworkInterface(t *testing.T) {
	path := filepath.Join(testsDir, "valid-network.yaml")
	loaded, err := LoadFile(path, args.Args{})

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	nm := &SystemInstall{}
	nm.AddNetworkInterface(loaded.NetworkInterfaces[0])
	if len(nm.NetworkInterfaces) != 1 {
		t.Fatal("Failed to add network interface to model")
	}
}

func TestDesktopISOType(t *testing.T) {
	path := filepath.Join(testsDir, "iso-desktop.yaml")
	loaded, err := LoadFile(path, args.Args{})

	loaded.AddUserBundle("user-basic")
	loaded.AddUserBundle("python3-basic")

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	if !loaded.IsDesktopInstall() {
		t.Fatal("Failed to detect Desktop ISO install from model")
	}
}

func TestDesktopUserISOType(t *testing.T) {
	path := filepath.Join(testsDir, "iso-good.yaml")
	loaded, err := LoadFile(path, args.Args{})

	loaded.AddUserBundle("user-basic")
	loaded.AddUserBundle("python3-basic")
	loaded.AddUserBundle("desktop-apps")

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	if !loaded.IsDesktopInstall() {
		t.Fatal("Failed to detect Desktop ISO install from model")
	}
}

func TestServerISOType(t *testing.T) {
	path := filepath.Join(testsDir, "iso-good.yaml")
	loaded, err := LoadFile(path, args.Args{})

	loaded.AddUserBundle("user-basic")
	loaded.AddUserBundle("python3-basic")

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	if loaded.IsDesktopInstall() {
		t.Fatal("Detected Desktop ISO install, but should be Server from model")
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
	loaded, err := LoadFile(path, args.Args{})

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	tmpFile, err := ioutil.TempFile("", "test-")
	if err != nil {
		t.Fatal("Could not create a temp file")
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	path = tmpFile.Name()
	if err = tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := loaded.WriteFile(path); err != nil {
		t.Fatal("Failed to write descriptor, should be valid")
	}

	// test writing to an invalid file
	if err := loaded.WriteFile("/invalid-dir/invalid.yaml"); err == nil {
		t.Fatal("Should have failed writing to an invalid file")
	}
}

func TestAddExtraKernelArguments(t *testing.T) {
	args := []string{"arg1", "arg2", "arg3"}

	si := &SystemInstall{}
	si.AddExtraKernelArguments(args)

	if si.KernelArguments == nil {
		t.Fatal("AddExtraKernelArguments() should had created a KernelArguments object")
	}

	if len(si.KernelArguments.Add) != len(args) {
		t.Fatal("AddExtraKernelArguments() didn't add all requested arguments")
	}

	for _, curr := range args {
		if !utils.StringSliceContains(si.KernelArguments.Add, curr) {
			t.Fatal("AddExtraKernelArguments() didn't add all the requested arguments")
		}
	}

	l := len(si.KernelArguments.Add)

	// testing duplication checks
	si.AddExtraKernelArguments(args)

	if l < len(si.KernelArguments.Add) {
		t.Fatal("The duplication check has failed")
	}
}

func TestRemoveKernelArguments(t *testing.T) {
	args := []string{"arg1", "arg2", "arg3"}

	si := &SystemInstall{}
	si.RemoveKernelArguments(args)

	if si.KernelArguments == nil {
		t.Fatal("RemoveKernelArguments() should had created a KernelArguments object")
	}

	if len(si.KernelArguments.Remove) != len(args) {
		t.Fatal("RemoveKernelArguments() didn't add all requested arguments")
	}

	for _, curr := range args {
		if !utils.StringSliceContains(si.KernelArguments.Remove, curr) {
			t.Fatal("RemoveKernelArguments() didn't add all the requested arguments")
		}
	}

	l := len(si.KernelArguments.Remove)

	// testing duplication check
	si.RemoveKernelArguments(args)

	if l < len(si.KernelArguments.Remove) {
		t.Fatal("The duplication check has failed")
	}
}

func TestClearExtraKernelArguments(t *testing.T) {
	args := []string{"arg1", "arg2", "arg3"}

	si := &SystemInstall{}
	si.AddExtraKernelArguments(args)

	si.ClearExtraKernelArguments()

	l := len(si.KernelArguments.Add)

	if l > 0 {
		t.Fatal("Extra Add arguments failed to be cleared")
	}
}

func TestClearRemoveKernelArguments(t *testing.T) {
	args := []string{"arg1", "arg2", "arg3"}

	si := &SystemInstall{}
	si.RemoveKernelArguments(args)

	si.ClearRemoveKernelArguments()

	l := len(si.KernelArguments.Remove)

	if l > len(si.KernelArguments.Remove) {
		t.Fatal("Remove arguments failed to be cleared")
	}
}

func TestAddEncryptedTargetMedia(t *testing.T) {
	path := filepath.Join(testsDir, "encrypt-valid-descriptor.yaml")
	loaded, err := LoadFile(path, args.Args{})

	if err != nil {
		t.Fatal("Failed to load a valid descriptor")
	}

	nm := &SystemInstall{}
	nm.AddTargetMedia(loaded.TargetMedias[0])
	if len(nm.TargetMedias) != 1 {
		t.Fatal("Failed to add target media to model")
	}
	cl := len(nm.TargetMedias)

	// AddTargetMedia() should always add non equal medias
	clone := loaded.TargetMedias[0].Clone()
	clone.Name = clone.Name + "-cloned"

	// Check for encryption passphrase needed; should not
	if !nm.EncryptionRequiresPassphrase() {
		t.Fatal("nm.EncryptionRequiresPassphrase() must always be true")
	}

	nm.AddTargetMedia(clone)
	if len(nm.TargetMedias) == cl {
		t.Fatal("AddTargetMedia() failed to add a cloned and modified target media")
	}
}

func TestBackupFile(t *testing.T) {
	var err error
	path := filepath.Join(testsDir, "valid-ister-full-physical.json")
	cf := strings.TrimSuffix(path, filepath.Ext(path)) + ".yaml"
	md, err := JSONtoYAMLConfig(path)
	if err == nil {
		_, _ = md.WriteYAMLConfig(cf)
		defer func() { _ = os.Remove(cf) }()
	}

	info, err := os.Stat(cf)
	if os.IsNotExist(err) {
		t.Fatalf("%s should already exist and shouldn't return an error: %v", cf, err)
	}

	mt := info.ModTime()
	suffix := fmt.Sprintf("-%d-%02d-%02d-%02d%02d%02d",
		mt.Year(), mt.Month(), mt.Day(),
		mt.Hour(), mt.Minute(), mt.Second())
	bf := strings.TrimSuffix(cf, filepath.Ext(cf)) + suffix + ".yaml"

	md2, err := JSONtoYAMLConfig(path)
	if err == nil {
		defer func() {
			_ = os.Remove(path)
			_ = os.Remove(bf)
		}()
		path, err = md2.WriteYAMLConfig(path)
	}

	if err != nil {
		t.Fatalf("%s is a valid test and shouldn't return an error: %v", path, err)
	}
	info, err = os.Stat(cf)
	if os.IsNotExist(err) {
		t.Fatalf("%s should still exist and shouldn't return an error: %v", cf, err)
	}
	info, err = os.Stat(bf)
	if os.IsNotExist(err) {
		t.Fatalf("%s should exist and shouldn't return an error: %v", cf, err)
	}
}

func TestWriteScrubModelTargetMedias(t *testing.T) {
	si := &SystemInstall{}
	yaml, err := si.WriteScrubModelTargetMedias()
	defer func() { _ = os.Remove(yaml) }()
	if err != nil {
		t.Fatalf("WriteScrubModelTargetMedias shouldn't return an error: %v", err)
	}
}

func TestClearInstallSelected(t *testing.T) {
	si := &SystemInstall{}
	si.ClearInstallSelected()

	si.InstallSelected["name"] = storage.InstallTarget{Name: "name", Friendly: "friendly"}

	if si.InstallSelected["name"].Friendly != "friendly" {
		t.Fatalf("Name %s should exist with friendly as friendly", "name")
	}

	si.ClearInstallSelected()
	if si.InstallSelected["name"].Friendly == "friendly" {
		t.Fatalf("Name %s should NOT exist with friendly as friendly", "name")
	}
}

func TestInterActiveOfflineFail(t *testing.T) {
	si := &SystemInstall{}
	si.ClearInstallSelected()

	si.Offline = true

	if err := si.InteractiveOptionsValid(); err == nil {
		t.Fatalf("Interactive should fail with Offline set to true")
	}
}
func TestInterActiveIsoFail(t *testing.T) {
	si := &SystemInstall{}
	si.ClearInstallSelected()

	si.MakeISO = true

	if err := si.InteractiveOptionsValid(); err == nil {
		t.Fatalf("Interactive should fail with ISO set to true")
	}
}
func TestInterActivePass(t *testing.T) {
	si := &SystemInstall{}
	si.ClearInstallSelected()

	si.Offline = false
	si.MakeISO = false

	if err := si.InteractiveOptionsValid(); err != nil {
		t.Fatalf("Interactive should pass: %v", err)
	}
}
