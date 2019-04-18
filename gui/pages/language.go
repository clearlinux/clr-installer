// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"fmt"
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/language"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
)

// LanguagePage is a simple page to help with LanguagePage settings
type LanguagePage struct {
	controller  Controller
	model       *model.SystemInstall
	data        []*language.Language
	selected    *language.Language
	box         *gtk.Box
	searchEntry *gtk.SearchEntry
	scroll      *gtk.ScrolledWindow
	list        *gtk.ListBox
}

// NewLanguagePage returns a new LanguagePage
func NewLanguagePage(controller Controller, model *model.SystemInstall) (Page, error) {
	data, err := language.Load()
	if err != nil {
		return nil, err
	}

	page := &LanguagePage{
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
		desc, code := v.GetConfValues()

		box, err := setBox(gtk.ORIENTATION_VERTICAL, 0, "box-list-label")
		if err != nil {
			return nil, err
		}

		labelDesc, err := setLabel(desc, "list-label-description", 0.0)
		if err != nil {
			return nil, err
		}
		box.PackStart(labelDesc, false, false, 0)

		labelCode, err := setLabel(code, "list-label-code", 0.0)
		if err != nil {
			return nil, err
		}
		box.PackStart(labelCode, false, false, 0)

		page.list.Add(box)
	}

	return page, nil
}

func (page *LanguagePage) getCode() string {
	code := ""
	if page.model.Language != nil {
		code = page.model.Language.Code
	}

	if code == "" {
		code = language.DefaultLanguage
	}

	return code
}

func (page *LanguagePage) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	page.selected = page.data[row.GetIndex()]
	page.controller.SetButtonState(ButtonNext, true)
}

// Select row in the box, activate it and scroll to it
func (page *LanguagePage) activateRow(index int) {
	row := page.list.GetRowAtIndex(index)
	page.list.SelectRow(row)
	page.onRowActivated(page.list, row)
	scrollToView(page.scroll, page.list, &row.Widget)
}

func (page *LanguagePage) onChange(entry *gtk.SearchEntry) {
	search := getTextFromSearchEntry(entry)

	var setIndex bool
	var index int
	code := page.getCode() // Get current language
	for i, v := range page.data {
		vDesc, vCode := v.GetConfValues()
		term := fmt.Sprintf("%s %s", vDesc, vCode)
		if search != "" && !strings.Contains(strings.ToLower(term), strings.ToLower(search)) {
			page.list.GetRowAtIndex(i).Hide()
		} else {
			page.list.GetRowAtIndex(i).Show()
			if search == "" { // Get index of current language
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
		page.controller.SetButtonState(ButtonNext, false)
	}
}

// IsRequired will return true as we always need a LanguagePage
func (page *LanguagePage) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (page *LanguagePage) IsDone() bool {
	return page.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (page *LanguagePage) GetID() int {
	return PageIDWelcome
}

// GetIcon returns the icon for this page
func (page *LanguagePage) GetIcon() string {
	return "preferences-desktop-locale"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *LanguagePage) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *LanguagePage) GetSummary() string {
	return utils.Locale.Get("Select Language")
}

// GetTitle will return the title for this page
func (page *LanguagePage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *LanguagePage) StoreChanges() {
	page.controller.SetButtonState(ButtonNext, false) // TODO: Determine why the button is not actually being disabled
	page.model.Language = page.selected
	language.SetSelectionLanguage(page.model.Language.Code)
	utils.SetLocale(page.model.Language.Code)
}

// ResetChanges will reset this page to match the model
func (page *LanguagePage) ResetChanges() {
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
func (page *LanguagePage) GetConfiguredValue() string {
	if page.model.Language == nil {
		return ""
	}
	desc, code := page.model.Language.GetConfValues()
	return fmt.Sprintf("%s  [%s]", desc, code)
}
