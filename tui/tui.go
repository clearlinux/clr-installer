// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/syscheck"
	"github.com/clearlinux/clr-installer/utils"

	"github.com/VladimirMarkelov/clui"
	"github.com/nsf/termbox-go"
)

// Tui is the main tui data struct and holds data about the higher level data for this
// front end, it also implements the Frontend interface
type Tui struct {
	pages         []Page
	currPage      Page
	prevPage      Page
	model         *model.SystemInstall
	options       args.Args
	rootDir       string
	paniced       chan error
	installReboot bool
}

var (
	// errorLabelBg is a custom theme element, it has the error label background color definition
	errorLabelBg termbox.Attribute

	// errorLabelFg is a custom theme element, it has the error label foreground color definition
	errorLabelFg termbox.Attribute

	// infoLabelBg is a custom theme element, it has the info label background color definition
	infoLabelBg termbox.Attribute

	// infoLabelFg is a custom theme element, it has the info label foreground color definition
	infoLabelFg termbox.Attribute
)

// New creates a new Tui frontend instance
func New() *Tui {
	return &Tui{pages: []Page{}}
}

// MustRun is part of the Frontend interface implementation and tells the core that this
// frontend wants/must run.
func (tui *Tui) MustRun(args *args.Args) bool {
	if args.ForceGUI {
		msg := "Incompatible flag '--gui' for the text-based installer"
		fmt.Println(msg)
		log.Error(msg)
		return false
	}
	return true
}

// Run is part of the Frontend interface implementation and is the tui frontend main entry point
func (tui *Tui) Run(md *model.SystemInstall, rootDir string, options args.Args) (bool, error) {
	if err := md.InteractiveOptionsValid(); err != nil {
		fmt.Println(err)
		log.Error(err.Error())
		return false, nil
	}

	// First disable console messages
	err := cmd.RunAndLog("dmesg", "--console-off")
	if err != nil {
		log.Warning("Failed to disable dmesg on console: %v", err)
	}
	// Defer enabling console messages
	defer func() {
		err := cmd.RunAndLog("dmesg", "--console-on")
		if err != nil {
			log.Warning("Failed to enable dmesg on console: %v", err)
		}
	}()

	clui.InitLibrary()
	defer clui.DeinitLibrary()

	tui.model = md
	tui.options = options
	themeDir, err := utils.LookupThemeDir()
	if err != nil {
		return false, err
	}

	// When using the Interactive Installer we always want to copy network
	// configurations to the target system
	tui.model.CopyNetwork = options.CopyNetwork

	// When using the Interactive Installer we want to copy configurations
	// from /etc/swupd by default to the target system
	if !options.CopySwupdSet {
		tui.model.CopySwupd = true
	}

	clui.SetThemePath(themeDir)

	themeName := "clr-installer"
	if options.HighContrast {
		themeName = "high-contrast"
	}
	if !clui.SetCurrentTheme(themeName) {
		log.Warning("Could not set theme: %s", themeName)
	}

	errorLabelBg = clui.RealColor(clui.ColorDefault, "ErrorLabel", "Back")
	errorLabelFg = clui.RealColor(clui.ColorDefault, "ErrorLabel", "Text")
	infoLabelBg = clui.RealColor(clui.ColorDefault, "InfoLabel", "Back")
	infoLabelFg = clui.RealColor(clui.ColorDefault, "InfoLabel", "Text")

	tui.rootDir = rootDir
	tui.paniced = make(chan error, 1)

	menus := []struct {
		desc string
		fc   func(*Tui) (Page, error)
	}{
		{"timezone", newTimezonePage},
		{"language", newLanguagePage},
		{"keyboard", newKeyboardPage},
		{"media config", newMediaConfigPage},
		{"network", newNetworkPage},
		{"proxy", newProxyPage},
		{"network validate", newNetworkValidatePage},
		{"network interface", newNetworkInterfacePage},
		{"main menu", newMenuPage},
		{"bundle selection", newBundlePage},
		{"add manager", newUserManagerPage},
		{"add user", newUseraddPage},
		{"hostname", newHostnamePage},
		{"telemetry enabling", newTelemetryPage},
		{"kernel cmdline", newKernelCMDLine},
		{"kernel selection", newKernelPage},
		{"install", newInstallPage},
		{"swupd mirror", newSwupdMirrorPage},
		{"autoupdate", newAutoUpdatePage},
		{"save config", newSaveConfigPage},
	}

	for _, menu := range menus {
		var page Page

		if page, err = menu.fc(tui); err != nil {
			return false, err
		}

		tui.pages = append(tui.pages, page)
	}

	tui.gotoPage(TuiPageMenu, tui.currPage)

	var paniced error

	go func() {
		if paniced = <-tui.paniced; paniced != nil {
			clui.Stop()
			log.ErrorError(paniced)
		}
	}()

	// Run system check, if fail report error and exit
	if retErr := syscheck.RunSystemCheck(true); retErr != nil {
		msg := "System failed to pass pre-install checks." + "\n" +
			retErr.Error() + "\n\n" +
			"The application will now exit."

		if dialog, err := CreateWarningDialogBox(msg); err == nil {
			dialog.OnClose(func() {
				clui.Stop()
				log.ErrorError(retErr)
			})
		} else {
			log.Warning("Failed to create warning dialog: %s", err)
		}
	}

	clui.MainLoop()

	if paniced != nil {
		if errLog := md.Telemetry.LogRecord("tuipanic", 3, paniced.Error()); errLog != nil {
			log.Error("Failed to log Telemetry fail record: %s", "tuipanic")
		}
		log.RequestCrashInfo()
		return false, paniced
	}

	return tui.installReboot, nil
}

func (tui *Tui) gotoPage(id int, currPage Page) {
	if tui.currPage != nil && !isPopUpPage(id) {
		if tui.currPage.GetWindow() != nil {
			tui.currPage.GetWindow().SetVisible(false)
			tui.currPage.DeActivate()

			// TODO clui is not hiding cursor when we hide/destroy an edit widget
			termbox.HideCursor()
		}
	}

	tui.currPage = tui.getPage(id)
	tui.prevPage = currPage

	tui.currPage.Activate()
	if !isPopUpPage(id) {
		tui.currPage.GetWindow().SetVisible(true)
		clui.ActivateControl(tui.currPage.GetWindow(), tui.currPage.GetActivated())
	}
}

func (tui *Tui) getPage(page int) Page {
	for _, curr := range tui.pages {
		if curr.GetID() == page {
			return curr
		}
	}

	return nil
}
