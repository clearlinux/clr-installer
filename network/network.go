// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package network

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
)

// Interface is a network interface representation and wraps the net' package Interface struct
type Interface struct {
	Name        string
	Addrs       []*Addr
	DHCP        bool
	Gateway     string
	DNS         string
	userDefined bool
}

// Version used for reading and writing YAML
type interfaceYAMLMarshal struct {
	Name    string  `yaml:"name,omitempty"`
	Addrs   []*Addr `yaml:"addrs,omitempty"`
	DHCP    string  `yaml:"dhcp,omitempty"`
	Gateway string  `yaml:"gateway,omitempty"`
	DNS     string  `yaml:"dns,omitempty"`
}

// Addr wraps the net' package Addr struct
type Addr struct {
	IP      string
	NetMask string
	Version int
}

const (
	// IPv4 identifies the addr version as ipv4
	IPv4 = iota

	// IPv6 identifies the addr version as ipv6
	IPv6

	configDir = "/etc/systemd/network/"

	versionURLPath = "/usr/share/defaults/swupd/contenturl"
)

var (
	validIPExp = regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.{1})){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?){1}$`)
	gwExp      = regexp.MustCompile(`(default via )(.*)( dev.*)`)
	dnsExp     = regexp.MustCompile("(nameserver) (.*)")
)

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (i *Interface) IsUserDefined() bool {
	return i.userDefined
}

// MarshalYAML marshals Interface into YAML format
func (i *Interface) MarshalYAML() (interface{}, error) {
	var im interfaceYAMLMarshal

	im.Name = i.Name
	im.Addrs = i.Addrs
	im.DHCP = strconv.FormatBool(i.DHCP)
	im.Gateway = i.Gateway
	im.DNS = i.DNS

	return im, nil
}

// UnmarshalYAML unmarshals Interface from YAML format
func (i *Interface) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var im interfaceYAMLMarshal

	if err := unmarshal(&im); err != nil {
		return err
	}

	i.Name = im.Name
	i.Addrs = im.Addrs
	i.Gateway = im.Gateway
	i.DNS = im.DNS
	i.userDefined = false

	if im.DHCP != "" {
		dhcp, err := strconv.ParseBool(im.DHCP)
		if err != nil {
			return err
		}

		i.DHCP = dhcp
	}

	return nil
}

// AddAddr adds a new interface set with the provided arguments to a given Interface
func (i *Interface) AddAddr(IP string, NetMask string, Version int) {
	i.Addrs = append(i.Addrs, &Addr{IP: IP, NetMask: NetMask, Version: Version})
}

// HasIPv4Addr will loopup an addr with Version set to ipv4
func (i *Interface) HasIPv4Addr() bool {
	for _, curr := range i.Addrs {
		if curr.Version == IPv4 {
			return true
		}
	}

	return false
}

// VersionString returns a string representation for a given addr version (ipv4/ipv6)
func (a *Addr) VersionString() string {
	if a.Version == IPv4 {
		return "ipv4"
	}

	return "ipv6"
}

// Gateway return the current default gateway addr
func Gateway() (string, error) {
	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "ip", "route", "show", "default")
	if err != nil {
		return "", errors.Wrap(err)
	}

	result := w.String()
	if result == "" {
		return "", nil
	}

	if !gwExp.MatchString(result) {
		return "", errors.Errorf("Could not parse gateway configuration")
	}

	return strings.TrimSpace(gwExp.ReplaceAllString(result, `$2`)), nil
}

// DNSServer returns the current configured resolver address
func DNSServer() (string, error) {
	var buff []byte
	var err error

	if buff, err = ioutil.ReadFile("/etc/resolv.conf"); err != nil {
		return "", errors.Wrap(err)
	}

	for _, line := range strings.Split(string(buff), "\n") {
		if !dnsExp.MatchString(line) {
			continue
		}

		return strings.TrimSpace(dnsExp.ReplaceAllString(line, `$2`)), nil
	}

	return "", nil
}

func isDHCP(iface string) (bool, error) {
	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "ip", "route", "show")
	if err != nil {
		return false, errors.Wrap(err)
	}

	for _, curr := range strings.Split(w.String(), "\n") {
		if strings.Contains(curr, iface) && strings.Contains(curr, "dhcp") {
			return true, nil
		}
	}

	return false, nil
}

// Interfaces lists all available network interfaces
func Interfaces() ([]*Interface, error) {
	result := []*Interface{}
	var err error

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.Wrap(err)
	}

	for _, curr := range ifaces {
		if curr.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}

		iface := &Interface{Name: curr.Name, Addrs: []*Addr{}}
		result = append(result, iface)

		addrs, err := curr.Addrs()
		if err != nil {
			return nil, errors.Wrap(err)
		}

		for _, cAddr := range addrs {
			var ip net.IP
			var ipNet *net.IPNet

			ip, ipNet, err = net.ParseCIDR(cAddr.String())
			if err != nil {
				return nil, errors.Wrap(err)
			}

			addr := &Addr{IP: ip.String(), NetMask: net.IP(ipNet.Mask).String(), Version: IPv4}

			if ip.To4() == nil {
				addr.Version = IPv6
			}

			iface.Addrs = append(iface.Addrs, addr)
		}

		iface.DHCP, err = isDHCP(curr.Name)
		if err != nil {
			return nil, err
		}

		iface.Gateway, err = Gateway()
		if err != nil {
			return nil, err
		}

		iface.DNS, err = DNSServer()
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func netMaskToCIDR(mask string) (num int, err error) {
	var tks = strings.Split(mask, ".")
	if len(tks) != 4 {
		return 0, errors.Errorf("Invalid mask: %s", mask)
	}

	var result uint32
	for _, octet := range tks {
		bt, err := strconv.ParseInt(octet, 10, 16)

		if err != nil {
			return 0, errors.Wrap(err)
		}

		result = result << 8
		result += uint32(bt)
	}

	bits := 0
	for result > 0 {
		rem := result & 1
		bits += int(rem)
		result = result >> 1
	}

	return bits, nil
}

func (i *Interface) applyStatic(root string, file *os.File) error {
	config := `[Match]
Name={{.Name}}

[Network]
DNS={{.DNS}}
Address={{.Address}}
Gateway={{.Gateway}}
`

	var address string

	for _, curr := range i.Addrs {
		if curr.Version != IPv4 {
			continue
		}

		cidrd, err := netMaskToCIDR(curr.NetMask)
		if err != nil {
			return err
		}

		address = fmt.Sprintf("%s/%d", curr.IP, cidrd)
	}

	template := template.Must(template.New("").Parse(config))
	err := template.Execute(file, struct {
		Name    string
		DNS     string
		Address string
		Gateway string
	}{
		Name:    i.Name,
		DNS:     i.DNS,
		Gateway: i.Gateway,
		Address: address,
	})

	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// Apply does apply the interface configuration to the running system
func (i *Interface) Apply(root string) error {
	fileName := fmt.Sprintf("10-%s.network", i.Name)
	filePath := filepath.Join(root, configDir, fileName)

	if i.DHCP {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return nil
		}

		if err := os.Remove(filePath); err != nil {
			return err
		}

		return nil
	}

	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrap(err)
	}

	return i.applyStatic(root, f)
}

// Apply does apply the configurations of a set of interfaces to the running system
func Apply(root string, ifaces []*Interface) error {
	if root == "" {
		return errors.Errorf("Could not apply network settings, Invalid root directory: %s", root)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err = os.MkdirAll(configDir, 0777); err != nil {
			return errors.Wrap(err)
		}
	}

	for _, curr := range ifaces {
		if !curr.IsUserDefined() {
			log.Info("Interface %s was not changed, skipping config apply.")
			continue
		}

		err := curr.Apply(root)
		if err != nil {
			return err
		}
	}

	return nil
}

// Restart restarts the network services
func Restart() error {
	err := cmd.RunAndLog("systemctl", "restart", "systemd-networkd", "systemd-resolved")
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// VerifyConnectivity tests if the network configuration is working
func VerifyConnectivity() error {
	var versionURL []byte
	var err error

	if versionURL, err = ioutil.ReadFile(versionURLPath); err != nil {
		return errors.Errorf("Read version file %s: %v", versionURLPath, err)
	}

	return CheckURL(string(versionURL))
}

// CheckURL tests if the given URL is accessible
func CheckURL(url string) error {
	args := []string{
		"timeout",
		"--kill-after=10s",
		"10s",
		"curl",
		"--no-sessionid",
		"-o",
		"/dev/null",
		"-s",
		"-f",
		url,
	}

	if err := cmd.Run(nil, args...); err != nil {
		log.Debug("curl failed : %q", err)
		return errors.Wrap(err)
	}

	return nil
}

// IsValidIP returns empty string if IP address is valid
func IsValidIP(str string) string {
	if !validIPExp.MatchString(str) {
		return "Invalid"
	}

	return ""
}
