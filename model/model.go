// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/clearlinux/clr-installer/args"
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

// Version of Clear Installer.
// Also used by the Makefile for releases.
// Default to the version of the program
// but may be overridden for demo/documentation mode.
var Version = "1.0.1"
var testAlias = []string{}

// SystemInstall represents the system install "configuration", the target
// medias, bundles to install and whatever state a install may require
type SystemInstall struct {
	TargetMedias      []*storage.BlockDevice `yaml:"targetMedia"`
	NetworkInterfaces []*network.Interface   `yaml:"networkInterfaces"`
	Keyboard          *keyboard.Keymap       `yaml:"keyboard,omitempty,flow"`
	Language          *language.Language     `yaml:"language,omitempty,flow"`
	Bundles           []string               `yaml:"bundles,omitempty,flow"`
	HTTPSProxy        string                 `yaml:"httpsProxy,omitempty,flow"`
	Telemetry         *telemetry.Telemetry   `yaml:"telemetry,omitempty,flow"`
	Timezone          *timezone.TimeZone     `yaml:"timezone,omitempty,flow"`
	Users             []*user.User           `yaml:"users,omitempty,flow"`
	KernelArguments   *kernel.Arguments      `yaml:"kernel-arguments,omitempty,flow"`
	Kernel            *kernel.Kernel         `yaml:"kernel,omitempty,flow"`
	PostReboot        bool                   `yaml:"postReboot,omitempty,flow"`
	SwupdMirror       string                 `yaml:"swupdMirror,omitempty,flow"`
	PostArchive       bool                   `yaml:"postArchive,omitempty,flow"`
	Hostname          string                 `yaml:"hostname,omitempty,flow"`
	AutoUpdate        bool                   `yaml:"autoUpdate,omitempty,flow"`
	TelemetryURL      string                 `yaml:"telemetryURL,omitempty,flow"`
	TelemetryTID      string                 `yaml:"telemetryTID,omitempty,flow"`
	TelemetryPolicy   string                 `yaml:"telemetryPolicy,omitempty,flow"`
	PreInstall        []*InstallHook         `yaml:"pre-install,omitempty,flow"`
	PostInstall       []*InstallHook         `yaml:"post-install,omitempty,flow"`
	Version           uint                   `yaml:"version,omitempty,flow"`
	StorageAlias      []*StorageAlias        `yaml:"block-devices,omitempty,flow"`
	LegacyBios        bool                   `yaml:"legacyBios,omitempty,flow"`
	Environment       map[string]string      `yaml:"env,omitempty,flow"`
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
	DeviceFile bool
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

// Validate checks the model for possible inconsistencies or "minimum required"
// information
func (si *SystemInstall) Validate() error {
	// si will be nil if we fail to unmarshal (coverage tests has a case for that)
	if si == nil {
		return errors.ValidationErrorf("model is nil")
	}

	if si.TargetMedias == nil || len(si.TargetMedias) == 0 {
		return errors.ValidationErrorf("System Installation must provide a target media")
	}

	for _, curr := range si.TargetMedias {
		if err := curr.Validate(si.LegacyBios); err != nil {
			return err
		}
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

	// Default to archiving by default
	result.PostArchive = true

	// Default to Auto Updating enabled by default
	result.AutoUpdate = true

	if _, err := os.Stat(path); err == nil {
		configStr, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrap(err)
		}

		err = yaml.Unmarshal(configStr, &result)
		if err != nil {
			return nil, errors.Wrap(err)
		}
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

	if result.Version > 0 {
		result.AutoUpdate = false
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

// WriteFile writes a yaml formatted representation of si into the provided file path
func (si *SystemInstall) WriteFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	b, err := yaml.Marshal(si)
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

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return nil
}
