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
