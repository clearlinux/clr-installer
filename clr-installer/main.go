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
	"reflect"
	"regexp"
	"strings"
	"syscall"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/crypt"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/frontend"
	"github.com/clearlinux/clr-installer/keyboard"
	"github.com/clearlinux/clr-installer/language"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/massinstall"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/telemetry"
	"github.com/clearlinux/clr-installer/timezone"
	"github.com/clearlinux/clr-installer/tui"
	"github.com/clearlinux/clr-installer/utils"
)

var (
	frontEndImpls []frontend.Frontend
	classExp      = regexp.MustCompile(`(?im)(\w+)`)
)

func fatal(err error) {
	log.ErrorError(err)
	panic(err)
}

func initFrontendList() {
	frontEndImpls = []frontend.Frontend{
		massinstall.New(),
		tui.New(),
	}
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
		model.Version = "X.Y.Z"
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
		hashed, errHash := crypt.Crypt(options.PamSalt)
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

	// First verify we are running as 'root' user which is required
	// for most of the Installation commands
	if errString := utils.VerifyRootUser(); errString != "" {
		fmt.Println(errString)
		log.Error("Not running as root: %v", errString)
		return
	}

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

	log.Debug("Loading config file: %s", cf)
	if md, err = model.LoadFile(cf, options); err != nil {
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
