// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/VladimirMarkelov/clui"
)

// KernelCMDLine is the Page implementation for the kernel cmd line configuration page
type KernelCMDLine struct {
	BasePage
	addKernelArgEdit *clui.EditField
	remKernelArgEdit *clui.EditField
}

const (
	kernelArgsHelp = `Note: The boot manager tool will first include the "Add Extra Arguments"
      items, then the "Remove Arguments" items are removed. The final
      argument list contains the kernel bundle's configured arguments
      and the ones configured by the user.`
)

// GetConfiguredValue Returns the string representation of currently value set
func (pp *KernelCMDLine) GetConfiguredValue() string {
	result := ""

	if pp.getModel().KernelArguments != nil {
		values := []string{}

		addKernelArgs := strings.Join(pp.getModel().KernelArguments.Add, " ")
		remKernelArgs := strings.Join(pp.getModel().KernelArguments.Remove, " ")

		if addKernelArgs != "" {
			values = append(values, fmt.Sprintf("Add: %s", addKernelArgs))
		}

		if remKernelArgs != "" {
			values = append(values, fmt.Sprintf("Remove: %s", remKernelArgs))
		}

		result = strings.Join(values, " | ")
	}

	if result == "" {
		return "No kernel command line configuration defined"
	}

	return result
}

// Activate sets the kernel cmd line configuration with the current model's value
func (pp *KernelCMDLine) Activate() {
	if pp.getModel().KernelArguments == nil {
		return
	}

	addKernelArgs := strings.Join(pp.getModel().KernelArguments.Add, " ")
	remKernelArgs := strings.Join(pp.getModel().KernelArguments.Remove, " ")

	pp.addKernelArgEdit.SetTitle(addKernelArgs)
	pp.remKernelArgEdit.SetTitle(remKernelArgs)
}

func newKernelCMDLine(tui *Tui) (Page, error) {
	page := &KernelCMDLine{}
	page.setupMenu(tui, TuiPageKernelCMDLine, "Kernel Command Line", NoButtons, TuiPageMenu)

	clui.CreateLabel(page.content, 2, 2, "Add or Remove Extra Kernel Command Line Arguments",
		Fixed)

	helpLabel := clui.CreateLabel(page.content, 2, 5, kernelArgsHelp, Fixed)
	helpLabel.SetMultiline(true)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 20, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "Add Extra Arguments:")

	newFieldLabel(lblFrm, "Remove Arguments:")

	fldFrm := clui.CreateFrame(frm, 30, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	iframe := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	iframe.SetPack(clui.Vertical)

	page.addKernelArgEdit = clui.CreateEditField(iframe, 1, "", Fixed)

	iframe = clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	iframe.SetPack(clui.Vertical)

	page.remKernelArgEdit = clui.CreateEditField(iframe, 1, "", Fixed)

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
		addKernelArguments := page.addKernelArgEdit.Title()
		remKernelArguments := page.remKernelArgEdit.Title()

		if addKernelArguments != "" {
			page.getModel().AddExtraKernelArguments(strings.Split(addKernelArguments, " "))
		}

		if remKernelArguments != "" {
			page.getModel().RemoveKernelArguments(strings.Split(remKernelArguments, " "))
		}

		done := page.addKernelArgEdit.Title() != "" || page.remKernelArgEdit.Title() != ""
		page.SetDone(done)

		page.GotoPage(TuiPageMenu)
	})

	page.activated = page.addKernelArgEdit

	return page, nil
}
