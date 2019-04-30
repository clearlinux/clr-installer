// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package swupd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/network"
)

var (
	// CoreBundles represents the core bundles installed in the Verify() operation
	CoreBundles = []string{
		"os-core",
		"os-core-update",
		"openssh-server",
	}
)

// SoftwareUpdater abstracts the swupd executable, environment and operations
type SoftwareUpdater struct {
	rootDir            string
	stateDir           string
	format             string
	contentURL         string
	versionURL         string
	skipDiskSpaceCheck bool
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
func New(rootDir string, options args.Args) *SoftwareUpdater {
	stateDir := options.SwupdStateDir

	if stateDir == "" {
		stateDir = filepath.Join(rootDir, "/var/lib/swupd")
	}

	return &SoftwareUpdater{
		rootDir,
		stateDir,
		options.SwupdFormat,
		options.SwupdContentURL,
		options.SwupdVersionURL,
		options.SwupdSkipDiskSpaceCheck,
	}
}

func (s *SoftwareUpdater) setExtraFlags(args []string) []string {
	if s.format != "" {
		args = append(args, fmt.Sprintf("--format=%s", s.format))
	}

	if s.contentURL != "" {
		args = append(args, fmt.Sprintf("--contenturl=%s", s.contentURL))
	}

	if s.versionURL != "" {
		args = append(args, fmt.Sprintf("--versionurl=%s", s.versionURL))
	}

	return args
}

// Verify runs "swupd verify" operation
func (s *SoftwareUpdater) Verify(version string, mirror string, verifyOnly bool) error {
	args := []string{
		"swupd",
		"verify",
	}

	args = s.setExtraFlags(args)

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

	if verifyOnly {
		return nil
	}

	args = []string{
		"swupd",
		"bundle-add",
	}

	if s.skipDiskSpaceCheck {
		args = append(args, "--skip-diskspace-check")
	}

	args = s.setExtraFlags(args)

	args = append(args,
		fmt.Sprintf("--path=%s", s.rootDir),
		fmt.Sprintf("--statedir=%s", s.stateDir),
	)

	// Remove the 'os-core' bundle as it is already
	// installed and will cause a failure
	for _, bundle := range CoreBundles {
		if bundle != "os-core" {
			args = append(args, bundle)
		}
	}

	err = cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// VerifyWithBundles runs "swupd verify" operation with all bundles
func (s *SoftwareUpdater) VerifyWithBundles(version string, mirror string, bundles []string) error {
	args := []string{
		"swupd",
		"verify",
	}

	args = s.setExtraFlags(args)

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
			"-B",
		}...)

	// Remove the 'os-core' bundle as it is already
	// installed and will cause a failure
	allBundles := []string{}
	for _, bundle := range CoreBundles {
		if bundle != "os-core" {
			allBundles = append(allBundles, bundle)
		}
	}
	// Additional bundles
	for _, bundle := range bundles {
		if IsCoreBundle(bundle) {
			log.Debug("Bundle %s was already installed with the core bundles, skipping", bundle)
			continue
		}
		allBundles = append(allBundles, bundle)
	}

	args = append(args, strings.Join(allBundles, ","))

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

	return nil
}

// Update executes the "swupd update" operation
func (s *SoftwareUpdater) Update() error {
	args := []string{
		"swupd",
		"update",
		"--keepcache",
		fmt.Sprintf("--path=%s", s.rootDir),
		fmt.Sprintf("--statedir=%s", s.stateDir),
	}

	log.Info("Checking for swupd updates")

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
		"chroot",
		s.rootDir,
		"systemctl",
		"mask",
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
		"swupd",
		"mirror",
	}

	return getMirror(args, "Host")
}

// GetTargetMirror executes the "swupd mirror" to find the Target's mirror
func (s *SoftwareUpdater) GetTargetMirror() (string, error) {
	args := []string{
		"swupd",
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
		"swupd",
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
		"swupd",
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
		"swupd",
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
		"swupd",
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
		"swupd",
		"bundle-add",
	}

	if s.skipDiskSpaceCheck {
		args = append(args, "--skip-diskspace-check")
	}

	args = s.setExtraFlags(args)

	args = append(args,
		fmt.Sprintf("--path=%s", s.rootDir),
		fmt.Sprintf("--statedir=%s", s.stateDir),
		bundle,
	)

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// LoadBundleList loads the bundle definitions
func LoadBundleList(model *model.SystemInstall) ([]*Bundle, error) {
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

	// Read the bundles from the optional bundle list file
	if err = json.Unmarshal(data, &root); err != nil {
		return nil, errors.Wrap(err)
	}

	// Filter out the bundles which will always be installed
	filteredBundles := []*Bundle{}

	for _, bundle := range root.Bundles {
		if !model.ContainsBundle(bundle.Name) {
			filteredBundles = append(filteredBundles, bundle)
		}
	}

	return filteredBundles, nil
}

// CleanUpState removes the swupd state content directory
func (s *SoftwareUpdater) CleanUpState() error {

	log.Debug("Removing swupd state directory: %s", s.stateDir)

	err := os.RemoveAll(s.stateDir)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}
