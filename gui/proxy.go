// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gotk3/gotk3/glib"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/proxy"
)

const (
	gProxySchema        = "org.gnome.system.proxy"
	dconfProxyDir       = "/system/proxy/"
	installerDefaultUID = "1000"
)

// SetupGnomeProxy configures the Gnome proxy function
// in the proxy package
func SetupGnomeProxy() {
	proxy.SetGetProxyValueFunc(GnomeGetProxyValue)
	proxy.SetPreProxyFunc(SyncNetworkProxies)
}

// GnomeGetProxyValue first check the Gnome network settings for
// proxy settings, then falls back to Bash environment variables
func GnomeGetProxyValue(prefix string) string {
	result := ""

	gProxy := glib.SettingsNew(gProxySchema)
	if gProxy != nil {
		proxyMode := gProxy.GetString("mode")
		log.Debug("Gnome Proxy Mode: %s", proxyMode)

		// TODO: Consider transferring these settings to the
		// target install system
		// dbus-run-session dconf dump /system/proxy/ > file_for_target
		// Since these are user specific settings, we would need to find
		// as way to import them for each interactive user created on
		// the target system using some post installation hook
		switch proxyMode {
		case "none":
			// No value to pull
		case "manual":
			if prefix == "no" {
				noHosts := []string{}
				hosts := gProxy.GetStrv("ignore-hosts")
				if len(hosts) > 1 {
					for _, host := range hosts {
						if strings.Contains(host, "/") {
							log.Debug("Skipping no_proxy hosts %q, no CIDR support", host)
						} else {
							noHosts = append(noHosts, host)
						}
					}
					result = strings.Join(noHosts, ",")
				}
			} else {
				gProxyPrefix := glib.SettingsNew(gProxySchema + "." + prefix)
				if gProxyPrefix != nil {
					host := gProxyPrefix.GetString("host")
					if host != "" {
						result = host
						port := gProxyPrefix.GetInt("port")
						if port != 0 {
							result = result + fmt.Sprintf(":%d", port)
						}
					}
				}
			}
		case "auto":
			if prefix == "no" {
				// no_proxy is handle by the wpad.dat script with
				// connections which are DIRECT
			} else {
				autoProxyURL := gProxy.GetString("autoconfig-url")
				if autoProxyURL != "" {
					log.Debug("We should probably download and use %q", autoProxyURL)
					// TODO:
					// Overwrite the value from pacdiscovery?
					// /run/pacrunner/wpad.dat
					// Restart pacrunner?
				}
			}
		default:
			log.Warning("Unknown Gnome Proxy Mode: ", proxyMode)
		}
	}

	if result == "" {
		value := os.Getenv(prefix + "_proxy")
		if value != "" {
			result = value
		}
	}

	return result
}

// SyncNetworkProxies copies the current values from the Network Proxy
// for the non-privileged user to the root Gnome environment
func SyncNetworkProxies() {
	// To avoid recursion, since this function is called as part of the standard
	// cmd run, we have our only customer local exec function.
	sudoUser := os.Getenv("SUDO_USER") // launched by sudo
	tag := "sudo_user"
	if sudoUser == "" { // no SUDO_USER defined
		sudoUser = "#" + os.Getenv("PKEXEC_UID") // launched by pkexec (polkit)
		tag = "PKEXEC_UID"
	}
	if sudoUser == "#" { // no PKEXEC_UID defined
		sudoUser = "#" + installerDefaultUID // fallback
		tag = "fallback UID"
	}
	log.Debug("sync user is %s=%s", tag, sudoUser)

	dumpCmd := exec.Command(
		"sudo", fmt.Sprintf("--user=%s", sudoUser),
		"dbus-run-session", "dconf", "dump", dconfProxyDir,
	)
	loadCmd := exec.Command(
		"dbus-run-session", "dconf", "load", dconfProxyDir,
	)

	reader, writer := io.Pipe()

	dumpCmd.Stdout = writer
	loadCmd.Stdin = reader

	var err error
	if err = dumpCmd.Start(); err == nil {
		if err = loadCmd.Start(); err != nil {
			log.Warning("Error starting dconf load: %v", err)
		}
	} else {
		log.Warning("Error starting dconf dump: %v", err)
	}

	if err = dumpCmd.Wait(); err == nil {
		if err = writer.Close(); err != nil {
			log.Warning("Error closing writer for dconf dump: %v", err)
		}
	} else {
		log.Warning("Error waiting for dconf dump: %v", err)
	}

	if err = loadCmd.Wait(); err == nil {
		if err = reader.Close(); err != nil {
			log.Warning("Error closing reader for dconf load: %v", err)
		}
	} else {
		log.Warning("Error waiting for dconf load: %v", err)
	}
}
