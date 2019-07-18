// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"strings"

	"github.com/VladimirMarkelov/clui"

	"github.com/clearlinux/clr-installer/telemetry"
)

// TelemetryPage is the Page implementation for the telemetry configuration page
type TelemetryPage struct {
	BasePage
}

// GetDone returns the current value of a page's done flag
func (tp *TelemetryPage) GetDone() bool {
	return tp.getModel().Telemetry.IsUserDefined()
}

// GetConfiguredValue Returns the string representation of currently value set
func (tp *TelemetryPage) GetConfiguredValue() string {
	if tp.getModel().Telemetry.IsUserDefined() {
		if tp.getModel().IsTelemetryEnabled() {
			return "Enabled"
		}
		return "Disabled"
	}
	return "No choice made"
}

func newTelemetryPage(tui *Tui) (Page, error) {
	page := &TelemetryPage{
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}
	page.setupMenu(tui, TuiPageTelemetry, "Telemetry", BackButton|ConfirmButton, TuiPageMenu)

	// Set one blank line between items for readability
	page.content.SetGaps(0, 1)

	titleLbl := clui.CreateLabel(page.content, 2, 1, telemetry.Title, Fixed)
	titleLbl.SetMultiline(false)

	helpCnt := strings.Count(telemetry.Help, "\n")
	helpLbl := clui.CreateLabel(page.content, 2, helpCnt, telemetry.Help, Fixed)
	helpLbl.SetMultiline(true)

	aboutLbl := clui.CreateLabel(page.content, 2, 2, "For more details, see: \n"+telemetry.TelemetryAboutURL, Fixed)
	aboutLbl.SetMultiline(true)

	lastWidth, _ := helpLbl.Size()
	policyLength := len(telemetry.Policy)
	estHeight := policyLength / lastWidth
	if policyLength%lastWidth != -1 {
		estHeight++
	}
	if estHeight > 6 { // maximum space for Policy
		estHeight = 6
	}
	policyLbl := clui.CreateLabel(page.content, 2, estHeight, telemetry.Policy, Fixed)
	policyLbl.SetMultiline(true)

	md := page.getModel()
	if md.Telemetry.IsRequested() {
		noticeLbl := clui.CreateLabel(page.content, 2, 2, telemetry.RequestNotice, Fixed)
		noticeLbl.SetMultiline(true)
	}

	page.backBtn.SetTitle("No")
	page.backBtn.SetSize(12, 1)

	page.confirmBtn.SetTitle("Yes")
	page.confirmBtn.SetSize(12, 1)

	return page, nil
}

// DeActivate sets the model value and adjusts the "confirm" flag for this page
func (tp *TelemetryPage) DeActivate() {
	model := tp.getModel()

	if tp.action == ActionConfirmButton {
		model.EnableTelemetry(true)
	} else if tp.action == ActionBackButton {
		model.EnableTelemetry(false)
	}

	tp.SetDone(true)
	model.Telemetry.SetUserDefined(true)
}

// Activate activates the proper button depending on the current model value
// if telemetry is enabled in the data model then the confirm button will be active
// otherwise the back button will be activated.
func (tp *TelemetryPage) Activate() {
	if tp.getModel().Telemetry.Enabled {
		tp.activated = tp.confirmBtn
	} else {
		tp.activated = tp.backBtn
	}
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (tp *TelemetryPage) GetConfigDefinition() int {
	return ConfigNotDefined
}
