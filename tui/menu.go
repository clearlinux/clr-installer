// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"time"

	"github.com/VladimirMarkelov/clui"

	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/swupd"
)

// MenuPage is the Page implementation for the main menu page
type MenuPage struct {
	BasePage
	installBtn *SimpleButton
	tabGroup   *TabGroup
	reqTab     *TabPage
	advTab     *TabPage
}

func (page *MenuPage) addMenuItem(item Page, tab *TabPage) *MenuButton {
	fw, _ := tab.frame.Size()

	btn := CreateMenuButton(tab.frame, MenuButtonStatusDefault, item.GetMenuTitle(), fw)
	btn.SetStyle("Main")
	btn.SetAlign(AlignLeft)
	btn.SetActive(false)

	item.SetMenuButton(btn)

	btn.OnClick(func(ev clui.Event) {
		tab.activeMenu = btn
		page.GotoPage(item.GetID())
	})

	return btn
}

// Activate is called when the page is "shown" and it repaints the main menu based on the
// available menu pages and their confirm/unconfirm status
func (page *MenuPage) Activate() {
	requiredDone := true
	previous := false
	activeSet := false

	// if we're returning to the "advanced" tab then simply sets the previously
	// active menu item
	if page.advTab.IsVisible() {
		page.activated = page.advTab.activeMenu
		activeSet = true
	}

	// if we're returning to the "required" tab then we iterate over not yet
	// completed required "tasks" and select the missing one
	for _, curr := range page.tui.pages {
		if curr.GetMenuTitle() == "" || curr.GetID() == page.GetID() {
			continue
		}

		tab := page.reqTab

		if !curr.IsRequired() {
			tab = page.advTab
		}

		if page.tui.prevPage != nil {
			// Is this menu option match the previous page?
			previous = page.tui.prevPage.GetID() == curr.GetID()
		}

		btn := curr.GetMenuButton()

		if btn == nil {
			btn = page.addMenuItem(curr, tab)
		}

		btn.SetMenuItemValue(curr.GetConfiguredValue())
		btn.SetStatus(GetMenuStatus(curr))

		if curr.IsRequired() {
			// Does the menu item added have the data set completed?
			completed := GetMenuStatus(curr) != MenuButtonStatusDefault

			// If we haven't found the first active choice, set it
			if !activeSet && !completed {
				// Make last button added Active
				page.activated = btn
				activeSet = true
			}

			// Special case if the previous page and the data set is not completed
			// we want THIS to be the active choice for easy return
			if previous && !completed && !activeSet {
				// Make last button added Active
				page.activated = btn
				activeSet = true
			}

			requiredDone = requiredDone && completed
		}
	}

	var err error
	if page.getModel() != nil {
		if err = page.getModel().Validate(); err != nil {
			log.Debug("Model not valid: %s", err.Error())
		}
	}

	if err == nil && requiredDone {
		page.installBtn.SetEnabled(true)
		page.activated = page.installBtn
	} else {
		// If we failed to validate, disable the Install button
		// It may have been enable previously -- do not assume disabled
		page.installBtn.SetEnabled(false)
		scrollTabToActive(page.activated, page.tabGroup)
	}
}

func scrollTabToActive(activated clui.Control, group *TabGroup) {
	if activated == nil {
		return
	}

	vFrame := group.GetVisibleFrame()

	_, cy, _, ch := vFrame.Clipper()
	vx, vy := vFrame.Pos()

	_, ay := activated.Pos()
	_, ah := activated.Size()

	if ay+ah > cy+ch || ay < cy {
		diff := (cy + ch) - (ay + ah)
		ty := vy + diff
		vFrame.ScrollTo(vx, ty)
	}
}

func (page *MenuPage) launchConfirmInstallDialogBox() {
	if page.installBtn.Enabled() {
		if dialog, err := CreateConfirmInstallDialogBox(page.tui.model); err == nil {
			dialog.OnClose(func() {
				if dialog.Confirmed {
					// Remove user bundles when continuing install with an offline warning
					if !controller.NetworkPassing && len(dialog.modelSI.UserBundles) != 0 && swupd.IsOfflineContent() {
						page.tui.model.UserBundles = nil
					}
					page.GotoPage(TuiPageInstall)
					go func() {
						_ = network.DownloadInstallerMessage("Pre-Installation",
							network.PreInstallConf)
					}()
				}
			})
		}
	}
}

func newMenuPage(tui *Tui) (Page, error) {
	var err error

	page := &MenuPage{}
	page.setup(tui, TuiPageMenu, NoButtons, TuiPageMenu)

	// the menu is an special case, we have no paddings
	page.content.SetPaddings(0, 0)

	page.tabGroup = NewTabGroup(page.content, 1, ContentHeight)
	page.reqTab, err = page.tabGroup.AddTab("Required options", 'r')
	if err != nil {
		return nil, err
	}

	page.advTab, err = page.tabGroup.AddTab("Advanced options", 'a')
	if err != nil {
		return nil, err
	}

	cancelBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		go clui.Stop()
	})

	page.installBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Install", Fixed)
	page.installBtn.OnClick(func(ev clui.Event) {
		// Network needs to be validated before the install when offline content isn't supported
		// or additional bundles were added
		if (!swupd.IsOfflineContent() || len(page.tui.model.UserBundles) != 0) && !controller.NetworkPassing {
			if dialog, err := CreateNetworkTestDialogBox(page.tui.model); err == nil {
				dialog.OnClose(func() {
					page.launchConfirmInstallDialogBox()
				})
				if dialog.RunNetworkTest() {
					// Automatically close if it worked
					clui.RefreshScreen()
					time.Sleep(time.Second)
					dialog.Close()
				} else if !swupd.IsOfflineContent() {
					// Cannot install without network or offline content
					page.installBtn.SetEnabled(false)
				}
			}
		} else {
			page.launchConfirmInstallDialogBox()
		}
	})

	return page, nil
}
