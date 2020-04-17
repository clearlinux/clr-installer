// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

// BlockDevFilterFunc is a type for all filter functions
type BlockDevFilterFunc func(*BlockDevice) bool

// IsBlockDevAvailable is a function to test availability of a block device
func IsBlockDevAvailable(bd *BlockDevice) bool {
	if bd.IsAvailable() {
		return true
	}
	return false
}

// FilterBlockDevices is a filter function which runs zero or more filter_func on every BlockDevice in the slice
// and returns a filtered slice which satisfies them all
func FilterBlockDevices(bd []*BlockDevice, filterfunc ...BlockDevFilterFunc) []*BlockDevice {
	workingBDList := make([]*BlockDevice, 0)
	for _, bdevice := range bd {
		allFilterResult := true

		for _, filter := range filterfunc {
			if !filter(bdevice) {
				allFilterResult = false
				break
			}
		}

		if allFilterResult {
			workingBDList = append(workingBDList, bdevice)
		}
	}
	return workingBDList
}

// FindBlockDeviceDepthFirst runs the filterfunc in depth first manner and returns the first child
// which shows certain property
func FindBlockDeviceDepthFirst(bd *BlockDevice, filterfunc BlockDevFilterFunc) (bool, *BlockDevice) {
	var result *BlockDevice = nil
	var found bool = false

	if filterfunc(bd) {
		return true, bd
	}

	for _, child := range bd.Children {
		found, result = FindBlockDeviceDepthFirst(child, filterfunc)

		if found {
			break
		}
	}

	return found, result
}

// FindAllBlockDevices runs the filterfunc and returns a list of all blockdevices which
// satisfy the condition
func FindAllBlockDevices(bd *BlockDevice, filterfunc BlockDevFilterFunc) []*BlockDevice {
	var result []*BlockDevice = []*BlockDevice{}

	if filterfunc(bd) {
		result = append(result, bd)
	}

	for _, child := range bd.Children {
		result = append(result, FindAllBlockDevices(child, filterfunc)...)
	}

	return result
}
