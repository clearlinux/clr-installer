// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"

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
)

// New creates a new Tui frontend instance
func New() *Tui {
	return &Tui{pages: []Page{}}
}

// MustRun is part of the Frontend interface implementation and tells the core that this
// frontend wants/must run.
func (tui *Tui) MustRun(args *args.Args) bool {
	return true
}

func lookupThemeDir() (string, error) {
	var result string

	themeDirs := []string{
		os.Getenv("CLR_INSTALLER_THEME_DIR"),
	}

	src, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}

	if strings.Contains(src, "/.gopath/bin") {
		themeDirs = append(themeDirs, strings.Replace(src, "bin", "../themes", 1))
	}

	themeDirs = append(themeDirs, "/usr/share/clr-installer/themes/")

	for _, curr := range themeDirs {
		if _, err := os.Stat(curr); os.IsNotExist(err) {
			continue
		}

		result = curr
		break
	}

	if result == "" {
		panic(errors.Errorf("Could not find a theme dir"))
	}

	return result, nil
}

// Run is part of the Frontend interface implementation and is the tui frontend main entry point
func (tui *Tui) Run(md *model.SystemInstall, rootDir string, options args.Args) (bool, error) {
	clui.InitLibrary()
	defer clui.DeinitLibrary()

	tui.model = md
	tui.options = options
	themeDir, err := lookupThemeDir()
	if err != nil {
		return false, err
	}

	clui.SetThemePath(themeDir)

	if !clui.SetCurrentTheme("clr-installer") {
		panic("Could not change theme")
	}

	errorLabelBg = clui.RealColor(clui.ColorDefault, "ErrorLabel", "Back")
	errorLabelFg = clui.RealColor(clui.ColorDefault, "ErrorLabel", "Text")

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
		{"disk config", newDiskConfigPage},
		{"disk partition", newDiskPartitionPage},
		{"disk util", newDiskUtilPage},
		{"network", newNetworkPage},
		{"proxy", newProxyPage},
		{"network validate", newNetworkValidatePage},
		{"network interface", newNetworkInterfacePage},
		{"main menu", newMenuPage},
		{"bundle selection", newBundlePage},
		{"add manager", newUserManagerPage},
		{"add user", newUseraddPage},
		{"telemetry enabling", newTelemetryPage},
		{"kernel cmdline", newKernelCMDLine},
		{"kernel selection", newKernelPage},
		{"install", newInstallPage},
		{"swupd mirror", newSwupdMirrorPage},
		{"hostname", newHostnamePage},
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
