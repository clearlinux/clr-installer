// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/VladimirMarkelov/clui"
	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/swupd"
)

// BundlePage is the Page implementation for the proxy configuration page
type BundlePage struct {
	BasePage
}

// BundleCheck maps a map name and description with the actual checkbox
type BundleCheck struct {
	bundle *swupd.Bundle
	check  *clui.CheckBox
}

var (
	bundles = []*BundleCheck{}
)

// GetConfiguredValue Returns the string representation of currently value set
func (bp *BundlePage) GetConfiguredValue() string {
	bundles := bp.getModel().UserBundles

	if len(bundles) == 0 {
		return "No bundles selected"
	}

	return strings.Join(bundles, ", ")
}

func (bp *BundlePage) updateNetworkStatus() {
	bp.confirmBtn.SetEnabled(controller.NetworkPassing)
}

// Activate marks the checkbox selections based on the data model
func (bp *BundlePage) Activate() {
	bp.GetWindow().SetVisible(true)
	model := bp.getModel()

	for _, curr := range bundles {
		state := 0

		if model.ContainsUserBundle(curr.bundle.Name) {
			state = 1
		}

		curr.check.SetState(state)
	}

	bp.updateNetworkStatus()
}

func bundleCheck(bp *BundlePage) {
	text := "This requires a working network connection.\nProceed with a network test?"
	title := "Network Required"
	if dialogDecision, err := CreateConfirmCancelDialogBox(text, title); err == nil {
		dialogDecision.OnClose(func() {
			if dialogDecision.Confirmed {
				// Network connection is required to add additional bundles
				if !controller.NetworkPassing {
					if dialogNet, err := CreateNetworkTestDialogBox(bp.getModel()); err == nil {
						if dialogNet.RunNetworkTest() {
							// Automatically close if it worked
							clui.RefreshScreen()
							time.Sleep(time.Second)
							dialogNet.Close()
							bp.updateNetworkStatus()
						}
					}
				}
			}

			/* Dialog box at end while closing will uncheck all the checked boxes
			 * if the network test did not pass
			 */
			if !controller.NetworkPassing {
				for _, curr := range bundles {
					if curr.check.State() == 1 {
						curr.check.SetState(0)
					}
				}
			}
		})
	}
}

func newBundlePage(tui *Tui) (Page, error) {
	page := &BundlePage{}
	page.setupMenu(tui, TuiPageBundle, "Select Additional Bundles", NoButtons, TuiPageMenu)

	bdls, err := swupd.LoadBundleList(page.getModel())
	if err != nil {
		return nil, err
	}

	for _, curr := range bdls {
		bundles = append(bundles, &BundleCheck{curr, nil})
	}

	clui.CreateLabel(page.content, 2, 2, "Select Additional Bundles", Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, 14, BorderNone, Fixed)
	frm.SetPack(clui.Vertical)
	frm.SetScrollable(true)

	lblFrm := clui.CreateFrame(frm, AutoSize, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(2, 0)

	for _, curr := range bundles {
		lbl := fmt.Sprintf("%s: %s", curr.bundle.Name, curr.bundle.Desc)
		curr.check = clui.CreateCheckBox(lblFrm, AutoSize, lbl, AutoSize)
		curr.check.SetPack(clui.Horizontal)
		curr.check.OnChange(func(ev int) {
			if ev == 1 && !controller.NetworkPassing {
				bundleCheck(page)
			}
		})
	}

	fldFrm := clui.CreateFrame(frm, 30, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	cancelBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageMenu)
	})

	page.confirmBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Confirm", Fixed)
	page.confirmBtn.OnClick(func(ev clui.Event) {
		anySelected := false
		for _, curr := range bundles {
			if curr.check.State() == 1 {
				page.getModel().AddUserBundle(curr.bundle.Name)
				anySelected = true
			} else {
				page.getModel().RemoveUserBundle(curr.bundle.Name)
			}
		}

		page.SetDone(anySelected)
		page.GotoPage(TuiPageMenu)
	})

	return page, nil
}
