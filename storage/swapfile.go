// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"os"
	"path/filepath"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
)

const (
	// SwapfileName is the default name of the swap file to create
	SwapfileName = "/var/swapfile"
)

// CreateSwapFile is responsible for generating a valid swapfile
// on the installation target
func CreateSwapFile(rootDir string, sizeString string) error {
	size, err := ParseVolumeSize(sizeString)
	if err != nil {
		return err
	}

	// size is in bytes, but we will only create swapfile in MB increments
	swapFileSize := size / (1024 * 1024)

	swapFile := filepath.Join(rootDir, SwapfileName)

	if err := allocateSwapFile(swapFile, swapFileSize); err != nil {
		return err
	}
	args := []string{
		"mkswap",
		swapFile,
	}

	if err := cmd.RunAndLog(args...); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func allocateSwapFile(swapFile string, blockCount uint64) error {
	// The block size is always in MB
	block := make([]byte, 1024*1024)

	// The permissions on the swap file should always be 0600
	f, err := os.OpenFile(swapFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	// Write bytes to file
	bytesWritten := 0

	var i uint64
	for i = 0; i < blockCount; i++ {
		byteCount, err := f.Write(block)
		if err != nil {
			return err
		}
		bytesWritten += byteCount
	}

	log.Debug("allocateSwapFile: Wrote %d bytes.", bytesWritten)

	return nil
}
