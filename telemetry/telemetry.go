// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package telemetry

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// RequiredBundle the bundle needed to use telemetry on the target
	RequiredBundle = "telemetrics"

	// Default Telemetry configuration file
	customTelemetryConf = "/etc/telemetrics/telemetrics.conf"
	telemetrySpoolDir   = "/var/spool/telemetry"

	// Title is a predefined text to display on the Telemetry
	Title = `Enable Telemetry`
	// Help is a predefined text to display on the Telemetry
	// screen for interactive installations
	Help = `Allow Clear Linux* OS to collect anonymized system data and usage
statistics for continuous improvement?  These reports only relate to
operating system details - no personally identifiable information
is collected.
`

	// TelemetryAboutURL is the URL to reference for telemetry details
	TelemetryAboutURL = `https://clearlinux.org/documentation/clear-linux/concepts/telemetry-about`

	// RequestNotice is a common text string to be displayed when enabling
	// telemetry by default on local networks
	RequestNotice = `NOTICE: Enabling Telemetry preferred by default on internal networks.`

	// Default Telemetry server
	defaultTelemtryServer = "clr.telemetry.intel.com"

	maxPayload = 8 * 1024
	baseClass  = "org.clearlinux/clr-installer"

	// Detect hypervisor if running in VM
	envCmd = "/usr/bin/systemd-detect-virt"

	// Configuration template
	configTemplate = `[settings]
server=%s
tidheader=X-Telemetry-TID:\s%s
`
)

var (
	// Policy is the default Telemetry policy to be displayed
	// during interactive installations. Overridden by command line or
	// configuration options
	Policy = "Intel's privacy policy can be found at: http://www.intel.com/privacy."

	// ProgVersion is the version of this Clear Installer set from the Model
	// since telemetry is a component of the model, can directly include the model here
	ProgVersion string

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

// randomString generates hex string
func randomString() (string, error) {
	randData := make([]byte, 256)
	_, err := rand.Read(randData)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5.Sum(randData)), nil
}

func init() {
	// Initialize the event record ID
	eventID, _ = randomString()
}

// SetUserDefined set the user defined flag
func (tl *Telemetry) SetUserDefined(userDefined bool) {
	tl.userDefined = userDefined
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
	if tl.userDefined {
		return tl.Enabled, nil
	}
	return nil, nil
}

// UnmarshalYAML unmarshals Telemetry from YAML format
func (tl *Telemetry) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var enabled bool

	if err := unmarshal(&enabled); err != nil {
		return err
	}

	// Including telemetry in the YAML will set the default
	// as if the user had selected it in the UI
	tl.Enabled = enabled
	tl.userDefined = true
	return nil
}

// SetEnable sets the enabled flag and sets this is an user defined configuration
func (tl *Telemetry) SetEnable(enable bool) {
	tl.Enabled = enable

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

		// Bounds checking to ensure we do not get stuck
		if len(ips) > 256 {
			ips = ips[:256]
		}

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

	// Ensure the customer configuration file directory exists
	targetConfFile := filepath.Join(rootDir, customTelemetryConf)
	targetConfDir := filepath.Dir(targetConfFile)
	if err := utils.MkdirAll(targetConfDir, 0755); err != nil {
		return err
	}

	var confFile = fmt.Sprintf(configTemplate, tl.URL, tl.TID)
	// Write the new file
	writeErr := ioutil.WriteFile(targetConfFile, []byte(confFile), 0644)
	if writeErr != nil {
		return writeErr
	}

	log.Debug("Created Telemetry server configuration file with URL %q and tag %q", tl.URL, tl.TID)

	return nil
}

// CopyTelemetryRecords copies the local spooled telemetry records
// to the target system to be uploaded when the target system is
// booted and telemetry is enabled.
func (tl *Telemetry) CopyTelemetryRecords(rootDir string) error {
	// Get directory ownership
	spoolDirInfo, err := os.Stat(telemetrySpoolDir)
	if err != nil {
		return fmt.Errorf("Unable to stat %q, %s", filepath.Join(telemetrySpoolDir), err)
	}
	sys := spoolDirInfo.Sys()
	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("Could not stat telemetry %q", telemetrySpoolDir)
	}

	err = filepath.Walk(telemetrySpoolDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Warning("Failure accessing a path %q: %v\n", path, err)
				return nil
			}
			target := filepath.Join(rootDir, path)
			if info.IsDir() {
				// Telemetry spool directory is flat
				return nil
			}
			if err := utils.CopyFile(path, target); err != nil {
				log.Warning("Failed to copy telemetry %q", target)
			}
			// Ensure all contents is owned correctly
			if err := os.Chown(target, int(stat.Uid), int(stat.Gid)); err != nil {
				log.Warning("Failed to change ownership of %q to UID:%d, GID:%d",
					target, stat.Uid, stat.Gid)
			}

			return nil
		})

	if err != nil {
		return fmt.Errorf("Failed to archive telemetry records")
	}

	return nil
}

// LogRecord generates and saves a Telemetry record
func (tl *Telemetry) LogRecord(class string, severity int, payload string) error {

	w := bytes.NewBuffer(nil)
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
		"--no-post",
		"--echo",
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

	err := cmd.Run(w, args...)
	if err != nil {
		return errors.Wrap(err)
	}

	recordName, err := randomString()
	if err != nil {
		return errors.Wrap(err)
	}
	recordName = recordName[:6]
	telemetryFilename := filepath.Join(telemetrySpoolDir, recordName)

	if err := ioutil.WriteFile(telemetryFilename, w.Bytes(), 0644); err != nil {
		log.Info(err.Error())
		return errors.Wrap(err)
	}

	return nil
}

// RunningEnvironment returns the name of the hypervisor if running in a
// virtual machine, otherwise none
func (tl *Telemetry) RunningEnvironment() string {

	out, err := exec.Command(envCmd).Output()
	if err == nil {
		return strings.TrimRight(string(out), "\n")
	}

	return "none"
}
