// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"log"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/gui/utils"
	"github.com/clearlinux/clr-installer/hostname"
	"github.com/clearlinux/clr-installer/model"
)

// HostnamePage is a simple page to enter the hostname
type HostnamePage struct {
	controller      Controller
	model           *model.SystemInstall
	box             *gtk.Box
	hostnameEntry   *gtk.Entry
	hostnameWarning *gtk.Label
}

// NewHostnamePage returns a new NewHostnamePage
func NewHostnamePage(controller Controller, model *model.SystemInstall) (Page, error) {
	page := &HostnamePage{
		controller: controller,
		model:      model,
	}
	var err error

	page.box, err = utils.SetBox(gtk.ORIENTATION_VERTICAL, 0, 8)
	if err != nil {
		return nil, err
	}

	// Hostname entry
	page.hostnameEntry, err = gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	page.hostnameEntry.SetMaxLength(63)
	page.box.PackStart(page.hostnameEntry, false, false, 0)

	// Hostname warning
	page.hostnameWarning, err = gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	page.hostnameWarning.SetXAlign(0)
	page.hostnameWarning.SetYAlign(0)
	sc, err := page.hostnameWarning.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass("installer-warning-msg")
	page.box.PackStart(page.hostnameWarning, true, true, 10)

	// Generate signal on Hostname entry change
	if _, err := page.hostnameEntry.Connect("changed", page.onChange); err != nil {
		return nil, err
	}

	return page, nil
}

func (page *HostnamePage) onChange(entry *gtk.Entry) error {
	host, err := utils.GetTextFromEntry(entry)
	if err != nil {
		return err
	}
	warning := ""
	warning = hostname.IsValidHostname(host)
	if host != "" && warning != "" {
		page.hostnameWarning.SetLabel(warning)
		page.controller.SetButtonState(ButtonConfirm, false)

	} else {
		page.hostnameWarning.SetLabel("")
		page.controller.SetButtonState(ButtonConfirm, true)
	}
	return nil
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
	return "Assign Hostname"
}

// GetTitle will return the title for this page
func (page *HostnamePage) GetTitle() string {
	return "Assign a Hostname for the installation target"
}

// StoreChanges will store this pages changes into the model
func (page *HostnamePage) StoreChanges() {
	host, err := utils.GetTextFromEntry(page.hostnameEntry) // TODO: Handle error
	if err != nil {
		log.Fatal(err)
	}
	page.model.Hostname = host
}

// ResetChanges will reset this page to match the model
func (page *HostnamePage) ResetChanges() {
	host := page.model.Hostname
	err := utils.SetTextInEntry(page.hostnameEntry, host) // TODO: Handle error
	if err != nil {
		log.Fatal(err)
	}
	page.hostnameWarning.SetLabel("")
}

// GetConfiguredValue returns our current config
func (page *HostnamePage) GetConfiguredValue() string {
	if page.model.Hostname == "" {
		return "No target system hostname assigned"
	}
	return page.model.Hostname
}
