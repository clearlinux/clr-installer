// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

// MediaOpts group the set of media related options
type MediaOpts struct {
	LegacyBios         bool   `yaml:"legacyBios,omitempty,flow"`
	SkipValidationSize bool   `yaml:"skipValidationSize,omitempty,flow"`
	SkipValidationAll  bool   `yaml:"skipValidationAll,omitempty,flow"`
	SwapFileSize       string `yaml:"swapFileSize,omitempty,flow"`
	SwapFileSet        bool   `yaml:"-"`
}

type blockDeviceOps struct {
	makeFsCommand   func(bd *BlockDevice, args []string) ([]string, error)
	makeFsArgs      []string
	makePartCommand func(bd *BlockDevice) (string, error)
}

// ByBDName implements sort.Interface for []*BlockDevice based on the Name field.
type ByBDName []*BlockDevice

func (a ByBDName) Len() int      { return len(a) }
func (a ByBDName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

var (
	bdOps = map[string]*blockDeviceOps{
		"ext2":  {commonMakeFsCommand, []string{"-v", "-F"}, commonMakePartCommand},
		"ext3":  {commonMakeFsCommand, []string{"-v", "-F"}, commonMakePartCommand},
		"ext4":  {commonMakeFsCommand, []string{"-v", "-F", "-b", "4096"}, commonMakePartCommand},
		"btrfs": {commonMakeFsCommand, []string{"-f"}, commonMakePartCommand},
		"xfs":   {commonMakeFsCommand, []string{"-f"}, commonMakePartCommand},
		"f2fs":  {commonMakeFsCommand, []string{"-f"}, commonMakePartCommand},
		"swap":  {swapMakeFsCommand, []string{}, swapMakePartCommand},
		"vfat":  {commonMakeFsCommand, []string{"-F32"}, vfatMakePartCommand},
	}

	guidMap = map[string]string{
		"/":     "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709",
		"/home": "933AC7E1-2EB4-4F13-B844-0E14E2AEF915",
		"/srv":  "3B8F8425-20E0-4F3B-907F-1A25A76F98E8",
		"swap":  "0657FD6D-A4AB-43C4-84E5-0933C84B4F4F",
		"efi":   "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
	}

	mountedPoints   []string
	mountedEncrypts []string

	minBootSize = uint64(100) * (1000 * 1000) // 100MB recommend for 4-5 kernels

	minSwapSize = uint64(32) * (1024 * 1024)       // 32MiB recommend smallest for memory crunch times
	maxSwapSize = uint64(8) * (1024 * 1024 * 1024) // 8GiB recommend maximum for memory crunch times
)

type blockDeviceDestroyOps struct {
	RundestroyCommand func(bd *BlockDevice, disk string, dryRun *[]string) error
}

var bdDestroyOps = map[BlockDeviceType]*blockDeviceDestroyOps{
	BlockDeviceTypePart:       {removePart},
	BlockDeviceTypeLVM2Volume: {removeLogicalVolumeNoop},
	BlockDeviceTypeRAID0:      {removeRaidType},
	BlockDeviceTypeRAID1:      {removeRaidType},
	BlockDeviceTypeRAID4:      {removeRaidType},
	BlockDeviceTypeRAID5:      {removeRaidType},
	BlockDeviceTypeRAID6:      {removeRaidType},
	BlockDeviceTypeRAID10:     {removeRaidType},
}

func getBlockDevicesLsblkJSON(opts ...string) ([]*BlockDevice, error) {
	w := bytes.NewBuffer(nil)
	args := []string{lsblkBinary, "--exclude", "1,2,11", "-J", "-b", "-O"}

	args = append(args, opts...)

	err := cmd.Run(w, args...)
	if err != nil {
		return nil, err
	}

	bds, err := parseBlockDevicesDescriptor(w.Bytes())
	if err == nil {
		return bds, nil
	}

	return []*BlockDevice{}, nil
}

// MakeFs runs mkfs.* commands for a BlockDevice definition
func (bd *BlockDevice) MakeFs() error {
	if bd.Type == BlockDeviceTypeDisk {
		return errors.Errorf("Trying to run MakeFs() against a disk, partition required")
	}

	if op, ok := bdOps[bd.FsType]; ok {
		if cmd, err := op.makeFsCommand(bd, op.makeFsArgs); err == nil {
			return makeFs(bd, cmd)
		}
	}

	return errors.Errorf("MakeFs() not implemented for filesystem: %s", bd.FsType)
}

func makeFs(bd *BlockDevice, args []string) error {
	if bd.Options != "" {
		args = append(args, strings.Split(bd.Options, " ")...)
	}

	args = append(args, bd.GetMappedDeviceFile())

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	// Updated the UUID and LABEL now that we made the fs
	err = bd.updatePartitionInfo()
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func (bd *BlockDevice) updatePartitionInfo() error {
	if bd.Type == BlockDeviceTypeDisk {
		return errors.Errorf("Trying to run updatePartitionInfo() against a disk, partition required")
	}

	var err error

	blkid := bytes.NewBuffer(nil)
	devFile := bd.GetDeviceFile()

	// Read the partition blkid info
	err = cmd.Run(blkid,
		"blkid",
		"--probe",
		devFile,
		"--output",
		"export",
	)
	if err != nil {
		log.Warning("updatePartitionInfo() had an error reading blkid %q",
			fmt.Sprintf("%s", blkid.String()))
		return err
	}

	for _, line := range strings.Split(blkid.String(), "\n") {
		fields := strings.Split(line, "=")
		if len(fields) == 2 {
			if fields[0] == "LABEL" {
				bd.Label = fields[1]
				log.Debug("updatePartitionInfo: Updated %s LABEL: %s", devFile, bd.Label)
			} else if fields[0] == "UUID" {
				bd.UUID = fields[1]
				log.Debug("updatePartitionInfo: Updated %s UUID: %s", devFile, bd.UUID)
			}
		} else {
			log.Debug("updatePartitionInfo: Ignoring unknown line: %s", line)
		}
	}

	return err
}

// getGUID determines the partition type guid either based on:
//   + mount point
//   + file system type (i.e swap)
//   + or if it's the "special" efi case
func (bd *BlockDevice) getGUID() (string, error) {
	if guid, ok := guidMap[bd.MountPoint]; ok {
		return guid, nil
	}

	if guid, ok := guidMap[bd.FsType]; ok {
		return guid, nil
	}

	if bd.FsType == "vfat" && bd.MountPoint == "/boot" {
		return guidMap["efi"], nil
	}

	return "none", errors.Errorf("Could not determine the guid for: %s", bd.Name)
}

func (bd *BlockDevice) isStandardMount() bool {
	standard := false

	for mnt := range guidMap {
		if bd.MountPoint == mnt {
			standard = true
		}
	}

	if bd.MountPoint == "/boot" {
		standard = true
	}

	return standard
}

// Mount will mount a block devices bd considering its mount point and the
// root directory
func (bd *BlockDevice) Mount(root string) error {
	if bd.Type == BlockDeviceTypeDisk {
		return errors.Errorf("Trying to run mountFs() against a disk, partition required")
	}

	targetPath := filepath.Join(root, bd.MountPoint)

	return mountFs(bd.GetMappedDeviceFile(), targetPath, bd.FsType, syscall.MS_RELATIME)
}

// When you specify a start (or end) position to the parted mkpart command,
// it internally generates a range of acceptable values centered on the value
// you specify, and extends equally on both sides by half the unit size you
// used but ONLY when you use K or M (or G); using B or any of the XiB will
// not auto align.
// We choose M to provide a 1M wide window for a possible optimal value.
func getStartEndMB(start uint64, end uint64) string {
	startMB := (start / (1000 * 1000))
	endMB := (end / (1000 * 1000))

	strStart := fmt.Sprintf("%dM", startMB)
	if start < 1 {
		strStart = "0%"
	}

	strEnd := fmt.Sprintf("%dM", endMB)
	if end < 1 {
		strEnd = "-1"
	}

	return strStart + " " + strEnd
}

// WritePartitionLabel make a device a 'gpt' partition type
// Only call when we are wiping and reusing the entire disk
func (bd *BlockDevice) writePartitionLabel(wholeDisk bool) error {
	if !wholeDisk {
		log.Debug("WritePartitionTable: partial disk, skipping mklabel for %s", bd.Name)
		return nil
	}

	if bd.Type != BlockDeviceTypeDisk && bd.Type != BlockDeviceTypeLoop {
		return errors.Errorf("Type is partition, disk required")
	}

	mesg := utils.Locale.Get("Writing partition table to: %s", bd.Name)
	prg := progress.NewLoop(mesg)
	log.Info(mesg)
	args := []string{
		"parted",
		"-s",
		bd.GetDeviceFile(),
		"mklabel",
		"gpt",
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		prg.Failure()
		return errors.Wrap(err)
	}

	prg.Success()

	return nil
}

// setPartitionGUIDs is a helper function to WritePartitionTable takes a prepared
// guid map of GUIDS->device names and uses sgdisk to update the
// guid partition table for the disk
func (bd *BlockDevice) setPartitionGUIDs(guids map[int]string) error {
	var err error

	if len(guids) < 1 {
		log.Debug("No GUIDs to set for device: %s", bd.GetDeviceFile())
		return nil
	}

	log.Info("Setting GUIDs for device: %s", bd.GetDeviceFile())

	for idx, guid := range guids {
		if guid == "none" {
			continue
		}

		args := []string{
			"sgdisk",
			bd.GetDeviceFile(),
			fmt.Sprintf("--typecode=%d:%s", idx, guid),
		}

		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	return nil
}

func partitionUsingParted(bd *BlockDevice, dryRun *[]string, wholeDisk bool) error {
	var start uint64
	maxFound := false

	// Initialize the partition list before we add new ones
	currentPartitions := bd.getPartitionList()

	// Make the needed new partitions
	for _, curr := range bd.Children {
		if dryRun != nil {
			if curr.MakePartition {
				size, _ := HumanReadableSizeXiBWithPrecision(curr.Size, 1)
				*dryRun = append(*dryRun, fmt.Sprintf("%s: %s [%s]",
					bd.Name, utils.Locale.Get(AddPartitionInfo), size))
			}
			continue
		}

		log.Debug("WritePartitionTable: processing child: %v", curr)
		baseArgs := []string{
			"parted",
			"-a",
			"optimal",
			bd.GetDeviceFile(),
			"unit", "MB",
			"--script",
			"--",
		}

		if !curr.MakePartition {
			log.Debug("WritePartitionTable: skipping partition %s", curr.Name)
			continue
		}

		var mkPart string

		op, found := bdOps[curr.FsType]
		if !found {
			return errors.Errorf("No makePartCommand() implementation for: %s",
				curr.FsType)
		}

		mkPart, err := op.makePartCommand(curr)
		if err != nil {
			return err
		}

		size := uint64(curr.Size)
		end := start + size
		if !wholeDisk {
			start, end = bd.getPartitionStartEnd(curr.partition)
		} else {
			log.Debug("WritePartitionTable: WholeDisk mode")
		}
		log.Debug("WritePartitionTable: start: %d, end: %d", start, end)

		if size < 1 {
			if maxFound {
				return errors.Errorf("Found more than one partition with size 0 for %s!", bd.Name)
			}
			maxFound = true
			end = 0
		}

		retries := 3
		for {
			mkPartCmd := mkPart + " " + getStartEndMB(start, end)
			log.Debug("WritePartitionTable: mkPartCmd: " + mkPartCmd)

			args := append(baseArgs, mkPartCmd)

			err = cmd.RunAndLog(args...)

			if err == nil || retries == 0 {
				break
			}

			// Move the start position ahead one MB in an attempt
			// to find a working optimal partition entry
			start = start + (1000 * 1000)

			retries--
		}
		if err != nil {
			return errors.Wrap(err)
		}

		// Get the new list of partitions
		newPartitions := bd.getPartitionList()
		// The current partition is new one added
		curr.SetPartitionNumber(findNewPartition(currentPartitions, newPartitions).Number)

		start = end
		currentPartitions = newPartitions
	}

	return nil
}

// removePhysicalvolume actually performs the operation of removal of a physical volume
func removePhysicalvolume(bd *BlockDevice, dryRun *[]string) error {
	if bd.FsType != BlockDeviceTypeLVM2GroupString {
		return errors.Errorf("Block Type is not physical volume")
	}

	args := []string{
		"pvremove",
		bd.GetMappedDeviceFile(),
	}

	if dryRun != nil {
		*dryRun = append(*dryRun, utils.Locale.Get("Remove physical volume: %s", bd.Name))
	} else {
		log.Info("Proceeding to remove physical volume: %s", bd.GetMappedDeviceFile())
		err := cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	return nil
}

/*
processPhysicalVolume actually performs a series of checks in steps and call
removePhysicalvolume to actuallt remove the PV
Steps:
------
1) List all the physical volume in system with their Volume groups
Filter all VG where our PV is a part. If no VG found, remove our PV else move on
2) Find volumes associated in VG and remove them
Query how many PVs associated with VG, if there is only one we remove PV, VG
else if more than 1, we only reduce that PV from the VG
3) Remove only that PV finally which is what the function was meant to do
*/
func processPhysicalVolume(bd *BlockDevice, dryRun *[]string) error {
	if bd.FsType != BlockDeviceTypeLVM2GroupString {
		return errors.Errorf("Block Type is not physical volume")
	}

	// Step 1: Get the volume group associated with physical volume
	args := []string{
		"pvdisplay",
		"--colon",
	}
	pvDisplayOutput := bytes.NewBuffer(nil)

	err := cmd.Run(pvDisplayOutput, args...)

	if err != nil {
		return errors.Wrap(err)
	}

	volumeGroup := ""

	for _, line := range strings.Split(pvDisplayOutput.String(), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) == 12 {
			if strings.TrimSpace(fields[0]) == bd.GetMappedDeviceFile() {
				volumeGroup = fields[1]
			}
		}
	}

	// If the above query tells no volume group assocated with Physical volume
	// then we only delete the physical volume
	if volumeGroup == "" {
		log.Warning("Could not find volume group for the Physical Volume: %s", bd.GetMappedDeviceFile())
		return removePhysicalvolume(bd, dryRun)
	}

	// Step 2: Get all volumes associated with volume group
	volGrpMapperName := filepath.Join("/dev/mapper/", volumeGroup)
	args = []string{
		"lvdisplay",
		volGrpMapperName,
		"--colon",
	}
	lvDisplayOutput := bytes.NewBuffer(nil)

	err = cmd.Run(lvDisplayOutput, args...)
	if err != nil {
		return errors.Wrap(err)
	}

	// We assume initially there are no lvs associated with Volume groups
	lvs := []string{}

	for _, line := range strings.Split(lvDisplayOutput.String(), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) == 13 {
			lvs = append(lvs, strings.TrimSpace(fields[0]))
		}
	}

	if dryRun != nil {
		if len(lvs) > 0 {
			*dryRun = append(*dryRun, utils.Locale.Get("Remove volumes : [%s]", strings.Join(lvs, ",")))
		}
	} else {
		if err := removeLogicalVolume(lvs...); err != nil {
			return err
		}
	}

	// Step 3: Query how many Physical volumes are associated with volume_group.
	args = []string{
		"vgdisplay",
		"--colon",
	}
	vgDisplayOutput := bytes.NewBuffer(nil)

	err = cmd.Run(vgDisplayOutput, args...)
	if err != nil {
		return errors.Wrap(err)
	}

	// It atleast needs to have the one..the physical volume we just queried
	pvCount := uint64(1)

	for _, line := range strings.Split(vgDisplayOutput.String(), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) == 17 {
			if strings.TrimSpace(fields[0]) == volumeGroup {
				if pvCount, err = strconv.ParseUint(fields[9], 10, 64); err != nil {
					return err
				}
			}
		}
	}

	// This means we can delete the volume group as thats the only PV associated with it
	if pvCount == 1 {
		args = []string{
			"vgremove",
			volumeGroup,
		}

		if dryRun != nil {
			*dryRun = append(*dryRun, utils.Locale.Get("Remove volume group: %s", volumeGroup))
		} else {
			log.Info("Volume Group: %s has only one Physical volume: %s associated with it",
				volumeGroup, bd.GetMappedDeviceFile())
			log.Info("Proceeding to deleting volume group: %s", volumeGroup)
			err := cmd.RunAndLog(args...)
			if err != nil {
				return errors.Wrap(err)
			}
		}
	} else {
		args = []string{
			"vgreduce",
			volumeGroup,
			bd.GetMappedDeviceFile(),
		}

		if dryRun != nil {
			*dryRun = append(*dryRun, utils.Locale.Get("Remove physical volume:%s from volume group: %s", volumeGroup))
		} else {
			log.Info("Volume Group: %s has other Physical volumes associated with it", volumeGroup)
			log.Info("Proceeding to instead reducing volume group: %s by physical volume: %s",
				volumeGroup, bd.GetMappedDeviceFile())
			err := cmd.RunAndLog(args...)
			if err != nil {
				return errors.Wrap(err)
			}
		}
	}

	// Step 3: once volume group has been take care of, we delete physical volume
	if err = removePhysicalvolume(bd, dryRun); err != nil {
		return err
	}

	return nil
}

// We delete LVs while iterating their physical volumes
// removeLogicalVolumeNoop needs to be a No-op
func removeLogicalVolumeNoop(bd *BlockDevice, disk string, dryRun *[]string) error {
	return nil
}

// removeLogicalVolume actually runs commmands to remove list of
// volumes passed to it. We usually pass all volumes from a VG
func removeLogicalVolume(mappedDeviceName ...string) error {
	for num, lvmappedname := range mappedDeviceName {
		log.Debug("Removing logical volume %d", num+1)

		args := []string{
			"lvremove",
			lvmappedname,
			"-y",
		}

		err := cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	return nil
}

func reverseLookParent(raidChild *BlockDevice, disk string, filterFunc BlockDevFilterFunc) (*BlockDevice, error) {
	bds, err := getBlockDevicesLsblkJSON(disk)

	if err != nil {
		return nil, err
	}

	for _, bd := range bds {
		found, parent := FindBlockDeviceDepthFirst(bd, filterFunc)

		if found && parent != nil {
			return parent, nil
		}
	}

	return nil, errors.Errorf("Could not find parent")
}

func removeRaidType(bd *BlockDevice, disk string, dryRun *[]string) error {
	raidParentFinder := func(b *BlockDevice) bool {
		if b.FsType == "linux_raid_member" {
			for _, ch := range b.Children {
				if ch.Name == bd.Name {
					return true
				}
			}
		}
		return false
	}

	parent, err := reverseLookParent(bd, disk, raidParentFinder)
	if dryRun == nil {
		log.Debug("removeRaidType: Found parent: %s for child: %s", parent.GetDeviceFile(), bd.GetDeviceFile())
	}

	if parent == nil {
		return err
	}

	if dryRun == nil {
		log.Warning("Strategy 1: Failing the RAID part: %s failed for RAID: %s", parent.GetDeviceFile(), bd.Name)
		args := []string{"mdadm", "--fail", bd.GetDeviceFile(), parent.GetDeviceFile()}

		err = cmd.RunAndLog(args...)
		if err != nil {
			goto strategy2
		}

		log.Warning("Removing the RAID part: %s from RAID: %s", bd.GetDeviceFile(), bd.Name)
		args = []string{"mdadm", "--remove", bd.GetDeviceFile(), parent.GetDeviceFile()}

		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}

		log.Warning("Zeroing RAID part super-block: %s", parent.GetDeviceFile())
		args = []string{"mdadm", "--zero-superblock", parent.GetDeviceFile()}

		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}

		return nil

	strategy2:
		log.Warning("Strategy 2: Stopping RAID: %s", bd.GetDeviceFile())
		args = []string{"mdadm", "--stop", bd.GetDeviceFile()}

		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}

		log.Warning("Zeroing RAID part super-block: %s", parent.GetDeviceFile())
		args = []string{"mdadm", "--zero-superblock", parent.GetDeviceFile()}

		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}
	} else {
		*dryRun = append(*dryRun, utils.Locale.Get("This will degrade RAID: %s", bd.Name))
		*dryRun = append(*dryRun, parent.Name+": "+utils.Locale.Get("Remove partition from RAID %s", bd.Name))
	}

	return nil
}

func removePart(bd *BlockDevice, disk string, dryRun *[]string) error {
	if bd.Type != BlockDeviceTypePart {
		return errors.Errorf("Type is not a partition")
	}

	partNum := bd.GetPartitionNumber()

	if partNum == 0 {
		return errors.Errorf("Could not find partition number")
	}

	partParentFinder := func(b *BlockDevice) bool {
		if b.Type == BlockDeviceTypeDisk || b.Type == BlockDeviceTypeLoop ||
			b.isRaidType() || b.Type == BlockDeviceTypeLVM2Volume {
			for _, ch := range b.Children {
				if ch.Name == bd.Name {
					return true
				}
			}
		}
		return false
	}

	parent, err := reverseLookParent(bd, disk, partParentFinder)
	if dryRun == nil {
		log.Debug("removePart: Found parent: %s for child: %s", parent.GetDeviceFile(), bd.GetDeviceFile())
	}

	if parent == nil {
		return err
	}

	if bd.FsType == BlockDeviceTypeLVM2GroupString {
		return processPhysicalVolume(bd, dryRun)
	}

	args := []string{"parted", parent.GetDeviceFile(), "--script", "--", "rm", strconv.FormatUint(partNum, 10)}

	if dryRun == nil {
		log.Warning("Deleting part: %s from disk: %s", bd.Name, parent.Name)
		err = cmd.RunAndLog(args...)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	return nil
}

func (bd *BlockDevice) cleanUpDisk(disk string, dryRun *[]string) error {
	var err error = nil

	for _, ch := range bd.Children {
		if err = ch.cleanUpDisk(disk, dryRun); err != nil {
			return err
		}
		if destroyOp, okay := bdDestroyOps[ch.Type]; okay {
			err = destroyOp.RundestroyCommand(ch, disk, dryRun)
		}
	}

	return err
}

// WritePartitionTable writes the defined partitions to the actual block device
func (bd *BlockDevice) WritePartitionTable(wholeDisk bool, dryRun *[]string) error {
	if bd.Type != BlockDeviceTypeDisk && bd.Type != BlockDeviceTypeLoop &&
		bd.Type != BlockDeviceTypeRAID0 && bd.Type != BlockDeviceTypeRAID1 && bd.Type != BlockDeviceTypeRAID4 &&
		bd.Type != BlockDeviceTypeRAID5 && bd.Type != BlockDeviceTypeRAID6 && bd.Type != BlockDeviceTypeRAID10 {
		return errors.Errorf("Type is partition, disk required")
	}

	if wholeDisk {
		if dryRun == nil {
			log.Info("Cleaning disk %s", bd.GetDeviceFile())
		}

		disk := bd.GetDeviceFile()
		bds, err := getBlockDevicesLsblkJSON(disk)

		if err != nil {
			return err
		}

		if len(bds) != 1 {
			return errors.Errorf("Multiple entries found for same device: %s", disk)
		}

		if err = bds[0].cleanUpDisk(disk, dryRun); err != nil {
			return err
		}
	}

	var prg progress.Progress
	var err error

	if dryRun != nil {
		if wholeDisk {
			*dryRun = append(*dryRun, bd.Name+": "+utils.Locale.Get(PartitioningWarning))
		}
	} else {
		//write the partition label
		if err := bd.writePartitionLabel(wholeDisk); err != nil {
			return err
		}

		mesg := utils.Locale.Get("Updating partition table for: %s", bd.Name)
		prg = progress.NewLoop(mesg)
		log.Info(mesg)
	}

	// Sort the partitions by name before writing the partition table
	log.Debug("Partitions before sorting:")
	for _, part := range bd.Children {
		part.logDetails()
	}

	sort.Sort(ByBDName(bd.Children))

	log.Debug("Partitions after sorting:")
	for _, part := range bd.Children {
		part.logDetails()
		// Make sure each partition has a number set
		part.SetPartitionNumber(part.GetPartitionNumber())
	}

	// Make the needed new partitions
	if err := partitionUsingParted(bd, dryRun, wholeDisk); err != nil {
		return err
	}

	if dryRun == nil {
		guids := map[int]string{}

		// Now that all new partitions are created,
		// and we know their assigned numbers ...
		for _, curr := range bd.Children {
			var guid string
			guid, err = curr.getGUID()
			if err != nil {
				log.Warning("%s", err)
			}

			if curr.FsType != "swap" || curr.Type != BlockDeviceTypeCrypt {
				guids[int(curr.partition)] = guid
			}
		}

		// Remaining steps are performed inside setPartitionGUIDs
		if err = bd.setPartitionGUIDs(guids); err != nil {
			return err
		}

		prg.Success()
	} else {
		if partChanges := getPlannedPartitionChanges(bd); len(partChanges) > 0 {
			*dryRun = append(*dryRun, partChanges...)
		}
	}

	return nil
}

// PrepareInstallationMedia updates all of the installation medias to ensure
// installation can proceed. Media is only updated if dryRun is passed 'nil,
// otherwise a high level description, in the locale, is set in the passed
// slice of string
func PrepareInstallationMedia(targets map[string]InstallTarget,
	medias []*BlockDevice, mediaOpts MediaOpts, dryRun *[]string) error {
	for _, target := range targets {
		if dryRun != nil {
			if target.EraseDisk {
				*dryRun = append(*dryRun, target.Name+": "+utils.Locale.Get(DestructiveWarning))
			} else if target.DataLoss {
				*dryRun = append(*dryRun, target.Name+": "+utils.Locale.Get(DataLossWarning))
			} else if target.WholeDisk {
				*dryRun = append(*dryRun, target.Name+": "+utils.Locale.Get(SafeWholeWarning))
			} else {
				*dryRun = append(*dryRun, target.Name+": "+utils.Locale.Get(MediaToBeUsed))
			}
		}

		for _, curr := range medias {
			if target.Name == curr.Name {
				if err := curr.WritePartitionTable(target.WholeDisk, dryRun); err != nil {
					if dryRun != nil {
						*dryRun = append(*dryRun, FailedPartitionWarning)
					} else {
						return err
					}
				}
			}
		}
	}

	if err := setBootPartition(medias, mediaOpts, dryRun); err != nil {
		log.Warning("Could set boot information!")
		if dryRun != nil {
			*dryRun = append(*dryRun, FailedPartitionWarning)
		} else {
			return err
		}
	}

	// Ensure media changes become visible to kernel/OS
	if dryRun == nil {
		var prg progress.Progress
		mesg := utils.Locale.Get("Rescanning media")
		sleepTime := 4
		step := 0
		total := len(medias) + sleepTime
		prg = progress.MultiStep(total, mesg)

		for _, bd := range medias {
			if err := bd.PartProbe(); err != nil {
				return errors.Wrap(err)
			}
			step++
			prg.Partial(step)
		}

		for i := 0; i < sleepTime; i++ {
			time.Sleep(time.Duration(1) * time.Second)
			step++
			prg.Partial(step)
		}

		prg.Success()
	}

	return nil
}

func (bd *BlockDevice) getPartitionList() []*PartedPartition {
	var partitionList []*PartedPartition
	var err error

	partTable := bytes.NewBuffer(nil)
	devFile := bd.GetDeviceFile()

	if !utils.IntSliceContains([]int{BlockDeviceTypeDisk, BlockDeviceTypeLoop}, int(bd.Type)) {
		log.Warning("getPartitionList() called on non-disk %q", devFile)
		return partitionList
	}

	// Read the partition table for the device
	err = cmd.Run(partTable,
		"parted",
		"--machine",
		"--script",
		"--",
		devFile,
		"unit",
		"B",
		"print",
	)
	if err != nil {
		log.Warning("getPartitionList() had an error reading partition table %q",
			fmt.Sprintf("%s", partTable.String()))
		return partitionList
	}

	for _, line := range strings.Split(partTable.String(), ";\n") {
		partition := &PartedPartition{}

		fields := strings.Split(line, ":")
		if len(fields) == 7 {
			partition.Number, err = strconv.ParseUint(fields[0], 10, 64)
			if err != nil {
				log.Warning("getPartitionList: Failed to parse partition number from: %s", line)
			}
			partition.Start, err = strconv.ParseUint(strings.TrimRight(fields[1], "B"), 10, 64)
			if err != nil {
				log.Warning("getPartitionList: Failed to parse start position from: %s", line)
			}
			partition.End, err = strconv.ParseUint(strings.TrimRight(fields[2], "B"), 10, 64)
			if err != nil {
				log.Warning("getPartitionList: Failed to parse end position from: %s", line)
			}
			partition.Size, err = strconv.ParseUint(strings.TrimRight(fields[3], "B"), 10, 64)
			if err != nil {
				log.Warning("getPartitionList: Failed to parse partition size from: %s", line)
			}
			partition.FileSystem = fields[4]
			partition.Name = fields[5]
			partition.Flags = fields[6]

			partitionList = append(partitionList, partition)
		}
	}

	return partitionList
}

func findNewPartition(currentPartitions, newPartitions []*PartedPartition) *PartedPartition {
	newPartition := &PartedPartition{}
	if len(newPartitions) <= len(currentPartitions) {
		log.Warning("findNewPartition: number of new partitions is not greater than the current")
		return newPartition
	}
	if len(newPartitions)-len(currentPartitions) != 1 {
		log.Warning("findNewPartition: number of new partition is more than 1")
		return newPartition
	}

	for _, newPart := range newPartitions {
		found := true
		for _, curPart := range currentPartitions {
			if curPart.Number == newPart.Number {
				found = false
				continue
			}
		}

		if found {
			newPartition = newPart
			continue
		}
	}

	return newPartition
}

func (bd *BlockDevice) getPartitionTable() *bytes.Buffer {
	partTable := bytes.NewBuffer(nil)
	devFile := bd.GetDeviceFile()

	if !utils.IntSliceContains([]int{BlockDeviceTypeDisk, BlockDeviceTypeLoop}, int(bd.Type)) {
		log.Warning("getPartitionTable() called on non-disk %q", devFile)
		return partTable
	}

	// Read the partition table for the device
	err := cmd.Run(partTable,
		"parted",
		"--machine",
		"--script",
		"--",
		devFile,
		"unit",
		"B",
		"print",
		"free",
	)
	if err != nil {
		log.Warning("getPartitionTable() had an error reading partition table %q",
			fmt.Sprintf("%s", partTable.String()))
		empty := bytes.NewBuffer(nil)
		return empty
	}

	return partTable
}

func (bd *BlockDevice) getPartitionStartEnd(partNumber uint64) (uint64, uint64) {
	var start, end uint64
	devFile := bd.GetDeviceFile()

	if !utils.IntSliceContains([]int{BlockDeviceTypeDisk, BlockDeviceTypeLoop}, int(bd.Type)) {
		log.Warning("getPartitionStartEnd() called on non-disk %q", devFile)
		return start, end
	}

	for _, part := range bd.PartTable {
		if part.Number == partNumber {
			return part.Start, part.End
		}
	}

	log.Warning("getPartitionStartEnd() did not find partition %s for disk %q", partNumber, devFile)
	return start, end
}

// LargestContiguousFreeSpace returns the largest, contiguous block of free
// space in the partition table for the block device.
// If none found, returns {0, 0}
func (bd *BlockDevice) LargestContiguousFreeSpace(minSize uint64) (uint64, uint64) {
	var start, end, size uint64
	devFile := bd.GetDeviceFile()

	if !utils.IntSliceContains([]int{BlockDeviceTypeDisk, BlockDeviceTypeLoop}, int(bd.Type)) {
		log.Warning("LargestContiguousFreeSpace() called on non-disk %q", devFile)
		return start, end
	}

	size = minSize - 1

	for _, part := range bd.PartTable {
		if part.Number == 0 && part.FileSystem == "free" {
			if part.Size > size {
				start = part.Start
				end = part.End
			}
		}
	}

	return start, end
}

// AddFromFreePartition reduces the free partition by the size given
// User when adding a new partition to a disk from free space
func (bd *BlockDevice) AddFromFreePartition(parted *PartedPartition, child *BlockDevice) {
	var next uint64
	var partitionList []*PartedPartition
	devFile := bd.GetDeviceFile()

	if !utils.IntSliceContains([]int{BlockDeviceTypeDisk, BlockDeviceTypeLoop}, int(bd.Type)) {
		log.Warning("AddFromFreePartition() called on non-disk %q", devFile)
		return
	}

	const (
		maxPartitions = 127
	)

	found := false
	next = 1

	for !found && next < maxPartitions {
		present := false
		for _, partition := range bd.PartTable {
			if partition.Number == next {
				present = true
				break
			}
		}
		if present {
			next = next + 1
		} else {
			found = true
		}
	}

	if next >= maxPartitions {
		log.Warning("AddFromFreePartition() could not add new partition: %v", child)
		return
	}

	for _, partition := range bd.PartTable {
		// Find the partition to update/remove
		if partition.Number == parted.Number &&
			partition.Start == parted.Start {
			log.Debug("Found the free partition to update: %v", partition)

			addPart := partition.Clone()
			addPart.Number = next
			addPart.End = addPart.Start + (child.Size - 1)
			addPart.Size = child.Size
			addPart.FileSystem = ""
			log.Debug("Adding the new partition: %v", addPart)
			partitionList = append(partitionList, addPart)

			child.SetPartitionNumber(addPart.Number)
			bd.AddChild(child)
			log.Debug("Added new child partition: %v", child)

			newSize := partition.Size - addPart.Size
			newStart := addPart.End + 1

			log.Debug("Free partition newStart: %d, newSize: %d", newStart, newSize)
			if (int(partition.End) - int(newStart)) <= 0 {
				log.Debug("No Free space left: %v", partition)
				continue
			}

			if newSize > (10 * 1024 * 1024) {
				newPart := partition.Clone()
				newPart.Start = newStart
				newPart.Size = newSize
				log.Debug("Found enough free to add back: %v", newPart)
				partitionList = append(partitionList, newPart)
			}
			continue
		}

		log.Debug("Not the right partition, adding back: %v", partition)
		partitionList = append(partitionList, partition)
	}

	bd.PartTable = partitionList

	// Consolidate neighboring free partitions
	bd.consolidateFree()
}

func (bd *BlockDevice) consolidateFree() {
	last := &PartedPartition{}
	var newPartTable []*PartedPartition

	for _, part := range bd.PartTable {
		// Found a free partition
		if part.Number == 0 && part.FileSystem == "free" {
			// And the last partition was also free, then consolidate
			if last.Number == 0 && last.FileSystem == "free" {
				last.End = part.End
				last.Size = last.Size + part.Size
				continue
			}
		}

		newPart := part.Clone()
		newPartTable = append(newPartTable, newPart)
		last = newPart
	}

	bd.PartTable = newPartTable
}

// Populate the current partition table for a disk device
func (bd *BlockDevice) setPartitionTable(partTable *bytes.Buffer) {
	var partitionList []*PartedPartition
	devFile := bd.GetDeviceFile()

	if !utils.IntSliceContains([]int{BlockDeviceTypeDisk, BlockDeviceTypeLoop}, int(bd.Type)) {
		log.Warning("setPartitionTable() called on non-disk %q", devFile)
		return
	}

	var err error

	for _, line := range strings.Split(partTable.String(), ";\n") {
		partition := &PartedPartition{}

		log.Debug("setPartitionTable() line is %q", line)

		fields := strings.Split(line, ":")
		if len(fields) == 7 {
			partition.Number, err = strconv.ParseUint(fields[0], 10, 64)
			if err != nil {
				log.Warning("setPartitionTable: Failed to parse partition number from: %s", line)
			}
			partition.Start, err = strconv.ParseUint(strings.TrimRight(fields[1], "B"), 10, 64)
			if err != nil {
				log.Warning("setPartitionTable: Failed to parse start position from: %s", line)
			}
			partition.End, err = strconv.ParseUint(strings.TrimRight(fields[2], "B"), 10, 64)
			if err != nil {
				log.Warning("setPartitionTable: Failed to parse end position from: %s", line)
			}
			partition.Size, err = strconv.ParseUint(strings.TrimRight(fields[3], "B"), 10, 64)
			if err != nil {
				log.Warning("setPartitionTable: Failed to parse partition size from: %s", line)
			}
			partition.FileSystem = fields[4]
			partition.Name = fields[5]
			partition.Flags = fields[6]

			partitionList = append(partitionList, partition)
			continue
		}

		if len(fields) == 5 && fields[4] == "free" {
			partition.Number = 0 // We use 0 to special case as a free partition
			partition.Start, err = strconv.ParseUint(strings.TrimRight(fields[1], "B"), 10, 64)
			if err != nil {
				log.Warning("setPartitionTable: Failed to parse start position from: %s", line)
			}
			partition.End, err = strconv.ParseUint(strings.TrimRight(fields[2], "B"), 10, 64)
			if err != nil {
				log.Warning("setPartitionTable: Failed to parse end position from: %s", line)
			}
			partition.Size, err = strconv.ParseUint(strings.TrimRight(fields[3], "B"), 10, 64)
			if err != nil {
				log.Warning("setPartitionTable: Failed to parse partition size from: %s", line)
			}
			partition.FileSystem = fields[4]

			partitionList = append(partitionList, partition)
		}
	}

	bd.PartTable = partitionList
}

func getMakeFsLabel(bd *BlockDevice) []string {
	label := []string{}
	labelArg := "-L"

	if bd.Label != "" {
		maxLen := MaxLabelLength(bd.FsType)

		if bd.FsType == "vfat" {
			labelArg = "-n"
		}

		if bd.FsType == "f2fs" {
			labelArg = "-l"
		}

		if len(bd.Label) > maxLen {
			shortLabel := string(bd.Label[0:(maxLen - 1)])
			log.Warning("Truncating file system label '%s' to %d character label '%s'",
				bd.FsType, maxLen, shortLabel)
			bd.Label = shortLabel
		}

		label = append(label, labelArg, bd.Label)
	}

	return label
}

func commonMakeFsCommand(bd *BlockDevice, args []string) ([]string, error) {
	cmd := []string{
		fmt.Sprintf("mkfs.%s", bd.FsType),
	}

	label := getMakeFsLabel(bd)
	if len(label) > 0 {
		cmd = append(cmd, label...)
	}

	cmd = append(cmd, args...)

	return cmd, nil
}

func commonMakePartCommand(bd *BlockDevice) (string, error) {
	args := []string{
		"mkpart",
		bd.MountPoint,
	}

	return strings.Join(args, " "), nil
}

func makeEncryptedSwap(bd *BlockDevice) error {
	args := []string{
		"wipefs",
		bd.GetDeviceFile(),
	}

	err := cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	args = []string{
		"mkfs.ext2",
		"-L",
		filepath.Base(bd.GetMappedDeviceFile()),
		bd.GetDeviceFile(),
		"1M",
	}

	err = cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func swapMakeFsCommand(bd *BlockDevice, args []string) ([]string, error) {
	cmd := []string{
		"mkswap",
	}

	if bd.FsType == "swap" && bd.Type == BlockDeviceTypeCrypt {
		// Fake the standard command, and call the special function
		cmd = []string{
			"/bin/true",
		}
		if err := makeEncryptedSwap(bd); err != nil {
			return cmd, err
		}
	} else {
		label := getMakeFsLabel(bd)
		if len(label) > 0 {
			cmd = append(cmd, label...)
		}

		cmd = append(cmd, args...)
	}

	return cmd, nil
}

func swapMakePartCommand(bd *BlockDevice) (string, error) {
	partName := "linux-swap"

	if bd.FsType == "swap" && bd.Type == BlockDeviceTypeCrypt {
		mapped := fmt.Sprintf("eswap-%s", bd.Name)
		bd.MappedName = filepath.Join("mapper", mapped)
		partName = mapped
	}

	args := []string{
		"mkpart",
		partName,
	}

	return strings.Join(args, " "), nil
}

func vfatMakePartCommand(bd *BlockDevice) (string, error) {
	args := []string{
		"mkpart",
		"EFI",
		"fat32",
	}

	return strings.Join(args, " "), nil
}

// MakeImage create an image file considering the total block device size
func MakeImage(bd *BlockDevice, file string) error {
	size, err := bd.DiskSize()
	if err != nil {
		return errors.Wrap(err)
	}

	args := []string{
		"qemu-img",
		"create",
		"-f",
		"raw",
		file,
		fmt.Sprintf("%d", size),
	}

	err = cmd.RunAndLog(args...)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

// SetupLoopDevice sets up a loop device and return the loop device path
func SetupLoopDevice(file string) (string, error) {
	args := []string{
		"losetup",
		"--partscan",
		"--find",
		"--show",
		file,
	}

	buff := bytes.NewBuffer(nil)

	err := cmd.Run(buff, args...)
	if err != nil {
		return "", errors.Wrap(err)
	}

	result := buff.String()
	if result == "" {
		return result, errors.Errorf("Could not setup loop device")
	}

	return strings.Replace(result, "\n", "", -1), nil
}

// DetachLoopDevice detaches a loop device
func DetachLoopDevice(file string) {
	args := []string{
		"losetup",
		"-d",
		file,
	}

	_ = cmd.RunAndLog(args...)
}

// GenerateTabFiles creates the /etc mounting files if needed
func GenerateTabFiles(rootDir string, medias []*BlockDevice) error {
	var crypttab []string
	var fstab []string
	var errFound bool

	for _, curr := range medias {
		for _, ch := range curr.Children {
			// Handle Encrypted partitions
			var ctab []string
			var ftab []string

			if ch.Type == BlockDeviceTypeCrypt {
				if ch.FsType == "swap" {
					ctab = append(ctab, filepath.Base(ch.MappedName), ch.GetDeviceID(),
						"/dev/urandom",
						fmt.Sprintf("swap,offset=2048,cipher=%s,size=%d",
							EncryptCipher, EncryptKeySize))

					ftab = append(ftab, ch.GetMappedDeviceFile(), "none",
						"swap", "defaults", "0", "0")
				} else {
					if !ch.isStandardMount() {
						ctab = append(ctab, filepath.Base(ch.MappedName), ch.GetDeviceID())
						ftab = append(ftab, ch.GetMappedDeviceFile(), ch.MountPoint,
							ch.FsType, "defaults", "0", "2")
					}
				}
			} else {
				if !ch.isStandardMount() && ch.MountPoint != "" {
					ftab = append(ftab, ch.GetDeviceID(), ch.MountPoint,
						ch.FsType, "defaults", "0", "2")
				}
			}

			if len(ctab) > 0 {
				crypttab = append(crypttab, strings.Join(ctab, " "))
			}
			if len(ftab) > 0 {
				fstab = append(fstab, strings.Join(ftab, " "))
			}
		}
	}

	if len(crypttab) > 0 {
		etcDir := filepath.Join(rootDir, "etc")
		crypttabFile := filepath.Join(rootDir, "etc", "crypttab")
		lines := strings.Join(crypttab, "\n") + "\n"

		if err := utils.MkdirAll(etcDir, 0755); err != nil {
			log.Error("Failed to create %s dir: %v", etcDir, err)
			errFound = true
		}

		if err := ioutil.WriteFile(crypttabFile, []byte(lines), 0644); err != nil {
			log.Error("Failed to write crypttab: %v", err)
			errFound = true
		}
	}

	if len(fstab) > 0 {
		etcDir := filepath.Join(rootDir, "etc")
		fstabFile := filepath.Join(rootDir, "etc", "fstab")
		lines := strings.Join(fstab, "\n") + "\n"

		if err := utils.MkdirAll(etcDir, 0755); err != nil {
			log.Error("Failed to create %s dir: %v", etcDir, err)
			errFound = true
		}

		if err := ioutil.WriteFile(fstabFile, []byte(lines), 0644); err != nil {
			log.Error("Failed to write fstab: %v", err)
			errFound = true
		}
	}

	if errFound {
		return errors.Errorf("Error while creating mount files")
	}

	return nil
}

// InstallTarget describes a BlockDevice which is a valid installation target
type InstallTarget struct {
	Name      string // block device name
	Friendly  string // user friendly device name
	WholeDisk bool   // Can we use the whole disk?
	Removable bool   // Is this removable/hotswap media?
	EraseDisk bool   // Are we wiping the disk? New partition table
	DataLoss  bool   // Are we making changes which will lose data
	Advanced  bool   // Was this disk configured via advanced mode?
	FreeStart uint64 // Starting position of free space
	FreeEnd   uint64 // Ending position of free space
}

const (
	// MinimumServerInstallSize is the smallest installation size in bytes for a Server
	MinimumServerInstallSize = uint64(4) * (1000 * 1000 * 1000) // 4GB

	// MinimumDesktopInstallSize is the smallest installation size in bytes for a Desktop
	MinimumDesktopInstallSize = uint64(20) * (1000 * 1000 * 1000) // 20GB
)

func sortInstallTargets(targets []InstallTarget) []InstallTarget {
	sort.SliceStable(targets, func(i, j int) bool {
		// Ordering is:
		// -- Non-removable disks
		// -- Whole Disk
		// -- Disk with with largest free space

		if !targets[i].Removable && targets[j].Removable {
			return true
		}
		if targets[i].Removable && !targets[j].Removable {
			return false
		}

		if targets[i].WholeDisk && !targets[j].WholeDisk {
			return true
		}
		if !targets[i].WholeDisk && targets[j].WholeDisk {
			return false
		}

		iSize := targets[i].FreeEnd - targets[i].FreeStart
		jSize := targets[j].FreeEnd - targets[j].FreeStart
		return jSize <= iSize
	})

	return targets
}

// FindSafeInstallTargets creates an order list of possible installation targets
// Only disk with gpt partition are safe to use
// There must be at least 3 free partition in the table (gpt can have 127)
// There must be at least minSize free space on the disk
func FindSafeInstallTargets(rootSize uint64, medias []*BlockDevice) []InstallTarget {
	var installTargets []InstallTarget

	// Add the default boot and swap to the passed root size
	minSize := rootSize + bootSizeDefault
	minSizeStr, _ := HumanReadableSizeXiBWithPrecision(minSize, 1)

	FilterBlockDevices(medias,
		// Firstly, we filter out non-gpt partitions
		func(curr *BlockDevice) bool {
			if curr.PtType != "gpt" && curr.PtType != "" {
				log.Debug("FindSafeInstallTargets: ignoring disk %s with partition table type %s",
					curr.Name, curr.PtType)
				return false
			}
			return true
		},
		// Secondly, we filter out Block Devices with more than 125 existing partitions
		func(curr *BlockDevice) bool {
			if curr.Children != nil && len(curr.Children) > 125 {
				log.Debug("FindSafeInstallTargets: ignoring disk %s with too many partitions (%d)",
					curr.Name, len(curr.Children))
				return false
			}
			return true
		})

	for _, curr := range medias {
		// Thirdly, we want to select Block Devices which can support
		// the minSize required for installation If it satisfies,
		// it is a potential install target
		if curr.Size < minSize {
			currSizeStr, _ := HumanReadableSizeXiBWithPrecision(curr.Size, 1)
			log.Debug("FindSafeInstallTargets: Media %s (%s) smaller than minSize %s", curr.Name,
				currSizeStr, minSizeStr)
			continue
		}

		if curr.Children == nil || len(curr.Children) == 0 {
			// No partition type and no children we write the whole disk
			installTargets = append(installTargets,
				InstallTarget{Name: curr.Name, Friendly: curr.Model,
					WholeDisk: true, Removable: curr.RemovableDevice,
					FreeStart: 0, FreeEnd: curr.Size})
			log.Debug("FindSafeInstallTargets: found whole disk %s", curr.Name)
			continue
		}

		// Fourthly, we want to select Block Devices whose
		// largest contingous space satisfies the minSize required for installation
		if start, end := curr.LargestContiguousFreeSpace(minSize); start != 0 && end != 0 {
			installTargets = append(installTargets,
				InstallTarget{Name: curr.Name, Friendly: curr.Model,
					Removable: curr.RemovableDevice, FreeStart: start, FreeEnd: end})
			log.Debug("FindSafeInstallTargets: Room on disk %s: %d to %d", curr.Name, start, end)
			continue
		}

		log.Debug("FindSafeInstallTargets: Media %s does not have enough unallocated space minSize %s",
			curr.Name, minSizeStr)
	}

	return sortInstallTargets(installTargets)
}

// FindAllInstallTargets creates an order list of all possible installation targets
// There must be at least minSize free space on the disk
func FindAllInstallTargets(rootSize uint64, medias []*BlockDevice) []InstallTarget {
	var installTargets []InstallTarget

	// Add the default boot and swap to the passed root size
	minSize := rootSize + bootSizeDefault

	// All Disk are possible destructive installs
	FilterBlockDevices(medias,
		func(curr *BlockDevice) bool {
			if curr.Size >= minSize {
				target := InstallTarget{Name: curr.Name, Friendly: curr.Model,
					WholeDisk: true, Removable: curr.RemovableDevice, EraseDisk: true,
					FreeStart: 0, FreeEnd: curr.Size}

				installTargets = append(installTargets, target)
				log.Debug("FindAllInstallTargets: found whole disk %s", curr.Name)
				return true
			}

			currSizeStr, _ := HumanReadableSizeXiBWithPrecision(curr.Size, 1)
			minSizeStr, _ := HumanReadableSizeXiBWithPrecision(minSize, 1)
			log.Debug("FindAllInstallTargets: Media %s (%s) smaller than minSize %s", curr.Name,
				currSizeStr, minSizeStr)
			return false
		})

	return sortInstallTargets(installTargets)
}

// FindAdvancedInstallTargets creates a list of advanced installation targets
// We use Partition Labels to tag and convey which partitions should be used
// for an advanced installations.
//	CLR_BOOT = The /boot partition; must be vfat
//	CLR_SWAP = A swap partition to use; can be more than one
//	CLR_ROOT = The / root partition; must be ext[234], xfs or f2fs.
//		due to clr-boot-manager
//	CLR_MNT = Any additional partitions that should be
//		included in the install like /srv, /home, ...
//
// Appending "_E" to the label marks it for encryption; not valid for CLR_BOOT
// Appending "_F" to the label marks it for formatting (newfs)
func FindAdvancedInstallTargets(medias []*BlockDevice) []*BlockDevice {
	var targetMedias []*BlockDevice
	defaultFsType := "ext4"
	defaultBootFsType := "vfat"

	for _, curr := range medias {
		var installBlockDevice *BlockDevice
		clrAdded := false
		installBlockDevice = curr.Clone()

		for _, ch := range installBlockDevice.Children {
			clrFound := false
			label := ch.PartitionLabel

			if label != "" {
				log.Debug("FindAdvancedInstallTargets: Found partition %s with name %s", ch.Name, label)
			}

			for _, part := range strings.Split(label, "_") {
				lowerPart := strings.ToLower(part)

				// Filter out parts which don't start with CLR which
				// mean they are not meant for advanced Installation
				if !clrFound {
					if lowerPart == "clr" {
						log.Debug("FindAdvancedInstallTargets: Partition label contains clr %s", ch.Name)
						clrFound = true
					}
					continue
				}

				switch lowerPart {
				case "boot":
					if curr.Type == BlockDeviceTypeRAID1 ||
						curr.Type == BlockDeviceTypeRAID0 ||
						curr.Type == BlockDeviceTypeRAID4 ||
						curr.Type == BlockDeviceTypeRAID5 ||
						curr.Type == BlockDeviceTypeRAID6 ||
						curr.Type == BlockDeviceTypeRAID10 ||
						curr.Type == BlockDeviceTypeLVM2Volume {
						break
					}

					if ch.Type == BlockDeviceTypeCrypt {
						log.Warning("FindAdvancedInstallTargets: /boot can not be encrypted, skipping")
						ch.Type = BlockDeviceTypePart
					}
					log.Debug("FindAdvancedInstallTargets: Boot is %s", ch.Name)
					ch.LabeledAdvanced = true
					if ch.FsType == "" {
						log.Debug("FindAdvancedInstallTargets: No FsType set for %s, defaulting to %s",
							ch.Name, defaultBootFsType)
						ch.FsType = defaultBootFsType
						log.Debug("FindAdvancedInstallTargets: Forcing Format partition %s enabled", ch.Name)
						ch.FormatPartition = true
					}
					clrAdded = true
					ch.MountPoint = "/boot"
				case "root":
					log.Debug("FindAdvancedInstallTargets: Root is %s", ch.Name)
					ch.LabeledAdvanced = true
					if ch.FsType == "" {
						log.Debug("FindAdvancedInstallTargets: No FsType set for %s, defaulting to %s",
							ch.Name, defaultFsType)
						ch.FsType = defaultFsType
						log.Debug("FindAdvancedInstallTargets: Forcing Format partition %s enabled", ch.Name)
						ch.FormatPartition = true
					}
					clrAdded = true
					ch.MountPoint = "/"
				case "swap":
					log.Debug("FindAdvancedInstallTargets: Swap on %s", ch.Name)
					ch.LabeledAdvanced = true
					if ch.FsType == "" {
						log.Debug("FindAdvancedInstallTargets: No FsType set for %s, defaulting to %s",
							ch.Name, "swap")
						ch.FsType = "swap"
						log.Debug("FindAdvancedInstallTargets: Forcing Format partition %s enabled", ch.Name)
						ch.FormatPartition = true
					}
					clrAdded = true
				case "mnt":
					mntParts := strings.Split(label, "MNT_")
					if len(mntParts) == 2 {
						path := filepath.Clean(mntParts[1])
						if filepath.IsAbs(path) {
							log.Debug("FindAdvancedInstallTargets: Extra mount %q for %s", path, ch.Name)

							ch.MountPoint = path
							ch.LabeledAdvanced = true
							if ch.FsType == "" {
								log.Debug("FindAdvancedInstallTargets: No FsType set for %s, defaulting to %s",
									ch.Name, defaultFsType)
								ch.FsType = defaultFsType
								log.Debug("FindAdvancedInstallTargets: Forcing Format partition %s enabled", ch.Name)
								ch.FormatPartition = true
							}
							clrAdded = true
						}
					}
					break
				case "f":
					ch.FormatPartition = true
					log.Debug("FindAdvancedInstallTargets: Format partition %s enabled", ch.Name)
				}
			}

			if len(ch.Children) > 0 {
				targetMedias = append(targetMedias, FindAdvancedInstallTargets(ch.Children)...)
			}
		}

		if clrAdded {
			targetMedias = append(targetMedias, installBlockDevice)
		}
	}

	for _, curr := range targetMedias {
		for _, ch := range curr.Children {
			log.Debug("FindAdvancedInstallTargets: child: %+v", ch)
		}
	}

	return targetMedias
}

// HasAdvancedSwap check if the advanced media contain a swap partition
func HasAdvancedSwap(medias []*BlockDevice) bool {
	hasSwap := false

	for _, curr := range medias {
		for _, ch := range curr.Children {
			if ch.LabeledAdvanced && ch.PartitionLabel == "CLR_SWAP" {
				hasSwap = true
			}
		}
	}

	log.Debug("HasAdvancedSwap: %v", hasSwap)

	return hasSwap
}

// FormatInstallPortion is the common code for describing
// the amount of disk used
func FormatInstallPortion(target InstallTarget) string {
	portion := utils.Locale.Get("Partial")
	if target.WholeDisk || target.EraseDisk {
		portion = utils.Locale.Get("Entire Disk")
	}
	if target.Advanced {
		if target.EraseDisk {
			portion = ""
		} else {
			portion = utils.Locale.Get("Advanced")
		}
	}

	if portion != "" {
		portion = "[" + portion + "]"
	}

	return portion
}

func (a ByBDName) Less(i, j int) bool {
	iPartNum := devNameSuffixExp.FindString(a[i].Name)
	jPartNum := devNameSuffixExp.FindString(a[j].Name)

	// When both partitions end with a number and the partition names
	// without partition numbers match, use the partition numbers to
	// compare the partitions
	if iPartNum != "" && jPartNum != "" {
		iPartName := devNameSuffixExp.Split(a[i].Name, 2)[0]
		jPartName := devNameSuffixExp.Split(a[j].Name, 2)[0]

		if iPartName == jPartName {
			iNum, _ := strconv.Atoi(iPartNum)
			jNum, _ := strconv.Atoi(jPartNum)
			return iNum < jNum
		}
	}
	return a[i].Name < a[j].Name
}

// ServerValidatePartitions returns an array of validation error
// strings for the partitions based on a Server installation.
func ServerValidatePartitions(medias []*BlockDevice, mediaOpts MediaOpts) []string {
	advancedMode := false
	return validatePartitions(MinimumServerInstallSize, medias, mediaOpts, advancedMode)
}

// DesktopValidatePartitions returns an array of validation error
// strings for the partitions based on a Desktop installation.
func DesktopValidatePartitions(medias []*BlockDevice, mediaOpts MediaOpts) []string {
	advancedMode := false
	return validatePartitions(MinimumDesktopInstallSize, medias, mediaOpts, advancedMode)
}

// Helper functions for validatePartitions
func logPartitionWarning(bd *BlockDevice, format string, vargs ...interface{}) string {
	warning := utils.Locale.Get(format, vargs...)
	if bd == nil {
		log.Warning("validatePartitions: %s", warning)
	} else {
		log.Warning("validatePartitions: %s %v+", warning, bd)
	}
	return warning
}

// Helper functions for validatePartitions
func logPartitionSizeWarning(bd *BlockDevice, partSize uint64, label string) string {
	size, _ := HumanReadableSizeXiBWithPrecision(partSize, 1)
	return logPartitionMustBeWarning(bd, label, fmt.Sprintf(">= %s", size))
}

// Helper functions for validatePartitions
func logPartitionMustBeWarning(bd *BlockDevice, before, after string) string {
	return logPartitionWarning(bd, "%s must be %s", before, after)
}

// Helper functions for validatePartitions
func logMissingPartition(label string) string {
	return logPartitionWarning(nil, "Missing %s partition", label)
}

// Helper to validatePartitions for validating boot minimum size etc
func validateBoot(found *bool, bd *BlockDevice, skipSize bool, bootLabel string) []string {
	var results []string

	if bd.MountPoint == "/boot" {
		if *found {
			results = append(results, logPartitionWarning(bd, "Found multiple %s partitions", bootLabel))
		} else {
			*found = true
			if bd.FsType != "vfat" {
				results = append(results, logPartitionMustBeWarning(bd, bootLabel, "vfat"))
			}
		}
		if bd.Size == 0 {
			log.Warning("validatePartitions: Skipping %s size check due to zero size", bootLabel)
		} else if skipSize {
			log.Warning("validatePartitions: Skipping %s size check due to skipSize", bootLabel)
		} else {
			if bd.Size < minBootSize {
				results = append(results, logPartitionSizeWarning(bd, minBootSize, bootLabel))
			}
		}
	}

	return results
}

// Helper to validatePartitions for validating root minimum size etc
func validateRoot(found *bool, bd *BlockDevice,
	minRootSize uint64, skipSize bool, rootLabel string) (*BlockDevice, []string) {
	var rootBlockDevice *BlockDevice
	var results []string

	if *found {
		results = append(results, logPartitionWarning(bd, "Found multiple %s partitions", rootLabel))
	} else {
		*found = true
		rootBlockDevice = bd.Clone()
		if !(bd.FsType == "ext2" || bd.FsType == "ext3" ||
			bd.FsType == "ext4" || bd.FsType == "xfs" ||
			bd.FsType == "f2fs") {
			results = append(results, logPartitionMustBeWarning(bd, rootLabel, "ext*|xfs|f2fs"))
		}
	}

	if bd.Size == 0 {
		log.Warning("validatePartitions: Skipping %s size check due to zero size", rootLabel)
	} else if skipSize {
		log.Warning("validatePartitions: Skipping %s size check due to skipSize", rootLabel)
	} else {
		if bd.Size < minRootSize {
			results = append(results, logPartitionSizeWarning(bd, minRootSize, rootLabel))
		}
	}

	return rootBlockDevice, results
}

// Helper to validatePartitions for validating Swap minimum size etc
func validateSwap(found *bool, bd *BlockDevice, skipSize bool, swapLabel string) []string {
	var results []string

	*found = true
	if bd.Size == 0 {
		log.Warning("validatePartitions: Skipping swap size check due to zero size")
	} else if skipSize {
		log.Warning("validatePartitions: Skipping swap size check due to skipSize")
	} else {
		if bd.Size < minSwapSize {
			results = append(results, logPartitionSizeWarning(bd, minSwapSize, swapLabel))
		} else if bd.Size > maxSwapSize {
			size, _ := HumanReadableSizeXiBWithPrecision(maxSwapSize, 1)
			results = append(results, logPartitionMustBeWarning(bd, swapLabel, fmt.Sprintf("<= %s", size)))
		}
	}

	return results
}

// Helper to validatePartitions for validating /var
func validateBootLegacy(rootBlockDevice *BlockDevice, rootLabel, bootLabel string, mediaOpts MediaOpts) []string {
	var results []string

	if mediaOpts.SkipValidationAll {
		if mediaOpts.LegacyBios {
			if rootBlockDevice != nil {
				if !(rootBlockDevice.FsType == "ext2" || rootBlockDevice.FsType == "ext3" ||
					rootBlockDevice.FsType == "ext4") {
					// xfs currently not supported due to partition table of MBR requirement
					log.Warning("validatePartitions: legacyMode, invalid fsType: %s", rootBlockDevice.FsType)
					results = append(results,
						logPartitionMustBeWarning(rootBlockDevice, rootLabel, "ext[234]"))
				}
				if rootBlockDevice.Type == BlockDeviceTypeCrypt {
					log.Warning("validatePartitions: legacyMode without /boot can not be encrypted")
					results = append(results,
						logPartitionWarning(rootBlockDevice, "Encryption of %s is not supported", rootLabel))
				}
			}
		} else {
			results = append(results, logMissingPartition(bootLabel))
		}
	} else {
		results = append(results, logMissingPartition(bootLabel))
	}
	return results
}

// Helper to validatePartitions for validating /var
func validateVarPartition(rootBlockDevice *BlockDevice, skipSize bool, varSize uint64) []string {
	var results []string

	// Independent /var is discouraged for Clear Linux OS because
	// /var needs to be at nearly as large as / (root) due to
	// swupd stashing content in /var. About 70% of the / (root),
	// really /usr, is normally hard linked under /var.
	log.Warning("validatePartitions: Use for independent /var is discouraged" +
		" under Clear Linux OS due to swupd caching methods.")

	if !skipSize && rootBlockDevice != nil {
		// Enough Room /var
		root70 := uint64(float64(rootBlockDevice.Size) * 0.7)
		if varSize < root70 {
			vSize, _ := HumanReadableSizeXiBWithPrecision(varSize, 1)
			rSize, _ := HumanReadableSizeXiBWithPrecision(root70, 1)
			results = append(results, logPartitionMustBeWarning(nil,
				fmt.Sprintf("/var (%s)", vSize),
				fmt.Sprintf(">= 70%% / (%s)", rSize)))
		}
	}

	return results
}

// Helper to validatePartitions for validating Swap minimum size etc
func validateSwapFile(swapFileSize string, rootBlockDevice *BlockDevice,
	skipSize bool, varFound bool, varSize uint64) []string {
	var results []string
	var checkSwapSize uint64
	var err error

	if swapFileSize == "" {
		checkSwapSize = SwapFileSizeDefault
	} else {
		checkSwapSize, err = ParseVolumeSize(swapFileSize)
		if err != nil {
			results = append(results, logPartitionWarning(nil, "Could not interrupt %s", swapFileSize))
			return results
		}
	}
	checkSizeString, _ := HumanReadableSizeXiBWithPrecision(checkSwapSize, 1)

	if rootBlockDevice != nil {
		// Sanity check that there is enough room in the partition
		// for the creation of the swapfile
		swapFilePartition := "/"
		swapFilePartSize := rootBlockDevice.Size
		if varFound {
			swapFilePartition = "/var"
			swapFilePartSize = varSize
		}
		if checkSwapSize >= swapFilePartSize {
			size, _ := HumanReadableSizeXiBWithPrecision(swapFilePartSize, 1)
			results = append(results, logPartitionMustBeWarning(nil,
				fmt.Sprintf("swapfile (%s)", checkSizeString),
				fmt.Sprintf("< %s (%s)", swapFilePartition, size)))
		}

		if !skipSize {
			if checkSwapSize < minSwapSize {
				size, _ := HumanReadableSizeXiBWithPrecision(minSwapSize, 1)
				results = append(results, logPartitionMustBeWarning(nil,
					fmt.Sprintf("swapfile (%s)", checkSizeString),
					fmt.Sprintf(">= %s", size)))
			} else if checkSwapSize > maxSwapSize {
				size, _ := HumanReadableSizeXiBWithPrecision(maxSwapSize, 3)
				results = append(results, logPartitionMustBeWarning(nil,
					fmt.Sprintf("swapfile (%s)", checkSizeString),
					fmt.Sprintf("<= %s", size)))
			}

			// Room for swapfile in partition?
			// TODO: Do we need/want this?
			// Check if the swapfile is larger than 50% of storing partition
			fiftyPercent := swapFilePartSize / 2
			if checkSwapSize > fiftyPercent {
				size, _ := HumanReadableSizeXiBWithPrecision(fiftyPercent, 1)
				results = append(results, logPartitionMustBeWarning(nil,
					fmt.Sprintf("swapfile (%s)", checkSizeString),
					fmt.Sprintf("<= 50%% %s (%s)", swapFilePartition, size)))
			}
		}
	}

	return results
}

// validatePartitions returns an array of validation error strings
func validatePartitions(rootSize uint64, medias []*BlockDevice, mediaOpts MediaOpts, advancedMode bool) []string {
	results := []string{}
	rootLabel := "/ (root)"
	bootLabel := "/boot"
	swapLabel := "[swap]"
	varLabel := "/var"

	if advancedMode {
		rootLabel = "CLR_ROOT"
		bootLabel = "CLR_BOOT"
		swapLabel = "CLR_SWAP"
		varLabel = "CLR_MNT_/var"
	}

	bootFound := false
	swapFound := false
	rootFound := false
	varFound := false
	var varSize uint64
	var rootBlockDevice *BlockDevice

	// If we are validating without media, special case results
	if medias == nil || len(medias) == 0 {
		results = append(results, utils.Locale.Get("No Media Selected"))
		return results
	}

	for _, curr := range medias {
		for _, ch := range curr.Children {
			if ch.MountPoint == "/boot" || (advancedMode && ch.Label == bootLabel) {
				results = append(results, validateBoot(&bootFound, ch, mediaOpts.SkipValidationSize, bootLabel)...)
			}
			if ch.MountPoint == "/" || (advancedMode && ch.Label == rootLabel) {
				var newResults []string
				rootBlockDevice, newResults = validateRoot(&rootFound, ch, rootSize,
					mediaOpts.SkipValidationSize, rootLabel)
				results = append(results, newResults...)
			}
			if ch.FsType == "swap" || (advancedMode && ch.Label == swapLabel) {
				results = append(results, validateSwap(&swapFound, ch, mediaOpts.SkipValidationSize, swapLabel)...)
			}
			if ch.MountPoint == "/var" || (advancedMode && ch.Label == varLabel) {
				varFound = true
				varSize = ch.Size
			}
		}
	}

	if !rootFound || rootBlockDevice == nil {
		results = append(results, logMissingPartition(rootLabel))
	}

	if !bootFound {
		results = append(results, validateBootLegacy(rootBlockDevice, rootLabel, bootLabel, mediaOpts)...)
	}

	if varFound {
		results = append(results, validateVarPartition(rootBlockDevice,
			mediaOpts.SkipValidationSize, varSize)...)
	}

	// If no swap partition found or the swapfile size was manually set
	if !swapFound || mediaOpts.SwapFileSet {
		results = append(results, validateSwapFile(mediaOpts.SwapFileSize, rootBlockDevice,
			mediaOpts.SkipValidationSize, varFound, varSize)...)
	}

	return results
}

// ServerValidateAdvancedPartitions returns an array of validation error
// strings for the advanced partitions based on a Server installation.
func ServerValidateAdvancedPartitions(medias []*BlockDevice, mediaOpts MediaOpts) []string {
	return validateAdvancedPartitions(MinimumServerInstallSize, medias, mediaOpts)
}

// DesktopValidateAdvancedPartitions returns an array of validation error
// strings for the advanced partitions based on a Desktop installation.
func DesktopValidateAdvancedPartitions(medias []*BlockDevice, mediaOpts MediaOpts) []string {
	return validateAdvancedPartitions(MinimumDesktopInstallSize, medias, mediaOpts)
}

// validateAdvancedPartitions returns an array of validation error
// strings for the advanced partitions
func validateAdvancedPartitions(rootSize uint64, medias []*BlockDevice, mediaOpts MediaOpts) []string {
	results := []string{}

	labelMap := make(map[string]bool)

	advancedMode := true // This is Advanced Mode
	valResults := validatePartitions(rootSize, medias, mediaOpts, advancedMode)
	results = append(results, valResults...)

	for _, curr := range medias {
		for _, ch := range curr.Children {
			clrFound := false
			label := ch.PartitionLabel
			labelUpper := ""

			if label != "" {
				log.Debug("validateAdvancedPartitions: Found partition %s with name %s", ch.Name, label)
			}

			for _, part := range strings.Split(label, "_") {
				lowerPart := strings.ToLower(part)
				if labelUpper != "" {
					labelUpper = labelUpper + "_"
				}
				labelUpper = labelUpper + strings.ToUpper(part)

				if !clrFound {
					if lowerPart == "clr" {
						clrFound = true
					}
					continue
				}

				switch lowerPart {
				case "mnt":
					failed := false
					warning := utils.Locale.Get("Found invalid %s partition", label)

					mntParts := strings.Split(label, "MNT_")
					if len(mntParts) != 2 {
						failed = true
						log.Warning("validateAdvancedPartitions: %s %+v (%s)", warning, ch, "too many parts")
					}

					if !strings.HasPrefix(mntParts[1], "/") {
						failed = true
						log.Warning("validateAdvancedPartitions: %s %+v (%s)", warning, ch, "must start with '/'")
					}

					path := filepath.Clean(mntParts[1])
					if !filepath.IsAbs(path) {
						failed = true
						log.Warning("validateAdvancedPartitions: %s %+v (%s)", warning, ch, "must be an absolute path")
					}

					if labelMap[labelUpper] {
						failed = true
						log.Warning("validateAdvancedPartitions: %s %+v (%s)",
							warning, ch, "found duplicate partition label")
					} else {
						labelMap[labelUpper] = true
					}

					if failed {
						results = append(results, warning)
					}
					break
				}
			}
		}
	}

	return results
}

// AdvancedPartitionsRequireEncryption returns an array of validation error
// strings for the advanced partitions
func AdvancedPartitionsRequireEncryption(medias []*BlockDevice) bool {
	encryptionFound := false

	for _, curr := range medias {
		for _, ch := range curr.Children {
			clrFound := false
			label := ch.PartitionLabel

			for _, part := range strings.Split(label, "_") {
				lowerPart := strings.ToLower(part)

				if !clrFound {
					if lowerPart == "clr" {
						clrFound = true
					}
					continue
				}

				switch lowerPart {
				case "boot":
					break
					/*
						case "e":
							encryptionFound = true
					*/
				}
			}
		}
	}

	if encryptionFound {
		log.Debug("AdvancedPartitionsRequireEncryption Found at least one partition which requires encryption")
	}

	return encryptionFound
}

// GetAdvancedPartitions returns an array of strings for the
// assigned advanced partitions used
func GetAdvancedPartitions(medias []*BlockDevice) []string {
	results := []string{}

	formatter := func(child *BlockDevice) string {
		var name string
		if child.MountPoint != "" {
			name = child.Name + ":" + child.MountPoint
		} else {
			name = child.Name + ":" + child.FsType
		}
		if child.Type == BlockDeviceTypeCrypt {
			name = name + "*"
		}

		return name
	}

	for _, curr := range medias {
		for _, ch := range curr.Children {
			var found bool
			if strings.HasPrefix(ch.PartitionLabel, "CLR_BOOT") &&
				len(validateBoot(&found, ch, false, "CLR_BOOT")) == 0 {
				if found {
					results = append(results, formatter(ch))
				}
			}
			if strings.HasPrefix(ch.PartitionLabel, "CLR_SWAP") &&
				len(validateSwap(&found, ch, false, "CLR_SWAP")) == 0 {
				if found {
					ch.FsType = "swap"
					results = append(results, formatter(ch))
				}
			}
			if strings.HasPrefix(ch.PartitionLabel, "CLR_ROOT") {
				_, rootResults := validateRoot(&found, ch, 0, false, "CLR_ROOT")
				if len(rootResults) == 0 && found {
					results = append(results, formatter(ch))
				}
			}
			if strings.HasPrefix(ch.PartitionLabel, "CLR_MNT_/") {
				results = append(results, formatter(ch))
			}
		}
	}

	return results
}

// setBootPartition is a helper function to PrepareInstallationMedia
// Looks through all of the installation media to determine which
// partition will be the one from which the install boots
// either an explicit /boot or / (root) in legacy mode
func setBootPartition(medias []*BlockDevice, mediaOpts MediaOpts, dryRun *[]string) error {
	const (
		bootStyleDefault = "boot"
		bootStyleLegacy  = "legacy_boot"
	)

	logFormatError := func(format string, vargs ...interface{}) string {
		mesg := utils.Locale.Get(format, vargs...)
		log.Error(mesg)

		if dryRun != nil {
			*dryRun = append(*dryRun, mesg)
		}

		return mesg
	}

	style := bootStyleDefault
	var bootParent, bootBlockDevice, rootParent, rootBlockDevice *BlockDevice

	// Check if there is a bootable partition
	// Clear Linux OS only supports booting from a top level
	// block device; RAID, LVM, encryption, etc are not supported
	for _, bd := range medias {
		for _, curr := range bd.Children {
			// We have the standard /boot partition
			if curr.MountPoint == "/boot" {
				if bootBlockDevice != nil {
					return errors.Errorf(logFormatError("Found multiple %s partition names", curr.MountPoint))
				}
				bootParent = bd
				bootBlockDevice = curr
			}

			if curr.MountPoint == "/" {
				if rootBlockDevice != nil {
					return errors.Errorf(logFormatError("Found multiple %s partition names", curr.MountPoint))
				}
				rootParent = bd
				rootBlockDevice = curr
			}
		}
	}

	// In case we don't have a viable boot partition
	if bootBlockDevice == nil && !mediaOpts.LegacyBios {
		log.Error("No /boot and not in legacy mode!")
		return errors.Errorf(logFormatError("Found invalid %s partition name", "!BOOT"))
	}

	if bootBlockDevice == nil && rootBlockDevice == nil {
		log.Error("No /boot nor / (root) partition found!")
		return errors.Errorf(logFormatError("Found invalid %s partition name", "!BOOT/!ROOT"))
	}

	// If legacyBios mode
	if mediaOpts.LegacyBios {
		style = bootStyleLegacy

		// If legacyBios mode and we do not have a boot, use root
		if bootBlockDevice == nil {
			bootBlockDevice = rootBlockDevice
			bootParent = rootParent
			if dryRun != nil {
				*dryRun = append(*dryRun, bootBlockDevice.Name+": "+utils.Locale.Get(LegacyModeWarning))
				*dryRun = append(*dryRun, bootBlockDevice.Name+": "+utils.Locale.Get(LegacyNoBootWarning))
			}

			// Need to disable the ext 64bit mode
			// https://wiki.syslinux.org/wiki/index.php?title=Filesystem#ext
			if bootBlockDevice.FsType == "ext2" ||
				bootBlockDevice.FsType == "ext3" || bootBlockDevice.FsType == "ext4" {
				legacyExtFsOpt := "-O ^64bit"
				bootBlockDevice.Options = strings.Join([]string{bootBlockDevice.Options, legacyExtFsOpt}, " ")
				log.Warning("setBootPartition: legacy_boot on / requires option: %s", legacyExtFsOpt)
			}
		} else {
			if dryRun != nil {
				*dryRun = append(*dryRun, bootBlockDevice.Name+": "+utils.Locale.Get(LegacyModeWarning))
			}
		}
	}

	if dryRun == nil {
		var prg progress.Progress
		mesg := utils.Locale.Get("Setting boot partition: %s",
			fmt.Sprintf("%s [%s]", bootBlockDevice.Name, style))
		prg = progress.NewLoop(mesg)
		log.Info(mesg)

		args := []string{
			"parted",
			bootParent.GetDeviceFile(),
			fmt.Sprintf("set %d %s on", bootBlockDevice.partition, style),
		}

		if err := cmd.RunAndLog(args...); err != nil {
			return errors.Wrap(err)
		}

		prg.Success()
	}

	return nil
}

func getPlannedPartitionChanges(media *BlockDevice) []string {
	results := []string{}

	for _, ch := range media.Children {
		if ch.FormatPartition {
			partName := ch.Name
			if partName == "" {
				partName = ch.GetNewPartitionName(ch.partition)
			}

			part := fmt.Sprintf("%s: %s", partName,
				utils.Locale.Get(FormatPartitionInfo, ch.FsType))

			if ch.MountPoint != "" {
				part = part + fmt.Sprintf(" [%s]", ch.MountPoint)
			}

			if ch.Type == BlockDeviceTypeCrypt {
				part = part + " " + utils.Locale.Get("Encrypted")
			}

			results = append(results, part)
		} else if ch.MountPoint != "" || !ch.FsTypeNotSwap() {
			partName := ch.Name
			if partName == "" {
				partName = ch.GetNewPartitionName(ch.partition)
			}
			part := fmt.Sprintf("%s: %s", partName, utils.Locale.Get(UsePartitionInfo))

			if ch.MountPoint != "" {
				part = part + fmt.Sprintf(" [%s]", ch.MountPoint)
			} else if ch.FsType != "" {
				part = part + fmt.Sprintf(" (%s)", ch.FsType)
			}

			if ch.Type == BlockDeviceTypeCrypt {
				part = part + " " + utils.Locale.Get("Encrypted")
			}
			results = append(results, part)
		}
	}

	return results
}

// GetPlannedMediaChanges returns an array of strings with all of
// disk and partition planned changes to advise the user before start
func GetPlannedMediaChanges(targets map[string]InstallTarget, medias []*BlockDevice, mediaOpts MediaOpts) []string {
	results := []string{}

	if len(targets) != len(medias) {
		log.Warning("The number of install targets (%d) != media devices (%d)",
			len(targets), len(medias))

		for _, target := range targets {
			log.Warning("Install Target: %+v", target)
		}
		for _, curr := range medias {
			log.Warning("Media Device: %+v", curr)
		}
	}

	if err := PrepareInstallationMedia(targets, medias, mediaOpts, &results); err != nil {
		log.Warning("PrepareInstallationMedia: %+v", err)
	}

	if mediaOpts.SwapFileSize != "" {
		results = append(results, fmt.Sprintf("%s (%s)", SwapfileName, mediaOpts.SwapFileSize))
	}

	return results
}
