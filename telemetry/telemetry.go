// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package telemetry

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// Default Telemetry configuration file
	defaultTelemetryConf = "/usr/share/defaults/telemetrics/telemetrics.conf"
	customTelemetryConf  = "/etc/telemetrics/telemetrics.conf"
	telemetrySpoolDir    = "/var/spool/telemetry"

	// Title is a predefined text to display on the Telemetry
	Title = `Enable Telemetry`
	// Help is a predefined text to display on the Telemetry
	// screen for interactive installations
	Help = `Allow the Clear Linux OS for Intel Architecture to collect anonymous
reports to improve system stability? These reports only relate to
operating system details - no personally identifiable information is
collected.

See http://clearlinux.org/features/telemetry for more information.
`

	// RequestNotice is a common text string to be displayed when enabling
	// telemetry by default on local networks
	RequestNotice = "NOTICE: Enabling Telemetry preferred by default on internal networks"

	// Default Telemetry server
	defaultTelemtryServer = "clr.telemetry.intel.com"

	maxPayload = 8 * 1024
	baseClass  = "org.clearlinux/clr-installer"
)

var (
	// Policy is the default Telemetry policy to be displayed
	// during interactive installations. Overridden by command line or
	// configuration options
	Policy = "Intel's privacy policy can be found at: http://www.intel.com/privacy."

	// ProgVersion is the version of this Clear Installer set from the Model
	// since telemetry is a component of the model, can directly include the model here
	ProgVersion string

	serverExp = regexp.MustCompile(`(?im)^(\s*server\s*=\s*)(\S+)(\s*)$`)
	tidExp    = regexp.MustCompile(`(?im)^(\s*tidheader\s*=\s*X-Telemetry-TID\s*:\s*)(\S+)(\s*)$`)

	eventID string
)

// Telemetry represents the target system telemetry enabling flag
type Telemetry struct {
	Enabled     bool
	Defined     bool
	URL         string
	TID         string
	requested   bool
	server      string
	userDefined bool
}

func init() {
	// Initialize the event record ID
	randData := make([]byte, 256)
	_, err := rand.Read(randData)
	if err != nil {
		return
	}

	eventID = fmt.Sprintf("%x", md5.Sum(randData))
}

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (tl *Telemetry) IsUserDefined() bool {
	return tl.userDefined
}

// SetRequested set the Requested flag
func (tl *Telemetry) SetRequested(requested bool) {
	tl.requested = requested
}

// IsRequested returns true if we are requested telemetry be enabled
func (tl *Telemetry) IsRequested() bool {
	return tl.requested
}

// MarshalYAML marshals Telemetry into YAML format
func (tl *Telemetry) MarshalYAML() (interface{}, error) {
	return tl.Enabled, nil
}

// UnmarshalYAML unmarshals Telemetry from YAML format
func (tl *Telemetry) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var enabled bool

	if err := unmarshal(&enabled); err != nil {
		return err
	}

	tl.Enabled = enabled
	tl.userDefined = false
	return nil
}

// SetEnable sets the enabled flag and sets this is an user defined configuration
func (tl *Telemetry) SetEnable(enable bool) {
	tl.Enabled = enable
	tl.userDefined = true

	if tl.server == "" {
		tl.server = defaultTelemtryServer
	}
}

// SetTelemetryServer set new defaults for the Telemetry server
// to override the built-in defaults
func (tl *Telemetry) SetTelemetryServer(telmURL string, telmID string, telmPolicy string) error {
	u, err := url.Parse(telmURL)
	if err != nil {
		return fmt.Errorf("Could not determine provided telemetry server name from URL (%s): %v", telmURL, err)
	}

	if urlErr := network.CheckURL(telmURL); urlErr != nil {
		return fmt.Errorf("Server not responding")
	}

	log.Debug("Using Telemetry URL: %q with TID: %q", telmURL, telmID)
	tl.URL = telmURL
	tl.TID = telmID
	tl.server = u.Hostname()

	// Set the policy
	Policy = telmPolicy

	return nil
}

// IsUsingPrivateIP return true if the current image is resolving
// the Telemetry server to a Private network IP address
func (tl *Telemetry) IsUsingPrivateIP() bool {
	inside := false

	if ips, err := net.LookupIP(tl.server); err == nil {
		// Create networks for all known Private Networks
		_, ipNetPriv10, _ := net.ParseCIDR("10.0.0.0/8")
		_, ipNetPriv172, _ := net.ParseCIDR("172.16.0.0/12")
		_, ipNetPriv192, _ := net.ParseCIDR("192.168.0.0/16")

		for _, ip := range ips {
			if ip.DefaultMask() == nil {
				log.Warning("PrivateIP: Ignoring non-IPv4 IP address: %s", ip)
				continue
			}

			in := ipNetPriv10.Contains(ip) ||
				ipNetPriv172.Contains(ip) ||
				ipNetPriv192.Contains(ip)
			log.Debug("PrivateIP: Found IP: %s, Private IP?: %s", ip, strconv.FormatBool(in))
			if in {
				inside = true
			}
		}
	} else {
		log.Warning("PrivateIP: Could not determine network location: %v", err)
	}

	return inside
}

// CreateTelemetryConf create a custom Telemetry configuration file
// using the customer server and ID
func (tl *Telemetry) CreateTelemetryConf(rootDir string) error {

	defConfFile := filepath.Join(rootDir, defaultTelemetryConf)
	// Make sure we can read the default Telemetry configuration file
	defConf, readErr := ioutil.ReadFile(defConfFile)
	if readErr != nil {
		return readErr
	}

	// Ensure the customer configuration file directory exists
	targetConfFile := filepath.Join(rootDir, customTelemetryConf)
	targetConfDir := filepath.Dir(targetConfFile)
	if err := utils.MkdirAll(targetConfDir, 0755); err != nil {
		return err
	}

	// Replace the telemetry server
	targetConf := serverExp.ReplaceAll(defConf, []byte("${1}"+tl.URL+"${3}"))
	// Replace the telemetry ID
	targetConf = tidExp.ReplaceAll(targetConf, []byte("${1}"+tl.TID+"${3}"))

	// Write the new file
	writeErr := ioutil.WriteFile(targetConfFile, targetConf, 0644)
	if writeErr != nil {
		return writeErr
	}

	log.Debug("Created Telemetry server configuration file with URL %q and tag %q", tl.URL, tl.TID)

	return nil
}

// CreateLocalTelemetryConf creates a new local custom Telemetry configuration
// file to enable the uploading of telemetry records to the remote server.
// Necessary as we change the default hostname to localhost in the server URI
// during image creation to ensure record caching during the install.
func (tl *Telemetry) CreateLocalTelemetryConf() error {

	// Ensure the customer configuration file directory exists
	targetConfDir := filepath.Dir(customTelemetryConf)
	if err := utils.MkdirAll(targetConfDir, 0755); err != nil {
		return err
	}

	if err := utils.CopyFile(defaultTelemetryConf, customTelemetryConf); err != nil {
		log.Warning("Failed to copy telemetry config %q", customTelemetryConf)
	}

	log.Debug("Created Local Telemetry server configuration file %q", customTelemetryConf)

	return nil
}

// UpdateLocalTelemetryServer updates the local custom Telemetry configuration
// file using the customer server and ID
func (tl *Telemetry) UpdateLocalTelemetryServer() error {

	// Make sure we can read the current custom Telemetry configuration file
	origConf, readErr := ioutil.ReadFile(customTelemetryConf)
	if readErr != nil {
		return readErr
	}

	newConfFile := customTelemetryConf + ".new"
	newConf := serverExp.ReplaceAll(origConf, []byte("${1}"+tl.URL+"${3}"))
	// Replace the server
	// Write the new file
	writeErr := ioutil.WriteFile(newConfFile, newConf, 0644)
	if writeErr != nil {
		return writeErr
	}

	// Move the new file into place
	moveErr := os.Rename(newConfFile, customTelemetryConf)
	if moveErr != nil {
		return moveErr
	}

	log.Debug("Updated local Telemetry server configuration file with URL %q and tag %q", tl.URL, tl.TID)

	return nil
}

// RestartLocalTelemetryServer restart the Telemetry service
// required after changes to the configuration file
func (tl *Telemetry) RestartLocalTelemetryServer() error {
	args := []string{
		"systemctl",
		"restart",
		"telemd.service",
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// StopLocalTelemetryServer stops the Telemetry service
func (tl *Telemetry) StopLocalTelemetryServer() error {
	args := []string{
		"systemctl",
		"stop",
		"telemd.service",
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// CopyTelemetryRecords copies the local spooled telemetry records
// to the target system. If records could not be sent or telemetry
// was disable, place the unpublished records on the target system
// to be upload if telemetry is installed and enabled.
func (tl *Telemetry) CopyTelemetryRecords(rootDir string) error {
	err := filepath.Walk(telemetrySpoolDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Warning("Failure accessing a path %q: %v\n", path, err)
				return nil
			}
			target := filepath.Join(rootDir, path)
			if info.IsDir() {
				// Create the matching target directory
				if err := utils.MkdirAll(target, info.Mode()); err != nil {
					log.Warning("Failed to mkdir telemetry %q", target)
					return err
				}

				return nil
			}
			if err := utils.CopyFile(path, target); err != nil {
				log.Warning("Failed to copy telemetry %q", target)
			}

			// Ensure all contents is owned correctly
			sys := info.Sys()
			stat, ok := sys.(*syscall.Stat_t)
			if ok {
				if err := os.Chown(target, int(stat.Uid), int(stat.Gid)); err != nil {
					log.Warning("Failed to change ownership of %q to UID:%d, GID:%d",
						target, stat.Uid, stat.Gid)
				}
			} else {
				log.Warning("Could not stat telemetry %q", path)
			}

			return nil
		})

	if err != nil {
		return fmt.Errorf("Failed to archive telemetry records")
	}

	// tar cpf - /var/spool/telemetry | (cd /tmp/a; tar xBpf - )

	return nil
}

// LogRecord send a new Telemetry record to the service
func (tl *Telemetry) LogRecord(class string, severity int, payload string) error {

	if severity < 1 {
		log.Warning("Telemetry severity (%d) less than 1, defaulting to 1", severity)
		severity = 1
	} else if severity > 4 {
		log.Warning("Telemetry severity (%d) greater than 4, defaulting to 4", severity)
		severity = 4
	}

	payload = "version=" + ProgVersion + "\n" + payload
	paySize := len(payload)
	if paySize > maxPayload {
		drop := payload[maxPayload:]
		payload = payload[0:(maxPayload - 1)]
		log.Warning("Telemetry payload greater than %d bytes, truncating: %q", maxPayload, drop)
	}

	args := []string{
		"telem-record-gen",
		"--severity",
		fmt.Sprintf("%d", severity),
		"--class",
		fmt.Sprintf("%s/%s", baseClass, class),
	}
	if eventID != "" {
		args = append(args,
			[]string{
				"--event-id", eventID,
			}...)
	}
	args = append(args,
		[]string{
			"--payload", payload,
		}...)

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}
