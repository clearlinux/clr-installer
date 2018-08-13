// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/clearlinux/clr-installer/timezone"

	"github.com/VladimirMarkelov/clui"
)

// TimezonePage is the Page implementation for the timezone configuration page
type TimezonePage struct {
	BasePage
	avTimezones []*timezone.TimeZone
	tzListBox   *clui.ListBox
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *TimezonePage) GetConfigDefinition() int {
	tz := page.getModel().Timezone

	if tz == nil {
		return ConfigNotDefined
	} else if tz.IsUserDefined() {
		return ConfigDefinedByUser
	}

	return ConfigDefinedByConfig
}

// SetDone sets the selected timezone to data model
func (page *TimezonePage) SetDone(done bool) bool {
	page.done = done
	page.getModel().Timezone = page.avTimezones[page.tzListBox.SelectedItem()]
	return true
}

// DeActivate will reset the selection case the user has pressed cancel
func (page *TimezonePage) DeActivate() {
	if page.action == ActionDoneButton {
		return
	}

	for idx, curr := range page.avTimezones {
		if !curr.Equals(page.getModel().Timezone) {
			continue
		}

		page.tzListBox.SelectItem(idx)
		return
	}
}

func newTimezonePage(tui *Tui) (Page, error) {
	avTimezones, err := timezone.Load()
	if err != nil {
		return nil, err
	}

	page := &TimezonePage{
		avTimezones: avTimezones,
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}

	page.setupMenu(tui, TuiPageTimezone, "Choose Timezone", DoneButton|CancelButton, TuiPageMenu)

	lbl := clui.CreateLabel(page.content, 2, 2, "Select System Timezone", Fixed)
	lbl.SetPaddings(0, 2)

	page.tzListBox = clui.CreateListBox(page.content, AutoSize, 17, Fixed)

	defTimezone := 0
	for idx, curr := range page.avTimezones {
		page.tzListBox.AddItem(curr.Code)

		if curr.Equals(page.getModel().Timezone) {
			defTimezone = idx
		}
	}

	page.tzListBox.SelectItem(defTimezone)
	page.activated = page.doneBtn

	return page, nil
}
