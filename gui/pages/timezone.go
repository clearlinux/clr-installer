// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/timezone"
)

// TimezonePage is a simple page to help with TimezonePage settings
type TimezonePage struct {
	controller  Controller
	model       *model.SystemInstall
	data        []*timezone.TimeZone
	selected    *timezone.TimeZone
	box         *gtk.Box
	searchEntry *gtk.SearchEntry
	scroll      *gtk.ScrolledWindow
	list        *gtk.ListBox
}

// NewTimezonePage returns a new TimezonePage
func NewTimezonePage(controller Controller, model *model.SystemInstall) (Page, error) {
	data, err := timezone.Load()
	if err != nil {
		return nil, err
	}

	page := &TimezonePage{
		controller: controller,
		model:      model,
		data:       data,
	}

	// Box
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page")
	if err != nil {
		return nil, err
	}

	// SearchEntry
	page.searchEntry, err = setSearchEntry("search-entry")
	if err != nil {
		return nil, err
	}
	page.box.PackStart(page.searchEntry, false, false, 0)
	if _, err := page.searchEntry.Connect("search-changed", page.onChange); err != nil {
		return nil, err
	}

	// ScrolledWindow
	page.scroll, err = setScrolledWindow(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC, "scroller")
	if err != nil {
		return nil, err
	}
	page.box.PackStart(page.scroll, true, true, 5)

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
		box, err := setBox(gtk.ORIENTATION_VERTICAL, 0, "box-list-label")
		if err != nil {
			return nil, err
		}

		labelDesc, err := setLabel(v.Code, "list-label-description", 0.0)
		if err != nil {
			return nil, err
		}
		box.PackStart(labelDesc, false, false, 0)

		page.list.Add(box)
	}

	return page, nil
}

func (page *TimezonePage) getCode() string {
	code := page.GetConfiguredValue()
	if code == "" {
		code = timezone.DefaultTimezone
	}
	return code
}

func (page *TimezonePage) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	page.selected = page.data[row.GetIndex()]
	page.controller.SetButtonState(ButtonConfirm, true)
}

// Select row in the box, activate it and scroll to it
func (page *TimezonePage) activateRow(index int) {
	row := page.list.GetRowAtIndex(index)
	page.list.SelectRow(row)
	page.onRowActivated(page.list, row)
	scrollToView(page.scroll, page.list, &row.Widget)
}

func (page *TimezonePage) onChange(entry *gtk.SearchEntry) error {
	search, err := getTextFromSearchEntry(entry)
	if err != nil {
		return err
	}

	var setIndex bool
	var index int
	code := page.getCode() // Get current timezone
	for i, v := range page.data {
		if search != "" && !strings.Contains(strings.ToLower(v.Code), strings.ToLower(search)) {
			page.list.GetRowAtIndex(i).Hide()
		} else {
			page.list.GetRowAtIndex(i).Show()
			if search == "" { // Get index of current timezone
				if v.Code == code {
					index = i
					setIndex = true
				}
			} else { // Get index of first item in list
				if setIndex == false {
					index = i
					setIndex = true
				}
			}
		}
	}
	if setIndex == true {
		page.activateRow(index)
	} else {
		page.selected = nil
		page.controller.SetButtonState(ButtonConfirm, false)
	}
	return nil
}

// IsRequired will return true as we always need a TimezonePage
func (page *TimezonePage) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (page *TimezonePage) IsDone() bool {
	return page.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (page *TimezonePage) GetID() int {
	return PageIDTimezone
}

// GetIcon returns the icon for this page
func (page *TimezonePage) GetIcon() string {
	return "preferences-system-time"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *TimezonePage) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *TimezonePage) GetSummary() string {
	return "Choose Timezone"
}

// GetTitle will return the title for this page
func (page *TimezonePage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *TimezonePage) StoreChanges() {
	page.model.Timezone = page.selected
}

// ResetChanges will reset this page to match the model
func (page *TimezonePage) ResetChanges() {
	code := page.getCode()
	for i, v := range page.data {
		if v.Code == code {
			page.activateRow(i)
			break
		}
	}
	page.searchEntry.SetText("")
}

// GetConfiguredValue returns our current config
func (page *TimezonePage) GetConfiguredValue() string {
	if page.model.Timezone == nil {
		return ""
	}
	return page.model.Timezone.Code
}
