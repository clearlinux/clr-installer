// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package telemetry

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/utils"
)

var (
	testsDir string
)

func init() {
	testsDir = os.Getenv("TESTS_DIR")
}

func TestTelemetryDefaults(t *testing.T) {
	telem := &Telemetry{}

	if telem.IsUserDefined() == true {
		t.Fatal("Default value for telemetry user defined should be false")
	}

	if telem.IsRequested() == true {
		t.Fatal("Default value for telemetry requested should be false")
	}

	telem.SetRequested(true)
	if telem.IsRequested() != true {
		t.Fatal("Forced value for telemetry requested should be true")
	}

	telem.SetEnable(true)
	if telem.IsUserDefined() != true {
		t.Fatal("Value for telemetry user defined should be true")
	}
}

func TestTelemetryServer(t *testing.T) {
	url := "http://www.google.com"
	tid := "MyTid"
	policy := "Policy of the State"
	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server should not return error: %s", err)
	}

	url = "--3-3-3-3"
	if err := telem.SetTelemetryServer(url, tid, policy); err == nil {
		t.Fatalf("Setting telemetry server should return error: URL=%q, TID=%q, Policy=%q", url, tid, policy)
	}
	url = "http://not.real.domain.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err == nil {
		t.Fatalf("Setting telemetry server should return error: URL=%q, TID=%q, Policy=%q", url, tid, policy)
	}
}

func TestTelemetryServerPublicIP(t *testing.T) {
	var url string
	tid := "MyTid"
	policy := "Policy of the State"

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	url = "http://www.google.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server (%q) should not return error: %s", url, err)
	}
	if telem.IsUsingPrivateIP() == true {
		t.Fatalf("Telemetry server %q should be a Public IP", url)
	}
}

func TestTelemetryServerPrivateIP(t *testing.T) {
	var url string
	tid := "MyTid"
	policy := "Policy of the State"

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	if utils.IsRoot() {
		etcHosts := "/etc/hosts"
		hostData := []byte("192.168.168.168\tmylocalhost\n")
		if doesExist, existsErr := utils.FileExists(etcHosts); doesExist {
			// Save a copy of the hosts file
			copyHosts := etcHosts + ".orig"
			if copyErr := utils.CopyFile(etcHosts, copyHosts); copyErr != nil {
				t.Fatalf("Failed to copy hostfile %q: %v", etcHosts, copyErr)
			}

			defer func() {
				if copyErr := utils.CopyFile(copyHosts, etcHosts); copyErr != nil {
					t.Fatalf("Failed to restore hostfile %q: %v", etcHosts, copyErr)
				} else {
					_ = os.Remove(copyHosts)
				}
			}()

			// Add out host info to the existing file
			appendFile, appendErr := os.OpenFile(etcHosts, os.O_WRONLY|os.O_APPEND, 0644)
			if appendErr != nil {
				t.Fatalf("Failed to open hostfile %q for append: %v", etcHosts, appendErr)
			}
			if _, appendErr = appendFile.Write(hostData); appendErr != nil {
				t.Fatalf("Failed to update hostfile %q during append: %v", etcHosts, appendErr)
			}
			if appendErr = appendFile.Close(); appendErr != nil {
				t.Fatalf("Failed to close hostfile %q during append: %v", etcHosts, appendErr)
			}
		} else {
			if existsErr != nil {
				t.Fatalf("Telemetry: Failed to detect %q file for testing: %s", etcHosts, existsErr)
			}

			// Write the new file
			writeErr := ioutil.WriteFile(etcHosts, hostData, 0644)
			if writeErr != nil {
				t.Fatalf("Telemetry: Failed to create %q file for testing: %s", etcHosts, writeErr)
			}
			defer func() { _ = os.Remove(etcHosts) }()
		}

		url = "http://mylocalhost"
		if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
			t.Fatalf("Setting telemetry server (%q) should not return error: %s", url, err)
		}

		if telem.IsUsingPrivateIP() == false {
			t.Fatalf("Telemetry server %q should be a Private IP", url)
		}
	} else {
		t.Skip("Not running as 'root', skipping Private IP test")
	}

	url = "http://www.google.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server (%q) should not return error: %s", url, err)
	}
	if telem.IsUsingPrivateIP() == true {
		t.Fatalf("Telemetry server %q should be a Public IP", url)
	}
}

func TestWriteTargetConfig(t *testing.T) {
	var url string
	tid := "MyTid"
	policy := "Policy of the State"

	if !utils.IsClearLinux() {
		t.Skip("Not a Clear Linux system, skipping test")
	}

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	url = "http://www.google.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server (%q) should not return error: %s", url, err)
	}

	dir, err := ioutil.TempDir("", "clr-installer-telem-test")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	defConfFile := filepath.Join(dir, defaultTelemetryConf)
	defConfDir := filepath.Dir(defConfFile)
	if err = utils.MkdirAll(defConfDir, 0755); err != nil {
		t.Fatalf("Failed to create directories to write config file: %v", err)
	}
	if copyErr := utils.CopyFile(defaultTelemetryConf, defConfFile); copyErr != nil {
		t.Fatalf("Failed to copy default config file %q: %v", defaultTelemetryConf, copyErr)
	}
	err = telem.CreateTelemetryConf(dir)
	if err != nil {
		t.Fatalf("Should have succeeded to write config file: %v", err)
	}
}

func TestFailedToWriteTargetConfig(t *testing.T) {
	var url string
	tid := "MyTid"

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	if !utils.IsClearLinux() {
		t.Skip("Not a Clear Linux system, skipping test")
	}

	if utils.IsRoot() {
		t.Skip("Running as 'root', skipping test")
	}

	dir, err := ioutil.TempDir("", "clr-installer-telem-test")
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(dir, 0000)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	err = telem.CreateTelemetryConf(dir)
	if err == nil {
		t.Fatal("Should have failed to write config file")
	}
}

func TestWriteLocalConfig(t *testing.T) {
	var url string
	tid := "MyTid"
	policy := "Policy of the State"

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	if !utils.IsClearLinux() {
		t.Skip("Not a Clear Linux system, skipping test")
	}

	if utils.IsRoot() {
		custConf := "/etc/telemetrics/telemetrics.conf"
		if doesExist, existsErr := utils.FileExists(custConf); doesExist {
			// Save a copy of the custom file
			copyConf := custConf + ".orig"
			if copyErr := utils.CopyFile(custConf, copyConf); copyErr != nil {
				t.Fatalf("Failed to copy local conf %q: %v", custConf, copyErr)
			}

			defer func() {
				if copyErr := utils.CopyFile(copyConf, custConf); copyErr != nil {
					t.Fatalf("Failed to restore local conf %q: %v", custConf, copyErr)
				} else {
					_ = os.Remove(copyConf)
					_ = telem.RestartLocalTelemetryServer()
				}
			}()
		} else {
			if existsErr != nil {
				t.Fatalf("Telemetry: Failed to detect %q file for testing: %s", custConf, existsErr)
			}
		}
	} else {
		t.Skip("Not running as 'root', skipping Local Config write test")
	}

	url = "http://www.google.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server (%q) should not return error: %s", url, err)
	}

	err := telem.CreateLocalTelemetryConf()
	if err != nil {
		t.Fatal("Should have succeeded to write local config file")
	}

	err = telem.UpdateLocalTelemetryServer()
	if err != nil {
		t.Fatal("Should have succeeded to update local config file")
	}

	err = telem.StopLocalTelemetryServer()
	if err != nil {
		t.Fatal("Should have succeeded to stop local config file")
	}

	err = telem.RestartLocalTelemetryServer()
	if err != nil {
		t.Fatal("Should have succeeded to restart local config file")
	}
}

func TestCopyRecords(t *testing.T) {
	var url string
	tid := "MyTid"
	policy := "Policy of the State"

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	if !utils.IsClearLinux() {
		t.Skip("Not a Clear Linux system, skipping test")
	}

	if !utils.IsRoot() {
		t.Skip("Not running as 'root', skipping record copy test")
	}

	url = "http://www.google.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server (%q) should not return error: %s", url, err)
	}

	dir, err := ioutil.TempDir("", "clr-installer-telem-test")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	err = telem.CopyTelemetryRecords(dir)
	if err != nil {
		t.Fatal("Should have succeeded to copy telemetry records")
	}
}

// Since the Telemetry login is highly dependent on the contents of
// the default telemetry configuration file, this text ensures that
// the file has not changed.
func TestConfigNotChanged(t *testing.T) {
	if !utils.IsClearLinux() {
		t.Skip("Not a Clear Linux system, skipping test")
	}

	w := bytes.NewBuffer(nil)

	baseConf := filepath.Join(testsDir, "telemetrics.conf")

	err := cmd.Run(w, "diff", baseConf, defaultTelemetryConf)
	if err != nil {
		if result := w.String(); result != "" {
			t.Fatalf("Baseline telemetrics.conf file has changed:\n%s", result)
		} else {
			t.Fatalf("Failed to compare telemetrics.conf file : %v", err)
		}
	}
}
