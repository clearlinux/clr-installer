// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
)

var storageExp = regexp.MustCompile(`^([0-9]*(\.)?[0-9]*)([bkmgtp]{1}){0,1}$`)

func mountFs(device string, mPointPath string, fsType string, flags uintptr) error {
	var err error

	if _, err = os.Stat(mPointPath); os.IsNotExist(err) {
		if err = os.MkdirAll(mPointPath, 0777); err != nil {
			return errors.Errorf("mkdir %s: %v", mPointPath, err)
		}
	}

	if err = syscall.Mount(device, mPointPath, fsType, flags, ""); err != nil {
		return errors.Errorf("mount %s %s %s: %v", device, mPointPath, fsType, err)
	}
	log.Debug("Mounted ok: %s", mPointPath)
	// Store the mount point for later unmounting
	mountedPoints = append(mountedPoints, mPointPath)

	return err
}

func mountDevFs(rootDir string) error {
	mPointPath := filepath.Join(rootDir, "dev")

	return mountFs("/dev", mPointPath, "devtmpfs", syscall.MS_BIND)
}

func mountSysFs(rootDir string) error {
	mPointPath := filepath.Join(rootDir, "sys")

	return mountFs("/sys", mPointPath, "sysfs", syscall.MS_BIND)
}

func mountProcFs(rootDir string) error {
	mPointPath := filepath.Join(rootDir, "proc")

	return mountFs("/proc", mPointPath, "proc", syscall.MS_BIND)
}

// MountMetaFs mounts proc, sysfs and devfs in the target installation directory
func MountMetaFs(rootDir string) error {
	err := mountProcFs(rootDir)
	if err != nil {
		return err
	}

	err = mountSysFs(rootDir)
	if err != nil {
		return err
	}

	err = mountDevFs(rootDir)
	if err != nil {
		return err
	}

	return nil
}

// UmountAll unmounts all previously mounted devices
func UmountAll() error {
	var mountError error
	fails := make([]string, 0)

	// Ensure the top level mount point is unmounted last
	sort.Sort(sort.Reverse(sort.StringSlice(mountedPoints)))

	for _, point := range mountedPoints {
		if err := syscall.Unmount(point, syscall.MNT_FORCE|syscall.MNT_DETACH); err != nil {
			err = fmt.Errorf("umount %s: %v", point, err)
			log.ErrorError(err)
			fails = append(fails, point)
		} else {
			log.Debug("Unmounted ok: %s", point)
		}
	}

	for _, point := range mountedEncrypts {
		if err := unMapEncrypted(point); err != nil {
			err = fmt.Errorf("unmap encrypted %s: %v", point, err)
			log.ErrorError(err)
			fails = append(fails, "e-"+point)
		} else {
			log.Debug("Encrypted partition %q unmapped", point)
		}
	}

	if len(fails) > 0 {
		mountError = errors.Errorf("Failed to unmount: %v", fails)
	}

	return mountError
}

// HumanReadableSizeWithUnitAndPrecision converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced unit and precision
func HumanReadableSizeWithUnitAndPrecision(size uint64, unit string, precision int) (string, error) {
	unit = strings.ToUpper(unit)

	if size == 0 {
		return fmt.Sprintf("0"), nil
	}

	sizes := []struct {
		unit      string
		mask      float64
		precision int
	}{
		{"P", 1.0 * 1000.0 * 1000.0 * 1000.0 * 1000.0 * 1000.0, 5},
		{"T", 1.0 * 1000.0 * 1000.0 * 1000.0 * 1000.0, 4},
		{"G", 1.0 * 1000.0 * 1000.0 * 1000.0, 3},
		{"M", 1.0 * 1000.0 * 1000.0, 2},
		{"K", 1.0 * 1000.0, 1},
		{"B", 1.0, 0},
	}

	value := float64(size)
	for _, curr := range sizes {
		csize := value / curr.mask

		// No unit request, use default based on size
		if unit == "" {
			if csize < 1.0 {
				continue
			}
		} else if unit != curr.unit {
			continue
		}

		unit = curr.unit

		// No precision request, use default based on size
		if precision < 0 {
			precision = curr.precision
		}

		formatted := strconv.FormatFloat(csize, 'f', precision, 64)
		// Remove trailing zeroes (and unused decimal)
		formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
		if unit != "" && unit != "B" {
			formatted += unit
		}

		return formatted, nil
	}

	return "", errors.ValidationErrorf("Could not format disk/partition size")
}

// HumanReadableSizeWithPrecision converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced precision
func HumanReadableSizeWithPrecision(size uint64, precision int) (string, error) {
	return HumanReadableSizeWithUnitAndPrecision(size, "", precision)
}

// HumanReadableSizeWithUnit converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced unit
func HumanReadableSizeWithUnit(size uint64, unit string) (string, error) {
	return HumanReadableSizeWithUnitAndPrecision(size, unit, -1)
}

// HumanReadableSize converts the size representation in bytes to the closest
// human readable format i.e 10M, 1G, 2T etc
func HumanReadableSize(size uint64) (string, error) {
	return HumanReadableSizeWithUnitAndPrecision(size, "", -1)
}

// HumanReadableSizeXiBWithUnitAndPrecision converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced unit and precision
func HumanReadableSizeXiBWithUnitAndPrecision(size uint64, unit string, precision int) (string, error) {
	unit = strings.ToUpper(unit)

	if size == 0 {
		return fmt.Sprintf("0"), nil
	}

	sizes := []struct {
		unit      string
		mask      float64
		precision int
	}{
		{"P", 1.0 * 1024.0 * 1024.0 * 1024.0 * 1024.0 * 1024.0, 5},
		{"T", 1.0 * 1024.0 * 1024.0 * 1024.0 * 1024.0, 4},
		{"G", 1.0 * 1024.0 * 1024.0 * 1024.0, 3},
		{"M", 1.0 * 1024.0 * 1024.0, 2},
		{"K", 1.0 * 1024.0, 1},
		{"B", 1.0, 0},
	}

	value := float64(size)
	for _, curr := range sizes {
		csize := value / curr.mask

		// No unit request, use default based on size
		if unit == "" {
			if csize < 1.0 {
				continue
			}
		} else if unit != curr.unit {
			continue
		}

		unit = curr.unit

		// No precision request, use default based on size
		if precision < 0 {
			precision = curr.precision
		}

		formatted := strconv.FormatFloat(csize, 'f', precision, 64)
		// Remove trailing zeroes (and unused decimal)
		formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
		if unit != "" && unit != "B" {
			formatted += unit
		}

		return formatted, nil
	}

	return "", errors.ValidationErrorf("Could not format disk/partition size")
}

// HumanReadableSizeXiBWithPrecision converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced precision
func HumanReadableSizeXiBWithPrecision(size uint64, precision int) (string, error) {
	return HumanReadableSizeXiBWithUnitAndPrecision(size, "", precision)
}

// HumanReadableSizeXiBWithUnit converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced unit
func HumanReadableSizeXiBWithUnit(size uint64, unit string) (string, error) {
	return HumanReadableSizeXiBWithUnitAndPrecision(size, unit, -1)
}

// HumanReadableSizeXiB converts the size representation in bytes to the closest
// human readable format i.e 10M, 1G, 2T etc
func HumanReadableSizeXiB(size uint64) (string, error) {
	return HumanReadableSizeXiBWithUnitAndPrecision(size, "", -1)
}

// MaxLabelLength returns the maximum length of a label for
// the given file system type
func MaxLabelLength(fstype string) int {
	var maxLen int

	switch fstype {
	case "ext2", "ext3", "ext4":
		maxLen = 16
	case "swap":
		maxLen = 15
	case "xfs":
		maxLen = 12
	case "f2fs":
		maxLen = 512
	case "btrfs":
		maxLen = 255
	case "vfat":
		maxLen = 11
	default:
		maxLen = 11
		log.Warning("Unknown file system type %s, defaulting to %d character label", fstype, maxLen)
	}

	return maxLen
}

// ParseVolumeSize will parse a string formatted (1M, 10G, 2T) size and return its representation
// in bytes
func ParseVolumeSize(str string) (uint64, error) {
	var size uint64

	str = strings.ToLower(str)

	if !storageExp.MatchString(str) {
		return strconv.ParseUint(str, 0, 64)
	}

	unit := storageExp.ReplaceAllString(str, `$3`)
	fsize, err := strconv.ParseFloat(storageExp.ReplaceAllString(str, `$1`), 64)
	if err != nil {
		return 0, errors.Wrap(err)
	}

	switch unit {
	case "b":
		fsize = fsize * (1 << 0)
	case "k":
		fsize = fsize * (1 << 10)
	case "m":
		fsize = fsize * (1 << 20)
	case "g":
		fsize = fsize * (1 << 30)
	case "t":
		fsize = fsize * (1 << 40)
	case "p":
		fsize = fsize * (1 << 50)
	}

	size = uint64(math.Round(fsize))

	return size, nil
}

// ParseVolumeSizeXiB will parse a string formatted (1M, 10G, 2T) size and
// return its representation in power of 2 bytes
// M = MiB, G = GiB, T = TiB, ..
func ParseVolumeSizeXiB(str string) (uint64, error) {
	var size uint64

	str = strings.ToLower(str)

	if !storageExp.MatchString(str) {
		return strconv.ParseUint(str, 0, 64)
	}

	unit := storageExp.ReplaceAllString(str, `$3`)
	fsize, err := strconv.ParseFloat(storageExp.ReplaceAllString(str, `$1`), 64)
	if err != nil {
		return 0, errors.Wrap(err)
	}

	switch unit {
	// case "b":
	//	fsize = fsize * math.Exp2(0)
	case "k":
		fsize = fsize * math.Exp2(10)
	case "m":
		fsize = fsize * math.Exp2(20)
	case "g":
		fsize = fsize * math.Exp2(30)
	case "t":
		fsize = fsize * math.Exp2(40)
	case "p":
		fsize = fsize * math.Exp2(50)
	}

	size = uint64(math.Round(fsize))

	return size, nil
}
