// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package utils

import (
	"fmt"
	"io/ioutil"
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

// CopyFile copies src file to dest
func CopyFile(src string, dest string) error {
	destDir := filepath.Dir(dest)

	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf("no such file: %s", src)
		}
		return errors.Wrap(err)
	}

	if _, err = os.Stat(destDir); err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf("no such dest directory: %s", destDir)
		}
		return errors.Wrap(err)
	}

	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(dest, data, srcInfo.Mode()&os.ModePerm); err != nil {
		return err
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

// ExpandVariables iterates over vars map and replace all the ocorrences of ${var} or
// $var in the str string
func ExpandVariables(vars map[string]string, str string) string {
	// iterate over available variables
	for k, v := range vars {
		// tries to replace both ${var} and $var forms
		for _, rep := range []string{fmt.Sprintf("$%s", k), fmt.Sprintf("${%s}", k)} {
			if strings.Contains(str, rep) {
				return strings.Replace(str, rep, v, -1)
			}
		}
	}

	// if no variables are expanded return the original string
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
