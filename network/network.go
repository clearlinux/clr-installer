// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/coreos/go-systemd/dbus"
	"gopkg.in/yaml.v2"

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

// LangString hold strings for each translated language
// and one for default if the current language is unavailable
type LangString struct {
	Default string `yaml:"default,omitempty"`
	EnUs    string `yaml:"en_US,omitempty"`
}

// Messenger is an array of language string used to create
// an end user message for display
type Messenger struct {
	Messages []*LangString `yaml:"install-msg,omitempty"`
}

const (
	// RequiredBundle the bundle needed to use NetworkManager
	RequiredBundle = "NetworkManager"

	// IPv4 identifies the addr version as ipv4
	IPv4 = iota

	// IPv6 identifies the addr version as ipv6
	IPv6

	systemdNetworkdDir = "/etc/systemd/network"
	networkManagerDir  = "/etc/NetworkManager/system-connections"

	versionURLPath = "/usr/share/defaults/swupd/contenturl"

	// PreInstallConf is the name of the pre-installation message file
	PreInstallConf = "pre-install-msg.yaml"
	// PostInstallConf is the name of the pre-installation message file
	PostInstallConf = "post-install-msg.yaml"
	// PreGuiInstallConf is the name of the pre-installation message file
	PreGuiInstallConf = "pre-gui-install-msg.yaml"
	// PostGuiInstallConf is the name of the pre-installation message file
	PostGuiInstallConf = "post-gui-install-msg.yaml"
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

	installDataURLBase = "https://cdn.download.clearlinux.org/releases/%s/clear/config/image/.data/%s"
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
			return fmt.Sprintf("'%s' may only start with alphanumeric", label)
		}
		if !endsWithExp.MatchString(label) {
			return fmt.Sprintf("'%s' may only end with alphanumeric", label)
		}
		if !domainNameExp.MatchString(label) {
			return fmt.Sprintf("sub-domain '%s' may only contain alphanumeric hyphen", label)
		}
	}

	return ""
}

// IsValidURI checks for valid URIs that use the HTTPS or FILE protocol
func IsValidURI(uri string, allowInsecureHTTP bool) bool {
	_, err := url.ParseRequestURI(uri)
	if err != nil {
		return false
	}

	httpsPrefix := strings.HasPrefix(strings.ToLower(uri), "https:")
	if httpsPrefix {
		return true
	}

	filePrefix := strings.HasPrefix(strings.ToLower(uri), "file:")
	if filePrefix {
		return true
	}

	httpPrefix := strings.HasPrefix(strings.ToLower(uri), "http:")
	if httpPrefix {
		if allowInsecureHTTP {
			return true
		}
		msg := "HTTP is disabled, pass --allow-insecure-http to enable HTTP"
		fmt.Println(msg)
		log.Info(msg)
	}

	return false
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
	// TODO: Should we remove the absolute path? Absolute file path is used to ensure pkexec doesn't mess up PATH.
	err := cmd.Run(w, "/usr/bin/ip", "-j", "route", "show", "dev", i.Name)
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
	err := cmd.Run(w, "/usr/bin/ip", "route", "show")
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

// IsNetworkManagerActive is used to
// check if we are using systemd.networkd or NetworkManager to
// manage the wired connections.
// Clear Linux OS change from system.networkd to NetworkManager
// in April 2019 build ?????
func IsNetworkManagerActive() bool {
	const (
		sysNetStr = "systemd-networkd.service"
		netMgrStr = "NetworkManager.service"
	)
	// Assume we are new school
	networkManager := true
	nm := false
	systemd := false

	// Make the new dbus connection
	conn, err := dbus.New()
	defer conn.Close()
	if err != nil {
		log.Warning("Failed to connect to dbus")
		return networkManager
	}

	// Get the list of Units
	units, err := conn.ListUnits()
	if err != nil {
		log.Warning("Failed to get to dbus units")
		return networkManager
	}

	// Look for Network Manager and systemd.networkd
	for _, unit := range units {
		if strings.Contains(unit.Name, sysNetStr) {
			log.Debug("%s service is %s", unit.Name, unit.ActiveState)
			if unit.ActiveState == "active" {
				systemd = true
			}
		}
		if strings.Contains(unit.Name, netMgrStr) {
			log.Debug("%s service is %s", unit.Name, unit.ActiveState)
			if unit.ActiveState == "active" {
				nm = true
			}
		}
	}

	// If systemd.networkd is active, we use it for wired connections
	if systemd {
		networkManager = false
		log.Info("Wired networking managed by " + sysNetStr)
	} else if nm {
		networkManager = true
		log.Info("Wired networking managed by " + netMgrStr)
	}

	return networkManager
}

func (i *Interface) applyNetworkDStatic(root string, file *os.File) error {
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

// ApplyNetworkD does apply the interface configuration to the running system
// using systemd.networkd
func (i *Interface) ApplyNetworkD(root string) error {
	fileName := fmt.Sprintf("10-%s.network", i.Name)
	filePath := filepath.Join(root, systemdNetworkdDir, fileName)

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

	return i.applyNetworkDStatic(root, f)
}

func (i *Interface) applyNetworkManagerStatic(root string, file *os.File) error {
	needPacDiscover = true

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

	args := []string{
		"nmcli",
		"connection",
		"add",
		"type",
		"ethernet",
		"ifname",
		i.Name,
		"con-name",
		fmt.Sprintf("Wired-%s", i.Name),
		"ip4",
		address,
		"gw4",
		i.Gateway,
		"ipv4.method",
		"manual",
		"ipv4.dns",
		i.DNSServer,
		"ipv4.dns-search",
		i.DNSDomain,
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// ApplyNetworkManager does apply the interface configuration to the running system
// using Network Manager
func (i *Interface) ApplyNetworkManager(root string) error {
	fileName := fmt.Sprintf("Wired-%s.nmconnection", i.Name)
	filePath := filepath.Join(root, networkManagerDir, fileName)

	if i.DHCP {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return nil
		}

		if err := os.Remove(filePath); err != nil {
			return err
		}

		return nil
	}

	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrap(err)
	}

	return i.applyNetworkManagerStatic(root, f)
}

// Apply apply the configurations of a set of interfaces to the running system
// Determines the network manage type to generate the correct files
func Apply(root string, ifaces []*Interface) error {
	if root == "" {
		return errors.Errorf("Could not apply network settings, Invalid root directory: %s", root)
	}

	netMgr := IsNetworkManagerActive()

	if netMgr {
		if _, err := os.Stat(networkManagerDir); os.IsNotExist(err) {
			if err = os.MkdirAll(networkManagerDir, 0755); err != nil {
				return errors.Wrap(err)
			}
		}
	} else {
		if _, err := os.Stat(systemdNetworkdDir); os.IsNotExist(err) {
			if err = os.MkdirAll(systemdNetworkdDir, 0755); err != nil {
				return errors.Wrap(err)
			}
		}
	}

	for _, curr := range ifaces {
		if !curr.IsUserDefined() {
			log.Info("Interface %s was not changed, skipping config apply.", curr.Name)
			continue
		}

		if netMgr {
			err := curr.ApplyNetworkManager(root)
			if err != nil {
				return err
			}
		} else {
			err := curr.ApplyNetworkD(root)
			if err != nil {
				return err
			}
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
	netMgr := IsNetworkManagerActive()

	// TODO: pkexec might require the absolute path in GUI mode to ensure pkexec doesn't mess up PATH.
	if netMgr {
		err := cmd.RunAndLog("systemctl", "restart", "NetworkManager")
		if err != nil {
			return errors.Wrap(err)
		}
	} else {
		err := cmd.RunAndLog("systemctl", "restart", "systemd-networkd")
		if err != nil {
			return errors.Wrap(err)
		}
	}

	err := cmd.RunAndLog("systemctl", "restart", "systemd-resolved")
	if err != nil {
		return errors.Wrap(err)
	}

	err = cmd.RunAndLog("systemctl", "restart", "pacdiscovery")
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
		"/usr/bin/timeout",
		"--kill-after=10s",
		"10s",
		"/usr/bin/curl",
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

// FetchRemoteConfigFile given an config url fetches it from the network. This function
// currently supports only http/https protocol. After success return the local file path.
func FetchRemoteConfigFile(url string) (string, error) {
	// Get a temp filename to download to
	out, err := ioutil.TempFile("", "clr-installer-yaml-")
	if err != nil {
		return "", err
	}
	_ = out.Close()

	// Since Clear Linux automatically proxy are not support in golang via
	// the patch to libcurl, we need to use a system call to curl for now.
	// TODO: Change this back to native http.Get(url) once a better
	// proxy solution is deployed in the OS
	args := []string{
		"timeout",
		"--kill-after=30s",
		"30s",
		"curl",
		"--no-sessionid",
		"-o",
		out.Name(),
		"-s",
		"-f",
		url,
	}

	if err := cmd.Run(nil, args...); err != nil {
		log.Debug("FetchRemoteConfigFile failed : %q", err)
		defer func() { _ = os.Remove(out.Name()) }()
		return "", err
	}

	return out.Name(), nil
}

// DownloadInstallerMessage pulls down a message from a URL
// Intended for getting a message to display before or after
// the installation process
func DownloadInstallerMessage(header string, installConf string) string {
	var result Messenger

	downloadURL := fmt.Sprintf(installDataURLBase, utils.ClearVersion, installConf)
	msgFile, err := FetchRemoteConfigFile(downloadURL)
	if err != nil {
		log.Debug("Failed to download the %s message: %s", header, err)
		return ""
	}
	defer func() { _ = os.Remove(msgFile) }()

	configStr, err := ioutil.ReadFile(msgFile)
	if err != nil {
		log.Debug("Failed to read the %s file: %s", header, err)
		return ""
	}

	if err := yaml.Unmarshal(configStr, &result); err != nil {
		log.Debug("Failed to parse the %s YAML file: %s", header, err)
		return ""
	}

	if result.Messages == nil {
		log.Debug("%s has no message content", header)
		return ""
	}

	var lines []string
	found := false
	for _, message := range result.Messages {
		line := strings.TrimSpace(message.Default)
		if len(line) > 0 {
			// We found at least one line with non-whitespace
			found = true
		}
		lines = append(lines, strings.TrimSuffix(message.Default, "\n"))
	}

	msg := ""
	if found {
		msg = strings.Join(lines, "\n")
		log.Debug("%s message: %s", header, msg)
	}

	return msg
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
	systemNetworkPaths := []string{systemdNetworkdDir, networkManagerDir}

	for _, systemNetworkPath := range systemNetworkPaths {
		log.Debug("Checking network interfaces in %q to install to target", systemNetworkPath)
		if _, err := os.Stat(systemNetworkPath); err != nil {
			if os.IsNotExist(err) {
				log.Debug("No updated network interfaces in %q to install to target",
					systemNetworkPath)
				continue
			}
			log.Warning("Issue check interface %q: %v", systemNetworkPath, errors.Wrap(err))
			continue
		}

		if err := utils.CopyAllFiles(systemNetworkPath, rootDir); err != nil {
			log.Warning("Failed to copy image Network configuration data to %s", systemNetworkPath)
		} else {
			log.Info("Copied image Network configuration data to %s",
				filepath.Join(rootDir, systemNetworkPath))
		}
	}

	// We likely copied a static IP, so enable PacDiscovery
	err := EnablePacDiscovery(rootDir)
	if err != nil {
		return err
	}

	return nil
}
