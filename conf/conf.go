// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package conf

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func lookupDefaultFile(file string) (string, error) {
	isSourceTree, sourcePath, err := isRunningFromSourceTree()
	if err != nil {
		return "", err
	}

	// use the config from source code's etc dir if not installed binary
	if isSourceTree {
		sourceRoot := strings.Replace(sourcePath, "bin", filepath.Join(SourcePath, "etc"), 1)
		return filepath.Join(sourceRoot, file), nil
	}

	return filepath.Join(DefaultConfigDir, file), nil
}

// LookupBundleListFile looks up the bundle list definition
// Guesses if we're running from source code or from system, if we're running from
// source code directory then we load the source default file, otherwise tried to load
// the system installed file
func LookupBundleListFile() (string, error) {
	return lookupDefaultFile(BundleListFile)
}

// LookupKernelListFile looks up the kernel list definition
// Guesses if we're running from source code or from system, if we're running from
// source code directory then we load the source default file, otherwise load the system
// installed file
func LookupKernelListFile() (string, error) {
	return lookupDefaultFile(KernelListFile)
}

// LookupDefaultConfig looks up the install descriptor
// Guesses if we're running from source code our from system, if we're running from
// source code directory then we loads the source default file, otherwise tried to load
// the system installed file
func LookupDefaultConfig() (string, error) {
	return lookupDefaultFile(ConfigFile)
}

// FetchRemoteConfigFile given an config url fetches it from the network. This function
// currently supports only http/https protocol. After success return the local file path.
func FetchRemoteConfigFile(url string) (string, error) {
	out, err := ioutil.TempFile("", "clr-installer-yaml-")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = out.Close()
	}()

	resp, err := http.Get(url)
	if err != nil {
		defer func() { _ = os.Remove(out.Name()) }()
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		defer func() { _ = os.Remove(out.Name()) }()
		return "", err
	}

	return out.Name(), nil
}

// LookupChpasswdConfig looks up the chpasswd pam file used in the post install
func LookupChpasswdConfig() (string, error) {
	return lookupDefaultFile(ChpasswdPAMFile)
}
