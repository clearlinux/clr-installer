// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package telemetry

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
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

	telem.SetUserDefined(true)
	if telem.IsUserDefined() != true {
		t.Fatal("Value for telemetry user defined should be true")
	}

	telem.SetEnable(true)
	if telem.Enabled != true {
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

	err = telem.CreateTelemetryConf(dir)
	if err != nil {
		t.Fatalf("Should have succeeded to write config file: %v\n", err)
	}
}

func TestOptIn(t *testing.T) {
	var url string
	tid := "MyTid"

	if !utils.IsClearLinux() {
		t.Skip("Not a Clear Linux system, skipping test")
	}

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	rootDir, err := ioutil.TempDir("", "root-dir")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(rootDir)
	}()

	// Smoke test
	_ = telem.OptIn(rootDir)
}

func TestOptOut(t *testing.T) {
	var url string
	tid := "MyTid"

	if !utils.IsClearLinux() {
		t.Skip("Not a Clear Linux system, skipping test")
	}

	telem := &Telemetry{
		URL: url,
		TID: tid,
	}

	rootDir, err := ioutil.TempDir("", "root-dir")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(rootDir)
	}()

	// Smoke test
	_ = telem.OptOut(rootDir)
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

	_, err = os.Stat(telemetrySpoolDir)
	if err != nil {
		defer func() {
			_ = os.RemoveAll(telemetrySpoolDir)
		}()

		err = os.Mkdir(telemetrySpoolDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create telemetry spool dir, %s ", err)
		}
	}

	err = telem.CopyTelemetryRecords(dir)
	if err != nil {
		t.Fatalf("Should have succeeded to copy telemetry records, %s", err)
	}
}

// Exercise running environment function
func TestTelemetryRunningEnvironment(t *testing.T) {
	telem := &Telemetry{}
	hypervisor := telem.RunningEnvironment()
	t.Logf("TestTelemetryRunningEnvironment: hypervisor: %s", hypervisor)
}

// Validate randomString
func TestRandomString(t *testing.T) {
	if str, _ := randomString(); len(str) == 0 {
		t.Fatal("randomString should return more than 0 characters")
	}
}

func TestInstalledFail(t *testing.T) {
	telem := &Telemetry{}

	if telem.Installed("/tmp/invalid-dir-name") {
		t.Fatal("/tmp/invalid-dir-name should not have telemetry binary")
	}
}

// Generating record
func TestLogRecord(t *testing.T) {
	telem := &Telemetry{}
	if !utils.IsRoot() {
		t.Skip("Not running as 'root', skipping log record test")
	}

	_, err := os.Stat(telemetrySpoolDir)
	if err != nil {
		defer func() {
			_ = os.RemoveAll(telemetrySpoolDir)
		}()

		err = os.Mkdir(telemetrySpoolDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create telemetry spool dir, %s ", err)
		}
	}

	if err := telem.LogRecord("success", 2, "Hello"); err != nil {
		t.Logf("TestLogRecord failed with %s", err)
	}
}
