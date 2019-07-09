// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"net/url"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/utils"
)

// SwupdConfigPage is a page to change swupd configuration
type SwupdConfigPage struct {
	controller        Controller
	model             *model.SystemInstall
	box               *gtk.Box
	mirrorHeading     *gtk.Label
	mirrorDesc        *gtk.Label
	mirrorEntry       *gtk.Entry
	mirrorWarning     *gtk.Label
	autoUpdateHeading *gtk.Label
	autoUpdateDesc    *gtk.Label
	autoUpdateButton  *gtk.CheckButton
	autoUpdateNote    *gtk.Label
	done              bool
}

// NewSwupdConfigPage returns a new NewSwupdConfigPage
func NewSwupdConfigPage(controller Controller, model *model.SystemInstall) (Page, error) {
	var err error

	page := &SwupdConfigPage{
		controller: controller,
		model:      model,
		done:       true,
	}

	// Page Box
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page-new")
	if err != nil {
		return nil, err
	}

	// Mirror URL
	page.mirrorHeading, err = setLabel(utils.Locale.Get("Mirror URL"), "label-entry", 0.0)
	if err != nil {
		return nil, err
	}
	page.mirrorHeading.SetMarginStart(common.StartEndMargin)
	page.mirrorHeading.SetMarginTop(common.TopBottomMargin)
	page.mirrorHeading.SetHAlign(gtk.ALIGN_START)
	page.box.PackStart(page.mirrorHeading, false, false, 0)

	desc := utils.Locale.Get("Specify a different installation source %s than the default.", "(swupd) URL")
	desc += " " + utils.Locale.Get("%s sites must use a publicly signed CA.", "HTTPS")
	page.mirrorDesc, err = setLabel(desc, "", 0)
	if err != nil {
		return nil, err
	}
	page.mirrorDesc.SetMarginStart(common.StartEndMargin)
	page.box.PackStart(page.mirrorDesc, false, false, 10)

	page.mirrorEntry, err = setEntry("entry-no-top-margin")
	if err != nil {
		return nil, err
	}
	page.mirrorEntry.SetMarginStart(common.StartEndMargin)
	page.mirrorEntry.SetMarginEnd(common.StartEndMargin)
	page.box.PackStart(page.mirrorEntry, false, false, 0)

	page.mirrorWarning, err = setLabel("", "label-error", 0.0)
	if err != nil {
		return nil, err
	}
	page.mirrorWarning.SetMarginStart(common.StartEndMargin)
	page.mirrorWarning.SetMarginBottom(common.TopBottomMargin)
	page.box.PackStart(page.mirrorWarning, false, false, 0)
	if _, err := page.mirrorEntry.Connect("changed", page.onMirrorChange); err != nil {
		return nil, err
	}

	// Auto Updates
	page.autoUpdateHeading, err = setLabel(utils.Locale.Get("Automatic OS Updates"), "label-entry", 0.0)
	if err != nil {
		return nil, err
	}
	page.autoUpdateHeading.SetMarginStart(common.StartEndMargin)
	page.autoUpdateHeading.SetMarginTop(common.TopBottomMargin)
	page.autoUpdateHeading.SetHAlign(gtk.ALIGN_START)
	page.box.PackStart(page.autoUpdateHeading, false, false, 0)

	desc = utils.Locale.Get("Allow the Clear Linux OS to continuously update as new versions are released.")
	desc += "\n"
	desc += utils.Locale.Get("This is the default, preferred behavior for Clear Linux OS to ensure that the latest security concerns are always addressed.")
	desc += "\n"
	desc += utils.Locale.Get("This can also be enabled post installation using the command %s.", "\"swupd autoupdate --enable\"")
	desc += "\n"
	swupdDoc := "https://clearlinux.org/documentation/clear-linux/concepts/swupd-about"
	desc += utils.Locale.Get("See %s for more information.", swupdDoc)
	page.autoUpdateDesc, err = setLabel(desc, "", 0)
	if err != nil {
		return nil, err
	}
	page.autoUpdateDesc.SetMarginStart(common.StartEndMargin)
	page.autoUpdateDesc.SetMarginTop(common.TopBottomMargin)
	page.autoUpdateDesc.SetSelectable(true)
	page.box.PackStart(page.autoUpdateDesc, false, false, 0)

	page.autoUpdateButton, err = gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}
	page.autoUpdateButton.SetLabel("  " + utils.Locale.Get("Enable Auto Updates"))
	page.autoUpdateButton.SetMarginStart(14)         // Custom margin to align properly
	page.autoUpdateButton.SetHAlign(gtk.ALIGN_START) // Ensures that clickable area is only within the label
	page.box.PackStart(page.autoUpdateButton, false, false, 10)
	if _, err := page.autoUpdateButton.Connect("clicked", page.onAutoUpdateClick); err != nil {
		return nil, err
	}

	desc = utils.Locale.Get("NOTE:")
	desc += " "
	desc += "Disabling Automatic OS Updates puts your system at risk of missing critical security patches."
	page.autoUpdateNote, err = setLabel(desc, "", 0)
	if err != nil {
		return nil, err
	}
	page.autoUpdateNote.SetMarginStart(common.StartEndMargin)
	page.autoUpdateNote.SetSelectable(true)
	page.box.PackStart(page.autoUpdateNote, false, false, 0)

	return page, nil
}

func (page *SwupdConfigPage) onMirrorChange(entry *gtk.Entry) {
	mirror := getTextFromEntry(entry)
	page.mirrorWarning.SetText("")
	if mirror != "" {
		_, err := url.ParseRequestURI(mirror)
		if err != nil {
			page.mirrorWarning.SetText(utils.Locale.Get("Invalid URL"))
		}
	}

	page.setConfirmButton()
}

func (page *SwupdConfigPage) validateMirror() {
	page.controller.SetButtonState(ButtonConfirm, false)
	mirror := getTextFromEntry(page.mirrorEntry)
	page.mirrorWarning.SetText("")
	if mirror == "" {
		if page.model.SwupdMirror != "" {
			_, _ = swupd.UnSetHostMirror()
			page.model.SwupdMirror = mirror // success
		}
	} else {
		url, err := swupd.SetHostMirror(mirror)
		if err != nil {
			page.mirrorWarning.SetText(err.Error()) // failure
		} else {
			if url != mirror { // At this point, url and mirror are expected to be the same
				page.mirrorWarning.SetText(utils.Locale.Get("Mirror not set correctly")) // failure
			} else {
				page.model.SwupdMirror = mirror // success
			}
		}
	}

	page.setConfirmButton()
}

func (page *SwupdConfigPage) onAutoUpdateClick(button *gtk.CheckButton) {
	var removeStyle, addStyle string
	if button.GetActive() {
		removeStyle = "label-error"
		addStyle = ""
	} else {
		removeStyle = ""
		addStyle = "label-error"
	}

	sc, err := page.autoUpdateNote.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.RemoveClass(removeStyle)
		sc.AddClass(addStyle)
	}
}

func (page *SwupdConfigPage) setConfirmButton() {
	warning, err := page.mirrorWarning.GetText()
	if err != nil {
		log.ErrorError(err) // Just log trivial errors
	}
	if warning == "" {
		page.done = true
		page.controller.SetButtonState(ButtonConfirm, true)
	} else {
		page.done = false
		page.controller.SetButtonState(ButtonConfirm, false)
	}
}

// IsRequired will return false as we have default values
func (page *SwupdConfigPage) IsRequired() bool {
	return false
}

// IsDone checks if all the steps are completed
func (page *SwupdConfigPage) IsDone() bool {
	return page.done
}

// GetID returns the ID for this page
func (page *SwupdConfigPage) GetID() int {
	return PageIDConfigSwupd
}

// GetIcon returns the icon for this page
func (page *SwupdConfigPage) GetIcon() string {
	return "software-update-available-symbolic"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *SwupdConfigPage) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *SwupdConfigPage) GetSummary() string {
	return utils.Locale.Get("Software Updater Configuration")
}

// GetTitle will return the title for this page
func (page *SwupdConfigPage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *SwupdConfigPage) StoreChanges() {
	page.validateMirror()
	page.model.AutoUpdate = page.autoUpdateButton.GetActive()
}

// ResetChanges will reset this page to match the model
func (page *SwupdConfigPage) ResetChanges() {
	page.controller.SetButtonState(ButtonConfirm, true)
	setTextInEntry(page.mirrorEntry, page.model.SwupdMirror)
	page.autoUpdateButton.SetActive(page.model.AutoUpdate)
}

// GetConfiguredValue returns a string representation of the current config
func (page *SwupdConfigPage) GetConfiguredValue() string {
	var ret string
	if page.model.AutoUpdate {
		ret = utils.Locale.Get("Auto updates enabled")
	} else {
		ret = utils.Locale.Get("Auto updates disabled")
	}

	if page.model.SwupdMirror != "" {
		ret += utils.Locale.Get(". Custom mirror set.")
	}

	return ret
}
