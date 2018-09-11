// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"net/url"
	"time"

	"github.com/VladimirMarkelov/clui"
)

// ProxyPage is the Page implementation for the proxy configuration page
type ProxyPage struct {
	BasePage
	httpsProxyEdit    *clui.EditField
	httpsProxyWarning *clui.Label
	confirmBtn        *SimpleButton
}

// GetConfiguredValue Returns the string representation of currently value set
func (pp *ProxyPage) GetConfiguredValue() string {
	value := pp.getModel().HTTPSProxy

	if value == "" {
		return "No HTTPS proxy URL set"
	}

	return value
}

// Activate sets the https proxy with the current model's value
func (pp *ProxyPage) Activate() {
	pp.httpsProxyEdit.SetTitle(pp.getModel().HTTPSProxy)
	pp.httpsProxyWarning.SetTitle("")
}

func (pp *ProxyPage) setConfirmButton() {
	if pp.httpsProxyWarning.Title() == "" {
		pp.confirmBtn.SetEnabled(true)
	} else {
		pp.confirmBtn.SetEnabled(false)
	}
}

func newProxyPage(tui *Tui) (Page, error) {
	page := &ProxyPage{}
	page.setupMenu(tui, TuiPageProxy, "Proxy", NoButtons, TuiPageMenu)

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
	page.httpsProxyEdit.OnChange(func(ev clui.Event) {
		warning := ""
		userProxy := page.httpsProxyEdit.Title()

		if userProxy != "" {
			_, err := url.ParseRequestURI(page.httpsProxyEdit.Title())
			if err != nil {
				warning = "Invalid URL for Proxy Server"
			}
		}

		page.httpsProxyWarning.SetTitle(warning)
		page.setConfirmButton()
	})

	page.httpsProxyWarning = clui.CreateLabel(iframe, 1, 1, "", Fixed)
	page.httpsProxyWarning.SetMultiline(true)
	page.httpsProxyWarning.SetBackColor(errorLabelBg)
	page.httpsProxyWarning.SetTextColor(errorLabelFg)

	btnFrm := clui.CreateFrame(fldFrm, 30, 1, BorderNone, Fixed)
	btnFrm.SetPack(clui.Horizontal)
	btnFrm.SetGaps(1, 1)
	btnFrm.SetPaddings(2, 1)

	cancelBtn := CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageMenu)
	})

	page.confirmBtn = CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Confirm", Fixed)
	page.confirmBtn.OnClick(func(ev clui.Event) {
		proxy := page.httpsProxyEdit.Title()
		currentProxy := page.getModel().HTTPSProxy
		page.getModel().HTTPSProxy = proxy
		if dialog, err := CreateNetworkTestDialogBox(page.tui.model); err == nil {
			dialog.OnClose(func() {
				page.GotoPage(TuiPageMenu)
			})
			if dialog.RunNetworkTest() {
				page.SetDone(proxy != "")

				// Automatically close if it worked
				clui.RefreshScreen()
				time.Sleep(time.Second)
				dialog.Close()
			} else {
				page.getModel().HTTPSProxy = currentProxy
				page.SetDone(false)
			}
		}
	})

	page.activated = page.httpsProxyEdit

	return page, nil
}
