// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/boolset"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/kernel"
	"github.com/clearlinux/clr-installer/keyboard"
	"github.com/clearlinux/clr-installer/language"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/telemetry"
	"github.com/clearlinux/clr-installer/timezone"
	"github.com/clearlinux/clr-installer/user"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// DemoVersion is hard coded string we display in log files
	// when running in demo (aka documentation mode). We will
	// now use this as a flag to not include the version in UI.
	DemoVersion = "X.Y.Z"
)

// Version of Clear Installer.
// Also used by the Makefile for releases.
// Default to the version of the program
// but may be overridden for demo/documentation mode.
// Set by Go linker in the Makefile
var Version = "undefined"

// BuildDate is set by the Go linker with the build datetime
var BuildDate = "undefined"
var testAlias = []string{}

// SystemInstall represents the system install "configuration", the target
// medias, bundles to install and whatever state a install may require
type SystemInstall struct {
	InstallSelected   map[string]storage.InstallTarget `yaml:"-"`
	TargetMedias      []*storage.BlockDevice           `yaml:"targetMedia"`
	NetworkInterfaces []*network.Interface             `yaml:"networkInterfaces,omitempty,flow"`
	Keyboard          *keyboard.Keymap                 `yaml:"keyboard,omitempty,flow"`
	Language          *language.Language               `yaml:"language,omitempty,flow"`
	Bundles           []string                         `yaml:"bundles,omitempty,flow"`
	TargetBundles     []string                         `yaml:"targetBundles,omitempty,flow"`
	UserBundles       []string                         `yaml:"userBundles,omitempty,flow"`
	Offline           bool                             `yaml:"offline,omitempty,flow"`
	HTTPSProxy        string                           `yaml:"httpsProxy,omitempty,flow"`
	Telemetry         *telemetry.Telemetry             `yaml:"telemetry,omitempty,flow"`
	Timezone          *timezone.TimeZone               `yaml:"timezone,omitempty,flow"`
	Users             []*user.User                     `yaml:"users,omitempty,flow"`
	KernelArguments   *kernel.Arguments                `yaml:"kernel-arguments,omitempty,flow"`
	Kernel            *kernel.Kernel                   `yaml:"kernel,omitempty,flow"`
	PostReboot        bool                             `yaml:"postReboot,omitempty,flow"`
	SwupdMirror       string                           `yaml:"swupdMirror,omitempty,flow"`
	AllowInsecureHTTP bool                             `yaml:"AllowInsecureHTTP,omitempty,flow"`
	SwupdSkipOptional bool                             `yaml:"swupdSkipOptional,omitempty,flow"`
	PostArchive       *boolset.BoolSet                 `yaml:"postArchive,omitempty,flow"`
	Hostname          string                           `yaml:"hostname,omitempty,flow"`
	AutoUpdate        *boolset.BoolSet                 `yaml:"autoUpdate,flow"`
	TelemetryURL      string                           `yaml:"telemetryURL,omitempty,flow"`
	TelemetryTID      string                           `yaml:"telemetryTID,omitempty,flow"`
	TelemetryPolicy   string                           `yaml:"telemetryPolicy,omitempty,flow"`
	PreInstall        []*InstallHook                   `yaml:"pre-install,omitempty,flow"`
	PostInstall       []*InstallHook                   `yaml:"post-install,omitempty,flow"`
	PostImage         []*InstallHook                   `yaml:"post-image,omitempty,flow"`
	SwupdFormat       string                           `yaml:"swupdFormat,omitempty,flow"`
	Version           uint                             `yaml:"version,omitempty,flow"`
	StorageAlias      []*StorageAlias                  `yaml:"block-devices,omitempty,flow"`
	CopyNetwork       bool                             `yaml:"copyNetwork,omitempty,flow"`
	CopySwupd         bool                             `yaml:"copySwupd,omitempty,flow"`
	Environment       map[string]string                `yaml:"env,omitempty,flow"`
	CryptPass         string                           `yaml:"-"`
	MakeISO           bool                             `yaml:"iso,omitempty,flow"`
	ISOPublisher      string                           `yaml:"isoPublisher,omitempty,flow"`
	ISOApplicationID  string                           `yaml:"isoApplicationId,omitempty,flow"`
	KeepImage         bool                             `yaml:"keepImage,omitempty,flow"`
	LockFile          string                           `yaml:"-"`
	ClearCfFile       string                           `yaml:"-"`
	PreCheckDone      bool                             `yaml:"preCheckDone,omitempty,flow"`
	MediaOpts         storage.MediaOpts                `yaml:",inline"`
}

// SystemUsage is used to include additional information into the telemetry payload
type SystemUsage struct {
	InstallModel SystemInstall `yaml:",inline"`
	Hypervisor   string        `yaml:"hypervisor,omitempty,flow"`
}

// InstallHook is a commands to be executed in a given point of the install process
type InstallHook struct {
	Chroot bool   `yaml:"chroot,omitempty,flow"`
	Cmd    string `yaml:"cmd,omitempty,flow"`
}

// StorageAlias is used to expand variables in the targetMedia definitions
// a partition's block device name attribute could be declared in the form of:
//   Name: ${alias}p1
// where ${alias} was previously declared pointing to a block device file such as:
// block-devices : [
//   {name: "alias", file: "/dev/nvme0n1"}
// ]
type StorageAlias struct {
	Name       string `yaml:"name,omitempty,flow"`
	File       string `yaml:"file,omitempty,flow"`
	DeviceFile bool   `yaml:"devicefile,omitempty,flow"`
}

// ClearInstallSelected clears the map of Installation Selected targets
func (si *SystemInstall) ClearInstallSelected() {
	si.InstallSelected = map[string]storage.InstallTarget{}
}

// ClearExtraKernelArguments clears all of the of custom extra kernel arguments
func (si *SystemInstall) ClearExtraKernelArguments() {
	if si.KernelArguments == nil {
		si.KernelArguments = &kernel.Arguments{}
	}

	si.KernelArguments.Add = []string{}
}

// AddExtraKernelArguments adds a set of custom extra kernel arguments to be added to the
// clr-boot-manager configuration
func (si *SystemInstall) AddExtraKernelArguments(args []string) {
	if si.KernelArguments == nil {
		si.KernelArguments = &kernel.Arguments{}
	}

	for _, curr := range args {
		if utils.StringSliceContains(si.KernelArguments.Add, curr) {
			continue
		}

		si.KernelArguments.Add = append(si.KernelArguments.Add, curr)
	}
}

// ClearRemoveKernelArguments clears all of the of custom remove kernel arguments
func (si *SystemInstall) ClearRemoveKernelArguments() {
	if si.KernelArguments == nil {
		si.KernelArguments = &kernel.Arguments{}
	}

	si.KernelArguments.Remove = []string{}
}

// RemoveKernelArguments adds a set of kernel arguments to be "black listed" on
// clear-boot-manager, meaning these arguments will never end up in the boot manager
// entry configuration
func (si *SystemInstall) RemoveKernelArguments(args []string) {
	if si.KernelArguments == nil {
		si.KernelArguments = &kernel.Arguments{}
	}

	for _, curr := range args {
		if utils.StringSliceContains(si.KernelArguments.Remove, curr) {
			continue
		}

		si.KernelArguments.Remove = append(si.KernelArguments.Remove, curr)
	}
}

// ContainsBundle returns true if the data model has a bundle and false otherwise
func (si *SystemInstall) ContainsBundle(bundle string) bool {
	for _, curr := range si.Bundles {
		if curr == bundle {
			return true
		}
	}

	return false
}

// RemoveBundle removes a bundle from the data model
func (si *SystemInstall) RemoveBundle(bundle string) {
	bundles := []string{}

	for _, curr := range si.Bundles {
		if curr != bundle {
			bundles = append(bundles, curr)
		}
	}

	si.Bundles = bundles
}

// AddBundle adds a new bundle to the data model, we make sure to not duplicate entries
func (si *SystemInstall) AddBundle(bundle string) {
	for _, curr := range si.Bundles {
		if curr == bundle {
			return
		}
	}

	si.Bundles = append(si.Bundles, bundle)
}

// ContainsUserBundle returns true if the data model has a user bundle and false otherwise
func (si *SystemInstall) ContainsUserBundle(bundle string) bool {
	for _, curr := range si.UserBundles {
		if curr == bundle {
			return true
		}
	}

	return false
}

// RemoveUserBundle removes a user bundle from the data model
func (si *SystemInstall) RemoveUserBundle(bundle string) {
	bundles := []string{}

	for _, curr := range si.UserBundles {
		if curr != bundle {
			bundles = append(bundles, curr)
		}
	}

	si.UserBundles = bundles
}

// AddUserBundle adds a new user bundle to the data model, we make sure to not duplicate entries
func (si *SystemInstall) AddUserBundle(bundle string) {
	for _, curr := range si.UserBundles {
		if curr == bundle {
			return
		}
	}

	si.UserBundles = append([]string{bundle}, si.UserBundles...)
}

// OverrideBundles replaces the current bundles with the override list
// Sets the kernel to one of the kernel bundles, the rest remain in the list
func (si *SystemInstall) OverrideBundles(overrideBundles []string) {
	var kernelBundles []string
	si.Bundles = []string{} // Clear any existing bundles

	for _, bundle := range overrideBundles {
		if strings.HasPrefix(bundle, "kernel-") {
			kernelBundles = append(kernelBundles, bundle)
			continue
		}
		si.Bundles = append(si.Bundles, bundle) // Set Bundles
	}

	// Sort the kernels as we want one that had the CDROM loadable
	// kernel modules by default: native or LTS
	sort.Sort(sort.Reverse(sort.StringSlice(kernelBundles)))

	var foundKernel bool
	for _, bundle := range kernelBundles {
		// Only use the first kernel bundle
		if !foundKernel {
			foundKernel = true
			si.Kernel = &kernel.Kernel{Bundle: bundle}
		} else {
			si.Bundles = append(si.Bundles, bundle) // Add extra kernel bundles
			fmt.Printf("WARNING: Extra kernel bundle '%s' detected; '%s' already in use.\n",
				bundle, si.Kernel.Bundle)
		}
	}
}

// IsTargetDesktopInstall determines if this installation is a Desktop
// installation by check all bundle lists for any desktop bundles.
func (si *SystemInstall) IsTargetDesktopInstall() bool {
	isDesktop := false

	// Check the default bundle list
	for _, curr := range si.Bundles {
		if strings.Contains(strings.ToLower(curr), "desktop") {
			isDesktop = true
			break
		}
	}

	if !isDesktop {
		// Check the user bundle list
		for _, curr := range si.UserBundles {
			if strings.Contains(strings.ToLower(curr), "desktop") {
				isDesktop = true
				break
			}
		}
	}

	return isDesktop
}

// RemoveAllUsers remove from the data model all previously added user
func (si *SystemInstall) RemoveAllUsers() {
	si.Users = []*user.User{}
}

// AddUser adds a new user to the data model, this function also prevents duplicate entries
func (si *SystemInstall) AddUser(usr *user.User) {
	for _, curr := range si.Users {
		if curr.Equals(usr) {
			return
		}
	}

	si.Users = append(si.Users, usr)
}

// EncryptionRequiresPassphrase checks all partition to see if encryption was enabled
func (si *SystemInstall) EncryptionRequiresPassphrase(isAdvanced bool) bool {
	enabled := false

	for _, curr := range si.TargetMedias {
		enabled = enabled || curr.EncryptionRequiresPassphrase(isAdvanced)
	}

	return enabled
}

// Validate checks the model for possible inconsistencies or "minimum required"
// information
func (si *SystemInstall) Validate() error {
	// si will be nil if we fail to unmarshall (coverage tests has a case for that)
	if si == nil {
		return errors.ValidationErrorf("model is nil")
	}

	if si.TargetMedias == nil || len(si.TargetMedias) == 0 {
		return errors.ValidationErrorf("System Installation must provide a target media")
	}

	var results []string
	if si.IsTargetDesktopInstall() {
		results = storage.DesktopValidatePartitions(si.TargetMedias, si.MediaOpts)
	} else {
		results = storage.ServerValidatePartitions(si.TargetMedias, si.MediaOpts)
	}
	if len(results) > 0 && !si.MediaOpts.SkipValidationAll {
		return errors.ValidationErrorf(strings.Join(results, ", "))
	}

	if si.Timezone == nil {
		return errors.ValidationErrorf("Timezone not set")
	}

	if si.Keyboard == nil {
		return errors.ValidationErrorf("Keyboard not set")
	}

	if si.Language == nil {
		return errors.ValidationErrorf("System Language not set")
	}

	if si.Telemetry == nil {
		return errors.ValidationErrorf("Telemetry not acknowledged")
	}

	if si.Kernel == nil {
		return errors.ValidationErrorf("A kernel must be provided")
	}

	if len(si.ISOPublisher) > 128 {
		return errors.ValidationErrorf("isoPublisher must be shorter than 128 characters")
	}

	if len(si.ISOApplicationID) > 128 {
		return errors.ValidationErrorf("isoApplicationId must be shorter than 128 characters")
	}

	return nil
}

// AddTargetMedia adds a BlockDevice instance to the list of TargetMedias
// if bd was previously added to as a target media its pointer is updated
func (si *SystemInstall) AddTargetMedia(bd *storage.BlockDevice) {
	if si.TargetMedias == nil {
		si.TargetMedias = []*storage.BlockDevice{}
	}

	nList := []*storage.BlockDevice{bd}

	for _, curr := range si.TargetMedias {
		if !bd.Equals(curr) {
			nList = append(nList, curr)
		}
	}

	si.TargetMedias = nList
}

// AddNetworkInterface adds an Interface instance to the list of NetworkInterfaces
func (si *SystemInstall) AddNetworkInterface(iface *network.Interface) {
	if si.NetworkInterfaces == nil {
		si.NetworkInterfaces = []*network.Interface{}
	}

	si.NetworkInterfaces = append(si.NetworkInterfaces, iface)
}

// LoadFile loads a model from a yaml file pointed by path
func LoadFile(path string, options args.Args) (*SystemInstall, error) {
	var result SystemInstall

	if _, err := os.Stat(path); err == nil {
		configStr, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrap(err)
		}

		err = yaml.UnmarshalStrict(configStr, &result)
		if err != nil {
			return nil, errors.Wrap(err)
		}
	}

	// Default to archiving by default
	if result.PostArchive == nil {
		result.PostArchive = boolset.NewTrue()
	} else {
		result.PostArchive.SetDefault(true)
	}

	// Default to Auto Updating enabled by default
	if result.AutoUpdate == nil {
		result.AutoUpdate = boolset.NewTrue()
	} else {
		result.AutoUpdate.SetDefault(true)
	}

	// Set default Timezone if not defined
	if result.Timezone == nil {
		result.Timezone = &timezone.TimeZone{Code: timezone.DefaultTimezone}
	}

	// Set default Keyboard if not defined
	if result.Keyboard == nil {
		result.Keyboard = &keyboard.Keymap{Code: keyboard.DefaultKeyboard}
	}

	// Set default Language if not defined
	if result.Language == nil {
		result.Language = &language.Language{Code: language.DefaultLanguage}
	}

	// Running in VirtualBox force the default to 'kernel-lts' if
	// we are using the system default configuration file
	// See https://github.com/clearlinux/clr-installer/issues/203
	if options.ConfigFile == "" && utils.IsVirtualBox() {
		result.Kernel = &kernel.Kernel{Bundle: "kernel-lts"}
	}

	tmp := map[string]*StorageAlias{}

	for _, bds := range result.StorageAlias {
		tmp[bds.Name] = bds
	}

	for _, bds := range options.BlockDevices {
		var tks []string

		if tks = strings.Split(bds, ":"); len(tks) < 2 {
			continue
		}

		tmp[tks[0]] = &StorageAlias{Name: tks[0], File: tks[1]}
	}

	result.StorageAlias = []*StorageAlias{}

	for _, bds := range tmp {
		result.StorageAlias = append(result.StorageAlias, bds)
	}

	if len(result.StorageAlias) > 0 {
		alias := map[string]string{}
		keepMe := []*StorageAlias{}

		for _, curr := range result.StorageAlias {
			if !isAliasInUse(result.TargetMedias, curr) {
				continue
			}

			fi, err := os.Lstat(curr.File)
			inTestAlias := isTestAlias(curr.File)

			// could be an image file to be created so we fail only if the error doesn't
			// indicate the image file doesn't exist
			if err != nil && !inTestAlias && !os.IsNotExist(err) {
				return nil, errors.Wrap(err)
			}

			keepMe = append(keepMe, curr)

			if !inTestAlias && os.IsNotExist(err) {
				continue
			}

			if (fi != nil && fi.Mode()&os.ModeDevice == 0) && !inTestAlias {
				continue
			}

			curr.DeviceFile = true
			alias[curr.Name] = filepath.Base(curr.File)
		}

		// keep only the aliases we're using
		result.StorageAlias = keepMe

		for _, bd := range result.TargetMedias {
			bd.ExpandName(alias)
		}
	}

	if result.MediaOpts.SwapFileSize != "" {
		result.MediaOpts.SwapFileSet = true
	}

	return &result, nil
}

func isAliasInUse(bds []*storage.BlockDevice, alias *StorageAlias) bool {
	for _, curr := range bds {
		rep := fmt.Sprintf("${%s}", alias.Name)

		if strings.Contains(curr.Name, rep) {
			return true
		}

		if isAliasInUse(curr.Children, alias) {
			return true
		}
	}

	return false
}

func isTestAlias(file string) bool {
	if len(testAlias) == 0 {
		return false
	}

	return utils.StringSliceContains(testAlias, file)
}

// EnableTelemetry operates on the telemetry flag and enables or disables the target
// systems telemetry support based in enable argument
func (si *SystemInstall) EnableTelemetry(enable bool) {
	if si.Telemetry == nil {
		si.Telemetry = &telemetry.Telemetry{}
	}

	si.Telemetry.SetEnable(enable)
}

// IsTelemetryEnabled returns true if telemetry is enabled, false otherwise
func (si *SystemInstall) IsTelemetryEnabled() bool {
	if si.Telemetry == nil {
		return false
	}

	return si.Telemetry.Enabled
}

// IsTelemetryInstalled return true if telemetry tooling is present, false otherwise
func (si *SystemInstall) IsTelemetryInstalled() bool {
	return si.Telemetry.Installed("")
}

// WriteFile writes a yaml formatted representation of si into the provided file path
func (si *SystemInstall) WriteFile(path string) error {
	// Sanitized the model to item which should never be written
	var copyModel SystemInstall

	// Marshal current into bytes
	confBytes, bytesErr := yaml.Marshal(si)
	if bytesErr != nil {
		return errors.Wrap(bytesErr)
	}

	// Unmarshal into a copy
	if yamlErr := yaml.UnmarshalStrict(confBytes, &copyModel); yamlErr != nil {
		return errors.Wrap(bytesErr)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	// Screen item we never want stored in a YAML
	// SkipValidation flags are okay for ready, but we
	// never want to store them -- force use to set them
	// Setting to default means they are omitted
	copyModel.MediaOpts.SkipValidationAll = false
	copyModel.MediaOpts.SkipValidationSize = false

	b, err := yaml.Marshal(copyModel)
	if err != nil {
		return err
	}

	// Write our header
	_, err = f.WriteString("#clear-linux-config\n")
	if err != nil {
		return err
	}
	// Write our version
	_, err = f.WriteString("#generated by clr-installer:" + Version + "\n")
	if err != nil {
		return err
	}
	// Write datetime stamp
	t := time.Now().UTC()
	_, err = f.WriteString("#generated on: " + fmt.Sprintf("%d-%02d-%02d_%02d:%02d:%02d_UTC\n",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second()))
	if err != nil {
		return err
	}

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return nil
}

// WriteScrubModelTargetMedias writes out a copy the model with the
// TargetMedias removed to a temporary file
func (si *SystemInstall) WriteScrubModelTargetMedias() (string, error) {
	// Sanitized the model to remove media
	var cleanModel SystemInstall

	// Marshal current into bytes
	confBytes, bytesErr := yaml.Marshal(si)
	if bytesErr != nil {
		return "", errors.Wrap(bytesErr)
	}

	// Unmarshal into a copy
	if yamlErr := yaml.UnmarshalStrict(confBytes, &cleanModel); yamlErr != nil {
		return "", errors.Wrap(bytesErr)
	}

	// Sanitize the config data to remove any potential
	// Remove the target media
	cleanModel.TargetMedias = nil

	// Remove the Media swapfile if set as we might return with a swap partition
	cleanModel.MediaOpts.SwapFileSize = ""

	tmpYaml, err := ioutil.TempFile("", "clr-installer-noMedia-*.yaml")
	if err != nil {
		return "", errors.Errorf("Could not make YAML tempfile: %v", err)
	}

	if saveErr := cleanModel.WriteFile(tmpYaml.Name()); saveErr != nil {
		return "", errors.Errorf("Could not save config to %s", tmpYaml.Name())
	}

	return tmpYaml.Name(), nil
}

// InteractiveOptionsValid ensures that options which are not appropriate
// for interactive runs are screened
func (si *SystemInstall) InteractiveOptionsValid() error {
	if si.Offline {
		return fmt.Errorf("Incompatible flag '--offline' for the interactive installer")
	}
	if si.MakeISO {
		return fmt.Errorf("Incompatible flag '--iso' for the interactive installer")
	}

	return nil
}

// SetDefaultSwapFileSize defines the swapfile sized based on
// the storage default swapfile size
func (si *SystemInstall) SetDefaultSwapFileSize() {
	if si.MediaOpts.SwapFileSize == "" {
		si.MediaOpts.SwapFileSize, _ = storage.HumanReadableSizeXiBWithPrecision(storage.SwapFileSizeDefault, 1)
	}
}

// ResetDefaultSwapFileSize clears the swapfile size unless it
// has been explicitly set by the YAML or command line
func (si *SystemInstall) ResetDefaultSwapFileSize() {
	if !si.MediaOpts.SwapFileSet {
		si.MediaOpts.SwapFileSize = ""
	}
}
