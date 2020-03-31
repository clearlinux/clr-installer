// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/utils"
)

// A BlockDevice describes a block device and its partitions
type BlockDevice struct {
	Name            string             // device name
	MappedName      string             // mapped device name
	Model           string             // device model
	MajorMinor      string             // major:minor device number
	PtType          string             // partition table type
	FsType          string             // filesystem type
	UUID            string             // filesystem uuid
	Serial          string             // device serial number
	MountPoint      string             // where the device is mounted
	Label           string             // label for the filesystem; set with mkfs
	PartitionLabel  string             // label for the partition; set with cgdisk/parted/gparted
	Size            uint64             // size of the device
	Type            BlockDeviceType    // device type
	State           BlockDeviceState   // device state (running, live etc)
	ReadOnly        bool               // read-only device
	RemovableDevice bool               // removable device
	Children        []*BlockDevice     // children devices/partitions
	UserDefined     bool               // was this value set by user?
	MakePartition   bool               // Do we need to make a new partition?
	FormatPartition bool               // Do we need to format the partition?
	LabeledAdvanced bool               // Does this partition have a valid Advanced Label?
	Options         string             // arbitrary mkfs.* options
	available       bool               // was it mounted the moment we loaded?
	partition       uint64             // Assigned partition for media - can't set until after mkpart
	PartTable       []*PartedPartition // Existing Disk partition table from parted
}

// BlockDeviceState is the representation of a block device state (live, running, etc)
type BlockDeviceState int

// BlockDeviceType is the representation of a block device type (disk, part, rom, etc)
type BlockDeviceType int

const (
	// BlockDeviceTypeDisk identifies a BlockDevice as a disk
	BlockDeviceTypeDisk = iota

	// BlockDeviceTypePart identifies a BlockDevice as a partition
	BlockDeviceTypePart

	// BlockDeviceTypeRom identifies a BlockDevice as a rom
	BlockDeviceTypeRom

	// BlockDeviceTypeLVM2Group identifies a BlockDevice as a lvm2 group
	BlockDeviceTypeLVM2Group

	// BlockDeviceTypeLVM2Volume identifies a BlockDevice as a lvm2 volume
	BlockDeviceTypeLVM2Volume

	// BlockDeviceTypeRAID0 identifies a BlockDevice as a RAID0
	BlockDeviceTypeRAID0

	// BlockDeviceTypeRAID1 identifies a BlockDevice as a RAID1
	BlockDeviceTypeRAID1

	// BlockDeviceTypeRAID4 identifies a BlockDevice as a RAID4
	BlockDeviceTypeRAID4

	// BlockDeviceTypeRAID5 identifies a BlockDevice as a RAID5
	BlockDeviceTypeRAID5

	// BlockDeviceTypeRAID6 identifies a BlockDevice as a RAID6
	BlockDeviceTypeRAID6

	// BlockDeviceTypeRAID10 identifies a BlockDevice as a RAID10
	BlockDeviceTypeRAID10

	// BlockDeviceTypeCrypt identifies a BlockDevice as an encrypted partition (created with cryptsetup)
	BlockDeviceTypeCrypt

	// BlockDeviceTypeLoop identifies a BlockDevice as a loop device (created with losetup)
	BlockDeviceTypeLoop

	// BlockDeviceTypeUnknown identifies a BlockDevice as unknown
	BlockDeviceTypeUnknown

	// BlockDeviceStateUnknown identifies a BlockDevice in a unknown state
	BlockDeviceStateUnknown = iota

	// BlockDeviceStateRunning identifies a BlockDevice as running
	BlockDeviceStateRunning

	// BlockDeviceStateLive identifies a BlockDevice as live
	BlockDeviceStateLive

	// BlockDeviceStateConnected identifies a BlockDevice as Connected
	BlockDeviceStateConnected

	// MinimumPartitionSize is smallest size for any partition
	MinimumPartitionSize = 1048576

	// SafeWholeWarning specifies the warning message for whole disk partition
	SafeWholeWarning = "Selected media will be partitioned."

	// SafePartialWarning specifies the warning message for partial disk partition
	SafePartialWarning = "Selected media will have partitions added."

	// MediaToBeUsed identified a disk which will be used during the installation
	MediaToBeUsed = "Selected media will be used for installation."

	// LegacyModeWarning specifies the warning message we are using legacy bios mode
	LegacyModeWarning = "WARNING: Booting set for legacy BIOS mode."

	// LegacyNoBootWarning specifies the warning message we are using legacy bios mode
	// and there is not /boot partition; so will NOT be able to boot in EFI mode.
	LegacyNoBootWarning = "WARNING: system can not boot EFI mode due to no /boot partition."

	// PartitioningWarning specifies the warning message for writing partition table
	PartitioningWarning = "WARNING: New Partition table will be created."

	// LogicalVolumeWarning specifies the warning message when removing a logical volume
	LogicalVolumeWarning = "WARNING: Logical Volume will be removed."

	// DestructiveWarning specifies the warning message for destructive installation
	DestructiveWarning = "WARNING: Selected media will be erased."

	// DataLossWarning specifies the warning message for data loss installation
	DataLossWarning = "WARNING: Selected media will have data loss."

	// RemoveParitionWarning specifies the warning message for removing a media partition
	RemoveParitionWarning = "WARNING: partition will be removed."

	// AddPartitionInfo specifies the warning message for removing a media partition
	AddPartitionInfo = "Add new partition."

	// FailedPartitionWarning specifies the warning message when we can not find partitions
	FailedPartitionWarning = "WARNING: Failed to detected partition information."

	// FormatPartitionInfo specifies the warning message for formatting a media partition
	FormatPartitionInfo = "Format partition as %s."

	// UsePartitionInfo specifies the warning message for reusing a media partition
	UsePartitionInfo = "Use existing partition."

	// ConfirmInstallation specifies the installation warning title
	ConfirmInstallation = "Confirm Installation"

	// EncryptionPassphrase specifies the title for encryption passphrase dialog
	EncryptionPassphrase = "Encryption Passphrase"

	// PassphraseMessage specifies the text for encryption passphrase dialog
	PassphraseMessage = "Encryption requires a Passphrase"
)

var (
	avBlockDevices      []*BlockDevice
	lsblkBinary         = "lsblk"
	devNameSuffixExp    = regexp.MustCompile(`([0-9]*)$`)
	blockDeviceStateMap = map[BlockDeviceState]string{
		BlockDeviceStateRunning:   "running",
		BlockDeviceStateLive:      "live",
		BlockDeviceStateConnected: "Connected",
		BlockDeviceStateUnknown:   "",
	}
	blockDeviceTypeMap = map[BlockDeviceType]string{
		BlockDeviceTypeDisk:       "disk",
		BlockDeviceTypePart:       "part",
		BlockDeviceTypeCrypt:      "crypt",
		BlockDeviceTypeLoop:       "loop",
		BlockDeviceTypeRom:        "rom",
		BlockDeviceTypeLVM2Group:  "LVM2_member",
		BlockDeviceTypeLVM2Volume: "lvm",
		BlockDeviceTypeRAID0:      "raid0",
		BlockDeviceTypeRAID1:      "raid1",
		BlockDeviceTypeRAID4:      "raid4",
		BlockDeviceTypeRAID5:      "raid5",
		BlockDeviceTypeRAID6:      "raid6",
		BlockDeviceTypeRAID10:     "raid10",
		BlockDeviceTypeUnknown:    "",
	}
	aliasPrefixTable = map[string]string{
		"/dev/loop":   "p",
		"/dev/nvme":   "p",
		"/dev/mmcblk": "p",
	}

	bootSize = uint64(150 * (1000 * 1000))
	swapSize = uint64(256 * (1000 * 1000))
)

func getAliasSuffix(file string) string {
	for k, v := range aliasPrefixTable {
		if strings.HasPrefix(file, k) {
			return v
		}
	}

	return ""
}

// ExpandName expands variables in the Name attribute applying the values in the
// alias map
func (bd *BlockDevice) ExpandName(alias map[string]string) {
	tmap := map[string]string{}

	bd.Name = utils.ExpandVariables(alias, bd.Name)

	for k, v := range alias {
		tmap[k] = fmt.Sprintf("%s%s", v, getAliasSuffix(filepath.Join("/dev", v)))
	}

	for _, child := range bd.Children {
		child.Name = utils.ExpandVariables(tmap, child.Name)
	}
}

// GetNewPartitionName returns the name with the new partition number
func (bd *BlockDevice) GetNewPartitionName(partition uint64) string {
	// Replace the last set of digits with the current partition number
	return devNameSuffixExp.ReplaceAllString(bd.getBasePartitionName(), fmt.Sprintf("%d", partition))
}

// SetPartitionNumber is set when we add a new partition to a disk
// which stores the newly allocated partition number, and then corrects
// the devices partition name
func (bd *BlockDevice) SetPartitionNumber(partition uint64) {
	bd.partition = partition
}

// GetPartitionNumber get the partition number from either the set partition
// number value, or based on the partition name if the number is 0
func (bd *BlockDevice) GetPartitionNumber() uint64 {
	if bd.partition > 0 {
		return bd.partition
	}

	part := devNameSuffixExp.FindString(bd.Name)
	if len(part) > 0 {
		u, err := strconv.ParseUint(part, 10, 64)
		if err == nil {
			return u
		}
	}

	return 0
}

// GetDeviceFile formats the block device's file path
func (bd BlockDevice) GetDeviceFile() string {
	return filepath.Join("/dev/", bd.Name)
}

// GetMappedDeviceFile formats the block device's file path
// using the mapped device name
func (bd BlockDevice) GetMappedDeviceFile() string {

	if bd.MappedName != "" {
		return filepath.Join("/dev/", bd.MappedName)
	}

	return filepath.Join("/dev/", bd.Name)
}

// GetDeviceID returns an identifier for the block device
// First trying, label, then UUID, then finally the raw device
// String is suitable for the /etc/fstab
func (bd BlockDevice) GetDeviceID() string {
	if bd.Label != "" {
		return "LABEL=" + bd.Label
	}

	if bd.UUID != "" {
		return "UUID=" + bd.UUID
	}

	return bd.GetDeviceFile()
}

func (bt BlockDeviceType) String() string {
	return blockDeviceTypeMap[bt]
}

func parseBlockDeviceType(bdt string) (BlockDeviceType, error) {
	for k, v := range blockDeviceTypeMap {
		if v == bdt {
			return k, nil
		}
	}

	return BlockDeviceTypeUnknown, errors.Errorf("Unknown block device type: %s", bdt)
}

func (bs BlockDeviceState) String() string {
	return blockDeviceStateMap[bs]
}

func parseBlockDeviceState(bds string) (BlockDeviceState, error) {
	for k, v := range blockDeviceStateMap {
		if v == bds {
			return k, nil
		}
	}

	return BlockDeviceStateUnknown, errors.Errorf("Unrecognized block device state: %s", bds)
}

func (bd *BlockDevice) findFree(size uint64) *PartedPartition {
	var freePart *PartedPartition

	for _, part := range bd.PartTable {
		if part.Number == 0 && part.FileSystem == "free" {
			if part.Size >= size {
				freePart = part.Clone()
				break
			}
		}
	}

	return freePart
}

// Clone creates a copies a BlockDevice and its children
func (bd *BlockDevice) Clone() *BlockDevice {
	clone := &BlockDevice{
		Name:            bd.Name,
		MappedName:      bd.MappedName,
		Model:           bd.Model,
		MajorMinor:      bd.MajorMinor,
		FsType:          bd.FsType,
		UUID:            bd.UUID,
		Serial:          bd.Serial,
		MountPoint:      bd.MountPoint,
		Label:           bd.Label,
		PartitionLabel:  bd.PartitionLabel,
		Size:            bd.Size,
		Type:            bd.Type,
		State:           bd.State,
		ReadOnly:        bd.ReadOnly,
		RemovableDevice: bd.RemovableDevice,
		UserDefined:     bd.UserDefined,
		MakePartition:   bd.MakePartition,
		FormatPartition: bd.FormatPartition,
		LabeledAdvanced: bd.LabeledAdvanced,
		available:       bd.available,
		partition:       bd.partition,
		PartTable:       bd.PartTable,
	}

	clone.Children = []*BlockDevice{}

	for _, curr := range bd.Children {
		cc := curr.Clone()

		clone.Children = append(clone.Children, cc)
	}

	return clone
}

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (bd *BlockDevice) IsUserDefined() bool {
	return bd.UserDefined
}

// IsAvailable returns true if the media is not a installer media, returns false otherwise
func (bd *BlockDevice) IsAvailable() bool {
	return bd.available
}

// FsTypeNotSwap returns true if the file system type is not swap
func (bd *BlockDevice) FsTypeNotSwap() bool {
	return bd.FsType != "swap"
}

// DeviceHasSwap returns true if the block device has a swap partition
func (bd *BlockDevice) DeviceHasSwap() bool {
	hasSwap := false

	for _, part := range bd.Children {
		if part.FsType == "swap" {
			hasSwap = true
		}
	}
	return hasSwap
}

// RemoveChild removes a partition from disk block device
func (bd *BlockDevice) RemoveChild(child *BlockDevice) {
	copyBd := bd.Clone()

	bd.Children = nil

	for _, curr := range copyBd.Children {
		if curr.Name == child.Name {
			continue
		}

		curr.Name = ""
		bd.AddChild(curr)
	}
}

func (bd *BlockDevice) getBasePartitionName() string {
	partPrefix := ""

	if bd.Type == BlockDeviceTypeLoop ||
		strings.Contains(bd.Name, "nvme") ||
		strings.Contains(bd.Name, "mmcblk") {
		partPrefix = "p"
	}

	return fmt.Sprintf("%s%s", bd.Name, partPrefix)
}

// AddChild adds a partition to a disk block device
func (bd *BlockDevice) AddChild(child *BlockDevice) {
	if bd.Children == nil {
		bd.Children = []*BlockDevice{}
	}

	bd.Children = append(bd.Children, child)

	if child.Name == "" {
		if child.partition < 1 {
			child.Name = fmt.Sprintf("%s?", bd.getBasePartitionName())
		} else {
			child.Name = fmt.Sprintf("%s%d", bd.getBasePartitionName(), child.partition)
		}
	}
	log.Debug("AddChild: child.Name is %q", child.Name)
}

// HumanReadableSizeWithUnitAndPrecision converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced unit and precision
func (bd *BlockDevice) HumanReadableSizeWithUnitAndPrecision(unit string, precision int) (string, error) {
	return HumanReadableSizeWithUnitAndPrecision(bd.Size, unit, precision)
}

// HumanReadableSizeWithPrecision converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced precision
func (bd *BlockDevice) HumanReadableSizeWithPrecision(precision int) (string, error) {
	return bd.HumanReadableSizeWithUnitAndPrecision("", precision)
}

// HumanReadableSizeWithUnit converts the size representation in bytes to the
// closest human readable format i.e 10M, 1G, 2T etc with a forced unit
func (bd *BlockDevice) HumanReadableSizeWithUnit(unit string) (string, error) {
	return bd.HumanReadableSizeWithUnitAndPrecision(unit, -1)
}

// HumanReadableSize converts the size representation in bytes to the closest
// human readable format i.e 10M, 1G, 2T etc
func (bd *BlockDevice) HumanReadableSize() (string, error) {
	return bd.HumanReadableSizeWithUnitAndPrecision("", -1)
}

func listBlockDevices(userDefined []*BlockDevice) ([]*BlockDevice, error) {
	w := bytes.NewBuffer(nil)

	args := []string{"partprobe", "-s"}
	err := cmd.RunAndLog(args...)
	if err != nil {
		log.Warning("PartProbe has non-zero exit status: %s", err)
	}

	// Modified devices must be synchronized with udev before calling lsblk
	args = []string{"udevadm", "settle", "--timeout", "10"}
	err = cmd.RunAndLog(args...)
	if err != nil {
		log.Warning("udevadm has non-zero exit status: %s", err)
	}

	// Exclude memory(1), floppy(2), and SCSI CDROM(11) devices
	err = cmd.Run(w, lsblkBinary, "--exclude", "1,2,11", "-J", "-b", "-O")
	if err != nil {
		return nil, fmt.Errorf("%s", w.String())
	}

	bds, err := parseBlockDevicesDescriptor(w.Bytes())
	if err != nil {
		return nil, err
	}

	for _, bd := range bds {
		// Read the partition table for the device
		partTable := bd.getPartitionTable()
		bd.setPartitionTable(partTable)
	}

	if userDefined == nil || len(userDefined) == 0 {
		return bds, nil
	}

	merged := []*BlockDevice{}
	for _, loaded := range bds {
		added := false

		for _, udef := range userDefined {
			if !loaded.Equals(udef) {
				continue
			}

			merged = append(merged, udef)
			added = true
			break
		}

		if !added {
			merged = append(merged, loaded)
		}
	}

	return merged, nil
}

// UpdateBlockDevices updates the Label and UUID information only
// for existing available block devices
func UpdateBlockDevices(medias []*BlockDevice) error {

	bds, err := listBlockDevices(nil)
	if err != nil {
		return err
	}

	// Loop though all used media devices and update
	// the labels and uuid
	for _, media := range medias {
		updateBlockDevices(media, bds)
	}

	return nil
}

func updateBlockDevices(toBeUpdated *BlockDevice, updates []*BlockDevice) {
	for _, update := range updates {
		if toBeUpdated.Name == update.Name {
			if toBeUpdated.Children == nil {
				toBeUpdated.Label = update.Label
				toBeUpdated.UUID = update.UUID
				return
			}

			for _, child := range toBeUpdated.Children {
				updateBlockDevices(child, update.Children)
			}
		}
	}
}

// RescanBlockDevices clears current list available block devices and rescans
func RescanBlockDevices(userDefined []*BlockDevice) ([]*BlockDevice, error) {
	avBlockDevices = nil

	return ListAvailableBlockDevices(userDefined)
}

// ListAvailableBlockDevices Lists only available block devices
// where available means block devices not mounted or not in use by the host system
// userDefined will be inserted in the resulting list rather the loaded ones
func ListAvailableBlockDevices(userDefined []*BlockDevice) ([]*BlockDevice, error) {
	if avBlockDevices != nil {
		return avBlockDevices, nil
	}

	bds, err := listBlockDevices(userDefined)
	if err != nil {
		return nil, err
	}

	result := []*BlockDevice{}
	for _, curr := range bds {
		if !curr.IsAvailable() {
			continue
		}

		result = append(result, curr)
	}

	avBlockDevices = result
	return result, nil
}

// ListBlockDevices Lists all block devices
// userDefined will be inserted in the resulting list reather the loaded ones
func ListBlockDevices(userDefined []*BlockDevice) ([]*BlockDevice, error) {
	return listBlockDevices(userDefined)
}

// Equals compares two BlockDevice instances
func (bd *BlockDevice) Equals(cmp *BlockDevice) bool {
	if cmp == nil {
		return false
	}

	return bd.Name == cmp.Name && bd.Model == cmp.Model && bd.MajorMinor == cmp.MajorMinor
}

func isBlockDeviceAvailable(blockDevices []*BlockDevice) bool {
	available := true

	for _, bd := range blockDevices {
		// We ignore devices with any mount partition
		if bd.MountPoint != "" {
			available = false
			break
		}
		// We ignore devices if any partition has a CLR_ISO label
		if strings.Contains(bd.Label, "CLR_ISO") {
			available = false
			break
		}

		if bd.Children != nil {
			if available = isBlockDeviceAvailable(bd.Children); !available {
				break
			}
		}
	}

	return available
}

func parseBlockDevicesDescriptor(data []byte) ([]*BlockDevice, error) {
	root := struct {
		BlockDevices []*BlockDevice `json:"blockdevices"`
	}{}

	err := json.Unmarshal(data, &root)
	if err != nil {
		return nil, errors.Wrap(err)
	}

	for _, bd := range root.BlockDevices {
		bd.available = isBlockDeviceAvailable(bd.Children)

		// We ignore devices if the filesystem is squashfs
		if strings.Contains(bd.FsType, "squashfs") {
			bd.available = false
		}
	}

	return root.BlockDevices, nil
}

// PartProbe runs partprobe against the block device's file
func (bd *BlockDevice) PartProbe() error {
	args := []string{
		"partprobe",
		bd.GetDeviceFile(),
	}

	if err := cmd.RunAndLog(args...); err != nil {
		log.Warning("PartProbe has non-zero exit status: %s", err)
	}

	return nil
}

// DiskSize given a BlockDevice sum's up its size and children sizes
func (bd *BlockDevice) DiskSize() (uint64, error) {
	diskSize := bd.Size

	// otherwise, sum the children partitions to determine disk size
	var childSize uint64

	for _, ch := range bd.Children {
		if len(ch.Children) > 0 {
			size, err := ch.DiskSize()
			if err != nil {
				return 0, err
			}
			childSize += size
		} else {
			childSize += ch.Size
		}
	}

	if diskSize > 0 && childSize > diskSize {
		return 0, errors.Errorf("%s: Partition Sizes %d larger than Device Size: %d",
			bd.Name, childSize, diskSize)
	}

	// Return the Entire disk size if present
	if diskSize > 0 {
		return diskSize, nil
	}

	return childSize, nil
}

func (bd *BlockDevice) logDetails() {
	log.Debug("%s: fsType=%s, mount=%s, size=%d, type=%s", bd.Name, bd.FsType, bd.MountPoint, bd.Size, bd.Type)
}

// IsAdvancedConfiguration checks all partition to see if advanced labeling was enabled
func (bd *BlockDevice) IsAdvancedConfiguration() bool {
	advanced := bd.LabeledAdvanced

	for _, ch := range bd.Children {
		if len(ch.Children) > 0 {
			advanced = advanced || ch.IsAdvancedConfiguration()
		} else {
			advanced = advanced || ch.LabeledAdvanced
		}
	}

	return advanced
}
