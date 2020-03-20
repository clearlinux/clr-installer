// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package args

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/clearlinux/clr-installer/log"
)

var (
	testHTTPPort string
)

func init() {
	testHTTPPort = os.Getenv("TEST_HTTP_PORT")
	_ = os.Setenv(logFileEnvironVar, "")
}

func makeTestKernelCmd(cmd string) (string, error) {
	kernelCmd := []byte(cmd)
	tmpfile, err := ioutil.TempFile("/tmp", "kargTestCmd")
	if err != nil {
		return "", err
	}
	if _, err := tmpfile.Write(kernelCmd); err != nil {
		return tmpfile.Name(), err
	}
	if err := tmpfile.Close(); err != nil {
		return tmpfile.Name(), err
	}

	return tmpfile.Name(), nil
}

func serveHTTPDescFile(t *testing.T) (*http.Server, error) {
	srv := &http.Server{Addr: ":" + testHTTPPort}

	http.HandleFunc("/clr-installer.yaml", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprintf(w, "{}"); err != nil {
			t.Error(err)
		}
	})

	go func() {
		_ = srv.ListenAndServe()
	}()

	return srv, nil
}

func TestKernelCmdInvalidFile(t *testing.T) {

	var testArgs Args
	var err error

	// Check for read error
	kernelCmdlineFile = "/proc/not-a-real-filename"

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err == nil {
		t.Errorf("Failed to detect a valid error reading kernel command")
		return
	}
}

func TestParseArgsKernelCmdInvalidFile(t *testing.T) {
	var testArgs Args
	var err error

	// Check for read error
	kernelCmdlineFile = "/proc/not-a-real-filename"

	err = testArgs.ParseArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err == nil {
		t.Fatal("Failed to detect a valid error reading kernel command")
	}
}

func TestTelemetry(t *testing.T) {
	var testArgs Args
	var err error

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = append(os.Args, "--telemetry-url=http://telemetry")

	err = testArgs.ParseArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err == nil {
		t.Fatal("Telemetry should require both --telemetry-url and --telemetry-tid")
	}

	os.Args = currArgs
}

func TestKernelCmdDemoTrue(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	// Check for Demo mode set true
	kernelCmd = "root=PARTUUID=694da991-29f6-4cbd-ab72-6da064a799c0 quiet modprobe.blacklist=ccipciedrv,aalbus,aalrms,aalrmc console=tty0 console=ttyS0,115200n8 init=/usr/lib/systemd/systemd-bootchart initcall_debug tsc=reliable no_timer_check noreplace-smp kvm-intel.nested=1 rootfstype=ext4,btrfs,xfs,f2fs intel_iommu=igfx_off cryptomgr.notests rcupdate.rcu_expedited=1 i915.fastboot=1 rcu_nocbs=0-64 rw" + " " + kernelCmdlineDemo
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to setKernelArgs with error %q", err)
		return
	}

	if testArgs.DemoMode == false {
		t.Errorf("Failed to detect Demo Mode TRUE with error kernel command %q", kernelCmd)
	}
}

func TestKernelCmdDemoFalse(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	// Check for Demo mode set false
	kernelCmd = "root=PARTUUID=694da991-29f6-4cbd-ab72-6da064a799c0 quiet modprobe.blacklist=ccipciedrv,aalbus,aalrms,aalrmc console=tty0 console=ttyS0,115200n8 init=/usr/lib/systemd/systemd-bootchart initcall_debug tsc=reliable no_timer_check noreplace-smp kvm-intel.nested=1 rootfstype=ext4,btrfs,xfs,f2fs intel_iommu=igfx_off cryptomgr.notests rcupdate.rcu_expedited=1 i915.fastboot=1 rcu_nocbs=0-64 rw .demo"
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to setKernelArgs with error %q", err)
		return
	}

	if testArgs.DemoMode == true {
		t.Errorf("Failed to detect Demo Mode FALSE with error kernel command %q", kernelCmd)
	}
}

func TestKernelCmdConfPresent(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	// Check for configuration file present
	kernelCmd = "root=PARTUUID=694da991-29f6-4cbd-ab72-6da064a799c0 quiet modprobe.blacklist=ccipciedrv,aalbus,aalrms,aalrmc console=tty0 console=ttyS0,115200n8 init=/usr/lib/systemd/systemd-bootchart initcall_debug tsc=reliable no_timer_check noreplace-smp kvm-intel.nested=1 rootfstype=ext4,btrfs,xfs,f2fs intel_iommu=igfx_off cryptomgr.notests rcupdate.rcu_expedited=1 i915.fastboot=1 rcu_nocbs=0-64 rw" +
		" " + kernelCmdlineConf + "=http://google.com"
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to setKernelArgs with error %q", err)
		return
	}

	if testArgs.ConfigFile == "" {
		t.Errorf("Failed to detect Configuration File with kernel command %q", kernelCmd)
	}
}

func TestKernelCmdLogPresent(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	forcedLogLevel := "1"

	// Check for configuration file present
	kernelCmd = "root=PARTUUID=694da991-29f6-4cbd-ab72-6da064a799c0 quiet modprobe.blacklist=ccipciedrv,aalbus,aalrms,aalrmc console=tty0 console=ttyS0,115200n8 init=/usr/lib/systemd/systemd-bootchart initcall_debug tsc=reliable no_timer_check noreplace-smp kvm-intel.nested=1 rootfstype=ext4,btrfs,xfs,f2fs intel_iommu=igfx_off cryptomgr.notests rcupdate.rcu_expedited=1 i915.fastboot=1 rcu_nocbs=0-64 rw" +
		" " + kernelCmdlineLog + "=" + forcedLogLevel
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to setKernelArgs with error %q", err)
		return
	}

	var logLevel int
	if logLevel, _ = strconv.Atoi(forcedLogLevel); err != nil {
		t.Errorf("Invalid logLevel value '%s'", forcedLogLevel)
	}
	if testArgs.LogLevel != logLevel {
		t.Errorf("Failed to detect Log Level with kernel command %q", kernelCmd)
	}
}

func TestKernelCmdLogError(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	forcedLogLevel := "a"

	// Check for configuration file present
	kernelCmd = "root=PARTUUID=694da991-29f6-4cbd-ab72-6da064a799c0 quiet modprobe.blacklist=ccipciedrv,aalbus,aalrms,aalrmc console=tty0 console=ttyS0,115200n8 init=/usr/lib/systemd/systemd-bootchart initcall_debug tsc=reliable no_timer_check noreplace-smp kvm-intel.nested=1 rootfstype=ext4,btrfs,xfs,f2fs intel_iommu=igfx_off cryptomgr.notests rcupdate.rcu_expedited=1 i915.fastboot=1 rcu_nocbs=0-64 rw" +
		" " + kernelCmdlineLog + "=" + forcedLogLevel
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to setKernelArgs with error %q", err)
		return
	}

	if testArgs.LogLevel != 0 {
		t.Errorf("Failed to detect Log Level with bad kernel command %q", kernelCmd)
	}
}

func TestKernelCmdFileProtocol(t *testing.T) {
	var testArgs Args
	var kernelCmd string
	var err error

	// the remote fetch supports only http protocol for now
	kernelCmd = kernelCmdlineConf + "=file:///proc/cmdline"
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("setKernelArgs() should not fail with FILE protocol")
		return
	}
}

func TestKernelCmdValidFetch(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	// Check for configuration file present
	kernelCmd = kernelCmdlineConf + "=http://localhost:" + testHTTPPort + "/clr-installer.yaml"
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	srv, err := serveHTTPDescFile(t)
	if err != nil {
		t.Fatalf("Failed to serve http desc file with error %q", err)
	}

	defer func() {
		if err = srv.Shutdown(nil); err != nil {
			t.Fatal(err)
		}
	}()

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to setKernelArgs with error %q", err)
		return
	}

	if testArgs.ConfigFile == "" {
		t.Errorf("Failed to detect Configuration File with kernel command %q", kernelCmd)
	}
}

func TestKernelCmdConfEmpty(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	// Check for configuration file missing
	kernelCmd = "root=PARTUUID=694da991-29f6-4cbd-ab72-6da064a799c0 quiet modprobe.blacklist=ccipciedrv,aalbus,aalrms,aalrmc console=tty0 console=ttyS0,115200n8 init=/usr/lib/systemd/systemd-bootchart initcall_debug tsc=reliable no_timer_check noreplace-smp kvm-intel.nested=1 rootfstype=ext4,btrfs,xfs,f2fs intel_iommu=igfx_off cryptomgr.notests rcupdate.rcu_expedited=1 i915.fastboot=1 rcu_nocbs=0-64 rw" +
		" " + "nothere"
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.setKernelArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to setKernelArgs with error %q", err)
		return
	}

	if testArgs.ConfigFile != "" {
		t.Errorf("Found Configuration File value when should be empty with kernel command %q", kernelCmd)
	}
}

func TestConvertArg(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2], "--json-yaml", "fubar.json"}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()

	os.Args = currArgs
	if err != nil {
		t.Fatal("Failed to parse arguments")
	}
	t.Logf("testArgs.ConvertConfigFile: %s", testArgs.ConvertConfigFile)
	if testArgs.ConvertConfigFile != "fubar.json" {
		t.Fatal("Failed to parse config file for --json-yaml")
	}
}

func TestBundleArg(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2], "--bundles", "os-core,os-core-update"}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()

	os.Args = currArgs
	if err != nil {
		t.Fatal("Failed to parse arguments")
	}
	t.Logf("testArgs.Bundles: %v", testArgs.Bundles)
	if len(testArgs.Bundles) != 2 {
		t.Fatal("Failed to parse bundles")
	}
}

func TestTemplateArg(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	_ = os.Setenv(logFileEnvironVar, "/tmp/fubar.log")

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2], "--template", "fubar.yaml"}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()

	os.Args = currArgs
	if err != nil {
		t.Fatal("Failed to parse arguments")
	}
	t.Logf("testArgs.TemplateConfigFile: %s", testArgs.TemplateConfigFile)
	if testArgs.TemplateConfigFile != "fubar.yaml" {
		t.Fatal("Failed to parse config file for --template")
	}
}

func TestSwupdUrlMirrorFailArg(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2],
		"--swupd-url", "https://cdn.download.clearlinux.org/update/",
		"--swupd-mirror", "https://cdn.download.clearlinux.org/update/",
	}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()
	os.Args = currArgs
	if err == nil {
		t.Fatal("Should have failed to parse arguments")
	}
}
func TestSwupdUrlContentArg(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2],
		"--swupd-url", "https://cdn.download.clearlinux.org/update/",
		"--swupd-contenturl", "https://cdn.download.clearlinux.org/update/",
	}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()
	os.Args = currArgs
	if err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}
}
func TestSwupdUrlVersionArg(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2],
		"--swupd-url", "https://cdn.download.clearlinux.org/update/",
		"--swupd-versionurl", "https://cdn.download.clearlinux.org/update/",
	}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()
	os.Args = currArgs
	if err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}
}

func TestCheckAllBooleans(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2],
		"--demo", "--telemetry", "--reboot",
		"--iso", "--keep-image", "--allow-insecure-http", "--offline",
		"--cfPurge", "--swupd-skip-optional", "--archive", "--copy-swupd", "--high-contrast",
		"--skip-validation-size", "--skip-validation-all",
	}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()
	os.Args = currArgs
	if err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}
}
func TestCheckAllBooleansFalse(t *testing.T) {
	var testArgs Args

	currArgs := make([]string, len(os.Args))
	copy(currArgs, os.Args)

	os.Args = []string{currArgs[0], currArgs[1], currArgs[2],
		"--demo=0", "--telemetry=0", "--reboot=0",
		"--iso=0", "--keep-image=0", "--allow-insecure-http=0", "--offline=0",
		"--cfPurge=0", "--swupd-skip-optional=0", "--archive=0", "--copy-swupd=0", "--high-contrast=0",
		"--skip-validation-size=0", "--skip-validation-all=0",
	}
	t.Logf("Current os.Args: %v", os.Args)

	err := testArgs.setCommandLineArgs()
	os.Args = currArgs
	if err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}
}

func TestKernelAndCommandlineAllArgs(t *testing.T) {

	var testArgs Args
	var kernelCmd string
	var err error

	const confName = "command.conf"
	t.Logf("%v", os.Args)
	os.Args = append(os.Args, "--config="+confName, "--demo", "--telemetry", "--reboot",
		"--iso", "--keep-image", "--allow-insecure-http", "--offline",
		"--swupd-url", "https://cdn.download.clearlinux.org/update/")
	fmt.Println(os.Args)

	// Check for configuration file missing
	kernelCmd = "root=PARTUUID=694da991-29f6-4cbd-ab72-6da064a799c0 quiet modprobe.blacklist=ccipciedrv,aalbus,aalrms,aalrmc console=tty0 console=ttyS0,115200n8 init=/usr/lib/systemd/systemd-bootchart initcall_debug tsc=reliable no_timer_check noreplace-smp kvm-intel.nested=1 rootfstype=ext4,btrfs,xfs,f2fs intel_iommu=igfx_off cryptomgr.notests rcupdate.rcu_expedited=1 i915.fastboot=1 rcu_nocbs=0-64 rw" +
		" " + kernelCmdlineConf + "=http://google.com"
	kernelCmdlineFile, err = makeTestKernelCmd(kernelCmd)
	defer func() {
		_ = os.Remove(kernelCmdlineFile)
	}()
	if err != nil {
		t.Errorf("Failed to makeTestKernelCmd with error %q", err)
		return
	}

	err = testArgs.ParseArgs()
	if testArgs.CfDownloaded && testArgs.ConfigFile != "" {
		defer func() { _ = os.Remove(testArgs.ConfigFile) }()
	}
	if err != nil {
		t.Errorf("Failed to ParseArgs with error %q", err)
		return
	}

	if testArgs.Version != false {
		t.Errorf("Command Line 'version' is not defaulted to 'false'")
	}
	if testArgs.Reboot != true {
		t.Errorf("Command Line 'reboot' is not defaulted to 'true'")
	}
	if testArgs.Offline != true {
		t.Errorf("Command Line 'offline' is not defaulted to 'true'")
	}
	if testArgs.MakeISO != true {
		t.Errorf("Command Line 'iso' is not defaulted to 'true'")
	}
	if testArgs.AllowInsecureHTTP != true {
		t.Errorf("Command Line 'allow-insecure-http' is not defaulted to 'true'")
	}
	if testArgs.KeepImage != true {
		t.Errorf("Command Line 'keep-image' is not defaulted to 'true'")
	}
	if testArgs.ConfigFile != confName {
		t.Errorf("Command Line 'config' is %q, NOT overridden to %q", testArgs.ConfigFile, confName)
	}
	if testArgs.SwupdMirror != "" {
		t.Errorf("Command Line 'mirror' is not defaulted to ''")
	}
	if testArgs.SwupdSkipOptional != false {
		t.Errorf("Command Line '--swupd-skip-optional' is not defaulted to 'false'")
	}
	if testArgs.SwupdVersion != "" {
		t.Errorf("Command Line 'swupd-version' is not defaulted to ''")
	}
	if testArgs.PamSalt != "" {
		t.Errorf("Command Line 'genpwd' is not defaulted to ''")
	}
	if testArgs.LogLevel != log.LogLevelDebug {
		t.Errorf("Command Line 'log-level' is not defaulted to '%d'", log.LogLevelDebug)
	}
	if testArgs.LogFile == "" {
		t.Errorf("Command Line 'log-file' is NOT set to value")
	}
	if testArgs.SwupdURL == "" {
		t.Errorf("Command Line 'swupd-url' is NOT set to value")
	}
	if testArgs.SkipValidationSize != false {
		t.Errorf("Command Line '--skip-validation-size' is not defaulted to 'false'")
	}
	if testArgs.SkipValidationAll != false {
		t.Errorf("Command Line '--skip-validation-all' is not defaulted to 'false'")
	}
}
