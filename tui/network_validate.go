// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"time"

	"github.com/VladimirMarkelov/clui"

	"github.com/clearlinux/clr-installer/controller"
)

// NetworkValidatePage is a special PopUp Page implementation which implements
// the Page interface, but does not allocate a Window. Instead it will launch
// a modal PopUp window.
type NetworkValidatePage struct {
	tui       *Tui        // the Tui frontend reference
	menuTitle string      // the title used to show on main menu
	done      bool        // marks if an item is completed
	id        int         // the page id
	data      interface{} // arbitrary page context data
	required  bool        // marks if an item is required for the install
}

// GetID returns the current page's identifier
func (page *NetworkValidatePage) GetID() int {
	return page.id
}

// IsRequired returns if this Page is required to be completed for the Install
func (page *NetworkValidatePage) IsRequired() bool {
	return page.required
}

// GetWindow returns the current page's window control
func (page *NetworkValidatePage) GetWindow() *clui.Window {
	return nil
}

// GetActivated returns the control set as activated for a page
func (page *NetworkValidatePage) GetActivated() clui.Control {
	return nil
}

// GetMenuTitle returns the current page's title string
func (page *NetworkValidatePage) GetMenuTitle() string {
	return page.menuTitle
}

// SetData set the current value for the page's data member
func (page *NetworkValidatePage) SetData(data interface{}) {
	page.data = data
}

// GetData returns the current value of a page's data member
func (page *NetworkValidatePage) GetData() interface{} {
	return page.data
}

// SetDone sets the page's done flag
func (page *NetworkValidatePage) SetDone(done bool) bool {
	page.done = done
	return true
}

// GetDone returns the current value of a page's done flag
func (page *NetworkValidatePage) GetDone() bool {
	return controller.NetworkPassing
}

// Activate resets the page state
func (page *NetworkValidatePage) Activate() {
	if dialog, err := CreateNetworkTestDialogBox(page.tui.model); err == nil {
		dialog.OnClose(func() {
			page.tui.gotoPage(TuiPageAdvancedMenu, page.tui.currPage)
		})

		result := dialog.RunNetworkTest()
		page.SetDone(result)
		if result {
			// Automatically close if it worked
			clui.RefreshScreen()
			time.Sleep(time.Second)
			dialog.Close()
		}
	}
}

// DeActivate is a stub implementation for the DeActivate method of Page interface
func (page *NetworkValidatePage) DeActivate() {}

// GetConfigDefinition is a stub implementation
// the real implementation must check with the model and return:
//    + ConfigDefinedByUser: if the configuration was interactively defined by the user
//    + ConfigDefinedByConfig: if the configuration was provided by a config file
//    + ConfigNotDefined: if none of the above apply
func (page *NetworkValidatePage) GetConfigDefinition() int {
	return ConfigDefinedByUser
}

// GetButtonPrefix returns string for prefixing a menu button
func (page *NetworkValidatePage) GetButtonPrefix(item Page) string {
	prefix := MenuButtonPrefixUncompleted

	if item.GetDone() {
		prefix = MenuButtonPrefixCompletedByUser
	}

	return prefix
}

func newNetworkValidatePage(tui *Tui) (Page, error) {
	page := &NetworkValidatePage{}
	page.tui = tui
	page.menuTitle = "Test Network Settings"
	page.id = TuiPageNetworkValidate

	return page, nil
}
