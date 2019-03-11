// +build guiBuild

// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/clearlinux/clr-installer/frontend"
	"github.com/clearlinux/clr-installer/gui"
	"github.com/clearlinux/clr-installer/massinstall"
	"github.com/clearlinux/clr-installer/tui"
)

// The list of possible frontends to run for GUI
func initFrontendList() {
	frontEndImpls = []frontend.Frontend{
		massinstall.New(),
		gui.New(),
		tui.New(),
	}
}
