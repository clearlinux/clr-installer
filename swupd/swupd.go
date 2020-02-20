// Copyright © 2019 Intel Corporation
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
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// MirrorTitle specifies title of swupd mirror
	MirrorTitle = "Mirror URL"

	// MirrorDesc1 specifies line 1 of swupd mirror desc
	MirrorDesc1 = "Specify a different installation source (swupd) URL than the default."

	// MirrorDesc2 specifies line 2 of swupd mirror desc
	MirrorDesc2 = "HTTPS sites must use a publicly signed CA."

	// MirrorAllowInsecure specifies allow test regarding enable insecure installs
	MirrorAllowInsecure = "Allow installation over insecure connections (http://)"

	// AutoUpdateTitle specifies title of auto updates
	AutoUpdateTitle = "Automatic OS Updates"

	// AutoUpdateDesc1 specifies line 1 of auto updates desc
	AutoUpdateDesc1 = "Allow Clear Linux* OS to continuously update as new versions are released."

	// AutoUpdateDesc2 specifies line 2 of auto updates desc
	AutoUpdateDesc2 = "This is the default, preferred behavior for Clear Linux* OS to ensure that the latest security concerns are always addressed."

	// AutoUpdateDesc3 specifies line 3 of auto updates desc
	AutoUpdateDesc3 = "To enable this post installation use:"

	// AutoUpdateDesc4 specifies line 4 of auto updates desc
	AutoUpdateDesc4 = "See %s for more information."

	// AutoUpdateCommand specifies auto updates enable command
	AutoUpdateCommand = "\"swupd autoupdate --enable\""

	// AutoUpdateLink specifies document link of auto updates
	AutoUpdateLink = "https://docs.01.org/clearlinux/latest/guides/clear/swupd.html"

	// AutoUpdateLabel specifies label of auto updates
	AutoUpdateLabel = "Enable Auto Updates"

	// AutoUpdateWarning1 specifies part 1 of auto updates warning
	AutoUpdateWarning1 = "WARNING: Disabling Automatic OS Updates puts the system at risk of "

	// AutoUpdateWarning2 specifies part 2 of auto updates desc
	AutoUpdateWarning2 = "missing critical security patches."

	// InvalidURL specifies invalid url error message
	InvalidURL = "Invalid URL: Use HTTPS"

	// IncorrectMirror specifies incorrect mirror error message
	IncorrectMirror = "Mirror not set correctly"

	swupdConfigOverrideDir = "/etc/swupd"

	TargetPrefix = "Target OS: "

	OfflinePrefix = "Offline Content: "

	IsoPrefix = "ISO Initrd: "
)

var (
	// CoreBundles represents the core bundles installed in the Verify() operation
	CoreBundles = []string{
		"os-core",
		"os-core-update",
		"openssh-server",
	}
	prg     progress.Progress
	prgDesc string
)

// SoftwareUpdater abstracts the swupd executable, environment and operations
type SoftwareUpdater struct {
	rootDir            string
	stateDir           string
	stateDirCache      string
	certPath           string
	format             string
	contentURL         string
	versionURL         string
	mirrorURL          string
	downloadOnly       bool
	skipDiskSpaceCheck bool
	allowInsecureHTTP  bool
	skipOptional       bool
}

// Bundle maps a map name and description with the actual checkbox
type Bundle struct {
	Name string // Name the bundle name or id
	Desc string // Desc is the bundle long description
}

// Message represents data parsed from a JSON message sent by a swupd command
type Message struct {
	Type            string `json:"type"`
	Msg             string `json:"msg"`
	Section         string `json:"section"`
	Status          int    `json:"status"`
	CurrentStep     int    `json:"currentStep"`
	StepCompletion  int    `json:"stepCompletion"`
	StepDescription string `json:"stepDescription"`
}

// Process parses the output received from swup and process it according to its type
func (m Message) Process(printPrefix, line string) {

	var description string
	const total = 100

	log.Debug(line)

	// the JSON output of a swupd command, is a big array of JSON objects, like this:
	// [
	// { "type" : "start", "section" : "verify" },
	// ...,
	// { "type" : "end", "section" : "verify", "status" : 0 }
	// ]
	// since we are going to be reading line by line, we can ignore the '[' or ']'
	if line == "[" || line == "]" {
		return
	}

	// also remove the "," ath the end of the string if exist
	trimmedMsg := strings.TrimSuffix(line, ",")

	// decode the message assuming it is a JSON stream and ignore those that are not
	if err := json.Unmarshal([]byte(trimmedMsg), &m); err != nil {
		log.Error("error decoding JSON: %s", err)
		return
	}

	if m.Type == "progress" {
		// "pretty" descriptions for steps
		switch m.StepDescription {
		case "get_versions":
			description = utils.Locale.Get("Resolving OS versions")
		case "cleanup_download_dir":
			description = utils.Locale.Get("Cleaning up download directory")
		case "load_manifests":
			description = utils.Locale.Get("Downloading required manifests")
		case "consolidate_files":
			description = utils.Locale.Get("Resolving files that need to be installed")
		case "download_packs":
			description = utils.Locale.Get("Downloading required packs")
		case "extract_packs":
			description = utils.Locale.Get("Extracting required packs")
		case "check_files_hash":
			description = utils.Locale.Get("Verifying installed files")
		case "validate_fullfiles":
			description = utils.Locale.Get("Verifying staged files")
		case "download_fullfiles":
			description = utils.Locale.Get("Downloading missing files")
		case "extract_fullfiles":
			description = utils.Locale.Get("Extracting missing files")
		case "add_missing_files":
			description = utils.Locale.Get("Installing base OS and configured bundles")
		case "run_postupdate_scripts":
			description = utils.Locale.Get("Running post-update scripts")
		}

		// The printPrefix string is used to separate target, offline content,
		// and ISO installations.
		description = printPrefix + description

		if m.StepCompletion == -1 {
			if prgDesc != m.StepDescription {
				// create a new instance of the indeterminate progress bar with the correct description
				log.Debug("%s: Setting indeterminate progress for task %s", printPrefix, m.StepDescription)
				prg = progress.NewLoop(description)
				prgDesc = m.StepDescription
			}
			return
		}

		if prgDesc != m.StepDescription {
			// create a new instance of the step progress bar with the correct description
			log.Debug("%s: Setting progress for task %s", printPrefix, m.StepDescription)
			prg = progress.MultiStep(total, description)
			prgDesc = m.StepDescription
		}

		// report current % of completion
		prg.Partial(m.StepCompletion)
		if m.StepCompletion == total {
			log.Debug("%s: Task %s completed", printPrefix, m.StepDescription)
			prg.Success()
			prgDesc = ""
		}
	}
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

// IsOfflineContent checks that the offline content dir is valid for swupd to consume
func IsOfflineContent() bool {
	info, err := os.Stat(conf.OfflineContentDir)
	if err != nil {
		return false
	}

	// Check required permissions for offline content directory
	if !info.IsDir() || info.Mode().Perm() != os.FileMode(0700) {
		return false
	}

	return true
}

// New creates a new instance of SoftwareUpdater with the rootDir properly adjusted
func New(rootDir string, options args.Args, model *model.SystemInstall) *SoftwareUpdater {
	stateDir := options.SwupdStateDir
	if stateDir == "" {
		stateDir = filepath.Join(rootDir, "/var/lib/swupd")
	}

	stateDirCache := ""
	if IsOfflineContent() {
		stateDirCache = conf.OfflineContentDir
	}

	downloadOnly := false

	return &SoftwareUpdater{
		rootDir,
		stateDir,
		stateDirCache,
		options.SwupdCertPath,
		model.SwupdFormat,
		options.SwupdContentURL,
		options.SwupdVersionURL,
		model.SwupdMirror,
		downloadOnly,
		options.SwupdSkipDiskSpaceCheck,
		model.AllowInsecureHTTP,
		model.SwupdSkipOptional,
	}
}

func (s *SoftwareUpdater) setExtraFlags(args []string) []string {
	if s.allowInsecureHTTP {
		args = append(args, "--allow-insecure-http")
	}

	if s.certPath != "" {
		args = append(args, fmt.Sprintf("--certpath=%s", s.certPath))
	}

	if s.downloadOnly != false {
		args = append(args, fmt.Sprintf("--download"))
	}

	if s.format != "" {
		args = append(args, fmt.Sprintf("--format=%s", s.format))
	}

	if s.skipOptional {
		args = append(args, "--skip-optional")
	}

	if s.stateDirCache != "" {
		args = append(args, fmt.Sprintf("--statedir-cache=%s", s.stateDirCache))
	}

	if s.contentURL != "" {
		args = append(args, fmt.Sprintf("--contenturl=%s", s.contentURL))
	}

	if s.versionURL != "" {
		args = append(args, fmt.Sprintf("--versionurl=%s", s.versionURL))
	}

	return args
}

// GetStateDir returns the state directory
func (s *SoftwareUpdater) GetStateDir() string {
	return s.stateDir
}

// OSInstall runs "swupd os-install" operation with a bundle list
func (s *SoftwareUpdater) OSInstall(version, printPrefix string, bundles []string) error {
	args := []string{
		"swupd",
		"os-install",
	}

	args = s.setExtraFlags(args)

	if s.mirrorURL != "" {
		args = append(args, fmt.Sprintf("--url=%s", s.mirrorURL))
	}
	args = append(args,
		[]string{
			fmt.Sprintf("--path=%s", s.rootDir),
			fmt.Sprintf("--statedir=%s", s.stateDir),
			"-V",
			version,
			"--force",
			"--no-boot-update",
			"--json-output",
		}...)

	if len(bundles) > 0 {
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

		args = append(args, "-B", strings.Join(allBundles, ","))
	}

	m := Message{}
	err := cmd.RunAndProcessOutput(printPrefix, m, args...)
	if err != nil {
		err = fmt.Errorf("The swupd command \"%s\" failed with %s", strings.Join(args, " "), err)
		return errors.Wrap(err)
	}

	if s.mirrorURL != "" {
		args = []string{
			"swupd",
			"mirror",
			fmt.Sprintf("--path=%s", s.rootDir),
			"--set",
			s.mirrorURL,
		}

		if s.allowInsecureHTTP {
			args = append(args, "--allow-insecure-http")
		}

		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	return nil
}

// DownloadBundles downloads the bundle list to the OfflineContentDir within the installer image
func (s SoftwareUpdater) DownloadBundles(version string, bundles []string) error {
	var err error

	s.downloadOnly = true

	// The statedir-cache is set to the statedir that was used to install bundles to the
	// installer image which reduces the number of bundle downloads. The statedir specifies the
	// location to download offline content which is set ot the offline content directory within
	// the installer image.
	s.stateDirCache = s.stateDir
	s.stateDir = filepath.Join(s.rootDir, conf.OfflineContentDir)

	// Set swupd's --path argument to an empty directory so that all listed bundles will be
	// downloaded (Swupd skips over previously installed bundles)
	if s.rootDir, err = ioutil.TempDir("", "installerTmp-"); err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(s.rootDir) }()

	return s.OSInstall(version, OfflinePrefix, bundles)
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
func SetHostMirror(url string, allowInsecureHTTP bool) (string, error) {
	if urlErr := network.CheckURL(url); urlErr != nil {
		if strings.Contains(urlErr.Error(), "60") {
			return "", fmt.Errorf(utils.Locale.Get("SSL certificate problem"))
		}
		return "", fmt.Errorf(utils.Locale.Get("Server not responding"))
	}

	args := []string{
		"swupd",
		"mirror",
		"--set",
		url,
	}

	if allowInsecureHTTP {
		args = append(args, "--allow-insecure-http")
	}

	url, err := setMirror(args, "Host")
	if err == nil {
		if err = checkHostSwupd(allowInsecureHTTP); err != nil {
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

	if s.allowInsecureHTTP {
		args = append(args, "--allow-insecure-http")
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
			err = fmt.Errorf(utils.Locale.Get("Server does not report any version"))
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
func checkHostSwupd(allowInsecureHTTP bool) error {
	args := []string{
		"timeout",
		"--kill-after=5",
		"5",
		"swupd",
		"check-update",
	}

	if allowInsecureHTTP {
		args = append(args, "--allow-insecure-http")
	}

	return checkSwupd(args, "Host")
}

func parseSwupdMirror(data []byte) (string, error) {
	versionExp := regexp.MustCompile(`Version URL:\s+(\S+)`)
	match := versionExp.FindSubmatch(data)

	if len(match) != 2 {
		return "", errors.Errorf(utils.Locale.Get("Version URL of mirror not found"))
	}

	return string(match[1]), nil
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

// CreateConfig create swupd config file in the targets /etc directory
// to allow http for URLs
func CreateConfig(rootDir string) {
	configFile := filepath.Join(rootDir, swupdConfigOverrideDir, "config")

	log.Debug("Creating/Appending to swupd override file %q on install to target", configFile)

	swupdFh, err := os.OpenFile(configFile,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Warning("Error opening swupd config %q: %v", configFile, err)
	}
	defer func() { _ = swupdFh.Close() }()

	allowText := "\n" + "[GLOBAL]\n" +
		"# Allow updates over insecure connections\n" +
		"allow_insecure_http=true\n"

	if _, err := swupdFh.WriteString(allowText); err != nil {
		log.Warning("Error writing to swupd config %q: %v", configFile, err)
	}
}

// CopyConfigurations transfer the swupd override files to the target installation media
func CopyConfigurations(rootDir string) {
	log.Debug("Checking swupd override files in %q to install to target", swupdConfigOverrideDir)
	if _, err := os.Stat(swupdConfigOverrideDir); err != nil {
		if os.IsNotExist(err) {
			log.Debug("No swupd override files in %q to install to target",
				swupdConfigOverrideDir)
			return
		}
		return
	}

	if err := utils.CopyAllFiles(swupdConfigOverrideDir, rootDir); err != nil {
		log.Warning("Failed to copy swupd override configuration data to %s", swupdConfigOverrideDir)
	} else {
		log.Info("Copied swupd override configuration data to %s",
			filepath.Join(rootDir, swupdConfigOverrideDir))
	}
}
