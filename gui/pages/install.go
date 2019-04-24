// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"fmt"
	"time"

	"github.com/gotk3/gotk3/gtk"

	ctrl "github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

var (
	loopWaitDuration = 200 * time.Millisecond
)

// InstallPage is a specialised page type with no corresponding
// ContentView summary. It handles the actual install routine.
type InstallPage struct {
	controller Controller
	model      *model.SystemInstall
	layout     *gtk.Box

	pbar      *gtk.ProgressBar    // Progress bar
	list      *gtk.ListBox        // Scrolling list for messages
	selection int                 // Current progress selection
	scroll    *gtk.ScrolledWindow // Hold the list

	widgets map[int]*InstallWidget // mapping of widgets
}

// NewInstallPage constructs a new InstallPage.
func NewInstallPage(controller Controller, model *model.SystemInstall) (Page, error) {
	var err error

	// Create page
	page := &InstallPage{
		controller: controller,
		model:      model,
		widgets:    make(map[int]*InstallWidget),
		selection:  -1,
	}

	// Create layout
	page.layout, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}

	// Create scroller
	page.scroll, err = gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	page.scroll.SetMarginStart(48)
	page.scroll.SetMarginEnd(48)
	page.scroll.SetMarginTop(24)
	page.scroll.SetMarginBottom(24)
	page.layout.PackStart(page.scroll, true, true, 0)

	// Create list
	page.list, err = gtk.ListBoxNew()
	if err != nil {
		return nil, err
	}
	page.list.SetSelectionMode(gtk.SELECTION_NONE)
	page.scroll.Add(page.list)
	st, err := page.list.GetStyleContext()
	if err != nil {
		return nil, err
	}
	st.AddClass("scroller-main")

	// Create progressbar
	page.pbar, err = gtk.ProgressBarNew()
	if err != nil {
		return nil, err
	}

	// Sort out padding
	page.pbar.SetHAlign(gtk.ALIGN_FILL)
	page.pbar.SetMarginStart(24)
	page.pbar.SetMarginEnd(24)
	page.pbar.SetMarginBottom(12)
	page.pbar.SetMarginTop(12)

	// Throw it on the bottom of the page
	page.layout.PackEnd(page.pbar, false, false, 0)

	return page, nil
}

// IsRequired is just here for the Page API
func (install *InstallPage) IsRequired() bool {
	return true
}

// IsDone is just here for the Page API
func (install *InstallPage) IsDone() bool {
	return false
}

// GetID returns the ID for this page
func (install *InstallPage) GetID() int {
	return PageIDInstall
}

// GetSummary will return the summary for this page
func (install *InstallPage) GetSummary() string {
	return utils.Locale.Get("Installing Clear Linux* OS")
}

// GetTitle will return the title for this page
func (install *InstallPage) GetTitle() string {
	return utils.Locale.Get("Installing Clear Linux OS")
}

// GetIcon returns the icon for this page
func (install *InstallPage) GetIcon() string {
	return "system-software-install-symbolic"
}

// GetConfiguredValue returns nothing here
func (install *InstallPage) GetConfiguredValue() string {
	return ""
}

// GetRootWidget returns the root embeddable widget for this page
func (install *InstallPage) GetRootWidget() gtk.IWidget {
	return install.layout
}

// StoreChanges will store this pages changes into the model
func (install *InstallPage) StoreChanges() {}

// ResetChanges begins as our initial execution point as we're only going
// to get called when showing our page.
func (install *InstallPage) ResetChanges() {
	// Disable quit button
	install.controller.SetButtonState(ButtonBack, false)

	// Validate the model
	err := install.model.Validate()
	if err != nil {
		fmt.Println(err)
		return
	}

	utils.Locale.Get("Validation passed")

	// TODO: Disable closing of the installer
	go func() {
		// Become the progress hook
		progress.Set(install)

		go func() {
			_ = network.DownloadInstallerMessage("Pre-Installation",
				network.PreGuiInstallConf)
		}()

		// Go install it
		err := ctrl.Install(install.controller.GetRootDir(),
			install.model,
			install.controller.GetOptions(),
		)
		install.pbar.SetFraction(1.0)

		// TODO: Handle this moar better.
		if err != nil {
			panic(err)
		}

		go func() {
			_ = network.DownloadInstallerMessage("Post-Installation",
				network.PostGuiInstallConf)
		}()
		utils.Locale.Get("Installation completed")
		install.controller.SetButtonState(ButtonBack, true)
	}()

}

// Following methods are for the progress.Client API

// Desc will push a description box into the view for later marking
func (install *InstallPage) Desc(desc string) {
	fmt.Println(desc)

	// Increment selection
	install.selection++

	// do we have an old widget? if so, mark complete
	if install.selection > 0 {
		install.widgets[install.selection-1].Completed()
	}

	// Create new install widget
	widg, err := NewInstallWidget(desc)
	if err != nil {
		panic(err)
	}
	install.widgets[install.selection] = widg

	// Pack it into the list
	install.list.Add(widg.GetRootWidget())

	// Scroll to the new item
	row := install.list.GetRowAtIndex(install.selection)
	install.list.SelectRow(row)
	scrollToView(install.scroll, install.list, &row.Widget)
}

// Failure handles failure to install
func (install *InstallPage) Failure() {
	install.widgets[install.selection].MarkStatus(false)
	utils.Locale.Get("Failure")
}

// Success notes the install was successful
func (install *InstallPage) Success() {
	utils.Locale.Get("Success")
	install.widgets[install.selection].MarkStatus(true)
}

// LoopWaitDuration will return the duration for step-waits
func (install *InstallPage) LoopWaitDuration() time.Duration {
	return loopWaitDuration
}

// Partial handles an actual progress update
func (install *InstallPage) Partial(total int, step int) {
	install.pbar.SetFraction(float64(step) / float64(total))
}

// Step will step the progressbar in indeterminate mode
func (install *InstallPage) Step() {
	// Pulse twice for visual feedback
	install.pbar.Pulse()
	install.pbar.Pulse()
}
