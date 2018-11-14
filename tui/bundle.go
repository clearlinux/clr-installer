// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/VladimirMarkelov/clui"
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
	return strings.Join(bp.getModel().Bundles, ", ")
}

// Activate marks the checkbox selections based on the data model
func (bp *BundlePage) Activate() {
	model := bp.getModel()

	for _, curr := range bundles {
		state := 0

		if model.ContainsBundle(curr.bundle.Name) {
			state = 1
		}

		curr.check.SetState(state)
	}
}

func newBundlePage(tui *Tui) (Page, error) {
	bdls, err := swupd.LoadBundleList()
	if err != nil {
		return nil, err
	}

	for _, curr := range bdls {
		bundles = append(bundles, &BundleCheck{curr, nil})
	}

	page := &BundlePage{}
	page.setupMenu(tui, TuiPageBundle, "Bundle Selection", NoButtons, TuiPageMenu)

	clui.CreateLabel(page.content, 2, 2, "Select additional bundles to be added to the system", Fixed)

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

	confirmBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Confirm", Fixed)
	confirmBtn.OnClick(func(ev clui.Event) {
		anySelected := false
		for _, curr := range bundles {
			if curr.check.State() == 1 {
				page.getModel().AddBundle(curr.bundle.Name)
				anySelected = true
			} else {
				page.getModel().RemoveBundle(curr.bundle.Name)
			}
		}

		page.SetDone(anySelected)
		page.GotoPage(TuiPageMenu)
	})

	return page, nil
}
