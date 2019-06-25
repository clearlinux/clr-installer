// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/clearlinux/clr-installer/gui/common"

	"github.com/clearlinux/clr-installer/kernel"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
	"github.com/gotk3/gotk3/gtk"
	"strings"
)

// ConfigKernelPage is a page to change kernel installation configuration
type ConfigKernelPage struct {
	controller Controller
	model      *model.SystemInstall
	box        *gtk.Box
	addEntry   *gtk.Entry
	remEntry   *gtk.Entry
	addLabel   *gtk.Label
	remLabel   *gtk.Label
	scroll     *gtk.ScrolledWindow
	list       *gtk.ListBox
	data       []*kernel.Kernel
	selected   *kernel.Kernel
}

// NewConfigKernelPage returns a new NewConfigKernelPage
func NewConfigKernelPage(controller Controller, model *model.SystemInstall) (Page, error) {
	kernelArgsHelp :=
		utils.Locale.Get("Note: The boot manager tool will first include the \"Add Extra Arguments\" items.")
	kernelArgsHelp = kernelArgsHelp + "\n" +
		utils.Locale.Get("Then, the \"Remove Arguments\" items are removed.")
	kernelArgsHelp = kernelArgsHelp + "\n" +
		utils.Locale.Get("The final argument list contains the kernel bundle's configured arguments and the ones configured by the user.")

	data, err := kernel.LoadKernelList()
	if err != nil {
		return nil, err
	}
	page := &ConfigKernelPage{
		controller: controller,
		model:      model,
		data:       data,
	}

	// Box
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page")
	if err != nil {
		return nil, err
	}

	// ScrolledWindow
	page.scroll, err = setScrolledWindow(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC, "scroller")
	if err != nil {
		return nil, err
	}
	page.scroll.SetMarginStart(common.StartEndMargin)
	page.scroll.SetMarginEnd(common.StartEndMargin)
	page.box.PackStart(page.scroll, true, true, 0)

	// ListBox
	page.list, err = setListBox(gtk.SELECTION_SINGLE, true, "list-scroller")
	if err != nil {
		return nil, err
	}
	if _, err := page.list.Connect("row-activated", page.onRowActivated); err != nil {
		return nil, err
	}
	page.scroll.Add(page.list)

	// Create list data
	for _, v := range page.data {
		name := v.Name
		desc := utils.Locale.Get(v.Desc)

		//Add default label to kernel already in model
		if page.model.Kernel != nil {
			if v.Bundle == page.model.Kernel.Bundle {
				name = v.Name + utils.Locale.Get(" (default)")
			}
		}

		box, err := setBox(gtk.ORIENTATION_VERTICAL, 0, "box-list-label")
		if err != nil {
			return nil, err
		}

		labelName, err := setLabel(name, "list-label-name", 0.0)
		if err != nil {
			return nil, err
		}
		box.PackStart(labelName, false, false, 0)

		labelDesc, err := setLabel(desc, "list-label-desc", 0.0)
		if err != nil {
			return nil, err
		}
		box.PackStart(labelDesc, true, true, 0)

		page.list.Add(box)
	}

	// addLabel: Tell users they can add kernel command lines below
	addText := utils.Locale.Get("Add Extra Arguments")

	page.addLabel, err = setLabel(addText, "label-entry", 0.0)
	if err != nil {
		return nil, err
	}
	page.addLabel.SetMarginStart(common.StartEndMargin)
	page.addLabel.SetHAlign(gtk.ALIGN_START)
	page.box.PackStart(page.addLabel, false, false, 10)

	// addEntry: Args to add to the kernel command line
	page.addEntry, err = setEntry("entry")
	if err != nil {
		return nil, err
	}
	page.addEntry.SetMarginStart(common.StartEndMargin)
	page.addEntry.SetMarginEnd(common.StartEndMargin)
	page.addEntry.SetTooltipText(utils.Locale.Get(kernelArgsHelp))
	page.box.PackStart(page.addEntry, false, false, 0)

	// remLabel label
	remText := utils.Locale.Get("Remove Arguments")
	page.remLabel, err = setLabel(remText, "label-entry", 0.0)
	if err != nil {
		return nil, err
	}
	page.remLabel.SetMarginStart(common.StartEndMargin)
	page.remLabel.SetMarginEnd(common.StartEndMargin)
	page.box.PackStart(page.remLabel, false, false, 10)

	// remEntry: Args to remove to the kernel command line.
	page.remEntry, err = setEntry("entry")
	if err != nil {
		return nil, err
	}
	page.remEntry.SetMarginStart(common.StartEndMargin)
	page.remEntry.SetMarginEnd(common.StartEndMargin)
	page.remEntry.SetTooltipText(utils.Locale.Get(kernelArgsHelp))
	page.box.PackStart(page.remEntry, false, false, 0)

	return page, nil
}

func (page *ConfigKernelPage) getKern() *kernel.Kernel {
	if page.model.Kernel != nil {
		return page.model.Kernel
	}

	// if model is empty, return kernel-native
	for _, v := range page.data {
		if v.Bundle == "kernel-native" {
			return v
		}
	}

	return nil
}

func (page *ConfigKernelPage) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	page.selected = page.data[row.GetIndex()]
	page.controller.SetButtonState(ButtonConfirm, true)
}

// Select row in the box, activate it and scroll to it
func (page *ConfigKernelPage) activateRow(index int) {
	row := page.list.GetRowAtIndex(index)
	page.list.SelectRow(row)
	page.onRowActivated(page.list, row)
	scrollToView(page.scroll, page.list, &row.Widget)
}

// IsRequired will return false as we have default values
func (page *ConfigKernelPage) IsRequired() bool {
	return false
}

// IsDone checks if all the steps are completed
func (page *ConfigKernelPage) IsDone() bool {
	//Always true because all steps are optional
	return true
}

// GetID returns the ID for this page
func (page *ConfigKernelPage) GetID() int {
	return PageIDConfigKernel
}

// GetIcon returns the icon for this page
func (page *ConfigKernelPage) GetIcon() string {
	return "applications-engineering"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *ConfigKernelPage) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *ConfigKernelPage) GetSummary() string {
	return utils.Locale.Get("Kernel Configuration")
}

// GetTitle will return the title for this page
func (page *ConfigKernelPage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *ConfigKernelPage) StoreChanges() {
	adds, err := page.addEntry.GetText()
	if err != nil {
		log.Warning("Error getting entry text: ", err)
	}

	removes, err := page.remEntry.GetText()
	if err != nil {
		log.Warning("Error getting entry text: ", err)
	}

	if adds != "" {
		page.model.AddExtraKernelArguments(strings.Split(adds, " "))
	} else {
		page.model.KernelArguments.Add = nil
	}
	if removes != "" {
		page.model.RemoveKernelArguments(strings.Split(removes, " "))
	} else {
		page.model.KernelArguments.Remove = nil
	}

	page.model.Kernel = page.selected
}

// ResetChanges will reset this page to match the model
func (page *ConfigKernelPage) ResetChanges() {

	// Reset active row to match the model
	kern := page.getKern()
	if kern != nil {
		for i, v := range page.data {
			if strings.Contains(v.Bundle, kern.Bundle) {
				page.activateRow(i)
				break
			}
		}
	}

	// Reset both entries to match the model
	if page.model.KernelArguments != nil {
		page.addEntry.SetText(strings.Join(page.model.KernelArguments.Add, " "))
		page.remEntry.SetText(strings.Join(page.model.KernelArguments.Remove, " "))
	} else {
		page.addEntry.SetText("")
		page.remEntry.SetText("")
	}
}

// GetConfiguredValue returns a string representation of the current config
func (page *ConfigKernelPage) GetConfiguredValue() string {
	var ret string

	// Display current kernel/what is in the model
	if page.model.Kernel != nil {
		// By default model.Kernel contains only the Bundle information
		// So, lookup the human-friendly name
		for _, v := range page.data {
			if page.model.Kernel.Bundle == v.Bundle {
				ret = v.Name
				break
			}
		}
	}

	// The assumption made here is that these arguments were added by the user
	if page.model.KernelArguments != nil {
		if len(page.model.KernelArguments.Add) > 0 || len(page.model.KernelArguments.Remove) > 0 {
			ret = ret + utils.Locale.Get(" with custom command line arguments")
		}
	}

	// In case there is no default kernel for whatever reason (eg missing definitions)
	if ret == "" {
		return utils.Locale.Get("No custom options specified")
	}

	return ret
}
