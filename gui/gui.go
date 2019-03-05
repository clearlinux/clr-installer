// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/model"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

const (
	// ContentViewRequired is defined to map to the Required contentview
	ContentViewRequired = true

	// ContentViewAdvanced is defined to map to the Advanced contentview
	ContentViewAdvanced = false
)

// Gui is the main tui data struct and holds data about the higher level data for this
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
	sc, _ := gtk.CssProviderNew()

	customCSS := `
.scroller-special {
	background-image: none;
	background-color: #414449;
	color: #71C2E3;
}
window {
	background-color: #414449;
	border: none;
}
.installer-welcome-banner {
	background-color: transparent;
	border: none;
	background-image: url('/usr/share/backgrounds/clearlinux/color_logo_wire_2560x1440.png');
}

.invisible-titlebar {
	background-image: none;
	background-color: transparent;
	border: none;
}

.installer-header-box {
	background-color: #272B2E;
}

.installer-header-box image {
	color: #71C2E3;
}

.installer-switcher button {
	background-image: none;
	border-image: none;
	border: none;
	background-color: #272B2E;
	border: 1px solid #272B2E;
	border-radius: 0px;
	box-shadow: none;
}

.installer-switcher button:checked {
	background-image: none;
	border-image: none;
	border: none;
	background-color: #414449;
	border: 1px solid #414449;
	border-radius: 0px;
	box-shadow: none;
}

.installer-summary-widget .configured-value {
	font-size: 90%;
	color: white;
}

.nav-button {
	background-color: #5ECBF2;
	color: black;
	border-radius: 1px;
}
.nav-button:disabled {
	color: lighter(grey);
	background-color: lighter(#5ECBF2);
}
`

	if err := sc.LoadFromData(customCSS); err != nil {
		return false, err
	}
	screen, _ := gdk.ScreenGetDefault()
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
