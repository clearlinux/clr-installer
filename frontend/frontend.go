// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package frontend

import (
	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/model"
)

// Frontend is the common interface for the frontend entry point
type Frontend interface {
	// MustRun is the method where the frontend implementation tells the
	// core code that this frontend wants to run
	MustRun(args *args.Args) bool

	// Run is the actual entry point
	Run(md *model.SystemInstall, rootDir string, args args.Args) (bool, error)
}
