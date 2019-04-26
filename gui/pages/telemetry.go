// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
)

// Telemetry is a simple page to help with Telemetry settings
type Telemetry struct {
	model      *model.SystemInstall
	controller Controller
	box        *gtk.Box
	check      *gtk.CheckButton
	didConfirm bool
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

	check, err := gtk.CheckButtonNewWithLabel(utils.Locale.Get("Enable Telemetry"))
	if err != nil {
		return nil, err
	}
	check.SetHAlign(gtk.ALIGN_CENTER)
	box.PackStart(check, true, false, 0)

	return &Telemetry{
		controller: controller,
		model:      model,
		box:        box,
		check:      check,
		didConfirm: false,
	}, nil
}

// IsRequired will return true as we always need a Telemetry
func (t *Telemetry) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (t *Telemetry) IsDone() bool {
	return t.didConfirm
}

// GetID returns the ID for this page
func (t *Telemetry) GetID() int {
	return PageIDTelemetry
}

// GetIcon returns the icon for this page
func (t *Telemetry) GetIcon() string {
	return "network-transmit-receive"
}

// GetRootWidget returns the root embeddable widget for this page
func (t *Telemetry) GetRootWidget() gtk.IWidget {
	return t.box
}

// GetSummary will return the summary for this page
func (t *Telemetry) GetSummary() string {
	return utils.Locale.Get("Telemetry")
}

// GetTitle will return the title for this page
func (t *Telemetry) GetTitle() string {
	return utils.Locale.Get("Enable Telemetry")
}

// StoreChanges will store this pages changes into the model
func (t *Telemetry) StoreChanges() {
	t.didConfirm = true
	t.model.EnableTelemetry(t.check.GetActive())
}

// ResetChanges will reset this page to match the model
func (t *Telemetry) ResetChanges() {
	t.controller.SetButtonState(ButtonConfirm, true)
	t.check.SetActive(t.model.IsTelemetryEnabled())
}

// GetConfiguredValue returns our current config
func (t *Telemetry) GetConfiguredValue() string {
	if t.model.IsTelemetryEnabled() {
		return utils.Locale.Get("Enabled")
	}
	return utils.Locale.Get("Disabled")
}

// GetTelemetryMessage gets the telemetry message
func GetTelemetryMessage(model *model.SystemInstall) string {
	text := "<big>"
	text += utils.Locale.Get("Allow the Clear Linux* OS to collect anonymous reports to improve system stability?")
	text += "</big>\n\n"
	text += utils.Locale.Get("These reports only relate to operating system details. No personally identifiable information is collected.")
	text += "\n\n"
	text += utils.Locale.Get("Intel's privacy policy can be found at: http://www.intel.com/privacy.")

	if model.Telemetry.IsRequested() {
		text = text + "\n\n\n" +
			utils.Locale.Get("NOTICE: Enabling Telemetry preferred by default on internal networks.")
	}

	return text
}
