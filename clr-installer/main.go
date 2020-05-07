// Copyright © 2020 Intel Corporation
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
	"strconv"
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
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/syscheck"
	"github.com/clearlinux/clr-installer/telemetry"
	"github.com/clearlinux/clr-installer/timezone"
	"github.com/clearlinux/clr-installer/user"
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
		if telErr := md.Telemetry.SetTelemetryServer(md.TelemetryURL,
			md.TelemetryTID, md.TelemetryPolicy); telErr != nil {
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
		fmt.Println("Parse Args Error: " + err.Error())
		os.Exit(1)
	}

	// Configure logger
	f, err := log.SetOutputFilename(options.LogFile)

	defer func() { _ = f.Close() }()

	if err != nil {
		fmt.Println("Set Log Error: " + err.Error())
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
}

func callFrontEnd(options args.Args, md *model.SystemInstall, installReboot *bool,
	rootDir string, errChan chan error, done chan bool) {
	var err error
	for _, fe := range frontEndImpls {
		if !fe.MustRun(&options) {
			continue
		}

		*installReboot, err = fe.Run(md, rootDir, options)
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
}

func handleSignals(md *model.SystemInstall, done chan bool, sigs chan os.Signal) {
	s := <-sigs
	fmt.Println("Leaving...")
	if errLog := md.Telemetry.LogRecord("signaled", 2, "Interrupted by signal: "+s.String()); errLog != nil {
		log.Error("Failed to log Telemetry signal handler for: %s", s.String())
	}

	done <- true
}

func checkAndLoadConfigFile(options args.Args, md **model.SystemInstall) (string, error) {
	var err error

	cf := options.ConfigFile
	if options.ConfigFile == "" {
		if cf, err = conf.LookupDefaultConfig(); err != nil {
			return "", err
		}
	} else if network.IsValidURI(options.ConfigFile, options.AllowInsecureHTTP) {
		if cf, err = network.FetchRemoteConfigFile(options.ConfigFile); err != nil {
			fmt.Printf("Cannot access configuration file %q: %s\n", options.ConfigFile, err)
			return "", err
		}
		options.CfDownloaded = true
	} else if ok, err := utils.FileExists(options.ConfigFile); !ok || err != nil {
		return "", errors.Errorf("Cannot access configuration file %q", options.ConfigFile)
	}

	if filepath.Ext(cf) == ".json" {
		_, err = model.JSONtoYAMLConfig(cf)
		if err != nil {
			return "", err
		}
		cf, err = (*md).WriteYAMLConfig(cf)
		if err != nil {
			return "", err
		}
	}

	log.Debug("Loading config file: %s", cf)
	if *md, err = model.LoadFile(cf, options); err != nil {
		return "", err
	}

	return cf, nil
}

func processSwupdOptions(options args.Args, md *model.SystemInstall) {
	// Command line overrides the configuration file
	if options.SwupdMirror != "" {
		md.SwupdMirror = options.SwupdMirror
	}
	if options.SwupdFormat != "" {
		md.SwupdFormat = options.SwupdFormat
	}
	if options.SwupdSkipOptionalSet {
		md.SwupdSkipOptional = options.SwupdSkipOptional
	}
	if options.SwupdVersion != "" {
		if strings.EqualFold(options.SwupdVersion, "latest") {
			md.Version = 0
		} else {
			version, err := strconv.ParseUint(options.SwupdVersion, 10, 32)
			if err == nil {
				md.Version = uint(version)
				log.Debug("Forcing Clear Linux OS version to %d", md.Version)
			} else {
				log.Warning("Failed to parse swupd-version : %s; not-used!", options.SwupdVersion)
			}
		}
	}
	if options.CopySwupdSet {
		md.CopySwupd = options.CopySwupd
	}

	if options.AllowInsecureHTTPSet {
		md.AllowInsecureHTTP = options.AllowInsecureHTTP
	}

	if !md.AutoUpdate.IsSet() {
		osVersion, err := strconv.ParseUint(utils.ClearVersion, 10, 32)
		if err == nil {
			log.Debug("Current Clear Linux OS version is %d", md.Version)
			if md.Version != 0 && md.Version != uint(osVersion) {
				md.AutoUpdate.SetValue(false)
			}
		} else {
			log.Warning("Failed to parse Current Clear Linux OS: %s !", utils.ClearVersion)
			if md.Version != 0 {
				md.AutoUpdate.SetValue(false)
			}
		}

		if md.AutoUpdate.IsSet() {
			log.Debug("AutoUpdate is now set to %v", md.AutoUpdate.Value())
		}
	}
}

func processPamSaltOption(options args.Args) error {
	if status, err := user.IsValidPassword(options.PamSalt); !status {
		return fmt.Errorf(err)
	}

	hashed, errHash := encrypt.Crypt(options.PamSalt)
	if errHash != nil {
		return errHash
	}

	fmt.Println(hashed)
	return nil
}

func processNotStubImageOption(options args.Args, md *model.SystemInstall) error {
	var err error
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

	return nil
}

func processISOSetOption(options args.Args, md *model.SystemInstall) {
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
}

func processTemplateConfigFileOption(options args.Args, md *model.SystemInstall) error {
	if filepath.Ext(options.TemplateConfigFile) == ".yaml" {
		md.StorageAlias = append(md.StorageAlias,
			&model.StorageAlias{Name: "release", File: "release.img"})
		bd := &storage.BlockDevice{Size: storage.MinimumServerInstallSize,
			MappedName: "${release}", Name: "${release}"}
		storage.NewStandardPartitions(bd)
		md.AddTargetMedia(bd)
		if err := md.WriteFile(options.TemplateConfigFile); err != nil {
			return errors.Errorf("Failed to write YAML file (%v) %q", err, options.TemplateConfigFile)
		}
	} else {
		return errors.Errorf("Template file '%s' must end in '.yaml'", options.TemplateConfigFile)
	}

	return nil
}

func createAndAcquireLock(options args.Args, md *model.SystemInstall) (lockfile.Lockfile, error) {
	lockFile = strings.TrimSuffix(options.LogFile, ".log") + ".lock"
	lock, err := lockfile.New(lockFile)
	if err != nil {
		fmt.Printf("Cannot initialize lock. reason: %v\n", err)
		return "", err
	}

	err = lock.TryLock()
	if err != nil {
		fmt.Printf("Cannot lock %q, reason: %v\n", lock, err)
		return "", err
	}

	// Store the name of the LockFile which is needed during
	// interactive installs when launch the external partitioning tool
	md.LockFile = lockFile

	return lock, nil
}

func processCryptPassFileOption(options args.Args, md *model.SystemInstall) {
	if options.CryptPassFile != "" {
		content, cryptErr := ioutil.ReadFile(options.CryptPassFile)
		if cryptErr != nil {
			log.Warning("Could not read --crypt-file: %v", cryptErr)
		} else {
			md.CryptPass = strings.TrimSpace(string(content))
		}
	}
}

func processRebootOption(options args.Args, installReboot bool, md *model.SystemInstall) error {
	if options.Reboot && installReboot {
		_ = lock.Unlock()
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

func osExitForOptions(options args.Args) {
	// First verify we are running as 'root' user which is required
	// for most of the Installation commands
	if errString := utils.VerifyRootUser(); errString != "" {
		fmt.Println(errString)
		log.Error("Not running as root: %v", errString)
		os.Exit(126)
	}

	// Check for exclusive option
	if options.ForceTUI && options.ForceGUI {
		exclusive := "Options --tui and --gui are mutually exclusive."
		fmt.Println(exclusive)
		log.Error("Command Line Error: %s", exclusive)
		os.Exit(1)
	}

	if (options.ForceTUI || options.ForceGUI) &&
		(options.MakeISOSet && options.MakeISO) {
		exclusive := "Option --iso not compatible with --tui or --gui."
		fmt.Println(exclusive)
		log.Error("Command Line Error: %s", exclusive)
		os.Exit(1)
	}
}

// Check if Keyboard, timezone and Language option settings are correctly set
func checkKybdTzoneLangOptions(md *model.SystemInstall) error {
	if md.Keyboard != nil && !keyboard.IsValidKeyboard(md.Keyboard) {
		return fmt.Errorf("Invalid Keyboard '%s'", md.Keyboard.Code)
	}

	if md.Timezone != nil && !timezone.IsValidTimezone(md.Timezone) {
		return fmt.Errorf("Invalid Time Zone '%s'", md.Timezone.Code)
	}

	if md.Language != nil && !language.IsValidLanguage(md.Language) {
		return fmt.Errorf("Invalid Language '%s'", md.Language.Code)
	}

	return nil
}

// Try to parse to the ConvertFile and generate new Model and if that fails, return old copy of model back
func processConvertConfigFileOption(options args.Args, md *model.SystemInstall) (*model.SystemInstall, error) {
	copyModel := md
	var err error
	if options.ConvertConfigFile != "" && options.TemplateConfigFile != "" {
		return copyModel, errors.Errorf("Options --json-yaml and --template are mutually exclusive.")
	}

	if options.ConvertConfigFile != "" {
		if filepath.Ext(options.ConvertConfigFile) == ".json" {
			copyModel, err = model.JSONtoYAMLConfig(options.ConvertConfigFile)
		} else {
			err = errors.Errorf("Config file '%s' must end in '.json'", options.ConvertConfigFile)
		}
	}

	return copyModel, err
}

func processOptionsSaveIfSet(options args.Args, md *model.SystemInstall) {
	if options.RebootSet {
		md.PostReboot = options.Reboot
	}

	if options.OfflineSet {
		md.Offline = options.Offline
	}

	if options.ArchiveSet {
		md.PostArchive = options.Archive
	}

	if options.SkipValidationSizeSet {
		md.MediaOpts.SkipValidationSize = options.SkipValidationSize
	}
	if options.SkipValidationAllSet {
		md.MediaOpts.SkipValidationAll = options.SkipValidationAll
	}

	if options.SwapFileSize != "" {
		md.MediaOpts.SwapFileSize = options.SwapFileSize
		md.MediaOpts.SwapFileSet = true
	}
}

func processOptionsToModel(options args.Args, md *model.SystemInstall) {
	processCryptPassFileOption(options, md)

	processOptionsSaveIfSet(options, md)

	processSwupdOptions(options, md)

	processISOSetOption(options, md)
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
		return processPamSaltOption(options)
	}

	if options.Version {
		fmt.Println(path.Base(os.Args[0]) + ": " + model.Version)
		return nil
	}

	var md *model.SystemInstall

	cf := options.ConfigFile
	// Load config values from file to model
	if cf, err = checkAndLoadConfigFile(options, &md); err != nil {
		return err
	}
	if options.CfDownloaded {
		defer func() { _ = os.Remove(cf) }()
	}

	md.ClearInstallSelected()

	if md, err = processConvertConfigFileOption(options, md); err != nil {
		return err
	}

	if options.CfPurgeSet && options.CfPurge {
		defer func() { _ = os.Remove(cf) }()
		md.ClearCfFile = cf
	}

	log.Info("Querying Clear Linux version")
	if err := utils.ParseOSClearVersion(); err != nil {
		return err
	}

	processOptionsToModel(options, md)

	if len(options.Bundles) > 0 {
		md.OverrideBundles(options.Bundles)
		log.Info("Overriding bundle list from command line: %s", strings.Join(md.Bundles, ", "))
	}

	if options.ConvertConfigFile != "" {
		_, err := md.WriteYAMLConfig(options.ConvertConfigFile)
		if err != nil {
			return err
		}

		return nil
	}

	if options.TemplateConfigFile != "" {
		return processTemplateConfigFileOption(options, md)
	}

	// exit if certain conditions fail for certain options
	osExitForOptions(options)

	if lock, err = createAndAcquireLock(options, md); err != nil {
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

	if options.SwupdContentURL != "" && network.IsValidURI(options.SwupdContentURL, md.AllowInsecureHTTP) == false {
		return errors.Errorf("swupd-contenturl %s must use HTTPS or FILE protocol", options.SwupdContentURL)
	}

	if err = processNotStubImageOption(options, md); err != nil {
		return err
	}

	// check if Keyboard, timezone and Language options are correctly set
	if err = checkKybdTzoneLangOptions(md); err != nil {
		return err
	}

	// Set locale
	utils.SetLocale(md.Language.Code)

	// Run system check and exit
	if options.SystemCheck {
		return syscheck.RunSystemCheck(false)
	}

	installReboot := false

	// Figure out which FrontEnd's run to invoke and call it async
	go callFrontEnd(options, md, &installReboot, rootDir, errChan, done)

	// Run Telemetry terminate, run it async
	go handleSignals(md, done, sigs)

	select {
	case <-done:
		break
	case err = <-errChan:
		return err
	}

	// Stop the signal handlers
	// or we get a SIGTERM from reboot
	signal.Reset()

	return processRebootOption(options, installReboot, md)
}
