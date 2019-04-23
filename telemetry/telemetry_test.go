// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package telemetry

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/clearlinux/clr-installer/cmd"
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
		t.Fatalf("Setting telemetry server should not return error: %s\n", err)
	}

	url = "--3-3-3-3"
	if err := telem.SetTelemetryServer(url, tid, policy); err == nil {
		t.Fatalf("Setting telemetry server should return error: URL=%q, TID=%q, Policy=%q\n", url, tid, policy)
	}
	url = "http://not.real.domain.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err == nil {
		t.Fatalf("Setting telemetry server should return error: URL=%q, TID=%q, Policy=%q\n", url, tid, policy)
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
		t.Fatalf("Setting telemetry server (%q) should not return error: %s\n", url, err)
	}
	if telem.IsUsingPrivateIP() == true {
		t.Fatalf("Telemetry server %q should be a Public IP\n", url)
	}
}

// hello world, the web server
func HelloServer(w http.ResponseWriter, req *http.Request) {
	_, _ = io.WriteString(w, "hello, world!\n")
}
func TestTelemetryServerPrivateIP(t *testing.T) {
	const httpPort = "2222"
	const httpAddr = "192.168.168.168"

	var url string
	tid := "MyTid"
	policy := "Policy of the State"

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	if utils.IsRoot() {
		// Ensure we do not send this address through a proxy
		proxies := []string{httpAddr, os.Getenv("no_proxy")}
		if osErr := os.Setenv("no_proxy", strings.Join(proxies, ",")); osErr != nil {
			t.Fatalf("Failed to modify no_proxy for telemetry testing: %v\n", osErr)
		}
		proxies = []string{httpAddr, os.Getenv("NO_PROXY")}
		if osErr := os.Setenv("NO_PROXY", strings.Join(proxies, ",")); osErr != nil {
			t.Fatalf("Failed to modify NO_PROXY for telemetry testing: %v\n", osErr)
		}

		// https://linuxconfig.org/configuring-virtual-network-interfaces-in-linux
		// Find an interface name to extend
		ifaces, err := net.Interfaces()
		if err != nil {
			t.Fatalf("Failed to read network interfaces for telemetry testing: %v\n", err)
		}
		want := net.FlagUp | net.FlagBroadcast
		var ifaceName string
		for _, iface := range ifaces {
			if iface.Flags&want == want {
				ifaceName = iface.Name
				break
			}
		}
		if ifaceName == "" {
			ifaceName = "lo" // fallback
		}

		args := []string{
			"ifconfig",
			ifaceName + ":0",
			httpAddr,
		}
		cmdErr := cmd.RunAndLog(args...)
		if cmdErr != nil {
			t.Fatalf("Failed to create network interface for telemetry testing: %v\n", cmdErr)
		}

		// Remove this interface after the test
		defer func() {
			args := []string{
				"ifconfig",
				ifaceName + ":0",
				"down",
			}
			cmdErr := cmd.RunAndLog(args...)
			if cmdErr != nil {
				t.Fatalf("Failed to remote network interface for telemetry testing: %v\n", cmdErr)
			}
		}()

		// Start a micro http server
		go func() {
			http.HandleFunc("/", HelloServer)
			if err := http.ListenAndServe(httpAddr+":"+httpPort, nil); err != nil {
				t.Fatalf("Telemetry: Failed to create http server for testing: %s\n", err)
			}
		}()

		url = "http://" + httpAddr + ":" + httpPort + "/"
		_, getErr := http.Get(url)
		retries := 1
		for getErr != nil && retries <= 10 {
			t.Logf("Failed response (%d) from temp HTTP server: %v\n", retries, getErr)
			time.Sleep(time.Second * 1)
			retries++
			_, getErr = http.Get(url)
		}

		if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
			t.Fatalf("Setting telemetry server (%q) should not return error: %s\n", url, err)
		}

		if telem.IsUsingPrivateIP() == false {
			t.Fatalf("Telemetry server %q should be a Private IP\n", url)
		}
	} else {
		t.Skip("Not running as 'root', skipping Private IP test")
	}

	url = "http://www.google.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server (%q) should not return error: %s\n", url, err)
	}
	if telem.IsUsingPrivateIP() == true {
		t.Fatalf("Telemetry server %q should be a Public IP\n", url)
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
		t.Fatalf("Setting telemetry server (%q) should not return error: %s\n", url, err)
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
		t.Fatalf("Failed to create directories to write config file: %v\n", err)
	}
	if copyErr := utils.CopyFile(defaultTelemetryConf, defConfFile); copyErr != nil {
		t.Fatalf("Failed to copy default config file %q: %v\n", defaultTelemetryConf, copyErr)
	}
	err = telem.CreateTelemetryConf(dir)
	if err != nil {
		t.Fatalf("Should have succeeded to write config file: %v\n", err)
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
				t.Fatalf("Failed to copy local conf %q: %v\n", custConf, copyErr)
			}

			defer func() {
				if copyErr := utils.CopyFile(copyConf, custConf); copyErr != nil {
					t.Fatalf("Failed to restore local conf %q: %v\n", custConf, copyErr)
				} else {
					_ = os.Remove(copyConf)
					_ = telem.RestartLocalTelemetryServer()
				}
			}()
		} else {
			if existsErr != nil {
				t.Fatalf("Telemetry: Failed to detect %q file for testing: %s\n", custConf, existsErr)
			}
		}
	} else {
		t.Skip("Not running as 'root', skipping Local Config write test")
	}

	url = "http://www.google.com"
	if err := telem.SetTelemetryServer(url, tid, policy); err != nil {
		t.Fatalf("Setting telemetry server (%q) should not return error: %s\n", url, err)
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
		t.Fatalf("Setting telemetry server (%q) should not return error: %s\n", url, err)
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
			t.Fatalf("Baseline telemetrics.conf file has changed:%s\n", result)
		} else {
			t.Fatalf("Failed to compare telemetrics.conf file : %v\n", err)
		}
	}
}
