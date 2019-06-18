// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/telemetry"
	"github.com/clearlinux/clr-installer/utils"
)

// Telemetry is a simple page to help with Telemetry settings
type Telemetry struct {
	model      *model.SystemInstall
	controller Controller
	box        *gtk.Box
	didConfirm bool
	firstLoad  bool // Keeps track if the page was loaded for the first time.
}

// NewTelemetryPage returns a new TelemetryPage
func NewTelemetryPage(controller Controller, model *model.SystemInstall) (Page, error) {
	box, err := setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page-new")
	if err != nil {
		return nil, err
	}

	label, err := gtk.LabelNew(GetTelemetryMessage(model))
	if err != nil {
		return nil, err
	}
	label.SetUseMarkup(true)
	label.SetHAlign(gtk.ALIGN_CENTER)
	label.SetVAlign(gtk.ALIGN_CENTER)
	label.SetLineWrap(true)
	label.SetMaxWidthChars(70)
	label.SetMarginBottom(20)
	box.PackStart(label, true, false, 0)

	return &Telemetry{
		controller: controller,
		model:      model,
		box:        box,
		didConfirm: false,
		firstLoad:  true,
	}, nil
}

// IsRequired will return true as we always need a Telemetry
func (page *Telemetry) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (page *Telemetry) IsDone() bool {
	return page.didConfirm
}

// GetID returns the ID for this page
func (page *Telemetry) GetID() int {
	return PageIDTelemetry
}

// GetIcon returns the icon for this page
func (page *Telemetry) GetIcon() string {
	return "network-transmit-receive"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *Telemetry) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *Telemetry) GetSummary() string {
	return utils.Locale.Get("Telemetry")
}

// GetTitle will return the title for this page
func (page *Telemetry) GetTitle() string {
	return utils.Locale.Get("Enable Telemetry")
}

// StoreChanges will store this pages changes into the model
func (page *Telemetry) StoreChanges() {
	page.didConfirm = true
	page.model.EnableTelemetry(true)
}

// ResetChanges will reset this page to match the model
func (page *Telemetry) ResetChanges() {
	if page.firstLoad {
		page.didConfirm = false
		page.firstLoad = false
	} else {
		page.didConfirm = true
	}
	page.model.EnableTelemetry(false)
}

// GetConfiguredValue returns our current config
func (page *Telemetry) GetConfiguredValue() string {
	if page.model.IsTelemetryEnabled() {
		return utils.Locale.Get("Enabled")
	}
	return utils.Locale.Get("Disabled")
}

// GetTelemetryMessage gets the telemetry message
func GetTelemetryMessage(model *model.SystemInstall) string {
	text := "<big>"
	text += utils.Locale.Get("Allow Clear Linux* OS to collect anonymized system data and usage statistics for continuous improvement?")
	text += "</big>\n\n"
	text += utils.Locale.Get("These reports only relate to operating system details. No personally identifiable information is collected.")
	text += "\n\n"
	text += utils.Locale.Get("See %s for more information.", telemetry.TelemetryAboutURL)
	text += "\n\n"
	text += utils.Locale.Get(telemetry.Policy)

	if model.Telemetry.IsRequested() {
		text = text + "\n\n\n" +
			utils.Locale.Get(telemetry.RequestNotice)
	}

	return text
}
