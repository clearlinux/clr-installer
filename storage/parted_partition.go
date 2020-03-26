// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

// PartedPartition hold partition information
// Number 0 and FileSystem "free" are free spaces
type PartedPartition struct {
	Number     uint64 // partition number 0 indicates free space
	Start      uint64 // starting byte location
	End        uint64 // ending byte location
	Size       uint64 // size in bytes
	FileSystem string // file system Type
	Name       string // partition name
	Flags      string // flags for partition
}

// Clone creates a copies a PartedPartition
func (part *PartedPartition) Clone() *PartedPartition {
	clone := &PartedPartition{
		Number:     part.Number,
		Start:      part.Start,
		End:        part.End,
		Size:       part.Size,
		FileSystem: part.FileSystem,
		Name:       part.Name,
		Flags:      part.Flags,
	}

	return clone
}

// AddBootStandardPartition will add to disk a new standard Boot partition
func AddBootStandardPartition(disk *BlockDevice) uint64 {
	freePart := disk.findFree(bootSize)
	disk.AddFromFreePartition(freePart, &BlockDevice{
		Size:            bootSize,
		Type:            BlockDeviceTypePart,
		FsType:          "vfat",
		MountPoint:      "/boot",
		Label:           "boot",
		UserDefined:     true,
		MakePartition:   true,
		FormatPartition: true,
	})

	return bootSize
}

// AddSwapStandardPartition will add to disk a new standard Swap partition
func AddSwapStandardPartition(disk *BlockDevice) uint64 {
	freePart := disk.findFree(swapSize)
	disk.AddFromFreePartition(freePart, &BlockDevice{
		Size:            swapSize,
		Type:            BlockDeviceTypePart,
		FsType:          "swap",
		Label:           "swap",
		UserDefined:     true,
		MakePartition:   true,
		FormatPartition: true,
	})

	return swapSize
}

// AddRootStandardPartition will add to disk a new standard Root partition
func AddRootStandardPartition(disk *BlockDevice, rootSize uint64) {
	freePart := disk.findFree(rootSize)
	disk.AddFromFreePartition(freePart, &BlockDevice{
		Size:            rootSize,
		Type:            BlockDeviceTypePart,
		FsType:          "ext4",
		MountPoint:      "/",
		Label:           "root",
		UserDefined:     true,
		MakePartition:   true,
		FormatPartition: true,
	})
}

// NewStandardPartitions will add to disk a new set of partitions representing a
// default set of partitions required for an installation
func NewStandardPartitions(disk *BlockDevice) {
	disk.Children = nil
	newFreePart := &PartedPartition{
		Number:     0,
		Start:      0,
		End:        disk.Size,
		Size:       disk.Size,
		FileSystem: "free",
	}
	disk.PartTable = nil
	disk.PartTable = append(disk.PartTable, newFreePart)

	rootSize := uint64(disk.Size - bootSize - swapSize)

	freePart := disk.findFree(bootSize)
	disk.AddFromFreePartition(freePart, &BlockDevice{
		Size:            bootSize,
		Type:            BlockDeviceTypePart,
		FsType:          "vfat",
		MountPoint:      "/boot",
		Label:           "boot",
		UserDefined:     true,
		MakePartition:   true,
		FormatPartition: true,
	})

	freePart = disk.findFree(swapSize)
	disk.AddFromFreePartition(freePart, &BlockDevice{
		Size:            swapSize,
		Type:            BlockDeviceTypePart,
		FsType:          "swap",
		Label:           "swap",
		UserDefined:     true,
		MakePartition:   true,
		FormatPartition: true,
	})

	freePart = disk.findFree(rootSize)
	disk.AddFromFreePartition(freePart, &BlockDevice{
		Size:            rootSize,
		Type:            BlockDeviceTypePart,
		FsType:          "ext4",
		MountPoint:      "/",
		Label:           "root",
		UserDefined:     true,
		MakePartition:   true,
		FormatPartition: true,
	})
}
