// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package conf

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/clearlinux/clr-installer/utils"
)

const (
	// BundleListFile the file file for the containing the bundle list definition
	BundleListFile = "bundles.json"

	// LogFile is the installation log file name
	LogFile = "clr-installer.log"

	// ConfigFile is the install descriptor
	ConfigFile = "clr-installer.yaml"

	// ChpasswdPAMFile is the chpasswd pam configuration file
	ChpasswdPAMFile = "chpasswd"

	// DefaultConfigDir is the system wide default configuration directory
	DefaultConfigDir = "/usr/share/defaults/clr-installer"

	// CustomConfigDir directory contains custom configuration files
	// i.e per image configuration files
	CustomConfigDir = "/var/lib/clr-installer"

	// OfflineContentDir contains offline installation content
	OfflineContentDir = "/var/lib/clr-installer/offline-content"

	// KernelListFile is the file describing the available kernel bundles
	KernelListFile = "kernels.json"

	// SourcePath is the source path (within the .gopath)
	SourcePath = "src/github.com/clearlinux/clr-installer"
)

func isRunningFromSourceTree() (bool, string, error) {
	src, err := os.Executable()
	if err != nil {
		return false, src, err
	}
	src, err = filepath.Abs(filepath.Dir(src))
	if err != nil {
		return false, src, err
	}

	return !strings.HasPrefix(src, "/usr/bin"), src, nil
}

func lookupDefaultFile(file, pathPrefix string) (string, error) {
	if pathPrefix == "" {
		isSourceTree, sourcePath, err := isRunningFromSourceTree()
		if err != nil {
			return "", err
		}

		// use the config from source code's etc dir if not installed binary
		if isSourceTree {
			sourceRoot := strings.Replace(sourcePath, "bin", filepath.Join(SourcePath, "etc"), 1)
			return filepath.Join(sourceRoot, file), nil
		}
	}

	custom := filepath.Join(pathPrefix, CustomConfigDir, file)

	if ok, _ := utils.FileExists(custom); ok {
		return custom, nil
	}

	return filepath.Join(pathPrefix, DefaultConfigDir, file), nil
}

// LookupBundleListFile looks up the bundle list definition
// Guesses if we're running from source code or from system, if we're running from
// source code directory then we load the source default file, otherwise tried to load
// the system installed file
func LookupBundleListFile() (string, error) {
	return lookupDefaultFile(BundleListFile, "")
}

// LookupDefaultConfig looks up the install descriptor
// Guesses if we're running from source code our from system, if we're running from
// source code directory then we loads the source default file, otherwise tried to load
// the system installed file
func LookupDefaultConfig() (string, error) {
	return lookupDefaultFile(ConfigFile, "")
}

// LookupChpasswdConfig looks up the chpasswd pam file used in the post install
func LookupChpasswdConfig() (string, error) {
	return lookupDefaultFile(ChpasswdPAMFile, "")
}

// LookupDefaultChrootConfig looks up config file within the specified chroot
func LookupDefaultChrootConfig(path string) (string, error) {
	return lookupDefaultFile(ConfigFile, path)
}

// LookupDefaultChrootKernels looks up kernel list definition within the specified chroot
func LookupDefaultChrootKernels(path string) (string, error) {
	return lookupDefaultFile(KernelListFile, path)
}
