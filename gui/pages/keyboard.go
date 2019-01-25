// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/clearlinux/clr-installer/keyboard"
	"github.com/clearlinux/clr-installer/model"
	"github.com/gotk3/gotk3/gtk"
)

// Keyboard is a simple page to help with Keyboard settings
type Keyboard struct {
	controller Controller
	model      *model.SystemInstall
	keymaps    []*keyboard.Keymap
	selected   *keyboard.Keymap
	box        *gtk.Box
	scroll     *gtk.ScrolledWindow
	list       *gtk.ListBox
}

// NewKeyboardPage returns a new KeyboardPage
func NewKeyboardPage(controller Controller, model *model.SystemInstall) (Page, error) {
	keymaps, err := keyboard.LoadKeymaps()
	if err != nil {
		return nil, err
	}

	keyboard := &Keyboard{
		controller: controller,
		model:      model,
		keymaps:    keymaps,
	}

	keyboard.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	keyboard.box.SetBorderWidth(8)

	// Build storage for listbox
	keyboard.scroll, err = gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	keyboard.box.PackStart(keyboard.scroll, true, true, 0)
	keyboard.scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	// Build listbox
	keyboard.list, err = gtk.ListBoxNew()
	if err != nil {
		return nil, err
	}
	keyboard.list.SetSelectionMode(gtk.SELECTION_SINGLE)
	keyboard.list.SetActivateOnSingleClick(true)
	keyboard.list.Connect("row-activated", keyboard.onRowActivated)
	keyboard.scroll.Add(keyboard.list)
	// Remove background
	st, _ := keyboard.list.GetStyleContext()
	st.AddClass("scroller-special")

	for _, kmap := range keyboard.keymaps {
		lab, err := gtk.LabelNew("<big>" + kmap.Code + "</big>")
		if err != nil {
			return nil, err
		}
		lab.SetUseMarkup(true)
		lab.SetHAlign(gtk.ALIGN_START)
		lab.SetXAlign(0.0)
		lab.ShowAll()
		keyboard.list.Add(lab)
	}

	return keyboard, nil
}

func (k *Keyboard) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	if row == nil {
		k.controller.SetButtonState(ButtonConfirm, false)
		k.selected = nil
		return
	}
	// Go activate this.
	k.selected = k.keymaps[row.GetIndex()]
	k.controller.SetButtonState(ButtonConfirm, true)
}

// IsRequired will return true as we always need a Keyboard
func (k *Keyboard) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (k *Keyboard) IsDone() bool {
	return k.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (k *Keyboard) GetID() int {
	return PageIDKeyboard
}

// GetIcon returns the icon for this page
func (k *Keyboard) GetIcon() string {
	return "preferences-desktop-keyboard-shortcuts"
}

// GetRootWidget returns the root embeddable widget for this page
func (k *Keyboard) GetRootWidget() gtk.IWidget {
	return k.box
}

// GetSummary will return the summary for this page
func (k *Keyboard) GetSummary() string {
	return "Configure the Keyboard"
}

// GetTitle will return the title for this page
func (k *Keyboard) GetTitle() string {
	return k.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (k *Keyboard) StoreChanges() {
	k.model.Keyboard = k.selected
}

// ResetChanges will reset this page to match the model
func (k *Keyboard) ResetChanges() {
	code := keyboard.DefaultKeyboard
	if k.model.Keyboard.Code != "" {
		code = k.model.Keyboard.Code
	}

	// Preselect the timezone here
	for n, kb := range k.keymaps {
		if kb.Code != code {
			continue
		}

		// Select row in the box, activate it and scroll to it
		row := k.list.GetRowAtIndex(n)
		k.list.SelectRow(row)
		k.onRowActivated(k.list, row)
		scrollToView(k.scroll, k.list, &row.Widget)
	}
}

// GetConfiguredValue returns our current config
func (k *Keyboard) GetConfiguredValue() string {
	if k.model.Keyboard == nil {
		return ""
	}
	return k.model.Keyboard.Code
}
