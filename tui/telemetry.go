// Copyright © 2018 Intel Corporation
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

func newTelemetryPage(tui *Tui) (Page, error) {
	page := &TelemetryPage{
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}
	page.setupMenu(tui, TuiPageTelemetry, "Telemetry", BackButton|DoneButton, TuiPageMenu)

	// Set one blank line between items for readability
	page.content.SetGaps(0, 1)

	titleLbl := clui.CreateLabel(page.content, 2, 1, telemetry.Title, Fixed)
	titleLbl.SetMultiline(false)

	helpCnt := strings.Count(telemetry.Help, "\n")
	helpLbl := clui.CreateLabel(page.content, 2, helpCnt, telemetry.Help, Fixed)
	helpLbl.SetMultiline(true)

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

	page.backBtn.SetTitle("No, thanks")
	page.backBtn.SetSize(12, 1)

	page.doneBtn.SetTitle("Yes, enable telemetry!!")
	page.doneBtn.SetSize(25, 1)

	return page, nil
}

// DeActivate sets the model value and adjusts the "done" flag for this page
func (tp *TelemetryPage) DeActivate() {
	tp.getModel().EnableTelemetry(tp.action == ActionDoneButton)
	tp.SetDone(true)
}

// Activate activates the proper button depending on the current model value
// if telemetry is enabled in the data model then the done button will be active
// otherwise the back button will be activated.
func (tp *TelemetryPage) Activate() {
	if tp.getModel().Telemetry.Enabled {
		tp.activated = tp.doneBtn
	} else {
		tp.activated = tp.backBtn
	}
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (tp *TelemetryPage) GetConfigDefinition() int {

	if tp.getModel().Telemetry.Defined {
		return ConfigDefinedByConfig
	}

	return ConfigNotDefined
}
