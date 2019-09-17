// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"fmt"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	ctrl "github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
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
	info    *gtk.Label             // Display info during install
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

	page.info, err = setLabel("", "label-info", 0)
	if err != nil {
		return nil, err
	}
	page.info.SetMarginStart(24)
	page.info.SetMarginEnd(24)
	page.info.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	page.info.SetLineWrap(true)
	page.info.SetSelectable(true) // Make info label selectable
	page.layout.PackStart(page.info, false, false, 0)

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
	page.pbar.SetPulseStep(0.3)
	page.pbar.SetMarginTop(12)

	// Throw it on the bottom of the page
	page.layout.PackEnd(page.pbar, false, false, 0)

	return page, nil
}

// IsRequired is just here for the Page API
func (page *InstallPage) IsRequired() bool {
	return true
}

// IsDone is just here for the Page API
func (page *InstallPage) IsDone() bool {
	return false
}

// GetID returns the ID for this page
func (page *InstallPage) GetID() int {
	return PageIDInstall
}

// GetSummary will return the summary for this page
func (page *InstallPage) GetSummary() string {
	return utils.Locale.Get("Installing Clear Linux* OS")
}

// GetTitle will return the title for this page
func (page *InstallPage) GetTitle() string {
	return utils.Locale.Get("Installing Clear Linux* OS")
}

// GetIcon returns the icon for this page
func (page *InstallPage) GetIcon() string {
	return "system-software-install-symbolic"
}

// GetConfiguredValue returns nothing here
func (page *InstallPage) GetConfiguredValue() string {
	return ""
}

// GetRootWidget returns the root embeddable widget for this page
func (page *InstallPage) GetRootWidget() gtk.IWidget {
	return page.layout
}

// StoreChanges will store this pages changes into the model
func (page *InstallPage) StoreChanges() {}

// ResetChanges begins as our initial execution point as we're only going
// to get called when showing our page.
func (page *InstallPage) ResetChanges() {
	msg := utils.Locale.Get("Installation in progress.")
	msg = msg + " " + utils.Locale.Get("Please wait.")
	page.info.SetText(msg)

	// Validate the model
	err := page.model.Validate()
	if err != nil {
		fmt.Println(err)
		return
	}

	// TODO: Disable closing of the installer
	go func() {
		// Become the progress hook
		progress.Set(page)

		go func() {
			_ = network.DownloadInstallerMessage("Pre-Installation",
				network.PreGuiInstallConf)
		}()

		// Go install it
		err := ctrl.Install(page.controller.GetRootDir(),
			page.model,
			page.controller.GetOptions(),
		)

		// Temporary handling of errors
		if err != nil {
			text := utils.Locale.Get("Installation failed.")
			text = text + " " + utils.Locale.Get("See %s for details.", page.controller.GetOptions().LogFile)
			page.info.SetText(text)
			sc, err := page.info.GetStyleContext()
			if err != nil {
				log.Warning("Error getting style context: ", err) // Just log trivial error
			} else {
				sc.RemoveClass("label-info")
				sc.AddClass("label-warning")
			}
		} else {
			text := utils.Locale.Get("Installation successful.")
			page.info.SetText(text)
		}

		_, err = glib.IdleAdd(func() {
			page.pbar.SetFraction(1.0)
		})
		if err != nil {
			log.ErrorError(err) // TODO: Handle error in a better way
			return
		}

		go func() {
			_ = network.DownloadInstallerMessage("Post-Installation",
				network.PostGuiInstallConf)
		}()

		page.controller.SetButtonState(ButtonQuit, true)
	}()

}

// Following methods are for the progress.Client API

// Desc will push a description box into the view for later marking
func (page *InstallPage) Desc(desc string) {
	_, err := glib.IdleAdd(func() {
		fmt.Println(desc)

		// Increment selection
		page.selection++

		// do we have an old widget? if so, mark complete
		if page.selection > 0 {
			page.widgets[page.selection-1].Completed()
		}

		// Create new install widget
		widg, err := NewInstallWidget(desc)
		if err != nil {
			panic(err)
		}
		page.widgets[page.selection] = widg

		// Pack it into the list
		page.list.Add(widg.GetRootWidget())

		// Scroll to the new item
		row := page.list.GetRowAtIndex(page.selection)
		page.list.SelectRow(row)
		scrollToView(page.scroll, page.list, &row.Widget)
	})
	if err != nil {
		log.ErrorError(err) // TODO: Handle error in a better way
		return
	}
}

// Failure handles failure to install
func (page *InstallPage) Failure() {
	_, err := glib.IdleAdd(func() {
		page.widgets[page.selection].MarkStatus(false)
	})
	if err != nil {
		log.ErrorError(err) // TODO: Handle error in a better way
		return
	}
}

// Success notes the install was successful
func (page *InstallPage) Success() {
	_, err := glib.IdleAdd(func() {
		page.widgets[page.selection].MarkStatus(true)
	})
	if err != nil {
		log.ErrorError(err) // TODO: Handle error in a better way
		return
	}
}

// LoopWaitDuration will return the duration for step-waits
func (page *InstallPage) LoopWaitDuration() time.Duration {
	return common.LoopWaitDuration
}

// Partial handles an actual progress update
func (page *InstallPage) Partial(total int, step int) {
	_, err := glib.IdleAdd(func() {
		page.pbar.SetFraction(float64(step) / float64(total))
	})
	if err != nil {
		log.ErrorError(err) // TODO: Handle error in a better way
		return
	}
}

// Step will step the progressbar in indeterminate mode
func (page *InstallPage) Step() {
	_, err := glib.IdleAdd(func() {
		page.pbar.Pulse()
	})
	if err != nil {
		log.ErrorError(err) // TODO: Handle error in a better way
		return
	}
}
