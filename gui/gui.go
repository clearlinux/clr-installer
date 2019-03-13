// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// ContentViewRequired is defined to map to the Required contentview
	ContentViewRequired = true

	// ContentViewAdvanced is defined to map to the Advanced contentview
	ContentViewAdvanced = false

	// styleFile contains the style information of the installer
	styleFile = "style.css"
)

// Gui is the main gui data struct and holds data about the higher level data for this
// front end, it also implements the Frontend interface
type Gui struct {
	window        *Window
	model         *model.SystemInstall
	installReboot bool
}

// New creates a new Gui frontend instance
func New() *Gui {
	return &Gui{}
}

// MustRun is part of the Frontend interface implementation and tells the core that this
// frontend wants/must run.
func (gui *Gui) MustRun(args *args.Args) bool {
	if args.ForceTUI {
		return false
	}
	return gtk.InitCheck(nil) == nil
}

// Run is part of the Frontend interface implementation and is the gui frontend main entry point
func (gui *Gui) Run(md *model.SystemInstall, rootDir string, options args.Args) (bool, error) {
	gui.model = md
	gui.installReboot = false

	// Use dark theming if available to differentiate from other apps
	st, err := gtk.SettingsGetDefault()
	if err != nil {
		return false, err
	}
	if err := st.SetProperty("gtk-application-prefer-dark-theme", true); err != nil {
		return false, err
	}
	sc, err := gtk.CssProviderNew()
	if err != nil {
		return false, err
	}

	themeDir, err := utils.LookupThemeDir()
	if err != nil {
		return false, err
	}

	if err := sc.LoadFromPath(filepath.Join(themeDir, styleFile)); err != nil {
		return false, err
	}

	screen, err := gdk.ScreenGetDefault()
	if err != nil {
		return false, err
	}

	gtk.AddProviderForScreen(screen, sc, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	// Construct main window
	win, err := NewWindow(md, rootDir, options)
	if err != nil {
		return false, err
	}
	gui.window = win

	// Main loop
	gtk.Main()

	return gui.installReboot, nil
}
