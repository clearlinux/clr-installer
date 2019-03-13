// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"log"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/hostname"
	"github.com/clearlinux/clr-installer/model"
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
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page")
	if err != nil {
		return nil, err
	}

	// Entry
	page.entry, err = setEntry("entry")
	if err != nil {
		return nil, err
	}
	page.entry.SetMaxLength(63)
	page.box.PackStart(page.entry, false, false, 0)

	// Rules label
	page.rules, err = setLabel("You can use alpha-numeric characters and hyphens with a maximum of 63 characters", "", 0.0)
	if err != nil {
		return nil, err
	}
	page.box.PackStart(page.rules, false, false, 10)

	// Warning label
	page.warning, err = setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, err
	}
	page.box.PackStart(page.warning, false, false, 10)

	// Generate signal on Hostname entry change
	if _, err := page.entry.Connect("changed", page.onChange); err != nil {
		return nil, err
	}

	return page, nil
}

func (page *HostnamePage) onChange(entry *gtk.Entry) error {
	host, err := getTextFromEntry(entry)
	if err != nil {
		return err
	}
	warning := ""
	warning = hostname.IsValidHostname(host)
	if host != "" && warning != "" {
		page.warning.SetLabel(warning)
		page.controller.SetButtonState(ButtonConfirm, false)

	} else {
		page.warning.SetLabel("")
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
	return "Assign Hostname"
}

// StoreChanges will store this pages changes into the model
func (page *HostnamePage) StoreChanges() {
	host, err := getTextFromEntry(page.entry) // TODO: Handle error
	if err != nil {
		log.Fatal(err)
	}
	page.model.Hostname = host
}

// ResetChanges will reset this page to match the model
func (page *HostnamePage) ResetChanges() {
	host := page.model.Hostname
	err := setTextInEntry(page.entry, host) // TODO: Handle error
	if err != nil {
		log.Fatal(err)
	}
	page.warning.SetLabel("")
}

// GetConfiguredValue returns our current config
func (page *HostnamePage) GetConfiguredValue() string {
	if page.model.Hostname == "" {
		return "No target system hostname assigned"
	}
	return page.model.Hostname
}
