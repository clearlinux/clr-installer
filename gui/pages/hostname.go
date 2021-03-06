// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/hostname"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
)

// HostnamePage is a simple page to enter the hostname
type HostnamePage struct {
	controller Controller
	model      *model.SystemInstall
	box        *gtk.Box
	entry      *gtk.Entry
	rules      *gtk.Label
	warning    *gtk.Label
}

// NewHostnamePage returns a new NewHostnamePage
func NewHostnamePage(controller Controller, model *model.SystemInstall) (Page, error) {
	page := &HostnamePage{
		controller: controller,
		model:      model,
	}
	var err error

	// Box
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page-new")
	if err != nil {
		return nil, err
	}

	// Entry
	page.entry, err = setEntry("entry")
	if err != nil {
		return nil, err
	}
	page.entry.SetMaxLength(hostname.MaxHostnameLength)
	page.entry.SetMarginStart(common.StartEndMargin)
	page.entry.SetMarginEnd(common.StartEndMargin)
	page.box.PackStart(page.entry, false, false, 0)

	// Rules label
	rulesText := utils.Locale.Get(
		"Can use alphanumeric characters and - with a maximum of %d characters.",
		hostname.MaxHostnameLength)
	page.rules, err = setLabel(rulesText, "label-rules", 0.0)
	if err != nil {
		return nil, err
	}
	page.rules.SetMarginStart(common.StartEndMargin)
	page.rules.SetHAlign(gtk.ALIGN_START)
	page.box.PackStart(page.rules, false, false, 10)

	// Warning label
	page.warning, err = setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, err
	}
	page.warning.SetMarginStart(common.StartEndMargin)
	page.warning.SetMarginEnd(common.StartEndMargin)
	page.box.PackStart(page.warning, false, false, 10)

	// Generate signal on Hostname entry change
	_ = page.entry.Connect("changed", page.onChange)

	return page, nil
}

func (page *HostnamePage) onChange(entry *gtk.Entry) {
	host := getTextFromEntry(entry)
	warning := ""
	warning = hostname.IsValidHostname(host)
	if host != "" && warning != "" {
		page.warning.SetLabel(warning)
		page.controller.SetButtonState(ButtonConfirm, false)
	} else {
		page.warning.SetLabel("")
		page.controller.SetButtonState(ButtonConfirm, true)
	}
}

// IsRequired will return false as we have default values
func (page *HostnamePage) IsRequired() bool {
	return false
}

// IsDone checks if all the steps are completed
func (page *HostnamePage) IsDone() bool {
	return page.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (page *HostnamePage) GetID() int {
	return PageIDHostname
}

// GetIcon returns the icon for this page
func (page *HostnamePage) GetIcon() string {
	return "computer"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *HostnamePage) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *HostnamePage) GetSummary() string {
	return utils.Locale.Get("Assign Hostname")
}

// GetTitle will return the title for this page
func (page *HostnamePage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *HostnamePage) StoreChanges() {
	host := getTextFromEntry(page.entry)
	page.model.Hostname = host
}

// ResetChanges will reset this page to match the model
func (page *HostnamePage) ResetChanges() {
	host := page.model.Hostname
	setTextInEntry(page.entry, host)
	page.warning.SetLabel("")
}

// GetConfiguredValue returns our current config
func (page *HostnamePage) GetConfiguredValue() string {
	if page.model.Hostname == "" {
		return utils.Locale.Get("No target system hostname assigned")
	}
	return page.model.Hostname
}
