// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package args

// Arguments which influence how this program executes
// Order of Precedence
// 1. Command Line Arguments -- Highest Priority
// 2. Kernel Command Line Arguments
// 3. Program defaults -- Lowest Priority

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/network"
	flag "github.com/spf13/pflag"
)

const (
	kernelCmdlineConf = "clri.descriptor"
	kernelCmdlineDemo = "clri.demo"
	kernelCmdlineLog  = "clri.loglevel"
	logFileEnvironVar = "CLR_INSTALLER_LOG_FILE"
)

var (
	kernelCmdlineFile = "/proc/cmdline"
)

// Args represents the user provided arguments
type Args struct {
	Version                 bool
	Reboot                  bool
	RebootSet               bool
	Offline                 bool
	OfflineSet              bool
	LogFile                 string
	ConfigFile              string
	CfDownloaded            bool
	CfPurge                 bool
	CfPurgeSet              bool
	AllowInsecureHTTP       bool
	AllowInsecureHTTPSet    bool
	CryptPassFile           string
	SwupdMirror             string
	SwupdStateDir           string
	SwupdCertPath           string
	SwupdStateClean         bool
	SwupdFormat             string
	SwupdVersion            string
	SwupdContentURL         string
	SwupdVersionURL         string
	SwupdURL                string
	SwupdSkipDiskSpaceCheck bool
	Telemetry               bool
	TelemetrySet            bool
	TelemetryURL            string
	TelemetryTID            string
	TelemetryPolicy         string
	PamSalt                 string
	LogLevel                int
	ForceTUI                bool
	ForceGUI                bool
	Archive                 bool
	ArchiveSet              bool
	DemoMode                bool
	BlockDevices            []string
	StubImage               bool
	ConvertConfigFile       string
	MakeISO                 bool
	MakeISOSet              bool
	KeepImage               bool
	KeepImageSet            bool
	SystemCheck             bool
	CopyNetwork             bool
}

func (args *Args) setKernelArgs() (err error) {
	var (
		kernelCmd string
		url       string
	)

	if kernelCmd, err = args.readKernelCmd(); err != nil {
		return err
	}

	// Parse the kernel command for relevant installer options
	for _, curr := range strings.Split(kernelCmd, " ") {
		curr = strings.TrimSpace(curr)
		if strings.HasPrefix(curr, kernelCmdlineConf+"=") {
			url = strings.Split(curr, "=")[1]
		} else if strings.HasPrefix(curr, kernelCmdlineDemo) {
			args.DemoMode = true
		} else if strings.HasPrefix(curr, kernelCmdlineLog) {
			logLevelString := strings.Split(curr, "=")[1]
			if logLevel, _ := strconv.Atoi(logLevelString); err != nil {
				log.Warning("Ignoring invalid kernel parameter %s='%s'", kernelCmdlineLog, logLevelString)
			} else {
				args.LogLevel = logLevel
			}
		}
	}

	if url != "" {
		var ffile string

		if ffile, err = network.FetchRemoteConfigFile(url); err != nil {
			return err
		}

		args.ConfigFile = ffile
		args.CfDownloaded = true
	}

	return nil
}

// readKernelCmd returns the kernel command line
func (args *Args) readKernelCmd() (string, error) {
	content, err := ioutil.ReadFile(kernelCmdlineFile)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (args *Args) setCommandLineArgs() (err error) {
	flag.BoolVarP(
		&args.Version, "version", "v", false, "Version of the Installer",
	)

	flag.BoolVar(
		&args.Reboot, "reboot", true, "Reboot after finishing",
	)

	flag.BoolVar(
		&args.Offline, "offline", false, "Install update content for minimal offline installation",
	)

	flag.BoolVar(
		&args.CfPurge, "cfPurge", false, "Remove ConfigFile after finishing",
	)

	flag.BoolVar(
		&args.ForceTUI, "tui", false, "Use TUI frontend",
	)

	flag.BoolVar(
		&args.ForceGUI, "gui", false, "Use GUI frontend",
	)

	flag.StringSliceVarP(
		&args.BlockDevices, "block-device", "b", args.BlockDevices,
		"Adds a new block-device's entry to configuration file. Format: <alias:filename>",
	)

	flag.StringVarP(
		&args.ConfigFile, "config", "c", args.ConfigFile, "Installation configuration file",
	)

	flag.StringVar(
		&args.CryptPassFile, "crypt-file", args.CryptPassFile, "File containing the cryptsetup password",
	)

	flag.StringVar(
		&args.SwupdMirror, "swupd-mirror", args.SwupdMirror, "Swupd --url; sets target mirror",
	)

	flag.StringVar(
		&args.SwupdStateDir, "swupd-state", args.SwupdStateDir, "Swupd --statedir",
	)

	flag.StringVar(
		&args.SwupdCertPath, "swupd-cert", args.SwupdCertPath, "Swupd --certpath",
	)

	flag.BoolVar(
		&args.SwupdStateClean, "swupd-clean",
		false, "Clean Swupd state-dir content after install",
	)

	flag.StringVar(
		&args.SwupdFormat, "swupd-format", args.SwupdFormat, "Swupd --format argument",
	)

	flag.StringVar(
		&args.SwupdVersion, "swupd-version", args.SwupdVersion, "Swupd --version argument",
	)

	flag.StringVar(
		&args.SwupdContentURL, "swupd-contenturl", args.SwupdContentURL,
		"Swupd --contenturl argument",
	)

	flag.StringVar(
		&args.SwupdVersionURL, "swupd-versionurl", args.SwupdVersionURL,
		"Swupd --versionurl argument",
	)

	flag.StringVar(
		&args.SwupdURL, "swupd-url", args.SwupdURL,
		"Swupd --url argument; use the same for content and version",
	)

	flag.BoolVar(
		&args.SwupdSkipDiskSpaceCheck, "swupd-skip-diskspace-check",
		true, "Swupd --skip-diskspace-check argument",
	)

	flag.BoolVar(
		&args.Telemetry, "telemetry", args.Telemetry, "Enable Telemetry",
	)

	flag.BoolVarP(
		&args.StubImage, "stub-image", "S", args.StubImage, "Creates the filesystems only - dont perform an actual install",
	)

	flag.StringVarP(
		&args.ConvertConfigFile, "json-yaml", "j", args.ConvertConfigFile, "Converts ister JSON config to clr-installer YAML config",
	)

	flag.StringVar(
		&args.TelemetryURL, "telemetry-url", args.TelemetryURL, "Telemetry server URL",
	)

	flag.StringVar(
		&args.TelemetryTID, "telemetry-tid", args.TelemetryTID, "Telemetry server TID",
	)

	flag.StringVar(
		&args.TelemetryPolicy, "telemetry-policy", args.TelemetryPolicy, "Telemetry Policy text",
	)

	flag.StringVar(
		&args.PamSalt, "genpass", "", "Generates a PAM compatible password hash based on the provided salt string",
	)

	flag.IntVarP(
		&args.LogLevel,
		"log-level",
		"l",
		args.LogLevel,
		fmt.Sprintf("%d (debug), %d (info), %d (warning), %d (error)",
			log.LogLevelDebug, log.LogLevelInfo, log.LogLevelWarning, log.LogLevelError),
	)

	flag.BoolVar(
		&args.AllowInsecureHTTP, "allow-insecure-http", false, "Allow installation over insecure connections",
	)

	flag.BoolVar(
		&args.Archive, "archive", true, "Archive data to target after finishing",
	)

	flag.BoolVar(
		&args.DemoMode, "demo", args.DemoMode, "Demonstration mode for documentation generation",
	)
	// We do not want this flag to be shown as part of the standard help message
	fflag := flag.Lookup("demo")
	if fflag != nil {
		fflag.Hidden = true
	}

	usr, err := user.Current()
	if err != nil {
		return err
	}

	var defaultLogFile string

	// use the env var CLR_INSTALLER_LOG_FILE to determine the log file path
	if defaultLogFile = os.Getenv(logFileEnvironVar); defaultLogFile == "" {
		defaultLogFile = filepath.Join(usr.HomeDir, conf.LogFile)
	}

	flag.StringVar(
		&args.LogFile, "log-file", defaultLogFile, "The log file path",
	)

	flag.BoolVar(
		&args.MakeISO, "iso", false, "Generate Hybrid ISO image (Legacy/UEFI bootable)",
	)

	flag.BoolVar(
		&args.KeepImage, "keep-image", true, "Keep the generated image file (when creating ISO)",
	)

	flag.BoolVar(
		&args.SystemCheck, "system-check", false, "Verify current system is compatible with Clear Linux and exit",
	)

	flag.BoolVar(
		&args.CopyNetwork, "copy-network", true, "Copy the network interface configuration files to target",
	)

	flag.ErrHelp = errors.New("Clear Linux Installer program")

	saveConfigFile := args.ConfigFile
	flag.Parse()
	// If we have a downloaded file, but it is overridden by command line, remove the tempfile
	if args.CfDownloaded && args.ConfigFile != saveConfigFile {
		_ = os.Remove(saveConfigFile)
	}

	fflag = flag.Lookup("telemetry")
	if fflag != nil {
		if fflag.Changed {
			args.TelemetrySet = true
		}
	}

	fflag = flag.Lookup("reboot")
	if fflag != nil {
		if fflag.Changed {
			args.RebootSet = true
		}
	}

	fflag = flag.Lookup("offline")
	if fflag != nil {
		if fflag.Changed {
			args.OfflineSet = true
		}
	}

	fflag = flag.Lookup("cfPurge")
	if fflag != nil {
		if fflag.Changed {
			args.CfPurgeSet = true
		}
	}

	fflag = flag.Lookup("allow-insecure-http")
	if fflag != nil {
		if fflag.Changed {
			args.AllowInsecureHTTPSet = true
		}
	}

	fflag = flag.Lookup("archive")
	if fflag != nil {
		if fflag.Changed {
			args.ArchiveSet = true
		}
	}

	fflag = flag.Lookup("iso")
	if fflag != nil {
		if fflag.Changed {
			args.MakeISOSet = true
		}
	}
	fflag = flag.Lookup("keep-image")
	if fflag != nil {
		if fflag.Changed {
			args.KeepImageSet = true
		}
	}

	if (args.TelemetryURL != "" && args.TelemetryTID == "") ||
		(args.TelemetryURL == "" && args.TelemetryTID != "") {
		return errors.New("Telemetry requires both --telemetry-url and --telemetry-tid")
	}

	if args.SwupdURL != "" {
		if args.SwupdMirror != "" {
			return errors.New("--swupd-url and --swupd-mirror are mutually exclusive")
		}

		if args.SwupdContentURL == "" {
			args.SwupdContentURL = args.SwupdURL
		} else {
			fmt.Printf("Warning: --swupd-contenturl overrides --swupd-url\n")
		}

		if args.SwupdVersionURL == "" {
			args.SwupdVersionURL = args.SwupdURL
		} else {
			fmt.Printf("Warning: --swupd-versionurl overrides --swupd-url\n")
		}
	}

	return nil
}

// ParseArgs will both parse the command line arguments to the program
// and read any options set on the kernel command line from boot-time
// setting the results into the Args member variables.
func (args *Args) ParseArgs() (err error) {
	// Set the default log level
	args.LogLevel = log.LogLevelDebug

	err = args.setKernelArgs()
	if err != nil {
		return err
	}

	err = args.setCommandLineArgs()
	if err != nil {
		return err
	}

	return nil
}
