// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"

	"github.com/nightlyone/lockfile"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/encrypt"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/frontend"
	"github.com/clearlinux/clr-installer/keyboard"
	"github.com/clearlinux/clr-installer/language"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/syscheck"
	"github.com/clearlinux/clr-installer/telemetry"
	"github.com/clearlinux/clr-installer/timezone"
	"github.com/clearlinux/clr-installer/utils"
)

var (
	frontEndImpls []frontend.Frontend
	classExp      = regexp.MustCompile(`(?im)(\w+)`)
	lockFile      = "/root/clr-installer.lock"
	lock          lockfile.Lockfile
)

func validateTelemetry(options args.Args, md *model.SystemInstall) error {
	if options.TelemetryPolicy != "" {
		md.TelemetryPolicy = options.TelemetryPolicy
	}
	// Make sure the both URL and TID are in the configuration file
	if (md.TelemetryURL != "" && md.TelemetryTID == "") ||
		(md.TelemetryURL == "" && md.TelemetryTID != "") {
		return errors.Errorf("Telemetry requires both telemetryUrl and telemetryTid in the configuration file")
	} else if md.TelemetryURL != "" && md.TelemetryPolicy == "" {
		log.Warning("Defining a Telemetry Policy is encouraged when specifying a Telemetry server")
	}

	// Telemetry is not in the config file AND not specified on the command line
	noTelemetryDefault := md.Telemetry == nil && !options.TelemetrySet

	// Ensure we have a Telemetry object
	md.EnableTelemetry(md.IsTelemetryEnabled())
	md.Telemetry.Defined = !noTelemetryDefault

	// Command line overrides the configuration file
	if options.TelemetrySet {
		md.EnableTelemetry(options.Telemetry)
	}
	if options.TelemetryURL != "" {
		md.TelemetryURL = options.TelemetryURL
		md.TelemetryTID = options.TelemetryTID
	}
	// Validate the specified telemetry server
	if md.TelemetryURL != "" {
		if telErr := md.Telemetry.SetTelemetryServer(md.TelemetryURL, md.TelemetryTID, md.TelemetryPolicy); telErr != nil {
			return telErr
		}

		if noTelemetryDefault {
			md.EnableTelemetry(true)
			log.Warning("Setting a Telemetry server enables Telemetry!")
		}
	}
	// This lowest priority for enabling/defaulting telemetry
	if telemetryEnable := md.Telemetry.IsUsingPrivateIP(); telemetryEnable {
		md.Telemetry.SetRequested(telemetryEnable)
		log.Info(telemetry.RequestNotice)
		if noTelemetryDefault {
			md.EnableTelemetry(telemetryEnable)
		}
	}

	return nil
}

func main() {
	var options args.Args

	if err := options.ParseArgs(); err != nil {
		os.Exit(1)
	}

	// Configure logger
	f, err := log.SetOutputFilename(options.LogFile)
	if err != nil {
		os.Exit(1)
	}
	log.SetLogLevel(options.LogLevel)

	// Begin installer execution
	if err := execute(options); err != nil {
		// Print and log errors with stack traces. To include stack traces, the
		// errors must be created with errors.Errorf or wrapped with errors.Wrap
		fmt.Println(err.Error())
		log.Error("%s", err)
		_ = f.Close()

		os.Exit(1)
	}
	_ = f.Close()
}

// execute is called by main to begin execution of the installer
func execute(options args.Args) error {
	var err error

	if options.DemoMode {
		model.Version = model.DemoVersion
	}
	// Make the Version of the program visible to telemetry
	telemetry.ProgVersion = model.Version

	log.Info(path.Base(os.Args[0]) + ": " + model.Version +
		", built on " + model.BuildDate)

	if options.PamSalt != "" {
		hashed, errHash := encrypt.Crypt(options.PamSalt)
		if errHash != nil {
			return errHash
		}

		fmt.Println(hashed)
		return nil
	}

	if options.Version {
		fmt.Println(path.Base(os.Args[0]) + ": " + model.Version)
		return nil
	}

	if options.ConvertConfigFile != "" {
		if filepath.Ext(options.ConvertConfigFile) == ".json" {
			_, err := model.JSONtoYAMLConfig(options.ConvertConfigFile)
			if err != nil {
				return err
			}
		} else {
			return errors.Errorf("Config file '%s' must end in '.json'", options.ConvertConfigFile)
		}
		return nil
	}

	// First verify we are running as 'root' user which is required
	// for most of the Installation commands
	if errString := utils.VerifyRootUser(); errString != "" {
		fmt.Println(errString)
		log.Error("Not running as root: %v", errString)
		return nil
	}

	// Check for exclusive option
	if options.ForceTUI && options.ForceGUI {
		exclusive := "Options --tui and --gui are mutually exclusive."
		fmt.Println(exclusive)
		log.Error("Command Line Error: %s", exclusive)
		return nil
	}

	if (options.ForceTUI || options.ForceGUI) &&
		(options.MakeISOSet && options.MakeISO) {
		exclusive := "Option --iso not compatible with --tui or --gui."
		fmt.Println(exclusive)
		log.Error("Command Line Error: %s", exclusive)
		return nil
	}

	lockFile = strings.TrimSuffix(options.LogFile, ".log") + ".lock"
	lock, err = lockfile.New(lockFile)
	if err != nil {
		fmt.Printf("Cannot initialize lock. reason: %v\n", err)
		return err
	}

	err = lock.TryLock()
	if err != nil {
		fmt.Printf("Cannot lock %q, reason: %v\n", lock, err)
		return err
	}

	defer func() { _ = lock.Unlock() }()

	initFrontendList()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	errChan := make(chan error)

	signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP,
		syscall.SIGABRT, syscall.SIGSTKFLT, syscall.SIGSYS)

	rootDir, err := ioutil.TempDir("", "install-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(rootDir) }()

	var md *model.SystemInstall
	cf := options.ConfigFile

	if options.ConfigFile == "" {
		if cf, err = conf.LookupDefaultConfig(); err != nil {
			return err
		}
	} else if network.IsValidURI(options.ConfigFile, options.AllowInsecureHTTP) {
		if cf, err = network.FetchRemoteConfigFile(options.ConfigFile); err != nil {
			fmt.Printf("Cannot acesss configuration file %q: %s\n", options.ConfigFile, err)
			return err
		}
		options.CfDownloaded = true
	} else {
		return errors.Errorf("No valid configuration file")
	}

	if options.CfDownloaded {
		defer func() { _ = os.Remove(cf) }()
	}

	if filepath.Ext(cf) == ".json" {
		cf, err = model.JSONtoYAMLConfig(cf)
		if err != nil {
			return err
		}
	}

	log.Debug("Loading config file: %s", cf)
	if md, err = model.LoadFile(cf, options); err != nil {
		return err
	}
	md.ClearInstallSelected()

	if options.CfPurgeSet && options.CfPurge {
		defer func() { _ = os.Remove(cf) }()
		md.ClearCfFile = cf
	}

	log.Info("Querying Clear Linux version")
	if err := utils.ParseOSClearVersion(); err != nil {
		return err
	}

	if options.CryptPassFile != "" {
		content, cryptErr := ioutil.ReadFile(options.CryptPassFile)
		if cryptErr != nil {
			log.Warning("Could not read --crypt-file: %v", cryptErr)
		} else {
			md.CryptPass = strings.TrimSpace(string(content))
		}
	}

	if options.RebootSet {
		md.PostReboot = options.Reboot
	}

	if options.OfflineSet {
		md.Offline = options.Offline
	}

	if options.ArchiveSet {
		md.PostArchive = options.Archive
	}

	// Command line overrides the configuration file
	if options.SwupdMirror != "" {
		md.SwupdMirror = options.SwupdMirror
	}

	if options.AllowInsecureHTTPSet {
		md.AllowInsecureHTTP = options.AllowInsecureHTTP
	}

	if options.SwupdContentURL != "" && network.IsValidURI(options.SwupdContentURL, md.AllowInsecureHTTP) == false {
		return errors.Errorf("swupd-contenturl %s must use HTTPS or FILE protocol", options.SwupdContentURL)
	}

	// Command line overrides the configuration file
	if options.MakeISOSet {
		md.MakeISO = options.MakeISO
		if options.KeepImageSet {
			md.KeepImage = options.KeepImage
		} else {
			md.KeepImage = false
		}
	} else {
		if options.KeepImageSet {
			md.KeepImage = options.KeepImage
		}
	}
	// If ISO not set in configuration file ensure we keep the image file
	if !md.MakeISO {
		md.KeepImage = true
	}

	// Store the name of the LockFile which is needed during
	// interactive installs when launch the external partitioning tool
	md.LockFile = lockFile

	if !options.StubImage {
		// Now validate the mirror from the config or command line
		if md.SwupdMirror != "" {
			var url string
			url, err = swupd.SetHostMirror(md.SwupdMirror, md.AllowInsecureHTTP)
			if err != nil {
				return err
			}
			log.Info("Using Swupd Mirror value: %q", url)
		}

		if err = validateTelemetry(options, md); err != nil {
			return err
		}
	}

	if md.Keyboard != nil && !keyboard.IsValidKeyboard(md.Keyboard) {
		return fmt.Errorf("Invalid Keyboard '%s'", md.Keyboard.Code)
	}

	if md.Timezone != nil && !timezone.IsValidTimezone(md.Timezone) {
		return fmt.Errorf("Invalid Time Zone '%s'", md.Timezone.Code)
	}

	if md.Language != nil && !language.IsValidLanguage(md.Language) {
		return fmt.Errorf("Invalid Language '%s'", md.Language.Code)
	}

	// Set locale
	utils.SetLocale(md.Language.Code)

	// Run system check and exit
	if options.SystemCheck {
		return syscheck.RunSystemCheck(false)
	}

	installReboot := false

	go func() {
		for _, fe := range frontEndImpls {
			if !fe.MustRun(&options) {
				continue
			}

			installReboot, err = fe.Run(md, rootDir, options)
			if err != nil {
				feName := classExp.FindString(reflect.TypeOf(fe).String())
				if feName == "" {
					feName = "unknown"
				}
				if errLog := md.Telemetry.LogRecord(feName, 3, err.Error()); errLog != nil {
					log.Error("Failed to log Telemetry fail record: %s", feName)
				}

				if errors.IsValidationError(err) {
					fmt.Println("Error: Invalid configuration:")
					errChan <- err
				} else {
					log.RequestCrashInfo()
					errChan <- err
				}
			}

			break
		}

		done <- true
	}()

	go func() {
		s := <-sigs
		fmt.Println("Leaving...")
		if errLog := md.Telemetry.LogRecord("signaled", 2, "Interrupted by signal: "+s.String()); errLog != nil {
			log.Error("Failed to log Telemetry signal handler for: %s", s.String())
		}
		done <- true
	}()

	select {
	case <-done:
		break
	case err = <-errChan:
		return err
	}

	// Stop the signal handlers
	// or we get a SIGTERM from reboot
	signal.Reset()

	if options.Reboot && installReboot {
		if err := cmd.RunAndLog("reboot"); err != nil {
			if errLog := md.Telemetry.LogRecord("reboot", 1, err.Error()); errLog != nil {
				log.Error("Failed to log Telemetry fail record: reboot")
			}
			return err
		}
		log.RequestCrashInfo()
	}
	return nil
}
