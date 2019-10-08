// Copyright © 2018 Intel Corporation
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
	offlineLabel *clui.Label
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

// Activate marks the checkbox selections based on the data model
func (bp *BundlePage) Activate() {
	bp.GetWindow().SetVisible(true)
	model := bp.getModel()

	// Network connection is required to add additional bundles
	if !controller.NetworkPassing {
		if dialog, err := CreateNetworkTestDialogBox(bp.tui.model); err == nil {
			if dialog.RunNetworkTest() {
				// Automatically close if it worked
				clui.RefreshScreen()
				time.Sleep(time.Second)
				dialog.Close()
			}
		}
	}

	for _, curr := range bundles {
		state := 0

		if model.ContainsUserBundle(curr.bundle.Name) {
			state = 1
		}

		curr.check.SetEnabled(controller.NetworkPassing)
		curr.check.SetState(state)
	}

	if controller.NetworkPassing {
		bp.offlineLabel.SetTitle("")
	} else {
		bp.offlineLabel.SetTitle("Network check failed.")
	}

	bp.confirmBtn.SetEnabled(controller.NetworkPassing)
	bp.offlineLabel.SetEnabled(!controller.NetworkPassing)
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

	page.offlineLabel = clui.CreateLabel(page.cFrame, 1, 2, "", 1)
	page.offlineLabel.SetBackColor(errorLabelBg)
	page.offlineLabel.SetTextColor(errorLabelFg)
	page.offlineLabel.SetAlign(clui.AlignRight)

	return page, nil
}
