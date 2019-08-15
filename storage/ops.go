// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
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

type blockDeviceOps struct {
	makeFsCommand   func(bd *BlockDevice, args []string) ([]string, error)
	makeFsArgs      []string
	makePartCommand func(bd *BlockDevice) (string, error)
}

var (
	bdOps = map[string]*blockDeviceOps{
		"ext2":  {commonMakeFsCommand, []string{"-v", "-F"}, commonMakePartCommand},
		"ext3":  {commonMakeFsCommand, []string{"-v", "-F"}, commonMakePartCommand},
		"ext4":  {commonMakeFsCommand, []string{"-v", "-F", "-b", "4096"}, commonMakePartCommand},
		"btrfs": {commonMakeFsCommand, []string{"-f"}, commonMakePartCommand},
		"xfs":   {commonMakeFsCommand, []string{"-f"}, commonMakePartCommand},
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
)

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

	return nil
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
func (bd *BlockDevice) WritePartitionLabel() error {
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
		return errors.Wrap(err)
	}

	prg.Success()

	return nil
}

// WritePartitionTable writes the defined partitions to the actual block device
func (bd *BlockDevice) WritePartitionTable(legacyBios bool, wholeDisk bool, dryRun *[]string) error {
	if bd.Type != BlockDeviceTypeDisk && bd.Type != BlockDeviceTypeLoop {
		return errors.Errorf("Type is partition, disk required")
	}

	var prg progress.Progress

	//write the partition label
	if dryRun != nil {
		if wholeDisk {
			*dryRun = append(*dryRun, bd.Name+": "+utils.Locale.Get(PartitioningWarning))
		}
	} else {
		if wholeDisk {
			if err := bd.WritePartitionLabel(); err != nil {
				return err
			}
		} else {
			log.Debug("WritePartitionTable: partial disk, skipping mklabel for %s", bd.Name)
		}

		mesg := utils.Locale.Get("Updating partition table for: %s", bd.Name)
		prg = progress.NewLoop(mesg)
		log.Info(mesg)
	}

	var err error
	var start uint64
	maxFound := false

	// First remove any user removed partitions
	if len(bd.removedParts) > 0 {
		rmArgs := []string{
			"parted",
			"-a",
			"optimal",
			bd.GetDeviceFile(),
			"unit", "MB",
			"--script",
			"--",
		}
		for _, curr := range bd.removedParts {
			if dryRun == nil {
				rmArgs = append(rmArgs, fmt.Sprintf("rm %d", curr))
			} else {
				*dryRun = append(*dryRun, fmt.Sprintf("%s: %s",
					bd.GetNewPartitionName(curr),
					utils.Locale.Get(RemoveParitionWarning)))
			}
		}

		if dryRun == nil {
			log.Debug("WritePartitionTable: remove partitions : %v", bd.removedParts)
			err = cmd.RunAndLog(rmArgs...)
			if err != nil {
				log.Warning("Failed to remove existing partition: %v (%s)", bd.removedParts, err)
			}
		}
	}

	// Initialize the partition list before we add new ones
	currentPartitions := bd.getPartitionList()

	// Sort the partitions by name before writing the partition table
	log.Debug("Partitions before sorting:")
	for _, part := range bd.Children {
		part.logDetails()
	}

	sort.Sort(ByBDName(bd.Children))

	log.Debug("Partitions after sorting:")
	for _, part := range bd.Children {
		part.logDetails()
	}

	// Make the needed new partitions
	for _, curr := range bd.Children {
		if dryRun != nil {
			if curr.MakePartition {
				size, _ := HumanReadableSizeWithPrecision(curr.Size, 1)
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

		mkPart, err = op.makePartCommand(curr)
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
			log.Debug("WritePartitionTable: mkPartCmd: %s", mkPartCmd)

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

	if dryRun == nil {
		var bootPartition uint64
		bootStyle := "boot"
		guids := map[int]string{}

		// Now that all new partitions are created,
		// and we know their assigned numbers ...
		for _, curr := range bd.Children {
			// First, check if we have the standard /boot partition
			// We have a /boot partition, use this
			if curr.MountPoint == "/boot" {
				bootPartition = curr.partition
				if legacyBios {
					bootStyle = "legacy_boot"
				}
			}

			var guid string
			guid, err = curr.getGUID()
			if err != nil {
				log.Warning("%s", err)
			}

			if curr.FsType != "swap" || curr.Type != BlockDeviceTypeCrypt {
				guids[int(curr.partition)] = guid
			}
		}

		prg.Success()

		msg := utils.Locale.Get("Adjusting filesystem configurations")
		prg = progress.MultiStep(len(guids), msg)
		log.Info(msg)
		cnt := 1
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

			prg.Partial(cnt)
			cnt = cnt + 1
		}

		// In case we didn't have a /boot partition, we
		// need to set / as boot
		for _, curr := range bd.Children {
			// Only check for / in new partitions
			if !curr.MakePartition {
				continue
			}

			if curr.MountPoint == "/" {
				// If legacyBios mode and we do not have a boot, use root
				if legacyBios && bootPartition == 0 {
					bootPartition = curr.partition
					bootStyle = "legacy_boot"
				}
			}
		}

		if bootPartition != 0 {
			args := []string{
				"parted",
				bd.GetDeviceFile(),
				fmt.Sprintf("set %d %s on", bootPartition, bootStyle),
			}

			err = cmd.RunAndLog(args...)
			if err != nil {
				return errors.Wrap(err)
			}
		}

		if err = bd.PartProbe(); err != nil {
			prg.Failure()
			return err
		}

		time.Sleep(time.Duration(4) * time.Second)

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

// RemovePartition remove a child from the disk and updates
// frees the space in the partition table
func (bd *BlockDevice) RemovePartition(child *BlockDevice) *PartedPartition {
	var removedPartition *PartedPartition
	devFile := bd.GetDeviceFile()

	if !utils.IntSliceContains([]int{BlockDeviceTypeDisk, BlockDeviceTypeLoop}, int(bd.Type)) {
		log.Warning("RemovePartition() called on non-disk %q", devFile)
		return removedPartition
	}

	deleteIndex := -1
	for idx, curr := range bd.Children {
		if curr.Name == child.Name {
			child.Parent = nil
			deleteIndex = idx
			break
		}
	}
	if deleteIndex < 0 {
		log.Warning("RemovePartition() fail to find (and remove) child: %v", child)
		return removedPartition
	}
	log.Debug("RemovePartition() found child partition index to delete %d", deleteIndex)
	// keep a reference to the child
	delelteChild := bd.Children[deleteIndex].Clone()
	// Remove the child for the block devices
	bd.Children = append(bd.Children[:deleteIndex], bd.Children[deleteIndex+1:]...)

	partString := devNameSuffixExp.FindString(delelteChild.Name)
	partNumber, err := strconv.ParseUint(partString, 10, 64)
	if err != nil {
		log.Warning("RemovePartition() fail to find child partition number: %v", child)
		return removedPartition
	}
	log.Debug("RemovePartition() Need to add partition %d to the remove list", partNumber)

	for _, partition := range bd.PartTable {
		// Find the partition to free/remove
		if partition.Number == partNumber {
			log.Debug("Found the partition to free partition: %v", partition)
			partition.Number = 0
			partition.FileSystem = "free"
			partition.Name = ""
			partition.Flags = ""
			removedPartition = partition.Clone()
			break
		}
	}

	// Consolidate neighboring free partitions
	bd.consolidateFree()

	if !delelteChild.MakePartition {
		bd.addRemovePartition(partNumber)
		log.Debug("RemovePartition() Add partition to be removed %d", partNumber)
	}

	return removedPartition
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

func getMakeFsLabel(bd *BlockDevice) []string {
	label := []string{}
	labelArg := "-L"

	if bd.Label != "" {
		maxLen := MaxLabelLength(bd.FsType)

		if bd.FsType == "vfat" {
			labelArg = "-n"
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
	MinimumServerInstallSize = 4294967296

	// MinimumDesktopInstallSize is the smallest installation size in bytes for a Desktop
	MinimumDesktopInstallSize = 21474836480
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
func FindSafeInstallTargets(minSize uint64, medias []*BlockDevice) []InstallTarget {
	var installTargets []InstallTarget

	for _, curr := range medias {
		// Either 'gpt' or no partition table type
		if curr.PtType != "gpt" && curr.PtType != "" {
			log.Debug("FindSafeInstallTargets(): ignoring disk %s with partition table type %s",
				curr.Name, curr.PtType)
			continue
		}

		if curr.Children != nil && len(curr.Children) > 125 {
			log.Debug("FindSafeInstallTargets(): ignoring disk %s with too many partitions (%d)",
				curr.Name, len(curr.Children))
			continue
		}

		if curr.Children == nil || len(curr.Children) == 0 {
			// No partition type and no children we write the whole disk
			installTargets = append(installTargets,
				InstallTarget{Name: curr.Name, Friendly: curr.Model,
					WholeDisk: true, Removable: curr.RemovableDevice,
					FreeStart: 0, FreeEnd: curr.Size})
			log.Debug("FindSafeInstallTargets(): found whole disk %s", curr.Name)
			continue
		}

		if start, end := curr.LargestContiguousFreeSpace(minSize); start != 0 && end != 0 {
			installTargets = append(installTargets,
				InstallTarget{Name: curr.Name, Friendly: curr.Model,
					Removable: curr.RemovableDevice, FreeStart: start, FreeEnd: end})
			log.Debug("FindSafeInstallTargets(): Room on disk %s: %d to %d", curr.Name, start, end)
			continue
		}
	}

	return sortInstallTargets(installTargets)
}

// FindAllInstallTargets creates an order list of all possible installation targets
func FindAllInstallTargets(medias []*BlockDevice) []InstallTarget {
	var installTargets []InstallTarget

	// All Disk are possible destructive installs
	for _, curr := range medias {
		target := InstallTarget{Name: curr.Name, Friendly: curr.Model,
			WholeDisk: true, Removable: curr.RemovableDevice, EraseDisk: true,
			FreeStart: 0, FreeEnd: curr.Size}

		installTargets = append(installTargets, target)
	}

	return sortInstallTargets(installTargets)
}

// FindModifyInstallTargets creates an order list of possible installation targets
// Only Disk with a 'gpt' partition table are candidates for modification
func FindModifyInstallTargets(medias []*BlockDevice) []InstallTarget {
	var installTargets []InstallTarget

	for _, curr := range medias {
		if curr.PtType != "gpt" {
			log.Debug("FindModifyInstallTargets(): ignoring disk %s with partition table type %s",
				curr.Name, curr.PtType)
			continue
		}

		target := InstallTarget{Name: curr.Name, Friendly: curr.Model,
			WholeDisk: true, Removable: curr.RemovableDevice, EraseDisk: true,
			FreeStart: 0, FreeEnd: curr.Size}

		installTargets = append(installTargets, target)
	}

	return sortInstallTargets(installTargets)
}

// FindAdvancedInstallTargets creates a list of advanced installation targets
// We use Partition Labels to tag and convey which partitions should be used
// for an advanced installations.
//	CLR_BOOT = The /boot partition; must be vfat
//	CLR_SWAP = A swap partition to use; can be more than one
//	CLR_ROOT = The / root partition; must be ext[234] or xfs
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
			clrMountFound := false
			label := ch.PartitionLabel

			if label != "" {
				log.Debug("FindAdvancedInstallTargets: Found partition %s with label %s", ch.Name, label)
			}

			for _, part := range strings.Split(label, "_") {
				lowerPart := strings.ToLower(part)
				if clrMountFound {
					log.Debug("AdvancedPartitioning: Extra mount %q for %s", part, ch.Name)
					ch.MountPoint = part
					if ch.FsType == "" {
						log.Debug("AdvancedPartitioning: No FsType set for %s, defaulting to %s", ch.Name, defaultFsType)
						ch.FsType = defaultFsType
						log.Debug("AdvancedPartitioning: Forcing Format partition %s enabled", ch.Name)
						ch.FormatPartition = true
					}
					clrAdded = true
					clrMountFound = false
					continue
				}

				if !clrFound {
					if lowerPart == "clr" {
						log.Debug("FindAdvancedInstallTargets: Partition label contains clr %s", ch.Name)
						clrFound = true
					}
					continue
				}

				switch lowerPart {
				case "boot":
					if ch.Type == BlockDeviceTypeCrypt {
						log.Warning("AdvancedPartitioning: /boot can no be encrypted, skipping")
						ch.Type = BlockDeviceTypePart
					}
					log.Debug("AdvancedPartitioning: Boot is %s", ch.Name)
					ch.MountPoint = "/boot"
					if ch.FsType == "" {
						log.Debug("AdvancedPartitioning: No FsType set for %s, defaulting to %s", ch.Name, defaultBootFsType)
						ch.FsType = defaultBootFsType
						log.Debug("AdvancedPartitioning: Forcing Format partition %s enabled", ch.Name)
						ch.FormatPartition = true
					}
					clrAdded = true
				case "root":
					log.Debug("AdvancedPartitioning: Root is %s", ch.Name)
					ch.MountPoint = "/"
					if ch.FsType == "" {
						log.Debug("AdvancedPartitioning: No FsType set for %s, defaulting to %s", ch.Name, defaultFsType)
						ch.FsType = defaultFsType
						log.Debug("AdvancedPartitioning: Forcing Format partition %s enabled", ch.Name)
						ch.FormatPartition = true
					}
					clrAdded = true
				case "swap":
					log.Debug("AdvancedPartitioning: Swap on %s", ch.Name)
					clrAdded = true
					ch.FsType = "swap"
				case "mnt":
					clrMountFound = true
					log.Debug("FindAdvancedInstallTargets: Extra mount found %s", ch.Name)
				case "e":
					if ch.MountPoint == "/boot" {
						log.Warning("AdvancedPartitioning: /boot can no be encrypted, skipping")
					} else {
						ch.Type = BlockDeviceTypeCrypt
						log.Debug("AdvancedPartitioning: Encrypt partition %s", ch.Name)
					}
				case "f":
					ch.FormatPartition = true
					log.Debug("AdvancedPartitioning: Format partition %s enabled", ch.Name)
				}
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

// FormatInstallPortion is the common code for describing
// the amount of disk used
func FormatInstallPortion(target InstallTarget) string {
	portion := utils.Locale.Get("Partial")
	if target.WholeDisk {
		portion = utils.Locale.Get("Entire Disk")
	}
	if target.EraseDisk {
		portion = utils.Locale.Get("Erase Disk")
	}
	if target.Advanced {
		portion = utils.Locale.Get("Advanced")
	}

	return "[" + portion + "]"
}

// ByBDName implements sort.Interface for []*BlockDevice based on the Name field.
type ByBDName []*BlockDevice

func (a ByBDName) Len() int           { return len(a) }
func (a ByBDName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByBDName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// ServerValidateAdvancedPartitions returns an array of validation error
// strings for the advanced partitions based on a Server installation.
func ServerValidateAdvancedPartitions(medias []*BlockDevice) []string {
	return validateAdvancedPartitions(MinimumServerInstallSize, medias)
}

// DesktopValidateAdvancedPartitions returns an array of validation error
// strings for the advanced partitions based on a Desktop installation.
func DesktopValidateAdvancedPartitions(medias []*BlockDevice) []string {
	return validateAdvancedPartitions(MinimumDesktopInstallSize, medias)
}

// validateAdvancedPartitions returns an array of validation error
// strings for the advanced partitions
func validateAdvancedPartitions(rootSize uint64, medias []*BlockDevice) []string {
	results := []string{}

	bootFound := false
	bootSize := uint64(100) * (1000 * 1000) // 100MB recommend for 4-5 kernels
	swapFound := false
	swapSize := uint64(32) * (1000 * 1000) // 32MB recommend smallest for memory crunch times
	rootFound := false

	for _, curr := range medias {
		for _, ch := range curr.Children {
			clrFound := false
			label := ch.PartitionLabel

			if label != "" {
				log.Debug("validateAdvancedPartitions: Found partition %s with label %s", ch.Name, label)
			}

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
					if bootFound {
						warning := utils.Locale.Get("Found multiple %s partition labels", "CLR_BOOT")
						results = append(results, warning)
						log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
					} else {
						bootFound = true
						if ch.FsType != "vfat" {
							warning := utils.Locale.Get("%s must be %s", "CLR_BOOT", "vfat")
							results = append(results, warning)
							log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
						}
					}
					if ch.Size < bootSize {
						size, _ := HumanReadableSizeWithPrecision(bootSize, 1)
						warning := utils.Locale.Get("%s must be %s", "CLR_BOOT",
							fmt.Sprintf(">= %s", size))
						results = append(results, warning)
						log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
					}
				case "root":
					if rootFound {
						warning := utils.Locale.Get("Found multiple %s partition labels", "CLR_ROOT")
						results = append(results, warning)
						log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
					} else {
						rootFound = true
						if !(ch.FsType == "ext2" || ch.FsType == "ext3" ||
							ch.FsType == "ext4" || ch.FsType == "xfs") {
							warning := utils.Locale.Get("%s must be %s", "CLR_ROOT", "ext*|xfs")
							results = append(results, warning)
							log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
						}
					}
					if ch.Size < rootSize {
						size, _ := HumanReadableSizeWithPrecision(rootSize, 1)
						warning := utils.Locale.Get("%s must be %s", "CLR_ROOT",
							fmt.Sprintf(">= %s", size))
						results = append(results, warning)
						log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
					}
				case "swap":
					if swapFound {
						warning := utils.Locale.Get("Found multiple %s partition labels", "CLR_SWAP")
						results = append(results, warning)
						log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
					} else {
						swapFound = true
					}
					if ch.Size < swapSize {
						size, _ := HumanReadableSizeWithPrecision(swapSize, 1)
						warning := utils.Locale.Get("%s must be %s", "CLR_SWAP",
							fmt.Sprintf(">= %s", size))
						results = append(results, warning)
						log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
					}
				case "e":
					if strings.HasPrefix(strings.ToLower(ch.PartitionLabel), "clr_boot") {
						warning := utils.Locale.Get("Encryption of %s is not supported", "CLR_BOOT")
						results = append(results, warning)
						log.Warning("validateAdvancedPartitions: %s %+v", warning, ch)
					}
				}
			}
		}
	}

	if !bootFound {
		warning := utils.Locale.Get("Missing %s partition label", "CLR_BOOT")
		results = append(results, warning)
		log.Warning("validateAdvancedPartitions: %s", warning)
	}

	if !swapFound {
		warning := utils.Locale.Get("Missing %s partition label", "CLR_SWAP")
		results = append(results, warning)
		log.Warning("validateAdvancedPartitions: %s", warning)
	}

	if !rootFound {
		warning := utils.Locale.Get("Missing %s partition label", "CLR_ROOT")
		results = append(results, warning)
		log.Warning("validateAdvancedPartitions: %s", warning)
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
				case "e":
					encryptionFound = true
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

	for _, curr := range medias {
		for _, ch := range curr.Children {
			if strings.HasPrefix(ch.PartitionLabel, "CLR_") {
				var name string
				if ch.MountPoint != "" {
					name = ch.Name + ":" + ch.MountPoint
				} else {
					name = ch.Name + ":" + ch.FsType
				}
				if ch.Type == BlockDeviceTypeCrypt {
					name = name + "*"
				}

				results = append(results, name)
			}
		}
	}

	return results
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
				utils.Locale.Get(FormattingPartitionInfo, ch.FsType))

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

			part := fmt.Sprintf("%s: %s", partName, utils.Locale.Get(ReusingPartitionInfo))

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
func GetPlannedMediaChanges(targets map[string]InstallTarget, medias []*BlockDevice) []string {
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

	for _, target := range targets {
		if target.EraseDisk {
			results = append(results, target.Name+": "+utils.Locale.Get(DestructiveWarning))
		} else if target.DataLoss {
			results = append(results, target.Name+": "+utils.Locale.Get(DataLossWarning))
		} else if target.WholeDisk {
			results = append(results, target.Name+": "+utils.Locale.Get(SafeWholeWarning))
		} else {
			results = append(results, target.Name+": "+utils.Locale.Get(MediaToBeUsed))
		}

		for _, curr := range medias {
			if target.Name == curr.Name {
				if err := curr.WritePartitionTable(false, target.WholeDisk, &results); err != nil {
					results = append(results, FailedPartitionWarning)
				}
				if partChanges := getPlannedPartitionChanges(curr); len(partChanges) > 0 {
					results = append(results, partChanges...)
				}
			}
		}
	}

	return results
}
