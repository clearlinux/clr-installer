// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/clearlinux/clr-installer/hostname"

	"github.com/VladimirMarkelov/clui"
)

// HostnamePage is the Page implementation for the hostname input
type HostnamePage struct {
	BasePage
	HostnameEdit    *clui.EditField
	HostnameWarning *clui.Label
	confirmBtn      *SimpleButton
	cancelBtn       *SimpleButton
	userDefined     bool
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *HostnamePage) GetConfigDefinition() int {
	hostname := page.getModel().Hostname

	if hostname == "" {
		return ConfigNotDefined
	} else if page.userDefined {
		return ConfigDefinedByUser
	}

	return ConfigDefinedByConfig
}

// Activate sets the hostname with the current model's value
func (page *HostnamePage) Activate() {
	page.HostnameEdit.SetTitle(page.getModel().Hostname)
	page.HostnameWarning.SetTitle("")
}

func (page *HostnamePage) setConfirmButton() {
	if page.HostnameWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

func newHostnamePage(tui *Tui) (Page, error) {
	page := &HostnamePage{}
	page.setupMenu(tui, TuiPageHostname, "Assign Hostname", NoButtons, TuiPageAdvancedMenu)

	clui.CreateLabel(page.content, 2, 2, "Assign a Hostname for the installation target", Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 20, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "Hostname:")

	fldFrm := clui.CreateFrame(frm, 30, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	iframe := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	iframe.SetPack(clui.Vertical)

	page.HostnameEdit = clui.CreateEditField(iframe, 1, "", Fixed)
	page.HostnameEdit.OnChange(func(ev clui.Event) {
		warning := ""
		host := page.HostnameEdit.Title()

		if host != "" {
			warning = hostname.IsValidHostname(host)
		}

		page.HostnameWarning.SetTitle(warning)
		page.setConfirmButton()
	})

	page.HostnameWarning = clui.CreateLabel(page.content, AutoSize, 1, "", Fixed)
	page.HostnameWarning.SetMultiline(true)
	page.HostnameWarning.SetBackColor(errorLabelBg)
	page.HostnameWarning.SetTextColor(errorLabelFg)

	page.cancelBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Cancel", Fixed)
	page.cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageAdvancedMenu)
	})

	page.confirmBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Confirm", Fixed)
	page.confirmBtn.OnClick(func(ev clui.Event) {
		page.confirmBtn.SetEnabled(false)
		hostname := page.HostnameEdit.Title()
		if hostname == "" {
			page.userDefined = false
			page.SetDone(false)
		} else {
			page.userDefined = true
			page.SetDone(true)
		}
		page.getModel().Hostname = hostname
		page.setConfirmButton()
		page.GotoPage(TuiPageAdvancedMenu)
	})
	page.confirmBtn.SetEnabled(false)

	page.activated = page.HostnameEdit

	return page, nil
}
