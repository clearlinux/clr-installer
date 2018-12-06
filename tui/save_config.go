// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/VladimirMarkelov/clui"

	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/log"
)

// SaveConfigPage is a special PopUp Page implementation which implements
// the Page interface, but does not allocate a Window. Instead it will launch
// a modal PopUp window.
type SaveConfigPage struct {
	tui        *Tui        // the Tui frontend reference
	menuTitle  string      // the title used to show on main menu
	done       bool        // marks if an item is completed
	id         int         // the page id
	data       interface{} // arbitrary page context data
	required   bool        // marks if an item is required for the install
	menuButton *MenuButton // the menu button reference
}

// GetID returns the current page's identifier
func (page *SaveConfigPage) GetID() int {
	return page.id
}

// IsRequired returns if this Page is required to be completed for the Install
func (page *SaveConfigPage) IsRequired() bool {
	return page.required
}

// GetWindow returns the current page's window control
func (page *SaveConfigPage) GetWindow() *clui.Window {
	return nil
}

// GetActivated returns the control set as activated for a page
func (page *SaveConfigPage) GetActivated() clui.Control {
	return nil
}

// GetMenuTitle returns the current page's title string
func (page *SaveConfigPage) GetMenuTitle() string {
	return page.menuTitle
}

// SetData set the current value for the page's data member
func (page *SaveConfigPage) SetData(data interface{}) {
	page.data = data
}

// GetData returns the current value of a page's data member
func (page *SaveConfigPage) GetData() interface{} {
	return page.data
}

// SetDone sets the page's done flag
func (page *SaveConfigPage) SetDone(done bool) bool {
	page.done = done
	return true
}

// GetDone returns the current value of a page's done flag
func (page *SaveConfigPage) GetDone() bool {
	return true
}

// Activate resets the page state
func (page *SaveConfigPage) Activate() {
	msg := ""
	if page.tui.model == nil {
		msg = "Model not configured"
		log.Warning("Attempt to save config: %s", msg)
		if _, err := CreateWarningDialogBox(msg); err != nil {
			log.Warning("Attempt to save config: warning dialog failed: %s", err)
		}
	}
	if saveErr := page.tui.model.WriteFile(conf.ConfigFile); saveErr != nil {
		msg = fmt.Sprintf("Failed to save config file: %v", saveErr)
		log.Warning("Attempt to save config: %s", msg)
		if _, err := CreateWarningDialogBox(msg); err != nil {
			log.Warning("Attempt to save config: warning dialog failed: %s", err)
		}
	} else {
		msg = fmt.Sprintf("Saved configuration to %q", conf.ConfigFile)
		if dialog, err := CreateInfoDialogBox(msg); err == nil {
			dialog.OnClose(func() {
				page.tui.gotoPage(TuiPageMenu, page.tui.currPage)
			})
		} else {
			log.Warning("Attempt to save config: info dialog failed: %s", err)
		}
	}
}

// DeActivate is a stub implementation for the DeActivate method of Page interface
func (page *SaveConfigPage) DeActivate() {}

// GetConfigDefinition is a stub implementation
// the real implementation must check with the model and return:
//    + ConfigDefinedByUser: if the configuration was interactively defined by the user
//    + ConfigDefinedByConfig: if the configuration was provided by a config file
//    + ConfigNotDefined: if none of the above apply
func (page *SaveConfigPage) GetConfigDefinition() int {
	return ConfigDefinedByUser
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *SaveConfigPage) GetConfiguredValue() string {
	return "Save the currently defined configuration to YAML file"
}

// GetMenuStatus returns the menu button status id
func (page *SaveConfigPage) GetMenuStatus(item Page) int {
	return MenuButtonStatusDefault
}

// GetMenuButton is a page implementation for network validate popup
func (page *SaveConfigPage) GetMenuButton() *MenuButton {
	return page.menuButton
}

// SetMenuButton is a no-op page implementation for network validate popup
func (page *SaveConfigPage) SetMenuButton(mb *MenuButton) {
	page.menuButton = mb
}

func newSaveConfigPage(tui *Tui) (Page, error) {
	page := &SaveConfigPage{}
	page.tui = tui
	page.menuTitle = "Save Configuration Settings"
	page.id = TuiPageSaveConfig

	return page, nil
}
