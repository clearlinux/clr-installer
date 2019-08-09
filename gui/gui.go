// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"fmt"
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
	window *Window // TODO: Technically there need NOT be a separate Window struct. Its contents can be in this Gui struct.
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

	// Can graphics be initialized?
	if gtk.InitCheck(nil) == nil {
		return true
	} else if args.ForceGUI {
		fmt.Println("Error: Could not initialize graphics.")
		return false
	} else {
		return false
	}

}

// Run is part of the Frontend interface implementation and is the gui frontend main entry point
func (gui *Gui) Run(md *model.SystemInstall, rootDir string, options args.Args) (bool, error) {

	// When using the Interactive Installer we always want to copy network
	// configurations to the target system
	md.CopyNetwork = options.CopyNetwork

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

	// Construct window
	gui.window, err = NewWindow(md, rootDir, options)
	if err != nil {
		// NOTE: Error popup dialog i.e Panic() should not be called for crashes during initial window creation
		// as gtk.Main() is not running at this point. Such errors are only logged.
		return false, err
	}

	// Configure the Gnome proxy function
	SetupGnomeProxy()

	// Main loop
	gtk.Main()

	return false, nil
}
