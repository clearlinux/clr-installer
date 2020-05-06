// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package utils

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/digitalocean/go-smbios/smbios"
	"github.com/leonelquinteros/gotext"

	"github.com/clearlinux/clr-installer/errors"
)

// ClearVersion is running version of the OS
var ClearVersion string

// ParseOSClearVersion parses the current version of the Clear Linux OS
func ParseOSClearVersion() error {
	var versionBuf []byte
	var err error

	// in order to avoid issues raised by format bumps between installers image
	// version and the latest released we assume the installers host version
	// in other words we use the same version swupd is based on
	if versionBuf, err = ioutil.ReadFile("/usr/lib/os-release"); err != nil {
		return errors.Errorf("Read version file /usr/lib/os-release: %v", err)
	}
	versionExp := regexp.MustCompile(`VERSION_ID=([0-9][0-9]*)`)
	match := versionExp.FindSubmatch(versionBuf)

	if len(match) < 2 {
		return errors.Errorf("Version not found in /usr/lib/os-release")
	}

	ClearVersion = string(match[1])

	return nil
}

// MkdirAll similar to go's standard os.MkdirAll() this function creates a directory
// named path, along with any necessary parents but also checks if path exists and
// takes no action if that's true.
func MkdirAll(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := os.MkdirAll(path, perm); err != nil {
		return errors.Errorf("mkdir %s: %v", path, err)
	}

	return nil
}

// CopyAllFiles copy all of the files in a directory recursively
func CopyAllFiles(srcDir string, destDir string) error {
	err := filepath.Walk(srcDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.Errorf("Failure accessing a path %q: %v\n", path, err)
			}
			target := filepath.Join(destDir, path)
			if info.IsDir() {
				// Create the matching target directory
				if err := MkdirAll(target, info.Mode()); err != nil {
					return errors.Errorf("Failed to mkdir %q", target)
				}

				return nil
			}
			if err := CopyFile(path, target); err != nil {
				return errors.Errorf("Failed to copy file %q", target)
			}

			// Ensure all contents is owned correctly
			sys := info.Sys()
			stat, ok := sys.(*syscall.Stat_t)
			if ok {
				if err := os.Chown(target, int(stat.Uid), int(stat.Gid)); err != nil {
					return errors.Errorf("Failed to change ownership of %q to UID:%d, GID:%d",
						target, stat.Uid, stat.Gid)
				}
			} else {
				return errors.Errorf("Could not stat file %q", path)
			}

			return nil
		})

	if err != nil {
		return fmt.Errorf("Failed to copy files")
	}

	return nil
}

// CopyFile copies src file to dest
func CopyFile(src string, dest string) error {
	destDir := filepath.Dir(dest)
	if _, err := os.Stat(destDir); err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf("no such dest directory: %s", destDir)
		}
		return errors.Wrap(err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf("no such file: %s", src)
		}
		return errors.Wrap(err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return errors.Wrap(err)
	}

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode()&os.ModePerm)
	if err != nil {
		return errors.Wrap(err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return errors.Wrap(err)
	}

	// Flush cache to disk
	if err := destFile.Sync(); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// FileExists returns true if the file or directory exists
// else it returns false and the associated error
func FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

// VerifyRootUser returns an error if we're not running as root
func VerifyRootUser() string {
	// ProgName is the short name of this executable
	progName := path.Base(os.Args[0])

	user, err := user.Current()
	if err != nil {
		return fmt.Sprintf("%s MUST run as 'root' user to install! (user=%s)",
			progName, "UNKNOWN")
	}

	if user.Uid != "0" {
		return fmt.Sprintf("%s MUST run as 'root' user to install! (user=%s)",
			progName, user.Uid)
	}

	return ""
}

// IsClearLinux checks if the current OS is Clear by looking for Swupd
// Mostly used in Go Testing
func IsClearLinux() bool {
	is := false

	if runtime.GOOS == "linux" {
		clearFile := "/usr/bin/swupd"
		if _, err := os.Stat(clearFile); !os.IsNotExist(err) {
			is = true
		}
	}

	return is
}

// IsRoot checks if the current User is root (UID 0)
// Mostly used in Go Testing
func IsRoot() bool {
	is := false

	user, err := user.Current()
	if err == nil {
		if user.Uid == "0" {
			is = true
		}
	}

	return is
}

// StringSliceContains returns true if sl contains str, returns false otherwise
func StringSliceContains(sl []string, str string) bool {
	for _, curr := range sl {
		if curr == str {
			return true
		}
	}
	return false
}

// IntSliceContains returns true if is contains value, returns false otherwise
func IntSliceContains(is []int, value int) bool {
	for _, curr := range is {
		if curr == value {
			return true
		}
	}
	return false
}

// IsCheckCoverage returns true if CHECK_COVERAGE variable is set
func IsCheckCoverage() bool {
	return os.Getenv("CHECK_COVERAGE") != ""
}

// IsStdoutTTY returns true if the stdout is attached to a tty
func IsStdoutTTY() bool {
	var termios syscall.Termios

	fd := os.Stdout.Fd()
	ptr := uintptr(unsafe.Pointer(&termios))
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCGETS, ptr, 0, 0, 0)

	return err == 0
}

// ExpandVariables iterates over vars map and replace all the occurrences of ${var} or
// $var in the str string
func ExpandVariables(vars map[string]string, str string) string {
	// iterate over available variables
	for k, v := range vars {
		// tries to replace both ${var} and $var forms
		for _, rep := range []string{fmt.Sprintf("$%s", k), fmt.Sprintf("${%s}", k)} {
			str = strings.ReplaceAll(str, rep, v)
		}
	}
	return str
}

// IsVirtualBox returns true if the running system is executed
// from within VirtualBox
// Attempt to parse the System Management BIOS (SMBIOS) and
// Desktop Management Interface (DMI) to determine if we are
// executing inside a VirtualBox. Ignoring error conditions and
// assuming we are not VirtualBox.
func IsVirtualBox() bool {
	virtualBox := false

	// Find SMBIOS data in operating system-specific location.
	rc, _, err := smbios.Stream()
	if err != nil {
		return virtualBox
	}

	// Be sure to close the stream!
	defer func() { _ = rc.Close() }()

	// Decode SMBIOS structures from the stream.
	// https://www.dmtf.org/sites/default/files/standards/documents/DSP0134_3.1.1.pdf
	d := smbios.NewDecoder(rc)
	ss, err := d.Decode()
	if err != nil {
		return virtualBox
	}

	for _, s := range ss {
		// 7.2 System Information (Type 1)
		if s.Header.Type == 1 {
			for _, str := range s.Strings {
				if strings.Contains(strings.ToLower(str), "virtualbox") {
					virtualBox = true
					return virtualBox
				}
			}
		}
	}

	return virtualBox
}

// LookupThemeDir returns the directory to use for reading
// theme files for the UI.
func LookupThemeDir() (string, error) {
	return lookupDir("/usr/share/clr-installer/themes", "CLR_INSTALLER_THEME_DIR")
}

// LookupLocaleDir returns the directory to use for reading
// locale files for the UI.
func LookupLocaleDir() (string, error) {
	return lookupDir("/usr/share/locale", "CLR_INSTALLER_LOCALE_DIR")
}

// lookupDir returns the full directory path of a directory.
// It will look in the local developers build area first,
// or the ENV variable, and finally the standard
// system install location
func lookupDir(dir, env string) (string, error) {
	var result string

	fullDir := []string{
		os.Getenv(env),
	}

	src, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}

	if strings.Contains(src, "/.gopath/bin") {
		fullDir = append(fullDir, strings.Replace(src, "bin", "../"+filepath.Base(dir), 1))
	}

	fullDir = append(fullDir, dir+"/")

	for _, curr := range fullDir {
		if _, err := os.Stat(curr); os.IsNotExist(err) {
			continue
		}

		result = curr
		break
	}

	if result == "" {
		panic(errors.Errorf("Could not find a %s dir", dir))
	}

	return result, nil
}

// Locale is used to access the localization functions
var Locale *gotext.Locale

// Ensure Locale always has a default
func init() {
	SetLocale("en_US.UTF-8")
}

// SetLocale sets the locale of the installer based on the selected language
func SetLocale(language string) {
	dir, err := LookupLocaleDir()
	if err != nil {
		log.Fatal(err)
		Locale = nil
	} else {
		Locale = gotext.NewLocale(dir, language)
		Locale.AddDomain("clr-installer")
	}
}

// LookupISOTemplateDir returns the directory to use for reading
// template files for ISO creation. It will look in the local developers
// build area first, or the ENV variable, and finally the standard
// system install location
func LookupISOTemplateDir() (string, error) {
	var result string

	isoTemplateDirs := []string{
		os.Getenv("CLR_INSTALLER_ISO_TEMPLATE_DIR"),
	}

	src, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}

	if strings.Contains(src, "/.gopath/bin") {
		isoTemplateDirs = append(isoTemplateDirs, strings.Replace(src, "bin", "../iso_templates", 1))
	}

	isoTemplateDirs = append(isoTemplateDirs, "/usr/share/clr-installer/iso_templates/")

	for _, curr := range isoTemplateDirs {
		if _, err := os.Stat(curr); os.IsNotExist(err) {
			continue
		}

		result = curr
		break
	}

	if result == "" {
		panic(errors.Errorf("Could not find a ISO templates dir"))
	}

	return result, nil
}

// RunDiskPartitionTool creates and executes a script which launches
// the disk partitioning tool and then returns to the installer.
func RunDiskPartitionTool(tmpYaml string, lockFile string, diskUtilCmd string,
	remove []string, gui bool) (string, error) {
	// We need to save the current state model for the relaunch of clr-installer
	tmpBash, err := ioutil.TempFile("", "clr-installer-diskUtil-*.sh")
	if err != nil {
		return "", errors.Errorf("Could not make BASH tempfile: %v", err)
	}
	defer func() { _ = tmpBash.Close() }()

	var content bytes.Buffer
	_, _ = fmt.Fprintf(&content, "#!/bin/bash\n")
	// To ensure another instance is not launched, first recreate the
	// installer lock file using the PID of the running script
	_, _ = fmt.Fprintf(&content, "echo $$ > %s\n", lockFile)

	_, _ = fmt.Fprintf(&content, "result=0\n")

	args := append(os.Args, "--config", tmpYaml)
	if gui {
		_, _ = fmt.Fprintf(&content, "if [ -n \"${SUDO_USER}\" ]; then\n")
		_, _ = fmt.Fprintf(&content, "    sudo --user=${SUDO_USER} xhost +si:localuser:root\n")
		_, _ = fmt.Fprintf(&content, "    result=$(( ${result} + $? ))\n")
		_, _ = fmt.Fprintf(&content, "fi\n")
	}
	_, _ = fmt.Fprintf(&content, "echo Switching to Disk Partitioning tool\n")
	if !gui {
		_, _ = fmt.Fprintf(&content, "sleep 2\n")
	}
	_, _ = fmt.Fprintf(&content, "%s\n", diskUtilCmd)
	_, _ = fmt.Fprintf(&content, "result=$(( ${result} + $? ))\n")
	if !gui {
		_, _ = fmt.Fprintf(&content, "sleep 1\n")
	}
	_, _ = fmt.Fprintf(&content, "echo Checking partitions with partprobe\n")
	_, _ = fmt.Fprintf(&content, "/usr/bin/partprobe\n")
	_, _ = fmt.Fprintf(&content, "result=$(( ${result} + $? ))\n")
	if !gui {
		_, _ = fmt.Fprintf(&content, "sleep 1\n")
	}
	_, _ = fmt.Fprintf(&content, "echo Restarting Clear Linux OS Installer ...\n")
	if gui {
		args = append(args, "--gui")
	} else {
		_, _ = fmt.Fprintf(&content, "sleep 2\n")
		args = append(args, "--tui")
	}

	args = append(args, "--cfPurge")

	_, _ = fmt.Fprintf(&content, "if [ ${result} -eq 0 ]; then\n")
	_, _ = fmt.Fprintf(&content, "    /bin/rm %s\n", tmpBash.Name())
	for _, file := range remove {
		_, _ = fmt.Fprintf(&content, "    /bin/rm -rf %s\n", file)
	}
	_, _ = fmt.Fprintf(&content, "else\n")
	_, _ = fmt.Fprintf(&content, "    /bin/cp -a /root/clr-installer.log /root/clr-installer.log.$$\n")
	_, _ = fmt.Fprintf(&content, "fi\n")

	_, _ = fmt.Fprintf(&content, "/bin/rm %s\n", lockFile)

	allArgs := strings.Join(args, " ")
	_, err = fmt.Fprintf(&content, "exec %s\n", allArgs)
	if err != nil {
		return "", errors.Errorf("Could not write BASH buffer: %v", err)
	}
	if _, err := tmpBash.Write(content.Bytes()); err != nil {
		return "", errors.Errorf("Could not write BASH tempfile: %v", err)
	}
	_ = os.Chmod(tmpBash.Name(), 0700)

	return tmpBash.Name(), nil
}

// HostHasEFI check if the running host supports EFI booting
func HostHasEFI() bool {
	if _, err := os.Stat("/sys/firmware/efi"); os.IsNotExist(err) {
		return false
	}

	return true
}
