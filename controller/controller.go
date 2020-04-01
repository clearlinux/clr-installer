// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/hostname"
	"github.com/clearlinux/clr-installer/isoutils"
	"github.com/clearlinux/clr-installer/kernel"
	"github.com/clearlinux/clr-installer/keyboard"
	"github.com/clearlinux/clr-installer/language"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/proxy"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/telemetry"
	"github.com/clearlinux/clr-installer/timezone"
	cuser "github.com/clearlinux/clr-installer/user"
	"github.com/clearlinux/clr-installer/utils"
)

var (
	// NetworkPassing is used to track if the latest network configuration
	// is passing; changes in proxy, etc.
	NetworkPassing bool
)

const (
	// NetWorkManager is the application to manage network.
	NetWorkManager = "Network Manager"
)

func sortMountPoint(bds []*storage.BlockDevice) []*storage.BlockDevice {
	sort.Slice(bds[:], func(i, j int) bool {
		return filepath.HasPrefix(bds[j].MountPoint, bds[i].MountPoint)
	})

	return bds
}

// Install is the main install controller, this is the entry point for a full
// installation
//nolint: gocyclo  // TODO: Refactor this
func Install(rootDir string, model *model.SystemInstall, options args.Args) error {
	var err error
	var version string
	var prg progress.Progress
	var encryptedUsed bool

	vars := map[string]string{
		"chrootDir": rootDir,
		"yamlDir":   filepath.Dir(options.ConfigFile),
	}

	for k, v := range model.Environment {
		vars[k] = v
	}

	preConfFile := log.GetPreConfFile()

	if err = model.WriteFile(preConfFile); err != nil {
		log.Error("Failed to write pre-install YAML file (%v) %q", err, preConfFile)
	}

	advanced := false
	for _, tm := range model.TargetMedias {
		advanced = advanced || tm.IsAdvancedConfiguration()
	}

	if model.EncryptionRequiresPassphrase(advanced) && model.CryptPass == "" {
		model.CryptPass = storage.GetPassPhrase()
		if model.CryptPass == "" {
			return errors.Errorf("Can not create encrypted file system, no passphrase")
		}
	}

	if !options.StubImage {
		if err = applyHooks("pre-install", vars, model.PreInstall); err != nil {
			return err
		}
	}

	if model.Version == 0 {
		version = "latest"
	} else {
		version = fmt.Sprintf("%d", model.Version)
	}

	log.Debug("Clear Linux version: %s", version)

	// do we have the minimum required to install a system?
	if err = model.Validate(); err != nil {
		return err
	}

	// Using MassInstaller (non-UI) the network will not have been checked yet
	if !NetworkPassing && !options.StubImage && !swupd.IsOfflineContent() && len(model.UserBundles) != 0 {
		if err = ConfigureNetwork(model); err != nil {
			return err
		}
	}

	expandMe := []*storage.BlockDevice{}
	detachMe := []string{}
	removeMe := []string{}
	aliasMap := map[string]string{}
	usingPhysicalMedia := true

	// prepare image file, case the user has declared image alias then create
	// the image, setup the loop device, prepare the variable expansion
	for _, alias := range model.StorageAlias {
		var file string

		if alias.DeviceFile {
			continue
		}

		// create the image and add the alias name to the variable expansion list
		for _, tm := range model.TargetMedias {
			if tm.Name == fmt.Sprintf("${%s}", alias.Name) {
				if err = storage.MakeImage(tm, alias.File); err != nil {
					return err
				}

				expandMe = append(expandMe, tm)
				usingPhysicalMedia = false
			}
		}

		// Add the image file to the hooks variables
		vars["imageFile"] = alias.File

		file, err = storage.SetupLoopDevice(alias.File)
		if err != nil {
			return errors.Wrap(err)
		}

		aliasMap[alias.Name] = filepath.Base(file)
		detachMe = append(detachMe, file)
		if !model.KeepImage {
			removeMe = append(removeMe, alias.File)
		}

		retry := 5

		// wait the loop device to be prepared and available with 5 retry attempts
		for {
			var ok bool

			if ok, err = utils.FileExists(file); err != nil {
				for _, file := range detachMe {
					storage.DetachLoopDevice(file)
				}

				return errors.Wrap(err)
			}

			if ok || retry == 0 {
				break
			}

			retry--
			time.Sleep(time.Second * 1)
		}
	}

	// defer detaching used loop devices
	defer func() {
		for _, file := range detachMe {
			storage.DetachLoopDevice(file)
		}

		// Now that image is unmounted, run post-image hooks
		if err = applyHooks("post-image", vars, model.PostImage); err != nil {
			log.Error("Error during post-image hook: %q", err)
		}

		// The request to keep image may have changed during execution
		// such as dynamically decided to not make an ISO or failing to
		// make an ISO
		if !model.KeepImage {
			for _, file := range removeMe {
				log.Debug("Removing raw image file: %s", file)
				if err = os.Remove(file); err != nil {
					log.Warning("Failed to remove image file: %s", file)
				}
			}
		}

		// Final message to user that the installation has fully completed
		msg := utils.Locale.Get("Installation Steps Complete")
		prg = progress.NewLoop(msg)
		log.Info(msg)
		prg.Success()
	}()

	// expand block device's name case we've detected image replacement cases
	for _, tm := range expandMe {
		oldName := tm.Name
		tm.ExpandName(aliasMap)
		newName := tm.Name
		// Remap the InstallSelected
		model.InstallSelected[newName] = model.InstallSelected[oldName]
		delete(model.InstallSelected, oldName)
	}

	mountPoints := []*storage.BlockDevice{}

	if usingPhysicalMedia {
		if model.MakeISO {
			msg := "Flag --iso not valid for physical media; disabling"
			fmt.Println(msg)
			log.Warning(msg)
			model.MakeISO = false
		}
	}

	// prepare all the target block devices
	for _, curr := range model.TargetMedias {
		var wholeDisk bool
		if val, ok := model.InstallSelected[curr.Name]; ok {
			wholeDisk = val.WholeDisk
		}
		// based on the description given, write the partition table
		if err = curr.WritePartitionTable(model.LegacyBios, wholeDisk, nil); err != nil {
			return err
		}

		// prepare the blockdevice's partitions filesystem
		for _, ch := range curr.Children {
			if ch.Type == storage.BlockDeviceTypeCrypt {
				encryptedUsed = true

				if ch.FsTypeNotSwap() {
					msg := utils.Locale.Get("Mapping %s partition to an encrypted partition", ch.Name)
					prg = progress.NewLoop(msg)
					log.Info(msg)
					if err = ch.MapEncrypted(model.CryptPass); err != nil {
						prg.Failure()
						return err
					}
					prg.Success()
				}
			}

			// if we have a mount point set it for future mounting
			if ch.MountPoint != "" {
				mountPoints = append(mountPoints, ch)
			}

			// Do not overwrite File System content for pre-existing
			if !ch.FormatPartition {
				msg := utils.Locale.Get("Skipping new file system for %s", ch.Name)
				log.Debug(msg)
				continue
			}

			msg := utils.Locale.Get("Writing %s file system to %s", ch.FsType, ch.Name)
			if ch.MountPoint != "" {
				msg = msg + fmt.Sprintf(" '%s'", ch.MountPoint)
			}
			prg = progress.NewLoop(msg)
			log.Info(msg)
			if err = ch.MakeFs(); err != nil {
				prg.Failure()
				return err
			}
			prg.Success()
		}
	}

	// Update the target devices current labels and UUIDs
	if scanErr := storage.UpdateBlockDevices(model.TargetMedias); scanErr != nil {
		return scanErr
	}

	if options.StubImage {
		return nil
	}

	// mount all the prepared partitions
	for _, curr := range sortMountPoint(mountPoints) {
		log.Info("Mounting: %s", curr.MountPoint)

		if err = curr.Mount(rootDir); err != nil {
			return err
		}
	}

	defer func() {
		log.Info("Umounting rootDir: %s", rootDir)
		if storage.UmountAll() != nil {
			log.Warning("Failed to umount volumes")
			return
		}

		log.Info("Removing rootDir: %s", rootDir)
		if err = os.RemoveAll(rootDir); err != nil {
			log.Warning("Failed to remove rootDir: %s", rootDir)
		}
	}()

	err = storage.MountMetaFs(rootDir)
	if err != nil {
		return err
	}

	// If we are using NetworkManager add the basic bundle
	if network.IsNetworkManagerActive() {
		model.AddBundle(network.RequiredBundle)
	}

	// Add in the User Defined bundles
	for _, curr := range model.UserBundles {
		model.AddBundle(curr)
	}

	if model.Telemetry.Enabled {
		model.AddBundle(telemetry.RequiredBundle)
	}

	if len(model.Users) > 0 {
		model.AddBundle(cuser.RequiredBundle)
	}

	if model.Timezone.Code != timezone.DefaultTimezone {
		model.AddBundle(timezone.RequiredBundle)
	}

	if model.Keyboard.Code != keyboard.DefaultKeyboard {
		model.AddBundle(keyboard.RequiredBundle)
	}

	if model.Language.Code != language.DefaultLanguage {
		model.AddBundle(language.RequiredBundle)
	}

	if encryptedUsed {
		model.AddBundle(storage.RequiredBundle)
		kernelArgs := []string{storage.KernelArgument}
		model.AddExtraKernelArguments(kernelArgs)
	}

	msg := utils.Locale.Get("Writing mount files")
	prg = progress.NewLoop(msg)
	log.Info(msg)
	if err = storage.GenerateTabFiles(rootDir, model.TargetMedias); err != nil {
		prg.Failure()
		return err
	}
	prg.Success()

	if model.KernelArguments != nil && len(model.KernelArguments.Add) > 0 {
		cmdlineDir := filepath.Join(rootDir, "etc", "kernel")
		cmdlineFile := filepath.Join(cmdlineDir, "cmdline")
		cmdline := strings.Join(model.KernelArguments.Add, " ")

		if err = utils.MkdirAll(cmdlineDir, 0755); err != nil {
			return err
		}

		if err = ioutil.WriteFile(cmdlineFile, []byte(cmdline), 0644); err != nil {
			return err
		}
	}

	if model.KernelArguments != nil && len(model.KernelArguments.Remove) > 0 {
		cmdlineDir := filepath.Join(rootDir, "etc", "kernel", "cmdline-removal.d")
		cmdlineFile := filepath.Join(cmdlineDir, "clr-installer.conf")
		cmdline := strings.Join(model.KernelArguments.Remove, " ")

		if err = utils.MkdirAll(cmdlineDir, 0755); err != nil {
			return err
		}

		if err = ioutil.WriteFile(cmdlineFile, []byte(cmdline), 0644); err != nil {
			return err
		}
	}

	if prg, err = contentInstall(rootDir, version, model, options); err != nil {
		prg.Failure()
		return err
	}

	if err = configureTimezone(rootDir, model); err != nil {
		// Just log the error, not setting the timezone is not reason to fail the install
		log.Error("Error setting timezone: %v", err)
	}

	if err = configureKeyboard(rootDir, model); err != nil {
		// Just log the error, not setting the keyboard is not reason to fail the install
		log.Error("Error setting keyboard: %v", err)
	}

	if err = configureLanguage(rootDir, model); err != nil {
		// Just log the error, not setting the language is not reason to fail the install
		log.Error("Error setting language locale: %v", err)
	}

	if err = cuser.Apply(rootDir, model.Users); err != nil {
		return err
	}

	if model.Hostname != "" {
		if err = hostname.SetTargetHostname(rootDir, model.Hostname); err != nil {
			return err
		}
	}

	if model.CopyNetwork {
		if err = network.CopyNetworkInterfaces(rootDir); err != nil {
			return err
		}
	}

	if model.CopySwupd {
		swupd.CopyConfigurations(rootDir)
	}

	if model.AllowInsecureHTTP {
		swupd.CreateConfig(rootDir)
	}

	if model.Telemetry.URL != "" {
		if err = model.Telemetry.CreateTelemetryConf(rootDir); err != nil {
			return err
		}
	}

	if err = applyHooks("post-install", vars, model.PostInstall); err != nil {
		return err
	}

	msg = utils.Locale.Get("Saving the installation results")
	prg = progress.NewLoop(msg)
	log.Info(msg)
	if err = saveInstallResults(rootDir, model); err != nil {
		log.ErrorError(err)
	}
	prg.Success()

	if model.MakeISO {
		log.Info("Generating ISO image")
		if err = generateISO(rootDir, model, options); err != nil {
			log.ErrorError(err)
		}
	}

	msg = utils.Locale.Get("Installation completed")
	prg = progress.NewLoop(msg)
	log.Info(msg)
	prg.Success()

	return nil
}

func applyHooks(name string, vars map[string]string, hooks []*model.InstallHook) error {
	locName := utils.Locale.Get(name)
	msg := utils.Locale.Get("Running %s hooks", locName)
	prg := progress.NewLoop(msg)
	log.Info(msg)

	for idx, curr := range hooks {
		if err := runInstallHook(vars, curr); err != nil {
			prg.Failure()
			return err
		}
		prg.Partial(idx)
	}

	prg.Success()
	return nil
}

func runInstallHook(vars map[string]string, hook *model.InstallHook) error {
	args := []string{}
	vars["chrooted"] = "0"

	if hook.Chroot {
		args = append(args, []string{"chroot", vars["chrootDir"]}...)
		vars["chrooted"] = "1"

		restoreFile := copyHostResolvToTarget(vars["chrootDir"])
		defer func() {
			restoreTargetResolv(vars["chrootDir"], restoreFile)
		}()
	}

	exec := utils.ExpandVariables(vars, hook.Cmd)
	args = append(args, []string{"bash", "-l", "-c", exec}...)

	if err := cmd.RunAndLogWithEnv(vars, args...); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// use the current host's version to bootstrap the sysroot, then update to the
// latest one and start adding new bundles
// for the bootstrap we use the hosts's swupd and the following operations are
// executed using the target swupd
func contentInstall(rootDir string, version string, md *model.SystemInstall, options args.Args) (progress.Progress, error) {

	var prg progress.Progress

	sw := swupd.New(rootDir, options, md)

	// Currently, ISO image generation supports only a single kernel.
	// Hence, skip ISO generation if multiple kernel bundles are present.
	// TODO: Remove this logic when ISO generation supports multiple kernels.
	if md.MakeISO {
		for _, curr := range md.Bundles {
			if strings.HasPrefix(curr, "kernel-") {
				msg := "ISO image generation supports only a single kernel. Setting ISO generation to false.\n"
				log.Warning(msg)
				fmt.Printf("Warning: %s", msg)
				md.MakeISO = false
				md.KeepImage = true
				break
			}
		}
	}

	bundles := md.Bundles

	if md.Kernel.Bundle != "none" {
		bundles = append(bundles, md.Kernel.Bundle)
	}

	if swupd.IsOfflineContent() {
		msg := utils.Locale.Get("Copying cached content to target media")
		prg = progress.NewLoop(msg)
		log.Info(msg)

		if err := utils.ParseOSClearVersion(); err != nil {
			return prg, err
		}
		log.Info("Overriding version from %s to %s to enable offline install", version, utils.ClearVersion)
		version = utils.ClearVersion

		// Copying offline content here is a performance optimization and is not a hard
		// failure because Swupd may be able to successfully copy offline content or
		// install over the network.
		if err := copyOfflineToStatedir(rootDir, sw.GetStateDir()); err != nil {
			log.Warning("Failed to copy offline content: %s", err)
		}

		prg.Success()
	} else if md.AutoUpdate {
		version = "latest"
	}

	msg := utils.Locale.Get("Installing base OS and configured bundles")
	log.Info(msg)

	log.Debug("Installing bundles: %s", strings.Join(bundles, ", "))
	if err := sw.OSInstall(version, swupd.TargetPrefix, bundles); err != nil {
		// If the swupd command failed to run there wont be a progress
		// bar, so we need to create a new one that we can fail
		prg = progress.NewLoop(msg)
		return prg, err
	}

	// Create custom config in the installer image to override default bundle list
	if md.TargetBundles != nil {
		if err := writeCustomConfig(rootDir, md); err != nil {
			prg = progress.NewLoop(msg)
			return prg, err
		}
	}

	if md.Offline {
		// Install minimum set of required bundles to offline content directory.
		log.Info("Installing offline content to the target")

		offlineBundles := []string{
			network.RequiredBundle,
			telemetry.RequiredBundle,
			cuser.RequiredBundle,
			timezone.RequiredBundle,
			keyboard.RequiredBundle,
			language.RequiredBundle,
			storage.RequiredBundle,
		}

		// Load default config from chroot for required bundles list
		bundleConfig, err := conf.LookupDefaultChrootConfig(rootDir)
		if err != nil {
			prg = progress.NewLoop(msg)
			return prg, err
		}
		loadedBundles, err := model.LoadFile(bundleConfig, options)
		if err != nil {
			prg = progress.NewLoop(msg)
			return prg, err
		}
		offlineBundles = append(offlineBundles, loadedBundles.Bundles...)

		// Load available kernel bundles from chroot
		loadedKernels, err := kernel.LoadKernelListChroot(rootDir)
		if err != nil {
			prg = progress.NewLoop(msg)
			return prg, err
		}
		for _, k := range loadedKernels {
			offlineBundles = append(offlineBundles, k.Bundle)
		}

		log.Debug("Downloading bundles: %s", strings.Join(offlineBundles, ", "))
		if err := sw.DownloadBundles(version, offlineBundles); err != nil {
			prg = progress.NewLoop(msg)
			return prg, err
		}
	}

	if !md.AutoUpdate {
		msg := utils.Locale.Get("Disabling automatic updates")
		prg = progress.NewLoop(msg)
		log.Info(msg)
		if err := sw.DisableUpdate(); err != nil {
			warnMsg := utils.Locale.Get("Disabling automatic updates failed")
			log.Warning(warnMsg)
			prg.Failure()
			return prg, err
		}
		prg.Success()
	}

	msg = utils.Locale.Get("Installing boot loader")
	prg = progress.NewLoop(msg)
	log.Info(msg)

	cbmPath := options.CBMPath
	if cbmPath == "" {
		cbmPath = fmt.Sprintf("%s/usr/bin/clr-boot-manager", rootDir)
	}

	args := []string{
		cbmPath,
		"update",
		"--image",
		fmt.Sprintf("--path=%s", rootDir),
	}

	envVars := map[string]string{
		"CBM_DEBUG": "1",
	}

	if md.LegacyBios {
		envVars["CBM_FORCE_LEGACY"] = "1"
	}

	err := cmd.RunAndLogWithEnv(envVars, args...)
	if err != nil {
		prg.Failure()
		return prg, errors.Wrap(err)
	}
	prg.Success()

	// Clean-up State Directory content
	if options.SwupdStateClean {
		msg = utils.Locale.Get("Cleaning Swupd state directory")
		prg = progress.NewLoop(msg)
		log.Info(msg)
		if err = sw.CleanUpState(); err != nil {
			log.ErrorError(err)
		}
		prg.Success()
	}

	return nil, nil
}

func copyOfflineToStatedir(rootDir, stateDir string) error {
	if err := utils.MkdirAll(filepath.Dir(stateDir), 0755); err != nil {
		return err
	}

	log.Debug("Overwriting stateDir with offline content")
	if err := os.RemoveAll(stateDir); err != nil {
		return err
	}

	if isoLoopDev := isoutils.GetIsoLoopDevice(); strings.Compare(isoLoopDev, "") != 0 {
		// Extract offline contents for ISO installer
		log.Debug("Extracting offline content in squashfs to target media")

		if err := isoutils.ExtractSquashfs(conf.OfflineContentDir, rootDir, isoLoopDev); err != nil {
			return err
		}
		if err := os.Rename(path.Join(rootDir, conf.OfflineContentDir), stateDir); err != nil {
			return err
		}
	} else {
		// Copy offline contents for img installer
		log.Debug("Copying offline content to target media")

		// The performance of utils.CopyAllFiles is much slower than cp when
		// copying a large number of files.
		args := []string{
			"cp",
			"-ar",
			conf.OfflineContentDir,
			stateDir,
		}
		err := cmd.RunAndLog(args...)
		if err != nil {
			return err
		}
	}
	return nil
}

// ConfigureNetwork applies the model/configured network interfaces
func ConfigureNetwork(model *model.SystemInstall) error {
	prg, err := configureNetwork(model)
	if err != nil {
		prg.Failure()
		NetworkPassing = false
		return err
	}

	NetworkPassing = true

	return nil
}

func configureNetwork(model *model.SystemInstall) (progress.Progress, error) {
	proxy.SetHTTPSProxy(model.HTTPSProxy)

	if len(model.NetworkInterfaces) > 0 {
		msg := "Applying network settings"
		prg := progress.NewLoop(msg)
		log.Info(msg)
		if err := network.Apply("/", model.NetworkInterfaces); err != nil {
			prg.Failure()
			return prg, err
		}
		prg.Success()

		msg = utils.Locale.Get("Restarting network interfaces")
		prg = progress.NewLoop(msg)
		log.Info(msg)
		if err := network.Restart(); err != nil {
			prg.Failure()
			return prg, err
		}
		prg.Success()
	}

	msg := utils.Locale.Get("Testing connectivity")
	attempts := 3
	prg := progress.NewLoop(msg)
	ok := false
	// 3 attempts to test connectivity
	for i := 0; i < attempts; i++ {
		time.Sleep(2 * time.Second)

		log.Info(msg)
		if err := network.VerifyConnectivity(); err == nil {
			ok = true
			break
		}
		log.Warning("Attempt to verify connectivity failed")

		// Restart networking if we failed
		// The likely gain is restarting pacdiscovery to fix autoproxy
		if err := network.Restart(); err != nil {
			log.Warning("Network restart failed")
			ok = false
			break
		}
	}

	if !ok {
		msg = utils.Locale.Get("Network check failed.")
		msg += " " + utils.Locale.Get("Use %s to configure network.", NetWorkManager)
		return prg, errors.Errorf(msg)
	}

	prg.Success()

	return nil, nil
}

// configureTimezone applies the model/configured Timezone to the target
func configureTimezone(rootDir string, model *model.SystemInstall) error {
	if model.Timezone.Code == timezone.DefaultTimezone {
		log.Debug("Skipping setting timezone " + model.Timezone.Code)
		return nil
	}

	msg := "Setting Timezone to " + model.Timezone.Code
	prg := progress.NewLoop(msg)
	log.Info(msg)

	err := timezone.SetTargetTimezone(rootDir, model.Timezone.Code)
	if err != nil {
		prg.Failure()
		return err
	}
	prg.Success()

	return nil
}

// configureKeyboard applies the model/configured keyboard to the target
func configureKeyboard(rootDir string, model *model.SystemInstall) error {
	if model.Keyboard.Code == keyboard.DefaultKeyboard {
		log.Debug("Skipping setting keyboard " + model.Keyboard.Code)
		return nil
	}

	msg := "Setting Keyboard to " + model.Keyboard.Code
	prg := progress.NewLoop(msg)
	log.Info(msg)

	err := keyboard.SetTargetKeyboard(rootDir, model.Keyboard.Code)
	if err != nil {
		prg.Failure()
		return err
	}
	prg.Success()

	return nil
}

// configureLanguage applies the model/configured language to the target
func configureLanguage(rootDir string, model *model.SystemInstall) error {
	if model.Language.Code == language.DefaultLanguage {
		log.Debug("Skipping setting language locale " + model.Language.Code)
		return nil
	}

	msg := utils.Locale.Get("Setting Language locale to %s", model.Language.Code)
	prg := progress.NewLoop(msg)
	log.Info(msg)

	err := language.SetTargetLanguage(rootDir, model.Language.Code)
	if err != nil {
		prg.Failure()
		return err
	}
	prg.Success()

	return nil
}

// saveInstallResults saves the results of the installation process
// onto the target media
func saveInstallResults(rootDir string, md *model.SystemInstall) error {
	var err error
	errMsgs := []string{}

	// Log a sanitized YAML file with Telemetry
	var cleanModel model.SystemInstall
	// Marshal current into bytes
	confBytes, bytesErr := yaml.Marshal(md)
	if bytesErr != nil {
		log.Error("Failed to generate a copy of YAML data (%v)", bytesErr)
		errMsgs = append(errMsgs, "Failed to generate YAML file")
	}
	// Unmarshal into a copy
	if yamlErr := yaml.Unmarshal(confBytes, &cleanModel); yamlErr != nil {
		errMsgs = append(errMsgs, "Failed to duplicate YAML file")
	}
	// Sanitize the config data to remove any potential
	// Personal Information from the data set
	cleanModel.Users = nil             // Remove User Info
	cleanModel.Hostname = ""           // Remove user defined hostname
	cleanModel.HTTPSProxy = ""         // Remove user defined Proxy
	cleanModel.SwupdMirror = ""        // Remove user defined Swupd Mirror
	cleanModel.NetworkInterfaces = nil // Remove Network information

	// Remove the Serial number from the target media
	for _, bd := range cleanModel.TargetMedias {
		bd.Serial = ""
	}

	var payload string
	hypervisor := md.Telemetry.RunningEnvironment()
	extendedModel := model.SystemUsage{
		InstallModel: cleanModel,
		Hypervisor:   hypervisor,
	}
	confBytes, bytesErr = yaml.Marshal(extendedModel)
	if bytesErr != nil {
		log.Error("Failed to generate a sanitized data (%v)", bytesErr)
		errMsgs = append(errMsgs, "Failed to generate a sanitized YAML file")
		payload = strings.Join(errMsgs, ";")
	} else {
		payload = string(confBytes[:])
	}

	if errLog := md.Telemetry.LogRecord("success", 1, payload); errLog != nil {
		log.Error("Failed to log Telemetry success record")
	}

	if md.PostArchive {
		log.Info("Saving Installation results to %s", rootDir)

		saveDir := filepath.Join(rootDir, "root")
		if err = utils.MkdirAll(saveDir, 0755); err != nil {
			// Fallback in the unlikely case we can't use root's home
			saveDir = rootDir
		}

		confFile := filepath.Join(saveDir, conf.ConfigFile)

		if err := md.WriteFile(confFile); err != nil {
			log.Error("Failed to write YAML file (%v) %q", err, confFile)
			errMsgs = append(errMsgs, "Failed to write YAML file")
		}

		logFile := filepath.Join(saveDir, conf.LogFile)

		if err := log.ArchiveLogFile(logFile); err != nil {
			errMsgs = append(errMsgs, "Failed to archive log file")
		}

	} else {
		log.Info("Skipping archiving of Installation results")
	}

	if md.IsTelemetryEnabled() {
		if md.IsTelemetryInstalled() {
			log.Info("Copying telemetry records to target system.")
			if err := md.Telemetry.CopyTelemetryRecords(rootDir); err != nil {
				log.Warning("Failed to copy image Telemetry data")
				errMsgs = append(errMsgs, "Failed to copy image Telemetry data")
			}
			log.Info("Running telemctl opt-in")
			if err := md.Telemetry.OptIn(rootDir); err != nil {
				log.Warning("Failed to opt-in to telemetry")
				errMsgs = append(errMsgs, "Failed to opt-in to telemetry")
			}
			if len(errMsgs) > 0 {
				return errors.Errorf("%s", strings.Join(errMsgs, ";"))
			}
		} else {
			log.Info("Telemetry is not present in the installer, skipping copying records")
		}
	} else {
		log.Info("Telemetry disabled, skipping record collection.")
		if md.Telemetry.Installed(rootDir) {
			log.Info("Running telemctl opt-out")
			if err := md.Telemetry.OptOut(rootDir); err != nil {
				log.Warning("Unable to opt-out, telemetry might not be present")
				errMsgs = append(errMsgs, "Unable to opt-out, telemetry might not be present")
			}
			if len(errMsgs) > 0 {
				return errors.Errorf("%s", strings.Join(errMsgs, ";"))
			}
		}
	}

	return nil
}

// generateISO creates an ISO image from the just created raw image
func generateISO(rootDir string, md *model.SystemInstall, options args.Args) error {
	var err error
	log.Info("Building ISO image")

	if !md.LegacyBios {
		for _, alias := range md.StorageAlias {
			if err = isoutils.MakeIso(rootDir, strings.TrimSuffix(alias.File, filepath.Ext(alias.File)), md, options); err != nil {
				md.KeepImage = true
				return err
			}
		}
	} else {
		err = fmt.Errorf("cannot create ISO images for configurations with LegacyBios enabled")
		log.ErrorError(err)
		md.KeepImage = true
		return err
	}

	return err
}

// writeCustomConfig creates a config file in the installer image that overrides the default
func writeCustomConfig(chrootPath string, md *model.SystemInstall) error {
	customPath := path.Join(chrootPath, conf.CustomConfigDir)

	if err := utils.MkdirAll(customPath, 0755); err != nil {
		return err
	}

	// Create basic config based on provided values
	customModel := &model.SystemInstall{
		Bundles:    md.TargetBundles,
		Keyboard:   md.Keyboard,
		Language:   md.Language,
		Timezone:   md.Timezone,
		Kernel:     md.Kernel,
		AutoUpdate: true, // We should always default to enabled
	}

	return customModel.WriteFile(path.Join(customPath, conf.ConfigFile))
}

// copyHostResolvToTarget first saves the original /etc/resolve.conf if it exists,
// then copies the hosts /etc/resolv.conf to the target to enable networking (DNS)
// in the chrooted environment.
// Used with restoreTargetResolv
func copyHostResolvToTarget(rootDir string) string {
	resolvConf := filepath.Join(rootDir, "etc", "resolv.conf")
	restoreFile := ""

	// make a backup if it exists
	if ok, _ := utils.FileExists(resolvConf); ok {
		restoreFile = fmt.Sprintf("%s.%d", resolvConf, os.Getpid())
		log.Debug("Saving a temp copy of file %q as %q", resolvConf, restoreFile)
		if err := utils.CopyFile(resolvConf, restoreFile); err != nil {
			log.Error("Failed to save file %q: %s", restoreFile, err)
			restoreFile = ""
		}
	}

	// copy the host to target
	log.Debug("Copying host's /etc/resolv.conf to target to enable DNS for networking during hook %q", resolvConf)
	if err := utils.CopyFile("/etc/resolv.conf", resolvConf); err != nil {
		log.Error("Failed to install file %q: %s", resolvConf, err)
	}

	return restoreFile
}

// restoreTargetResolv removed the temporary /etc/resolv.conf on the target system
// then restores the original /etc/resolve.conf if it exists,
// Used with copyHostResolvToTarget
func restoreTargetResolv(rootDir string, restoreFile string) {
	resolvConf := filepath.Join(rootDir, "etc", "resolv.conf")

	if restoreFile != "" {
		log.Debug("Restore original resolv.conf file %q", restoreFile)
		if ok, _ := utils.FileExists(restoreFile); ok {
			// restore the target file
			if err := utils.CopyFile(restoreFile, resolvConf); err != nil {
				log.Error("Failed to restore file %q: %s", resolvConf, err)
			}
		} else {
			log.Warning("Resolv.conf restore file missing: %s", restoreFile)
		}
	} else {
		log.Debug("Removing temp copy of file %q", resolvConf)
		if err := os.Remove(resolvConf); err != nil {
			log.Warning("Failed to clean up temporary network file: %q: %s", resolvConf, err)
		}
	}
}
