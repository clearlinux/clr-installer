// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/clearlinux/clr-installer/keyboard"

	"github.com/VladimirMarkelov/clui"
)

// KeyboardPage is the Page implementation for the keyboard configuration page
type KeyboardPage struct {
	BasePage
	avKeymaps  []*keyboard.Keymap
	kbdListBox *clui.ListBox
}

// GetConfiguredValue Returns the string representation of currently keyboard set
func (page *KeyboardPage) GetConfiguredValue() string {
	return page.getModel().Keyboard.Code
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *KeyboardPage) GetConfigDefinition() int {
	kbd := page.getModel().Keyboard

	if kbd == nil {
		return ConfigNotDefined
	} else if kbd.IsUserDefined() {
		return ConfigDefinedByUser
	}

	return ConfigDefinedByConfig
}

// SetDone sets the keyboard page flag done, and sets back the configuration to the data model
func (page *KeyboardPage) SetDone(done bool) bool {
	page.done = done
	page.getModel().Keyboard = page.avKeymaps[page.kbdListBox.SelectedItem()]
	return true
}

// DeActivate will reset the selection case the user has pressed cancel
func (page *KeyboardPage) DeActivate() {
	if page.action == ActionDoneButton {
		return
	}

	for idx, curr := range page.avKeymaps {
		if !curr.Equals(page.getModel().Keyboard) {
			continue
		}

		page.kbdListBox.SelectItem(idx)

		if err := keyboard.Apply(curr); err != nil {
			page.Panic(err)
		}

		return
	}
}

func newKeyboardPage(tui *Tui) (Page, error) {
	kmaps, err := keyboard.LoadKeymaps()
	if err != nil {
		return nil, err
	}

	page := &KeyboardPage{
		avKeymaps: kmaps,
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}

	page.setupMenu(tui, TuiPageKeyboard, "Configure the Keyboard",
		DoneButton|CancelButton, TuiPageMenu)

	lbl := clui.CreateLabel(page.content, 2, 2, "Select Keyboard", Fixed)
	lbl.SetPaddings(0, 2)

	page.kbdListBox = clui.CreateListBox(page.content, AutoSize, 10, Fixed)
	page.kbdListBox.SetStyle("List")

	defKeyboard := 0
	for idx, curr := range page.avKeymaps {
		page.kbdListBox.AddItem(curr.Code)

		if curr.Equals(page.getModel().Keyboard) {
			defKeyboard = idx
		}
	}
	page.kbdListBox.SelectItem(defKeyboard)
	page.kbdListBox.OnActive(func(active bool) {
		if active {
			page.kbdListBox.SetStyle("ListActive")
			return
		}

		page.kbdListBox.SetStyle("List")

		idx := page.kbdListBox.SelectedItem()
		selected := page.avKeymaps[idx]

		if page.getModel().Language.Code == selected.Code {
			return
		}

		if err := keyboard.Apply(selected); err != nil {
			page.Panic(err)
		}
	})

	frame := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frame.SetPack(clui.Vertical)

	lbl = clui.CreateLabel(frame, AutoSize, 1, "Test keyboard", Fixed)
	lbl.SetPaddings(0, 1)

	newEditField(frame, false, nil)

	page.activated = page.doneBtn

	return page, nil
}
