// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package network

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/clearlinux/clr-installer/utils"
)

func init() {
	utils.SetLocale("en_US.UTF-8")
}

func TestGoodURL(t *testing.T) {

	if err := CheckURL("http://www.google.com"); err != nil {
		t.Fatalf("Good HTTP URL failed: %s", err)
	}

	if err := CheckURL("https://www.google.com"); err != nil {
		t.Fatalf("Good HTTPS URL failed: %s", err)
	}

	if err := CheckURL("https://cdn.download.clearlinux.org/update/"); err != nil {
		t.Fatalf("Good Clear Linux HTTPS URL failed: %s", err)
	}
}

func TestBadURL(t *testing.T) {

	if err := CheckURL("http://www.google.zonk"); err == nil {
		t.Fatalf("Bad HTTP URL passed incorrectly: %s", err)
	}

	if err := CheckURL("https://www.google.zonk"); err == nil {
		t.Fatalf("Bad HTTPS URL passed incorrectly: %s", err)
	}
}

func TestValidURI(t *testing.T) {
	if !IsValidURI("https://www.google.com", false) {
		t.Fatalf("HTTPS URL failed incorrectly")
	}
	if !IsValidURI("file:///foo", false) {
		t.Fatalf("FILE URL failed incorrectly")
	}
	if IsValidURI("http://www.google.com", false) {
		t.Fatalf("HTTP URL with allowInsecureHTTP set to false passed incorrectly")
	}
	if !IsValidURI("http://www.google.com", true) {
		t.Fatalf("HTTP URL with allowInsecureHTTP set to true failed incorrectly")
	}
	if IsValidURI("ftp://www.google.com", false) {
		t.Fatalf("FTP URL passed incorrectly")
	}
}

func TestIpAddress(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{"10.0.0.1", ""},
		{"192.168.10.1", ""},
		{"10.0.0.0", ""},
		{"0.0.0.0", ""},
		{"0.0.0.0.0", "Invalid IP Addr"},
		{"0.0.0", "Invalid IP Addr"},
	}

	for _, curr := range tests {
		msg := IsValidIP(curr.addr)

		if msg != curr.expected {
			t.Fatalf("IsValidIP() expected to return %s but returned %s", curr.expected, msg)
		}
	}
}

func TestInterfaces(t *testing.T) {
	if utils.IsCheckCoverage() {
		t.Skip("Running on behalf of \"check-coverage\", skipping test")
	}

	ifaces, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	if len(ifaces) == 0 {
		t.Fatalf("Should have returned at least one interface")
	}
}

func TestYaml(t *testing.T) {
	if utils.IsCheckCoverage() {
		t.Skip("Running on behalf of \"check-coverage\", skipping test")
	}

	ifaces, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	marshaled, err := ifaces[0].MarshalYAML()
	if err != nil {
		t.Fatal(err)
	}

	if marshaled == nil {
		t.Fatalf("MarshalYAML() shouldn't have returned nil")
	}
}

func TestAddAddr(t *testing.T) {
	if utils.IsCheckCoverage() {
		t.Skip("Running on behalf of \"check-coverage\", skipping test")
	}

	list, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	iface := list[0]

	// Check for a gateway
	_, err = iface.GetGateway()
	if err != nil {
		t.Fatal(err)
	}
	// Check for a DNS
	_, _, err = iface.GetDNSInfo()
	if err != nil {
		t.Fatal(err)
	}

	ac := len(iface.Addrs)

	iface.AddAddr("10.0.0.1", "255.255.255.0", IPv4)
	if len(iface.Addrs) != ac+1 {
		t.Fatalf("Failed to add address to interface")
	}
}

func TestVersionString(t *testing.T) {
	addr := &Addr{IP: "10.0.0.1", NetMask: "255.255.255.0", Version: IPv4}

	ver := addr.VersionString()
	if ver != "ipv4" {
		t.Fatalf("VersionString() returned wrong value, expected ipv4 but got: %s", ver)
	}

	addr.Version = IPv6
	ver = addr.VersionString()
	if ver != "ipv6" {
		t.Fatalf("VersionString() returned wrong value, expected ipv6 but got: %s", ver)
	}
}

func TestNetmaskToCID(t *testing.T) {
	// test invalid netmask
	_, err := netMaskToCIDR("0")
	if err == nil {
		t.Fatalf("netMaskToCIDR() should have failed")
	}

	tests := []struct {
		mask string
		cidr int
	}{
		{"255.255.255.255", 32},
		{"255.255.255.0", 24},
		{"255.255.0.0", 16},
		{"255.0.0.0", 8},
		{"0.0.0.0", 0},
	}

	for _, curr := range tests {
		res, err := netMaskToCIDR(curr.mask)
		if err != nil {
			t.Fatal(err)
		}

		if res != curr.cidr {
			t.Fatalf("netMaskToCIDR() returned wrong value, expected: %d, got: %d", curr.cidr, res)
		}
	}
}

func TestApply(t *testing.T) {
	if !utils.IsRoot() {
		t.Skip("Not running as 'root', skipping test")
	}

	if utils.IsCheckCoverage() {
		t.Skip("Running on behalf of \"check-coverage\", skipping test")
	}

	dir, err := ioutil.TempDir("", "clr-installer-utest")
	if err != nil {
		t.Fatal(err)
	}

	netService := func() string {
		if IsNetworkManagerActive() {
			return networkManagerDir
		}
		return systemdNetworkdDir
	}

	etcDir := filepath.Join(dir, netService())
	if err = utils.MkdirAll(etcDir, 0755); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	list, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	if len(list) == 0 {
		t.Fatalf("Interfaces() Should have returned at least one interface")
	}

	static := &Interface{
		Name: "test-iface",
		Addrs: []*Addr{
			{"10.0.0.5", "255.255.255.0", IPv4},
		},
		DHCP:        false,
		Gateway:     "10.0.0.101",
		DNSServer:   "10.0.0.101",
		UserDefined: false,
	}

	list = append(list, static)

	// force apply
	for _, curr := range list {
		curr.UserDefined = true
	}

	// Apply again and test the non-interface Apply method
	if err = Apply(dir, list); err != nil {
		t.Error(err)
	}

	// should not fail to re-apply
	if err = Apply(dir, list); err != nil {
		t.Error(err)
	}
}

func TestHasIPv4Addr(t *testing.T) {
	iface := &Interface{}

	iface.AddAddr("10.0.0.1", "255.255.255.0", IPv4)
	if iface.HasIPv4Addr() == false {
		t.Fatalf("Interface has an ipv4 but HasIPv4Addr() returned false")
	}

	iface = &Interface{}
	if iface.HasIPv4Addr() == true {
		t.Fatalf("Interface has no ipv4 but HasIPv4Addr() returned true")
	}
}

func TestGoodDomains(t *testing.T) {
	tests := []struct {
		domain string
	}{
		{"intel.com"},
		{"com"},
		{"domain.google.com"},
		{"long.domain.google.com"},
	}

	for _, curr := range tests {
		if IsValidDomainName(curr.domain) != "" {
			t.Fatalf("'%s' should be a valid domain", curr.domain)
		}
	}
}

func TestBadDomains(t *testing.T) {
	tests := []struct {
		domain string
	}{
		{"intel.com."},
		{"foobar.11"},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com"},
		{"a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a." +
			"a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a." +
			"a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.a.com"},
		{""},
		{"domain.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com"},
		{"-google.com"},
		{"google-.com"},
		{"goo%gle.com"},
		{"g#oogle.com"},
		{"g!oogle.com"},
		{"go^ogle.com"},
	}

	for _, curr := range tests {
		if IsValidDomainName(curr.domain) == "" {
			t.Fatalf("'%s' should be an invalid domain", curr.domain)
		}
	}
}

func TestCopyNetwork(t *testing.T) {
	dir, err := ioutil.TempDir("", "clr-installer-utest")
	if err != nil {
		t.Fatal(err)
	}

	etcDir := filepath.Join(dir, "/etc/systemd/network")
	if err = utils.MkdirAll(etcDir, 0755); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	if err := CopyNetworkInterfaces(dir); err != nil {
		t.Fatalf("CopyNetworkInterfaces should not fail: '%s'", err)
	}
}

func TestGoodDownload(t *testing.T) {

	installDataURLBase = "https://cdn.download.clearlinux.org/releases/%s/clear/config/image/.data/%s"

	if msg := DownloadInstallerMessage("Pre", PreInstallConf); msg != "" {
		t.Fatalf("Good HTTP URL failed: %s", msg)
	}

	if err := CheckURL("https://www.google.com"); err != nil {
		t.Fatalf("Good HTTPS URL failed: %s", err)
	}

	if err := CheckURL("https://cdn.download.clearlinux.org/update/"); err != nil {
		t.Fatalf("Good Clear Linux HTTPS URL failed: %s", err)
	}
}
