// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/utils"
)

// SwupdConfigPage is a page to change swupd configuration
type SwupdConfigPage struct {
	controller           Controller
	model                *model.SystemInstall
	box                  *gtk.Box
	mirrorTitle          *gtk.Label
	mirrorDesc           *gtk.Label
	mirrorEntry          *gtk.Entry
	mirrorWarning        *gtk.Label
	insecureCheck        *gtk.CheckButton
	autoUpdateTitle      *gtk.Label
	autoUpdateDesc       *gtk.Label
	autoUpdateButton     *gtk.CheckButton
	autoUpdateWarning    *gtk.Label
	autoUpdateWarningMsg string
	done                 bool
}

// NewSwupdConfigPage returns a new NewSwupdConfigPage
func NewSwupdConfigPage(controller Controller, model *model.SystemInstall) (Page, error) {
	var err error

	page := &SwupdConfigPage{
		controller:           controller,
		model:                model,
		done:                 true,
		autoUpdateWarningMsg: swupd.AutoUpdateWarning1 + swupd.AutoUpdateWarning2,
	}

	// Page Box
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page-new")
	if err != nil {
		return nil, err
	}

	// Mirror URL
	page.mirrorTitle, err = setLabel(utils.Locale.Get(swupd.MirrorTitle), "label-entry", 0.0)
	if err != nil {
		return nil, err
	}
	page.mirrorTitle.SetMarginStart(common.StartEndMargin)
	page.mirrorTitle.SetMarginTop(common.TopBottomMargin)
	page.mirrorTitle.SetHAlign(gtk.ALIGN_START)
	page.box.PackStart(page.mirrorTitle, false, false, 0)

	desc := utils.Locale.Get(swupd.MirrorDesc1)
	desc += " " + utils.Locale.Get(swupd.MirrorDesc2)
	page.mirrorDesc, err = setLabel(desc, "", 0)
	if err != nil {
		return nil, err
	}
	page.mirrorDesc.SetMarginStart(common.StartEndMargin)
	page.mirrorDesc.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	page.mirrorDesc.SetLineWrap(true)
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
	page.mirrorWarning.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	page.mirrorWarning.SetLineWrap(true)
	page.box.PackStart(page.mirrorWarning, false, false, 0)
	if _, err := page.mirrorEntry.Connect("changed", page.onMirrorChange); err != nil {
		return nil, err
	}

	page.insecureCheck, err = gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}
	page.insecureCheck.SetLabel("  " + utils.Locale.Get(swupd.MirrorAllowInsecure))
	page.insecureCheck.SetMarginStart(14)         // Custom margin to align properly
	page.insecureCheck.SetHAlign(gtk.ALIGN_START) // Ensures that clickable area is only within the label
	page.box.PackStart(page.insecureCheck, false, false, 10)
	if _, err := page.insecureCheck.Connect("clicked", func(button *gtk.CheckButton) {
		if button.GetActive() {
			page.model.AllowInsecureHTTP = true
		} else {
			page.model.AllowInsecureHTTP = false
		}

		page.validateMirror()
	}); err != nil {
		return nil, err
	}

	separator, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	if err != nil {
		return nil, err
	}
	separator.ShowAll()
	page.box.Add(separator)

	// Auto Updates
	page.autoUpdateTitle, err = setLabel(utils.Locale.Get(swupd.AutoUpdateTitle), "label-entry", 0.0)
	if err != nil {
		return nil, err
	}
	page.autoUpdateTitle.SetMarginStart(common.StartEndMargin)
	page.autoUpdateTitle.SetMarginTop(common.TopBottomMargin)
	page.autoUpdateTitle.SetHAlign(gtk.ALIGN_START)
	page.box.PackStart(page.autoUpdateTitle, false, false, 10)

	desc = utils.Locale.Get(swupd.AutoUpdateDesc1)
	desc += "\n"
	desc += utils.Locale.Get(swupd.AutoUpdateDesc2)
	desc += "\n\n"
	desc += utils.Locale.Get(swupd.AutoUpdateDesc3) + " " + swupd.AutoUpdateCommand
	desc += "\n"
	desc += utils.Locale.Get(swupd.AutoUpdateDesc4, swupd.AutoUpdateLink)
	page.autoUpdateDesc, err = setLabel(desc, "", 0)
	if err != nil {
		return nil, err
	}
	page.autoUpdateDesc.SetMarginStart(common.StartEndMargin)
	page.autoUpdateDesc.SetMarginTop(common.TopBottomMargin)
	page.autoUpdateDesc.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	page.autoUpdateDesc.SetLineWrap(true)
	page.autoUpdateDesc.SetSelectable(true)
	page.box.PackStart(page.autoUpdateDesc, false, false, 0)

	page.autoUpdateButton, err = gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}
	page.autoUpdateButton.SetLabel("  " + utils.Locale.Get(swupd.AutoUpdateLabel))
	page.autoUpdateButton.SetMarginStart(14)         // Custom margin to align properly
	page.autoUpdateButton.SetHAlign(gtk.ALIGN_START) // Ensures that clickable area is only within the label
	page.box.PackStart(page.autoUpdateButton, false, false, 10)
	if _, err := page.autoUpdateButton.Connect("clicked", page.onAutoUpdateClick); err != nil {
		return nil, err
	}

	var warning, style string
	if page.model.AutoUpdate.Value() {
		style = ""
		warning = ""
	} else {
		style = "label-error"
		warning = utils.Locale.Get(page.autoUpdateWarningMsg)
	}
	page.autoUpdateWarning, err = setLabel(warning, style, 0)
	if err != nil {
		return nil, err
	}
	page.autoUpdateWarning.SetMarginStart(common.StartEndMargin)
	page.autoUpdateWarning.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	page.autoUpdateWarning.SetLineWrap(true)
	page.box.PackStart(page.autoUpdateWarning, false, false, 0)

	return page, nil
}

func (page *SwupdConfigPage) onMirrorChange(entry *gtk.Entry) {
	mirror := getTextFromEntry(entry)
	page.mirrorWarning.SetText("")
	if mirror != "" && network.IsValidURI(mirror, page.model.AllowInsecureHTTP) == false {
		page.mirrorWarning.SetText(utils.Locale.Get(swupd.InvalidURL))
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
		url, err := swupd.SetHostMirror(mirror, page.model.AllowInsecureHTTP)
		if err != nil {
			page.mirrorWarning.SetText(err.Error())
		} else {
			if url != strings.TrimRight(mirror, "/ ") { // swupd will drop all trailing /s
				page.mirrorWarning.SetText(utils.Locale.Get(swupd.IncorrectMirror)) // failure
			} else {
				page.model.SwupdMirror = url // success
			}
		}
	}

	page.setConfirmButton()
}

func (page *SwupdConfigPage) onAutoUpdateClick(button *gtk.CheckButton) {
	var removeStyle, addStyle, warning string
	if button.GetActive() {
		removeStyle = "label-error"
		addStyle = ""
		warning = ""
	} else {
		removeStyle = ""
		addStyle = "label-error"
		warning = utils.Locale.Get(page.autoUpdateWarningMsg)
	}

	page.autoUpdateWarning.SetText(warning)
	sc, err := page.autoUpdateWarning.GetStyleContext()
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
	page.model.AutoUpdate.SetValue(page.autoUpdateButton.GetActive())
}

// ResetChanges will reset this page to match the model
func (page *SwupdConfigPage) ResetChanges() {
	page.controller.SetButtonState(ButtonConfirm, true)
	setTextInEntry(page.mirrorEntry, page.model.SwupdMirror)
	page.autoUpdateButton.SetActive(page.model.AutoUpdate.Value())
}

// GetConfiguredValue returns a string representation of the current config
func (page *SwupdConfigPage) GetConfiguredValue() string {
	var ret string
	if page.model.AutoUpdate.Value() {
		ret = utils.Locale.Get("Auto updates enabled")
	} else {
		ret = utils.Locale.Get("Auto updates disabled")
	}

	if page.model.SwupdMirror != "" {
		ret += utils.Locale.Get(".") + " " + utils.Locale.Get("Custom mirror set.")
	}

	return ret
}
