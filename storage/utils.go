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

var storageExp = regexp.MustCompile(`^([0-9]*(\.)?[0-9]*)([bkmgtp]{1}(b|ib){0,1}){0,1}$`)

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

type convertLookup struct {
	unit      string
	mask      float64
	precision int
}

var (
	convertLookUpXB = []convertLookup{
		{"PB", 1.0 * 1000.0 * 1000.0 * 1000.0 * 1000.0 * 1000.0, 5},
		{"TB", 1.0 * 1000.0 * 1000.0 * 1000.0 * 1000.0, 4},
		{"GB", 1.0 * 1000.0 * 1000.0 * 1000.0, 3},
		{"MB", 1.0 * 1000.0 * 1000.0, 2},
		{"KB", 1.0 * 1000.0, 1},
		{"B", 1.0, 0},
	}
	convertLookUpXiB = []convertLookup{
		{"PiB", 1.0 * 1024.0 * 1024.0 * 1024.0 * 1024.0 * 1024.0, 5},
		{"TiB", 1.0 * 1024.0 * 1024.0 * 1024.0 * 1024.0, 4},
		{"GiB", 1.0 * 1024.0 * 1024.0 * 1024.0, 3},
		{"MiB", 1.0 * 1024.0 * 1024.0, 2},
		{"KiB", 1.0 * 1024.0, 1},
		{"B", 1.0, 0},
	}
)

func humanReadableSizeWithUnitAndPrecision(sizes []convertLookup,
	size uint64, unit string, precision int) (string, error) {
	unit = strings.ToUpper(unit)
	unit = strings.ReplaceAll(unit, "I", "i")

	if size == 0 {
		return fmt.Sprintf("0"), nil
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

// HumanReadableSizeXBWithUnitAndPrecision converts the size representation in bytes to the
// closest human readable format i.e 10MB, 1GB, 2TB etc with a forced unit and precision
func HumanReadableSizeXBWithUnitAndPrecision(size uint64, unit string, precision int) (string, error) {
	return humanReadableSizeWithUnitAndPrecision(convertLookUpXB, size, unit, precision)
}

// HumanReadableSizeXBWithPrecision converts the size representation in bytes to the
// closest human readable format i.e 10MB, 1GB, 2TB etc with a forced precision
func HumanReadableSizeXBWithPrecision(size uint64, precision int) (string, error) {
	return HumanReadableSizeXBWithUnitAndPrecision(size, "", precision)
}

// HumanReadableSizeXBWithUnit converts the size representation in bytes to the
// closest human readable format i.e 10MB, 1GB, 2TB etc with a forced unit
func HumanReadableSizeXBWithUnit(size uint64, unit string) (string, error) {
	return HumanReadableSizeXBWithUnitAndPrecision(size, unit, -1)
}

// HumanReadableSizeXB converts the size representation in bytes to the closest
// human readable format i.e 10M, 1G, 2T etc
func HumanReadableSizeXB(size uint64) (string, error) {
	return HumanReadableSizeXBWithUnitAndPrecision(size, "", -1)
}

// HumanReadableSizeXiBWithUnitAndPrecision converts the size representation in bytes to the
// closest human readable format i.e 10MiB, 1GiB, 2TiB etc with a forced unit and precision
func HumanReadableSizeXiBWithUnitAndPrecision(size uint64, unit string, precision int) (string, error) {
	return humanReadableSizeWithUnitAndPrecision(convertLookUpXiB, size, unit, precision)
}

// HumanReadableSizeXiBWithPrecision converts the size representation in bytes to the
// closest human readable format i.e 10MiB, 1GiB, 2TiB etc with a forced precision
func HumanReadableSizeXiBWithPrecision(size uint64, precision int) (string, error) {
	return HumanReadableSizeXiBWithUnitAndPrecision(size, "", precision)
}

// HumanReadableSizeXiBWithUnit converts the size representation in bytes to the
// closest human readable format i.e 10MiB, 1GiB, 2TiB etc with a forced unit
func HumanReadableSizeXiBWithUnit(size uint64, unit string) (string, error) {
	return HumanReadableSizeXiBWithUnitAndPrecision(size, unit, -1)
}

// HumanReadableSizeXiB converts the size representation in bytes to the closest
// human readable format i.e 10MiB, 1GiB, 2TiB etc
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

// ParseVolumeSize will parse a string formatted (1M, 10GiB, 2TB) size
// and return its representation in bytes
// Units without suffix 'B' or 'iB' are assumed to be powers of 10
// to ensure consistency with existing YAML files.
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
	case "k", "kb":
		fsize = fsize * (1 << 10)
	case "m", "mb":
		fsize = fsize * (1 << 20)
	case "g", "gb":
		fsize = fsize * (1 << 30)
	case "t", "tb":
		fsize = fsize * (1 << 40)
	case "p", "pb":
		fsize = fsize * (1 << 50)
	case "kib":
		fsize = fsize * math.Exp2(10)
	case "mib":
		fsize = fsize * math.Exp2(20)
	case "gib":
		fsize = fsize * math.Exp2(30)
	case "tib":
		fsize = fsize * math.Exp2(40)
	case "pib":
		fsize = fsize * math.Exp2(50)
	}

	size = uint64(math.Round(fsize))

	return size, nil
}
