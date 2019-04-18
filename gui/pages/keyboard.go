// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/keyboard"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
)

// KeyboardPage is a simple page to help with KeyboardPage settings
type KeyboardPage struct {
	controller  Controller
	model       *model.SystemInstall
	data        []*keyboard.Keymap
	selected    *keyboard.Keymap
	box         *gtk.Box
	searchEntry *gtk.SearchEntry
	scroll      *gtk.ScrolledWindow
	list        *gtk.ListBox
}

// NewKeyboardPage returns a new KeyboardPage
func NewKeyboardPage(controller Controller, model *model.SystemInstall) (Page, error) {
	data, err := keyboard.LoadKeymaps()
	if err != nil {
		return nil, err
	}

	page := &KeyboardPage{
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

func (page *KeyboardPage) getCode() string {
	code := page.GetConfiguredValue()
	if code == "" {
		code = keyboard.DefaultKeyboard
	}
	return code
}

func (page *KeyboardPage) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	page.selected = page.data[row.GetIndex()]
	page.controller.SetButtonState(ButtonConfirm, true)
}

// Select row in the box, activate it and scroll to it
func (page *KeyboardPage) activateRow(index int) {
	row := page.list.GetRowAtIndex(index)
	page.list.SelectRow(row)
	page.onRowActivated(page.list, row)
	scrollToView(page.scroll, page.list, &row.Widget)
}

func (page *KeyboardPage) onChange(entry *gtk.SearchEntry) {
	var setIndex bool
	var index int
	search := getTextFromSearchEntry(entry)
	code := page.getCode() // Get current keyboard
	for i, v := range page.data {
		if search != "" && !strings.Contains(strings.ToLower(v.Code), strings.ToLower(search)) {
			page.list.GetRowAtIndex(i).Hide()
		} else {
			page.list.GetRowAtIndex(i).Show()
			if search == "" { // Get index of current keyboard
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
}

// IsRequired will return true as we always need a KeyboardPage
func (page *KeyboardPage) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (page *KeyboardPage) IsDone() bool {
	return page.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (page *KeyboardPage) GetID() int {
	return PageIDKeyboard
}

// GetIcon returns the icon for this page
func (page *KeyboardPage) GetIcon() string {
	return "preferences-desktop-keyboard-shortcuts"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *KeyboardPage) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *KeyboardPage) GetSummary() string {
	return utils.Locale.Get("Select Keyboard")
}

// GetTitle will return the title for this page
func (page *KeyboardPage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *KeyboardPage) StoreChanges() {
	page.model.Keyboard = page.selected
}

// ResetChanges will reset this page to match the model
func (page *KeyboardPage) ResetChanges() {
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
func (page *KeyboardPage) GetConfiguredValue() string {
	if page.model.Keyboard == nil {
		return ""
	}
	return page.model.Keyboard.Code
}
