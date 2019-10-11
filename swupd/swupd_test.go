// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package swupd

import (
	"strings"
	"testing"
	"time"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

func init() {
	utils.SetLocale("en_US.UTF-8")
}

func TestGetHostMirror(t *testing.T) {
	if !utils.IsClearLinux() {
		t.Skip("Not running Clear Linux, skipping test")
	}

	if _, err := GetHostMirror(); err != nil {
		t.Fatalf("Getting Host Mirror failed: %s", err)
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
	if _, err := SetHostMirror(mirror, false); err == nil {
		t.Fatalf("Setting Bad Host Mirror failed: %s", err)
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
	if _, err := SetHostMirror(mirror, false); err != nil {
		t.Fatalf("Setting Good Host Mirror failed: %s", err)
	}

	// Remove the mirror
	if _, err := UnSetHostMirror(); err != nil {
		t.Fatalf("Unsetting Good Host Mirror failed: %s", err)
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
		{"openssh-server", true},
		{"os-core-update", true},
	}

	for _, curr := range tests {
		res := IsCoreBundle(curr.bundle)

		if res != curr.core {
			t.Fatalf("IsCoreBundle() returned %v for %s, expected %v", res, curr.bundle, curr.core)
		}
	}
}

func TestParseSwupdMirrorInvalid(t *testing.T) {
	_, err := parseSwupdMirror([]byte(""))
	if err == nil {
		t.Error("Should fail to parse empty string")
	}
}

func TestNewWithState(t *testing.T) {
	options := args.Args{
		SwupdStateDir: "/tmp/swupd-state",
	}

	si := &model.SystemInstall{}

	sw := New("/tmp/test", options, si)

	if sw.stateDir != "/tmp/swupd-state" {
		t.Fatalf("stateDir should be set to /tmp/swupd-state")
	}

	sw = New("/tmp/test", args.Args{}, si)
	if sw.stateDir != "/tmp/test/var/lib/swupd" {
		t.Fatalf("stateDir should not be set to: %s", sw.stateDir)
	}
}

type MockProgress struct {
	output      string
	description string
	percentage  int
	step        int
}

func (p *MockProgress) Desc(printPrefix, desc string) {
	p.description = strings.Join([]string{printPrefix, desc}, "")
}

func (p *MockProgress) Success() {
	p.output = "success"
}

func (p *MockProgress) Failure() {
	p.output = "failures"
}

func (p *MockProgress) Partial(total int, step int) {
	p.percentage = int((float32(step) / float32(total)) * 100)
}

func (p *MockProgress) Step() {
	p.step++
}

func (p *MockProgress) LoopWaitDuration() time.Duration {
	return time.Second
}

func TestProcess(t *testing.T) {
	var msg Message
	var mp MockProgress
	progress.Set(&mp)

	// messages from a different type than "progress" should be ignored for now
	jsonMsg := "{ \"type\" : \"start\", \"section\" : \"verify\" },"
	msg.Process("", jsonMsg)
	if mp.description != "" || mp.output != "" || mp.step != 0 || mp.percentage != 0 {
		t.Fatal("Message processed incorrectly. Type \"start\" not ignored.")
	}
	jsonMsg = "{ \"type\" : \"info\", \"msg\" : \"Verifying version 10 \" },"
	msg.Process("", jsonMsg)
	if mp.description != "" || mp.output != "" || mp.step != 0 || mp.percentage != 0 {
		t.Fatal("Message processed incorrectly. Type \"info\" not ignored.")
	}
	jsonMsg = "{ \"type\" : \"warning\", \"msg\" : \"helper script not found\" }"
	msg.Process("", jsonMsg)
	if mp.description != "" || mp.output != "" || mp.step != 0 || mp.percentage != 0 {
		t.Fatal("Message processed incorrectly. Type \"warning\" not ignored.")
	}
	jsonMsg = "{ \"type\" : \"end\", \"section\" : \"verify\", \"status\" : 0 }"
	msg.Process("", jsonMsg)
	if mp.description != "" || mp.output != "" || mp.step != 0 || mp.percentage != 0 {
		t.Fatal("Message processed incorrectly. Type \"end\" not ignored.")
	}

	// "progress" messages should be processed correctly
	jsonMsg = "{ \"type\" : \"progress\", \"currentStep\" : 5, \"stepCompletion\" : 80, \"stepDescription\" : \"download_packs\" },"
	msg.Process("", jsonMsg)
	if mp.description != "Downloading required packs" {
		t.Fatal("Message processed incorrectly. Expected: 'Downloading required packs', Actual:", mp.description)
	}
	if mp.percentage != 80 {
		t.Fatal("Message processed incorrectly. Expected: 80, Actual:", mp.percentage)
	}
	if mp.output != "" {
		t.Fatal("Message processed incorrectly. Expected: '', Actual:", mp.output)
	}
	jsonMsg = "{ \"type\" : \"progress\", \"currentStep\" : 8, \"stepCompletion\" : 100, \"stepDescription\" : \"add_missing_files\" },"
	msg.Process("", jsonMsg)
	if mp.description != "Installing base OS and configured bundles" {
		t.Fatal("Message processed incorrectly. Expected: 'Installing base OS and configured bundles', Actual:", mp.description)
	}
	if mp.percentage != 100 {
		t.Fatal("Message processed incorrectly. Expected: 100, Actual:", mp.percentage)
	}
	if mp.output != "success" {
		t.Fatal("Message processed incorrectly. Expected: success, Actual:", mp.output)
	}
}
