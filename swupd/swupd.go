// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package swupd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/network"
)

var (
	// CoreBundles represents the core bundles installed in the Verify() operation
	CoreBundles = []string{
		"os-core",
		"os-core-update",
	}
)

// SoftwareUpdater abstracts the swupd executable, environment and operations
type SoftwareUpdater struct {
	rootDir  string
	stateDir string
}

// Bundle maps a map name and description with the actual checkbox
type Bundle struct {
	Name string // Name the bundle name or id
	Desc string // Desc is the bundle long description
}

// IsCoreBundle checks if bundle is in the list of core bundles
func IsCoreBundle(bundle string) bool {
	for _, curr := range CoreBundles {
		if curr == bundle {
			return true
		}
	}
	return false
}

// New creates a new instance of SoftwareUpdater with the rootDir properly adjusted
func New(rootDir string) *SoftwareUpdater {
	return &SoftwareUpdater{rootDir, filepath.Join(rootDir, "/var/lib/swupd")}
}

// Verify runs "swupd verify" operation
func (s *SoftwareUpdater) Verify(version string, mirror string) error {
	args := []string{
		"swupd",
		"verify",
	}
	if mirror != "" {
		args = append(args, fmt.Sprintf("--url=%s", mirror))
	}
	args = append(args,
		[]string{
			fmt.Sprintf("--path=%s", s.rootDir),
			fmt.Sprintf("--statedir=%s", s.stateDir),
			"--install",
			"-m",
			version,
			"--force",
			"--no-scripts",
		}...)

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	if mirror != "" {
		args = []string{
			"swupd",
			"mirror",
			fmt.Sprintf("--path=%s", s.rootDir),
			"--set",
			mirror,
		}

		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	args = []string{
		"swupd",
		"bundle-add",
		fmt.Sprintf("--path=%s", s.rootDir),
		fmt.Sprintf("--statedir=%s", s.stateDir),
		"os-core-update",
	}

	err = cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// Update executes the "swupd update" operation
func (s *SoftwareUpdater) Update() error {
	args := []string{
		filepath.Join(s.rootDir, "/usr/bin/swupd"),
		"update",
		fmt.Sprintf("--path=%s", s.rootDir),
		fmt.Sprintf("--statedir=%s", s.stateDir),
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// DisableUpdate executes the "systemctl" to disable auto update operation
// "swupd autoupdate" currently does not --path
// See Issue https://github.com/clearlinux/swupd-client/issues/527
func (s *SoftwareUpdater) DisableUpdate() error {
	args := []string{
		filepath.Join(s.rootDir, "/usr/bin/systemctl"),
		fmt.Sprintf("--root=%s", s.rootDir),
		"mask",
		"--now",
		"swupd-update.service",
		"swupd-update.timer",
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// getMirror executes the "swupd mirror" to find the current mirror
func getMirror(swupdArgs []string, t string) (string, error) {
	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, swupdArgs...)
	if err != nil {
		return "", fmt.Errorf("%s", w.String())
	}

	url, err := parseSwupdMirror(w.Bytes())
	if err != nil {
		return "", err
	}

	log.Debug("%s swupd version URL: %s", t, url)

	return url, nil
}

// GetHostMirror executes the "swupd mirror" to find the Host's mirror
func GetHostMirror() (string, error) {
	args := []string{
		"/usr/bin/swupd",
		"mirror",
	}

	return getMirror(args, "Host")
}

// GetTargetMirror executes the "swupd mirror" to find the Target's mirror
func (s *SoftwareUpdater) GetTargetMirror() (string, error) {
	args := []string{
		filepath.Join(s.rootDir, "/usr/bin/swupd"),
		"mirror",
		fmt.Sprintf("--path=%s", s.rootDir),
	}

	return getMirror(args, "Target")
}

// setMirror executes the "swupd mirror" to set the current mirror
func setMirror(swupdArgs []string, t string) (string, error) {
	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, swupdArgs...)
	if err != nil {
		return "", fmt.Errorf("%s", w.String())
	}

	url, err := parseSwupdMirror(w.Bytes())
	if err != nil {
		return "", err
	}

	log.Debug("%s swupd version URL: %s", t, url)

	return url, nil
}

// SetHostMirror executes the "swupd mirror" to set the Host's mirror
func SetHostMirror(url string) (string, error) {

	if urlErr := network.CheckURL(url); urlErr != nil {
		return "", fmt.Errorf("Server not responding")
	}

	args := []string{
		"/usr/bin/swupd",
		"mirror",
		"--set",
		url,
	}

	url, err := setMirror(args, "Host")
	if err == nil {
		if err = checkHostSwupd(); err != nil {
			url = ""
			_, _ = UnSetHostMirror()
		}
	}

	return url, err
}

// SetTargetMirror executes the "swupd mirror" to set the Target's mirror
// URL error checking is not done as it is implied the URL was already
// verified as functional on the currently running Host
func (s *SoftwareUpdater) SetTargetMirror(url string) (string, error) {
	args := []string{
		filepath.Join(s.rootDir, "/usr/bin/swupd"),
		"mirror",
		fmt.Sprintf("--path=%s", s.rootDir),
		"--set",
		url,
	}

	return setMirror(args, "Target")
}

// unSetMirror executes the "swupd mirror" to unset the current mirror
func unSetMirror(swupdArgs []string, t string) (string, error) {
	w := bytes.NewBuffer(nil)

	err := cmd.Run(w, swupdArgs...)
	if err != nil {
		return "", fmt.Errorf("%s", w.String())
	}

	url, err := parseSwupdMirror(w.Bytes())
	if err != nil {
		return "", err
	}

	log.Debug("%s swupd version UNSET to URL: %s", t, url)

	return url, nil
}

// UnSetHostMirror executes the "swupd mirror" to unset the Host's mirror
func UnSetHostMirror() (string, error) {
	args := []string{
		"/usr/bin/swupd",
		"mirror",
		"--unset",
	}

	return unSetMirror(args, "Host")
}

// checkSwupd executes the "swupd check-update" to verify connectivity
func checkSwupd(swupdArgs []string, t string) error {
	w := bytes.NewBuffer(nil)

	err := cmd.Run(w, swupdArgs...)
	if err != nil {
		// Swupd uses exit status '1' when there are no updates (and no errors)
		if !strings.Contains(w.String(), "There are no updates available") {
			log.Debug("%s swupd check-update failed: %q", t, fmt.Errorf("%s", w.String()))
			err = fmt.Errorf("Server does not report any version")
		} else {
			log.Debug("%s swupd check-update results ignored: %q", t, err)
			err = nil
		}
	} else {
		log.Debug("%s swupd check-update passed: %q", t, fmt.Errorf("%s", w.String()))
	}

	return err
}

// checkHostSwupd executes the "swupd check-update" to verify the Host's mirror
func checkHostSwupd() error {
	args := []string{
		"timeout",
		"--kill-after=5",
		"5",
		"/usr/bin/swupd",
		"check-update",
	}

	return checkSwupd(args, "Host")
}

func parseSwupdMirror(data []byte) (string, error) {
	versionExp := regexp.MustCompile(`Version URL:\s+(\S+)`)
	match := versionExp.FindSubmatch(data)

	if len(match) != 2 {
		return "", errors.Errorf("swupd mirror Version URL not found")
	}

	return string(match[1]), nil
}

// BundleAdd executes the "swupd bundle-add" operation for a single bundle
func (s *SoftwareUpdater) BundleAdd(bundle string) error {
	args := []string{
		filepath.Join(s.rootDir, "/usr/bin/swupd"),
		"bundle-add",
		fmt.Sprintf("--path=%s", s.rootDir),
		fmt.Sprintf("--statedir=%s", s.stateDir),
		bundle,
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// LoadBundleList loads the bundle definitions
func LoadBundleList() ([]*Bundle, error) {
	path, err := conf.LookupBundleListFile()
	if err != nil {
		return nil, err
	}

	root := struct {
		Bundles []*Bundle `json:"bundles"`
	}{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err)
	}

	if err = json.Unmarshal(data, &root); err != nil {
		return nil, errors.Wrap(err)
	}

	return root.Bundles, nil
}
