// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/VladimirMarkelov/clui"
)

// KernelCMDLine is the Page implementation for the kernel cmd line configuration page
type KernelCMDLine struct {
	BasePage
	kernelCMDLineEdit *clui.EditField
}

// GetConfiguredValue Returns the string representation of currently value set
func (pp *KernelCMDLine) GetConfiguredValue() string {
	cmdLine := pp.getModel().KernelCMDLine

	if cmdLine == "" {
		return "No kernel command line append set"
	}

	return cmdLine
}

// Activate sets the kernel cmd line configuration with the current model's value
func (pp *KernelCMDLine) Activate() {
	pp.kernelCMDLineEdit.SetTitle(pp.getModel().KernelCMDLine)
}

func newKernelCMDLine(tui *Tui) (Page, error) {
	page := &KernelCMDLine{}
	page.setupMenu(tui, TuiPageKernelCMDLine, "Kernel Command Line", NoButtons, TuiPageMenu)

	clui.CreateLabel(page.content, 2, 2, "Configure the Kernel Command Line", Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 20, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "Extra Arguments:")

	fldFrm := clui.CreateFrame(frm, 30, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	iframe := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	iframe.SetPack(clui.Vertical)

	page.kernelCMDLineEdit = clui.CreateEditField(iframe, 1, "", Fixed)

	btnFrm := clui.CreateFrame(fldFrm, 30, 1, BorderNone, Fixed)
	btnFrm.SetPack(clui.Horizontal)
	btnFrm.SetGaps(1, 1)
	btnFrm.SetPaddings(2, 0)

	cancelBtn := CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageMenu)
	})

	confirmBtn := CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Confirm", Fixed)

	confirmBtn.OnClick(func(ev clui.Event) {
		page.getModel().KernelCMDLine = page.kernelCMDLineEdit.Title()
		page.SetDone(page.kernelCMDLineEdit.Title() != "")
		page.GotoPage(TuiPageMenu)
	})

	page.activated = page.kernelCMDLineEdit

	return page, nil
}
