// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/VladimirMarkelov/clui"
)

// AutoUpdatePage is the Page implementation for the auto update enable configuration page
type AutoUpdatePage struct {
	BasePage
}

const (
	autoUpdateHelp = `
Automatic OS Updates

Allow the Clear Linux OS to continuously update as new versions are
released. This is the default, preferred behavior for Clear Linux OS
to ensure the latest security concerns are always addressed.

See
https://clearlinux.org/documentation/clear-linux/concepts/swupd-about
for more information.

WARNING: Disabling Automatic OS Updates puts your system at risk of
missing critical security patches.
To enable post-installation, use:
    # swupd autoupdate --enable

`
)

// GetConfiguredValue Returns the string representation of currently value set
func (aup *AutoUpdatePage) GetConfiguredValue() string {
	if aup.getModel().AutoUpdate {
		return "Enabled"
	}
	return "Disabled"
}

func newAutoUpdatePage(tui *Tui) (Page, error) {
	page := &AutoUpdatePage{}

	page.setupMenu(tui, TuiPageAutoUpdate, "Automatic OS Updates",
		BackButton|ConfirmButton, TuiPageMenu)

	lbl := clui.CreateLabel(page.content, 2, 16, autoUpdateHelp, Fixed)
	lbl.SetMultiline(true)

	page.backBtn.SetTitle("No [Disable]")
	page.backBtn.SetSize(11, 1)

	page.confirmBtn.SetTitle("Yes [Enable, Default]")
	page.confirmBtn.SetSize(21, 1)

	return page, nil
}

// DeActivate sets the model value and adjusts the "confirm" flag for this page
func (aup *AutoUpdatePage) DeActivate() {
	model := aup.getModel()

	if aup.action == ActionConfirmButton {
		model.AutoUpdate = true
	} else if aup.action == ActionBackButton {
		model.AutoUpdate = false
	}
}

// Activate activates the proper button depending on the current model value.
// If Auto Update is enabled in the data model then the Confirm button will be active
// otherwise the Back button will be activated.
func (aup *AutoUpdatePage) Activate() {
	if aup.getModel().AutoUpdate {
		aup.activated = aup.confirmBtn
	} else {
		aup.activated = aup.backBtn
	}
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (aup *AutoUpdatePage) GetConfigDefinition() int {

	if aup.getModel().AutoUpdate {
		return ConfigDefinedByConfig
	}

	return ConfigDefinedByUser
}
