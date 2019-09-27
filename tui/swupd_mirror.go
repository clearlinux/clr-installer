// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"strings"

	"github.com/VladimirMarkelov/clui"

	"github.com/clearlinux/clr-installer/swupd"
)

// SwupdMirrorPage is the Page implementation for the swupd mirror page
type SwupdMirrorPage struct {
	BasePage
	swupdMirrorEdit    *clui.EditField
	swupdMirrorWarning *clui.Label
	insecureCheck      *clui.CheckBox
	confirmBtn         *SimpleButton
	cancelBtn          *SimpleButton
	userDefined        bool
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *SwupdMirrorPage) GetConfiguredValue() string {
	mirror := page.getModel().SwupdMirror

	if mirror == "" {
		return "No swupd mirror set"
	}

	return mirror
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *SwupdMirrorPage) GetConfigDefinition() int {
	mirror := page.getModel().SwupdMirror

	if mirror == "" {
		return ConfigNotDefined
	} else if page.userDefined {
		return ConfigDefinedByUser
	}

	return ConfigDefinedByConfig
}

// Activate sets the swupd mirror with the current model's value
func (page *SwupdMirrorPage) Activate() {
	page.swupdMirrorEdit.SetTitle(page.getModel().SwupdMirror)
	page.swupdMirrorWarning.SetTitle("")
}

func (page *SwupdMirrorPage) setConfirmButton() {
	if page.swupdMirrorWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

func (page *SwupdMirrorPage) validateMirror() {
	warning := ""
	userURL := page.swupdMirrorEdit.Title()

	if userURL != "" && swupd.IsValidMirror(userURL, page.getModel().AllowInsecureHTTP) == false {
		warning = swupd.InvalidURL
	}

	page.swupdMirrorWarning.SetTitle(warning)
	page.setConfirmButton()
}

func newSwupdMirrorPage(tui *Tui) (Page, error) {
	page := &SwupdMirrorPage{}
	page.setupMenu(tui, TuiPageSwupdMirror, "Swupd Mirror", NoButtons, TuiPageMenu)
	clui.CreateLabel(page.content, 2, 2, swupd.MirrorDesc1, Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 10, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(2, 0)
	title := swupd.MirrorTitle + ":"
	newFieldLabel(lblFrm, title)

	fldFrm := clui.CreateFrame(frm, 60, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	iframe := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	iframe.SetPack(clui.Vertical)

	page.swupdMirrorEdit = clui.CreateEditField(iframe, 1, "", Fixed)
	page.swupdMirrorEdit.OnChange(func(ev clui.Event) {
		page.validateMirror()
	})

	page.swupdMirrorWarning = clui.CreateLabel(iframe, 1, 1, "", Fixed)
	page.swupdMirrorWarning.SetMultiline(true)
	page.swupdMirrorWarning.SetBackColor(errorLabelBg)
	page.swupdMirrorWarning.SetTextColor(errorLabelFg)

	lbl := clui.CreateLabel(fldFrm, 2, AutoSize, swupd.MirrorDesc2, AutoSize)
	lbl.SetMultiline(true)

	checkFrame := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	checkFrame.SetPack(clui.Vertical)
	checkFrame.SetPaddings(2, 1)

	// Insecure Checkbox
	page.insecureCheck = clui.CreateCheckBox(checkFrame, AutoSize, swupd.MirrorAllowInsecure, AutoSize)
	if page.getModel().AllowInsecureHTTP {
		page.insecureCheck.SetState(1)
	}

	page.insecureCheck.OnChange(func(state int) {
		if state == 0 {
			page.getModel().AllowInsecureHTTP = false
		} else {
			page.getModel().AllowInsecureHTTP = true
		}

		page.validateMirror()
	})

	page.cancelBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Cancel", Fixed)
	page.cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageMenu)
	})

	page.confirmBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Confirm", Fixed)
	page.confirmBtn.OnClick(func(ev clui.Event) {
		page.confirmBtn.SetEnabled(false)
		mirror := page.swupdMirrorEdit.Title()
		if mirror == "" {
			if page.getModel().SwupdMirror != "" {
				_, _ = swupd.UnSetHostMirror()
				page.getModel().SwupdMirror = mirror
			}
			page.SetDone(false)
			page.GotoPage(TuiPageMenu)
			page.userDefined = false
		} else {
			url, err := swupd.SetHostMirror(mirror, page.getModel().AllowInsecureHTTP)
			if err != nil {
				page.swupdMirrorWarning.SetTitle(err.Error())
			} else {
				if url != strings.TrimRight(mirror, "/ ") {
					page.swupdMirrorWarning.SetTitle(swupd.IncorrectMirror + ": " + url)
				} else {
					page.userDefined = true
					page.getModel().SwupdMirror = url
					page.SetDone(true)
					page.GotoPage(TuiPageMenu)
				}
			}
		}
		page.setConfirmButton()
	})
	page.confirmBtn.SetEnabled(false)

	page.activated = page.swupdMirrorEdit

	return page, nil
}
