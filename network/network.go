// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package network

import (
	"bytes"
	"encoding/json"
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
	"github.com/clearlinux/clr-installer/utils"
)

// Interface is a network interface representation and wraps the net' package Interface struct
type Interface struct {
	Name        string
	Addrs       []*Addr
	DHCP        bool
	Gateway     string `json:"gateway,omitempty"`
	DNSServer   string
	DNSDomain   string
	UserDefined bool
	Metric      uint32 `json:"metric,omitempty"`
}

// Version used for reading and writing YAML
type interfaceYAMLMarshal struct {
	Name      string  `yaml:"name,omitempty"`
	Addrs     []*Addr `yaml:"addrs,omitempty"`
	DHCP      string  `yaml:"dhcp,omitempty"`
	Gateway   string  `yaml:"gateway,omitempty"`
	DNSServer string  `yaml:"dns,omitempty"`
	DNSDomain string  `yaml:"domain,omitempty"`
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
	startsWithExp = regexp.MustCompile(`^[0-9A-Za-z]`)
	endsWithExp   = regexp.MustCompile(`[0-9A-Za-z]$`)
	domainNameExp = regexp.MustCompile(`^[0-9A-Za-z]+[0-9A-Za-z-]*$`)

	numericOnlyExp = regexp.MustCompile(`^[0-9]+[0-9]*$`)

	validIPExp = regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.{1})){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?){1}$`)
	dnsExp     = regexp.MustCompile(`Current DNS Server:(.*)`)
	domainExp  = regexp.MustCompile(`DNS Domain:(.*)`)

	needPacDiscover = false
)

// IsValidDomainName returns error message or nil if is valid
func IsValidDomainName(domain string) string {
	// https://en.wikipedia.org/wiki/Domain_Name_System#Domain_name_syntax,_internationalization

	if len(domain) < 1 {
		return "Required field"
	}

	if len(domain) > 253 {
		return "Domain too long (> 253)"
	}

	labels := strings.Split(domain, ".")

	// This might be 127?
	if len(labels) > 126 {
		return "Domain has too many sub-domains"
	}

	top := labels[len(labels)-1]
	// never end on a dot
	if top == "" {
		return "Dot not allowed as last character"
	}
	// top level domain may not be all-numeric
	if numericOnlyExp.MatchString(top) {
		return "Top level domain can not be numeric"
	}

	for _, label := range labels {
		if len(label) > 63 {
			return fmt.Sprintf("too long (> 63) '%s'", label)
		}

		if !startsWithExp.MatchString(label) {
			return fmt.Sprintf("'%s' may only start with alpha-numeric", label)
		}
		if !endsWithExp.MatchString(label) {
			return fmt.Sprintf("'%s' may only end with alpha-numeric", label)
		}
		if !domainNameExp.MatchString(label) {
			return fmt.Sprintf("sub-domain '%s' may only contain alpha-numeric hyphen", label)
		}
	}

	return ""
}

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (i *Interface) IsUserDefined() bool {
	return i.UserDefined
}

// MarshalYAML marshals Interface into YAML format
func (i *Interface) MarshalYAML() (interface{}, error) {
	var im interfaceYAMLMarshal

	im.Name = i.Name
	im.Addrs = i.Addrs
	im.DHCP = strconv.FormatBool(i.DHCP)
	im.Gateway = i.Gateway
	im.DNSServer = i.DNSServer
	im.DNSDomain = i.DNSDomain

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
	i.DNSServer = im.DNSServer
	i.DNSDomain = im.DNSDomain
	i.UserDefined = false

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

// HasIPv4Addr will lookup an addr with Version set to ipv4
func (i *Interface) HasIPv4Addr() bool {
	for _, curr := range i.Addrs {
		if curr.Version == IPv4 {
			return true
		}
	}

	return false
}

// GetGateway returns the best gateway for the interface
func (i *Interface) GetGateway() (string, error) {
	const (
		maxUint32 = 1<<32 - 1
	)
	var gateway string
	var metric uint32 = maxUint32

	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "ip", "-j", "route", "show", "dev", i.Name)
	if err != nil {
		return "", errors.Wrap(err)
	}

	var interfaces []*Interface //`json:"interfaces"`

	err = json.Unmarshal(w.Bytes(), &interfaces)
	if err != nil {
		return "", errors.Wrap(err)
	}

	for _, inet := range interfaces {
		if inet.Gateway != "" {
			if inet.Metric <= metric {
				gateway = inet.Gateway
				metric = inet.Metric
			}
		}
	}

	return gateway, nil
}

// GetDNSInfo returns the DNS Server and Domain
func (i *Interface) GetDNSInfo() (string, string, error) {
	var dns string
	var domain string

	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "resolvectl", "--no-pager", "status", i.Name)
	if err != nil {
		return dns, domain, errors.Wrap(err)
	}

	lines := strings.Split(w.String(), "\n")
	for _, curr := range lines {
		if curr == "" {
			continue
		}

		if dnsExp.MatchString(curr) {
			dns = strings.TrimSpace(dnsExp.ReplaceAllString(curr, `$1`))
			continue
		}

		if domainExp.MatchString(curr) {
			domain = strings.TrimSpace(domainExp.ReplaceAllString(curr, `$1`))
			continue
		}
	}

	if dns == "" && domain == "" {
		log.Debug("Could not parse DNS Server nor Domain for %s", i.Name)
	} else {
		if domain == "" {
			log.Debug("Could not parse DNS Server for %s", i.Name)
		}
		if dns == "" {
			log.Debug("Could not parse DNS Domain for %s", i.Name)
		}
	}

	return dns, domain, err
}

// VersionString returns a string representation for a given addr version (ipv4/ipv6)
func (a *Addr) VersionString() string {
	if a.Version == IPv4 {
		return "ipv4"
	}

	return "ipv6"
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

		iface.Gateway, err = iface.GetGateway()
		if err != nil {
			return nil, err
		}

		iface.DNSServer, iface.DNSDomain, err = iface.GetDNSInfo()
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
	needPacDiscover = true

	config := `[Match]
Name={{.Name}}

[Network]
DNS={{.DNSServer}}
Address={{.Address}}
Gateway={{.Gateway}}
Domains={{.DNSDomain}}
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
		Name      string
		DNSServer string
		Address   string
		Gateway   string
		DNSDomain string
	}{
		Name:      i.Name,
		DNSServer: i.DNSServer,
		Gateway:   i.Gateway,
		DNSDomain: i.DNSDomain,
		Address:   address,
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
		if err = os.MkdirAll(configDir, 0755); err != nil {
			return errors.Wrap(err)
		}
	}

	for _, curr := range ifaces {
		if !curr.IsUserDefined() {
			log.Info("Interface %s was not changed, skipping config apply.", curr.Name)
			continue
		}

		err := curr.Apply(root)
		if err != nil {
			return err
		}
	}

	if needPacDiscover {
		err := EnablePacDiscovery(root)
		if err != nil {
			return err
		}
	}

	return nil
}

// Restart restarts the network services
func Restart() error {
	err := cmd.RunAndLog("systemctl", "restart", "systemd-networkd",
		"systemd-resolved", "pacdiscovery")
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
		return "Invalid IP Addr"
	}

	return ""
}

// EnablePacDiscovery turns on the pacdiscovery service
// Normally this service is enabled by a DHCP lease path, but
// it must be manually enabled if we set a static IP
func EnablePacDiscovery(rootDir string) error {
	args := []string{
		"chroot",
		rootDir,
		"systemctl",
		"enable",
		"pacdiscovery",
	}

	// Make sure we have an installation environment
	// and not a test environment
	systemctl := filepath.Join(rootDir, "/usr/sbin/systemctl")
	if _, err := os.Stat(systemctl); os.IsNotExist(err) {
		return nil
	}

	if err := cmd.RunAndLog(args...); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// CopyNetworkInterfaces transfer the user defined network interface files
// to the target installation media
func CopyNetworkInterfaces(rootDir string) error {
	const (
		systemNetworkPath = "/etc/systemd/network"
	)

	if _, err := os.Stat(systemNetworkPath); err != nil {
		if os.IsNotExist(err) {
			log.Info("No updated network interfaces to install to target")
			return nil
		}
		return errors.Wrap(err)
	}

	fileInfos, err := ioutil.ReadDir(systemNetworkPath)
	if err != nil {
		return errors.Wrap(err)
	}

	if err = utils.MkdirAll(filepath.Join(rootDir, systemNetworkPath), 0755); err != nil {
		return errors.Wrap(err)
	}

	for _, fileInfo := range fileInfos {
		src := filepath.Join(systemNetworkPath, fileInfo.Name())
		dest := filepath.Join(rootDir, src)
		log.Debug("Copying network interface file '%s' to '%s'", src, dest)
		copyErr := utils.CopyFile(src, dest)
		if copyErr != nil {
			// Only log an error, do not stop
			log.Error("Copy error: %s", copyErr)
		}
	}

	// We likely copied a static IP, so enable PacDiscovery
	err = EnablePacDiscovery(rootDir)
	if err != nil {
		return err
	}

	return nil
}
