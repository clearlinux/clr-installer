// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/language"
)

// LanguagePage is the Page implementation for the language configuration page
type LanguagePage struct {
	BasePage
	avLanguages []*language.Language
	langListBox *clui.ListBox
}

// GetConfiguredValue Returns the string representation of currently language set
func (page *LanguagePage) GetConfiguredValue() string {
	desc, code := page.getModel().Language.GetConfValues()
	return fmt.Sprintf("%-14s  %s", "["+code+"]", desc)
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *LanguagePage) GetConfigDefinition() int {
	lang := page.getModel().Language

	if lang == nil {
		return ConfigNotDefined
	} else if lang.IsUserDefined() {
		return ConfigDefinedByUser
	}

	return ConfigDefinedByConfig
}

// SetDone sets the keyboard page flag done, and sets back the configuration to the data model
func (page *LanguagePage) SetDone(done bool) bool {
	page.done = done
	page.getModel().Language = page.avLanguages[page.langListBox.SelectedItem()]
	return true
}

// DeActivate will reset the selection case the user has pressed cancel
func (page *LanguagePage) DeActivate() {
	if page.action == ActionConfirmButton {
		return
	}

	for idx, curr := range page.avLanguages {
		if !curr.Equals(page.getModel().Language) {
			continue
		}

		page.langListBox.SelectItem(idx)
		return
	}
}

func newLanguagePage(tui *Tui) (Page, error) {
	avLanguages, err := language.Load()
	if err != nil {
		return nil, err
	}

	page := &LanguagePage{
		avLanguages: avLanguages,
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}

	page.setupMenu(tui, TuiPageLanguage, "Choose Language", ConfirmButton|CancelButton, TuiPageMenu)

	lbl := clui.CreateLabel(page.content, 2, 2, "Select System Language", Fixed)
	lbl.SetPaddings(0, 2)

	page.langListBox = clui.CreateListBox(page.content, AutoSize, ContentHeight-1, Fixed)
	page.langListBox.SetStyle("List")

	page.langListBox.OnActive(func(active bool) {
		if active {
			page.langListBox.SetStyle("ListActive")
		} else {
			page.langListBox.SetStyle("List")
		}
	})

	page.langListBox.OnKeyPress(func(k term.Key) bool {
		if k == term.KeyEnter {
			if page.confirmBtn != nil {
				page.confirmBtn.ProcessEvent(clui.Event{Type: clui.EventKey, Key: k})
			}
			return true
		}

		return false
	})

	modelLanguage := page.getModel().Language
	defLanguage := 0
	for idx, curr := range page.avLanguages {
		desc, code := curr.GetConfValues()
		page.langListBox.AddItem(fmt.Sprintf("%-14s  %s", "["+code+"]", desc))

		if curr.Equals(modelLanguage) {
			defLanguage = idx
			modelLanguage.Tag = curr.Tag
		}
	}

	if len(page.avLanguages) > 0 {
		page.langListBox.SelectItem(defLanguage)
		page.activated = page.confirmBtn
	} else {
		page.langListBox.AddItem("No language data found: Defaulting to '" + language.DefaultLanguage + "'")
		page.activated = page.cancelBtn
		page.confirmBtn.SetEnabled(false)
	}

	return page, nil
}
