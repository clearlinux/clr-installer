// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/timezone"
	"github.com/gotk3/gotk3/gtk"
)

// Timezone is a simple page to help with timezone settings
type Timezone struct {
	controller Controller
	model      *model.SystemInstall
	timezones  []*timezone.TimeZone
	box        *gtk.Box
	scroll     *gtk.ScrolledWindow
	list       *gtk.ListBox
	selected   *timezone.TimeZone
}

// NewTimezonePage returns a new TimezonePage
func NewTimezonePage(controller Controller, model *model.SystemInstall) (Page, error) {
	tzones, err := timezone.Load()
	if err != nil {
		return nil, err
	}

	t := &Timezone{
		controller: controller,
		model:      model,
		timezones:  tzones,
	}

	t.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	t.box.SetBorderWidth(8)

	// Build storage for listbox
	t.scroll, err = gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	t.box.PackStart(t.scroll, true, true, 0)
	t.scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	// Build listbox
	t.list, err = gtk.ListBoxNew()
	if err != nil {
		return nil, err
	}
	t.list.SetSelectionMode(gtk.SELECTION_SINGLE)
	t.list.SetActivateOnSingleClick(true)
	t.list.Connect("row-activated", t.onRowActivated)
	t.scroll.Add(t.list)
	// Remove background
	st, _ := t.list.GetStyleContext()
	st.AddClass("scroller-special")

	for _, zone := range t.timezones {
		lab, err := gtk.LabelNew("<big>" + zone.Code + "</big>")
		if err != nil {
			return nil, err
		}
		lab.SetUseMarkup(true)
		lab.SetHAlign(gtk.ALIGN_START)
		lab.SetXAlign(0.0)
		lab.ShowAll()
		t.list.Add(lab)
	}

	return t, nil
}

func (t *Timezone) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	if row == nil {
		t.controller.SetButtonState(ButtonConfirm, false)
		t.selected = nil
		return
	}
	// Go activate this.
	t.selected = t.timezones[row.GetIndex()]
	t.controller.SetButtonState(ButtonConfirm, true)
}

// IsRequired will return true as we always need a timezone
func (t *Timezone) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (t *Timezone) IsDone() bool {
	return t.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (t *Timezone) GetID() int {
	return PageIDTimezone
}

// GetIcon returns the icon for this page
func (t *Timezone) GetIcon() string {
	return "preferences-system-time"
}

// GetRootWidget returns the root embeddable widget for this page
func (t *Timezone) GetRootWidget() gtk.IWidget {
	return t.box
}

// GetSummary will return the summary for this page
func (t *Timezone) GetSummary() string {
	return "Choose Timezone"
}

// GetTitle will return the title for this page
func (t *Timezone) GetTitle() string {
	return t.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (t *Timezone) StoreChanges() {
	t.model.Timezone = t.selected
}

// ResetChanges will reset this page to match the model
func (t *Timezone) ResetChanges() {
	code := timezone.DefaultTimezone
	if t.model.Timezone.Code != "" {
		code = t.model.Timezone.Code
	}

	// Preselect the timezone here
	for n, tz := range t.timezones {
		if tz.Code != code {
			continue
		}

		// Select row in the box, activate it and scroll to it
		row := t.list.GetRowAtIndex(n)
		t.list.SelectRow(row)
		scrollToView(t.scroll, t.list, &row.Widget)
		t.onRowActivated(t.list, row)
	}
}

// GetConfiguredValue returns our current config
func (t *Timezone) GetConfiguredValue() string {
	if t.model.Timezone == nil {
		return ""
	}
	return t.model.Timezone.Code
}
