// Copyright © 2018 Intel Corporation
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

func fatal(err error) {
	if lock != "" {
		lErr := lock.Unlock()
		if lErr != nil {
			fmt.Printf("Cannot lock %q, reason: %v\n", lock, lErr)
		}
	}

	log.ErrorError(err)
	panic(err)
}

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
		fatal(err)
	}

	if options.DemoMode {
		model.Version = model.DemoVersion
	}
	// Make the Version of the program visible to telemetry
	telemetry.ProgVersion = model.Version

	f, err := log.SetOutputFilename(options.LogFile)
	if err != nil {
		fatal(err)
	}
	defer func() {
		_ = f.Close()
	}()

	log.SetLogLevel(options.LogLevel)

	log.Info(path.Base(os.Args[0]) + ": " + model.Version +
		", built on " + model.BuildDate)

	if options.PamSalt != "" {
		hashed, errHash := encrypt.Crypt(options.PamSalt)
		if err != nil {
			panic(errHash)
		}

		fmt.Println(hashed)
		return
	}

	if options.Version {
		fmt.Println(path.Base(os.Args[0]) + ": " + model.Version)
		return
	}

	if options.ConvertConfigFile != "" {
		if filepath.Ext(options.ConvertConfigFile) == ".json" {
			_, err = model.JSONtoYAMLConfig(options.ConvertConfigFile)
			if err != nil {
				fatal(err)
			}
		} else {
			fatal(errors.Errorf("Config file '%s' must end in '.json'", options.ConvertConfigFile))
		}
		return
	}

	// First verify we are running as 'root' user which is required
	// for most of the Installation commands
	if errString := utils.VerifyRootUser(); errString != "" {
		fmt.Println(errString)
		log.Error("Not running as root: %v", errString)
		return
	}

	lockFile = strings.TrimSuffix(options.LogFile, ".log") + ".lock"
	lock, err := lockfile.New(lockFile)
	if err != nil {
		fmt.Printf("Cannot initialize lock. reason: %v\n", err)
		os.Exit(1)
	}

	err = lock.TryLock()
	if err != nil {
		fmt.Printf("Cannot lock %q, reason: %v\n", lock, err)
		os.Exit(1)
	}

	defer func() { _ = lock.Unlock() }()

	initFrontendList()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP,
		syscall.SIGABRT, syscall.SIGSTKFLT, syscall.SIGSYS)

	rootDir, err := ioutil.TempDir("", "install-")
	if err != nil {
		fatal(err)
	}
	defer func() { _ = os.RemoveAll(rootDir) }()

	var md *model.SystemInstall
	cf := options.ConfigFile

	if options.ConfigFile == "" {
		if cf, err = conf.LookupDefaultConfig(); err != nil {
			fatal(err)
		}
	}
	if options.CfDownloaded {
		defer func() { _ = os.Remove(cf) }()
	}

	if filepath.Ext(cf) == ".json" {
		cf, err = model.JSONtoYAMLConfig(cf)
		if err != nil {
			fatal(err)
		}
	}

	log.Debug("Loading config file: %s", cf)
	if md, err = model.LoadFile(cf, options); err != nil {
		fatal(err)
	}

	log.Info("Querying Clear Linux version")
	if err := utils.ParseOSClearVersion(); err != nil {
		fatal(err)
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

	if options.ArchiveSet {
		md.PostArchive = options.Archive
	}

	// Command line overrides the configuration file
	if options.SwupdMirror != "" {
		md.SwupdMirror = options.SwupdMirror
	}
	// If ISO not set in configuration file ensure we keep the image file
	if !md.MakeISO {
		md.KeepImage = true
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

	if !options.StubImage {
		// Now validate the mirror from the config or command line
		if md.SwupdMirror != "" {
			var url string
			url, err = swupd.SetHostMirror(md.SwupdMirror)
			if err != nil {
				fatal(err)
			} else {
				log.Info("Using Swupd Mirror value: %q", url)
			}
		}

		if err = validateTelemetry(options, md); err != nil {
			fatal(err)
		}
	}

	if md.Keyboard != nil && !keyboard.IsValidKeyboard(md.Keyboard) {
		fatal(fmt.Errorf("Invalid Keyboard '%s'", md.Keyboard.Code))
	}

	if md.Timezone != nil && !timezone.IsValidTimezone(md.Timezone) {
		fatal(fmt.Errorf("Invalid Time Zone '%s'", md.Timezone.Code))
	}

	if md.Language != nil && !language.IsValidLanguage(md.Language) {
		fatal(fmt.Errorf("Invalid Language '%s'", md.Language.Code))
	}

	// Set locale
	utils.SetLocale(md.Language.Code)

	// Run system check and exit
	if options.SystemCheck {
		err = syscheck.RunSystemCheck()
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
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
					fmt.Printf("  %s\n", err)
					os.Exit(1)
				} else {
					log.RequestCrashInfo()
					fatal(err)
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

	<-done

	// Stop the signal handlers
	// or we get a SIGTERM from reboot
	signal.Reset()

	if options.Reboot && installReboot {
		if err := cmd.RunAndLog("reboot"); err != nil {
			if errLog := md.Telemetry.LogRecord("reboot", 1, err.Error()); errLog != nil {
				log.Error("Failed to log Telemetry fail record: reboot")
			}
			fatal(err)
		}
		log.RequestCrashInfo()
	}
}
