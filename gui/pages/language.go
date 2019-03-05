// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/clearlinux/clr-installer/language"
	"github.com/clearlinux/clr-installer/model"
	"github.com/gotk3/gotk3/gtk"
)

// Language is a simple page to help with Language settings
type Language struct {
	controller Controller
	model      *model.SystemInstall
	langs      []*language.Language
	selected   *language.Language
	box        *gtk.Box
	scroll     *gtk.ScrolledWindow
	list       *gtk.ListBox
}

// NewLanguagePage returns a new LanguagePage
func NewLanguagePage(controller Controller, model *model.SystemInstall) (Page, error) {
	langs, err := language.Load()
	if err != nil {
		return nil, err
	}

	language := &Language{
		controller: controller,
		model:      model,
		langs:      langs,
	}

	language.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	language.box.SetBorderWidth(8)

	// Build storage for listbox
	language.scroll, err = gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	language.box.PackStart(language.scroll, true, true, 0)
	language.scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	// Build listbox
	language.list, err = gtk.ListBoxNew()
	if err != nil {
		return nil, err
	}
	language.list.SetSelectionMode(gtk.SELECTION_SINGLE)
	language.list.SetActivateOnSingleClick(true)
	if _, err := language.list.Connect("row-activated", language.onRowActivated); err != nil {
		return nil, err
	}
	language.scroll.Add(language.list)
	// Remove background
	st, _ := language.list.GetStyleContext()
	st.AddClass("scroller-special")

	for _, lang := range language.langs {
		lab, err := gtk.LabelNew("<big>" + lang.Code + "</big>")
		if err != nil {
			return nil, err
		}
		lab.SetUseMarkup(true)
		lab.SetHAlign(gtk.ALIGN_START)
		lab.SetXAlign(0.0)
		lab.ShowAll()
		language.list.Add(lab)
	}

	return language, nil
}

func (l *Language) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	if row == nil {
		l.controller.SetButtonState(ButtonConfirm, false)
		l.selected = nil
		return
	}
	// Go activate this.
	l.selected = l.langs[row.GetIndex()]
	l.controller.SetButtonState(ButtonConfirm, true)
}

// IsRequired will return true as we always need a Language
func (l *Language) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (l *Language) IsDone() bool {
	return l.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (l *Language) GetID() int {
	return PageIDLanguage
}

// GetIcon returns the icon for this page
func (l *Language) GetIcon() string {
	return "preferences-desktop-locale"
}

// GetRootWidget returns the root embeddable widget for this page
func (l *Language) GetRootWidget() gtk.IWidget {
	return l.box
}

// GetSummary will return the summary for this page
func (l *Language) GetSummary() string {
	return "Choose Language"
}

// GetTitle will return the title for this page
func (l *Language) GetTitle() string {
	return l.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (l *Language) StoreChanges() {
	l.model.Language = l.selected
}

// ResetChanges will reset this page to match the model
func (l *Language) ResetChanges() {
	code := language.DefaultLanguage
	if l.model.Language.Code != "" {
		code = l.model.Language.Code
	}

	// Preselect the timezone here
	for n, lang := range l.langs {
		if lang.Code != code {
			continue
		}

		// Select row in the box, activate it and scroll to it
		row := l.list.GetRowAtIndex(n)
		l.list.SelectRow(row)
		l.onRowActivated(l.list, row)
		scrollToView(l.scroll, l.list, &row.Widget)
	}
}

// GetConfiguredValue returns our current config
func (l *Language) GetConfiguredValue() string {
	if l.model.Language == nil {
		return ""
	}
	return l.model.Language.Code
}
