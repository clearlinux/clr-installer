// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

// Test program to inject random bytes into image files
// to help validate ISO checksums are working.

// Build: go build damage_file.go
// Usage: damage_file my.iso

package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path"
)

func main() {
	prog := path.Base(os.Args[0])

	var damageFile string
	var offset, count int64

	fs := flag.NewFlagSet(prog, flag.ExitOnError)

	fs.StringVar(&damageFile, "file", "", "File to be damaged")
	fs.Int64Var(&offset, "offset", -4194304, "Byte offset where to start damage")
	fs.Int64Var(&count, "count", 1024, "Number of bytes to damage")

	var Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", prog)
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Printf("Error: Failed to parse: %+v\n", os.Args[1:])
		os.Exit(1)
	}

	if len(damageFile) < 1 {
		Usage()
		os.Exit(1)
	}

	srcFile, err := os.OpenFile(damageFile, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("No such file: %s\n", damageFile)
		}
		fmt.Printf("Failed to open '%s': %s\n", damageFile, err)
		os.Exit(1)
	}
	defer srcFile.Close()

	// 0 means relative to the origin of the file,
	// 1 means relative to the current offset, and
	// 2 means relative to the end.
	whence := 0
	if offset < 0 {
		whence = 2
	}

	// Save the EOF
	saveEOF, err := srcFile.Seek(0, 2)
	if err != nil {
		fmt.Printf("Failed to seek '%s': %s\n", damageFile, err)
		os.Exit(1)
	}

	newPosition, err := srcFile.Seek(offset, whence)
	if err != nil {
		fmt.Printf("Failed to seek '%s': %s\n", damageFile, err)
		os.Exit(1)
	}

	fmt.Printf("Injecting %d zero bytes at position %d for %s\n", count, newPosition, damageFile)

	damage := make([]byte, count)
	if _, err := rand.Read(damage); err != nil {
		fmt.Printf("Failed to generate random data: %s\n", err)
		os.Exit(1)
	}

	if cnt, err := srcFile.Write(damage); err != nil {
		fmt.Printf("Failed to write into '%s' (%d): %s\n", damageFile, cnt, err)
		os.Exit(1)
	} else {
		fmt.Printf(" Injected %d zero bytes at position %d for %s\n", cnt, newPosition, damageFile)
	}

	// Seek to the end of the file
	if _, err := srcFile.Seek(saveEOF, 2); err != nil {
		fmt.Printf("Failed to seek to end-of-file '%s': %s\n", damageFile, err)
		os.Exit(1)
	}
}
