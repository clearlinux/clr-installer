// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/VladimirMarkelov/clui"
)

// ProxyPage is the Page implementation for the proxy configuration page
type ProxyPage struct {
	BasePage
	httpsProxyEdit *clui.EditField
}

// Activate sets the https proxy with the current model's value
func (pp *ProxyPage) Activate() {
	pp.httpsProxyEdit.SetTitle(pp.getModel().HTTPSProxy)
}

func newProxyPage(tui *Tui) (Page, error) {
	page := &ProxyPage{}
	page.setupMenu(tui, TuiPageProxy, "Proxy", NoButtons, TuiPageAdvancedMenu)

	clui.CreateLabel(page.content, 2, 2, "Configure the network proxy", Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 20, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "HTTPS Proxy:")

	fldFrm := clui.CreateFrame(frm, 30, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	iframe := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	iframe.SetPack(clui.Vertical)

	page.httpsProxyEdit = clui.CreateEditField(iframe, 1, "", Fixed)

	btnFrm := clui.CreateFrame(fldFrm, 30, 1, BorderNone, Fixed)
	btnFrm.SetPack(clui.Horizontal)
	btnFrm.SetGaps(1, 1)
	btnFrm.SetPaddings(2, 0)

	cancelBtn := CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageAdvancedMenu)
	})

	confirmBtn := CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Confirm", Fixed)
	confirmBtn.OnClick(func(ev clui.Event) {
		proxy := page.httpsProxyEdit.Title()
		page.getModel().HTTPSProxy = proxy
		page.SetDone(proxy != "")
		page.GotoPage(TuiPageAdvancedMenu)
	})

	page.activated = page.httpsProxyEdit

	return page, nil
}
