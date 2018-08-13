// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package hostname

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/clearlinux/clr-installer/utils"
)

func TestEmptyHostname(t *testing.T) {

	var host string
	var err string

	host = ""
	if err = IsValidHostname(host); err == "" {
		t.Fatalf("Empty hostname %q should fail", host)
	}
}

func TestInvalidHostnames(t *testing.T) {

	var host string
	var err string

	host = "-nogood"
	if err = IsValidHostname(host); err == "" {
		t.Fatalf("Hostname %q should fail", host)
	}

	host = "no@good"
	if err = IsValidHostname(host); err == "" {
		t.Fatalf("Hostname %q should fail", host)
	}
}

func TestTooLongHostname(t *testing.T) {

	var host string
	var err string

	host = "1234567890123456789012345678901234567890123456789012345678901234567890"
	if err = IsValidHostname(host); err == "" {
		t.Fatalf("Hostname %q should fail", host)
	}
}

func TestGoodHostnames(t *testing.T) {

	var host string
	var err string

	host = "clear-linux-host"
	if err = IsValidHostname(host); err != "" {
		t.Fatalf("Hostname %q should pass: %q", host, err)
	}

	host = "c"
	if err = IsValidHostname(host); err != "" {
		t.Fatalf("Hostname %q should pass: %q", host, err)
	}

	host = "clear01"
	if err = IsValidHostname(host); err != "" {
		t.Fatalf("Hostname %q should pass: %q", host, err)
	}

	host = "1"
	if err = IsValidHostname(host); err != "" {
		t.Fatalf("Hostname %q should pass: %q", host, err)
	}
}

func TestSaveHostname(t *testing.T) {

	rootDir, err := ioutil.TempDir("", "testhost-")
	if err != nil {
		t.Fatalf("Could not make temp dir for testing hostname: %q", err)
	}

	defer func() { _ = os.RemoveAll(rootDir) }()

	host := "hello"
	if err = SetTargetHostname(rootDir, host); err != nil {
		t.Fatalf("Could not SetTargetHostname to %q: %q", host, err)
	}
}

func TestFailedToCreateDir(t *testing.T) {
	if utils.IsRoot() {
		t.Skip("Not running as 'root', skipping test")
	}

	dir, err := ioutil.TempDir("", "clr-installer-utest")
	if err != nil {
		t.Fatal(err)
	}

	rootDir := filepath.Join(dir, "root")
	if err = utils.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(rootDir, 0000)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	if err = SetTargetHostname(rootDir, "testhost"); err == nil {
		t.Fatalf("Should have failed to create etc dir")
	}
}

func TestFailedToWrite(t *testing.T) {
	if utils.IsRoot() {
		t.Skip("Not running as 'root', skipping test")
	}

	dir, err := ioutil.TempDir("", "clr-installer-utest")
	if err != nil {
		t.Fatal(err)
	}

	etcDir := filepath.Join(dir, "etc")
	if err = utils.MkdirAll(etcDir, 0755); err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(etcDir, 0000)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	err = SetTargetHostname(dir, "testhost")
	if err == nil {
		t.Fatal("Should have failed to write hostname file")
	}
}
