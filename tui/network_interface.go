// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"time"

	"github.com/clearlinux/clr-installer/network"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
)

// NetworkInterfacePage is the Page implementation for the network configuration page
type NetworkInterfacePage struct {
	BasePage
	IPEdit         *clui.EditField
	IPWarning      *clui.Label
	NetMaskEdit    *clui.EditField
	NetMaskWarning *clui.Label
	GatewayEdit    *clui.EditField
	GatewayWarning *clui.Label
	DNSEdit        *clui.EditField
	DNSWarning     *clui.Label
	ifaceLbl       *clui.Label
	DHCPCheck      *clui.CheckBox
	confirmBtn     *SimpleButton

	defaultValues struct {
		IP      string
		NetMask string
		Gateway string
		DNS     string
		DHCP    bool
	}
}

func (page *NetworkInterfacePage) getSelectedInterface() *network.Interface {
	var iface *network.Interface
	var ok bool

	prevPage := page.tui.getPage(TuiPageNetwork)
	if iface, ok = prevPage.GetData().(*network.Interface); !ok {
		return nil
	}

	return iface
}

func (page *NetworkInterfacePage) clearAllWarnings() {
	page.IPWarning.SetTitle("")
	page.NetMaskWarning.SetTitle("")
	page.GatewayWarning.SetTitle("")
	page.DNSWarning.SetTitle("")

	page.setConfirmButton()
}

// Activate will set the fields with the selected interface info
func (page *NetworkInterfacePage) Activate() {
	sel := page.getSelectedInterface()

	page.ifaceLbl.SetTitle(sel.Name)
	page.IPEdit.SetTitle("")
	page.NetMaskEdit.SetTitle("")
	page.GatewayEdit.SetTitle(sel.Gateway)
	page.DNSEdit.SetTitle(sel.DNS)
	page.clearAllWarnings()

	page.defaultValues.Gateway = sel.Gateway
	page.defaultValues.DNS = sel.DNS
	page.defaultValues.DHCP = sel.DHCP

	showIPv4 := sel.HasIPv4Addr()
	for _, addr := range sel.Addrs {
		if showIPv4 && addr.Version != network.IPv4 {
			continue
		}

		page.IPEdit.SetTitle(addr.IP)
		page.NetMaskEdit.SetTitle(addr.NetMask)

		page.defaultValues.IP = addr.IP
		page.defaultValues.NetMask = addr.NetMask
		break
	}

	page.setDHCP(sel.DHCP)
}

func (page *NetworkInterfacePage) setConfirmButton() {
	if page.IPWarning.Title() == "" && page.NetMaskWarning.Title() == "" &&
		page.GatewayWarning.Title() == "" && page.DNSWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

func (page *NetworkInterfacePage) validateIPField(editField *clui.EditField, warnLabel *clui.Label) {

	warnLabel.SetTitle(network.IsValidIP(editField.Title()))

	page.setConfirmButton()
}

func (page *NetworkInterfacePage) getDHCP() bool {
	state := page.DHCPCheck.State()
	if state == 1 {
		return true
	}

	return false
}

func (page *NetworkInterfacePage) setDHCP(DHCP bool) {
	state := 0

	if DHCP {
		state = 1
	}

	page.DHCPCheck.SetState(state)
}

func newFieldLabel(frame *clui.Frame, text string) *clui.Label {
	lbl := clui.CreateLabel(frame, AutoSize, 2, text, Fixed)
	lbl.SetAlign(AlignRight)
	return lbl
}
func newErrorLabel(frame *clui.Frame) *clui.Label {
	lbl := clui.CreateLabel(frame, AutoSize, 2, "", Fixed)
	lbl.SetAlign(AlignLeft)
	lbl.SetMultiline(false)
	lbl.SetBackColor(errorLabelBg)
	lbl.SetTextColor(errorLabelFg)
	return lbl
}

func validateIPEdit(k term.Key, ch rune) bool {
	validKeys := []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.'}

	if k == term.KeyBackspace || k == term.KeyBackspace2 {
		return false
	}

	for _, curr := range validKeys {
		if curr == ch {
			return false
		}
	}

	return true
}

func newNetworkInterfacePage(tui *Tui) (Page, error) {
	page := &NetworkInterfacePage{}
	page.setup(tui, TuiPageInterface, NoButtons, TuiPageMenu)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 20, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "Interface:")
	newFieldLabel(lblFrm, "Ip address:")
	newFieldLabel(lblFrm, "Subnet mask:")
	newFieldLabel(lblFrm, "Gateway:")
	newFieldLabel(lblFrm, "DNS:")

	fldFrm := clui.CreateFrame(frm, 30, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	ifaceFrm := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	ifaceFrm.SetPack(clui.Vertical)

	page.ifaceLbl = clui.CreateLabel(ifaceFrm, AutoSize, 2, "", Fixed)
	page.ifaceLbl.SetAlign(AlignLeft)

	page.IPEdit, _ = newEditField(fldFrm, false, validateIPEdit)
	page.NetMaskEdit, _ = newEditField(fldFrm, false, validateIPEdit)
	page.GatewayEdit, _ = newEditField(fldFrm, false, validateIPEdit)
	page.DNSEdit, _ = newEditField(fldFrm, false, validateIPEdit)

	eLblFrm := clui.CreateFrame(frm, 20, AutoSize, BorderNone, Fixed)
	eLblFrm.SetPack(clui.Vertical)
	eLblFrm.SetPaddings(1, 0)

	newErrorLabel(eLblFrm) // ignore the interface
	page.IPWarning = newErrorLabel(eLblFrm)
	page.NetMaskWarning = newErrorLabel(eLblFrm)
	page.GatewayWarning = newErrorLabel(eLblFrm)
	page.DNSWarning = newErrorLabel(eLblFrm)

	page.IPEdit.OnChange(func(ev clui.Event) {
		page.validateIPField(page.IPEdit, page.IPWarning)
	})
	page.NetMaskEdit.OnChange(func(ev clui.Event) {
		page.validateIPField(page.NetMaskEdit, page.NetMaskWarning)
	})
	page.GatewayEdit.OnChange(func(ev clui.Event) {
		page.validateIPField(page.GatewayEdit, page.GatewayWarning)
	})
	page.DNSEdit.OnChange(func(ev clui.Event) {
		page.validateIPField(page.DNSEdit, page.DNSWarning)
	})

	dhcpFrm := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	dhcpFrm.SetPack(clui.Vertical)

	page.DHCPCheck = clui.CreateCheckBox(dhcpFrm, 1, "Automatic/dhcp", Fixed)

	page.DHCPCheck.OnChange(func(ev int) {
		enable := true

		if ev == 1 {
			enable = false
			page.clearAllWarnings()
		} else {
			page.validateIPField(page.IPEdit, page.IPWarning)
			page.validateIPField(page.NetMaskEdit, page.NetMaskWarning)
			page.validateIPField(page.GatewayEdit, page.GatewayWarning)
			page.validateIPField(page.DNSEdit, page.DNSWarning)
		}

		page.IPEdit.SetEnabled(enable)
		page.NetMaskEdit.SetEnabled(enable)
		page.GatewayEdit.SetEnabled(enable)
		page.DNSEdit.SetEnabled(enable)
	})

	btnFrm := clui.CreateFrame(fldFrm, 30, 1, BorderNone, Fixed)
	btnFrm.SetPack(clui.Horizontal)
	btnFrm.SetGaps(1, 1)
	btnFrm.SetPaddings(2, 0)

	cancelBtn := CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageNetwork)
	})

	page.confirmBtn = CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Confirm", Fixed)
	page.confirmBtn.OnClick(func(ev clui.Event) {

		IP := page.IPEdit.Title()
		NetMask := page.NetMaskEdit.Title()
		DHCP := page.getDHCP()
		Gateway := page.GatewayEdit.Title()
		DNS := page.DNSEdit.Title()
		changed := false

		if IP != page.defaultValues.IP {
			changed = true
		}

		if NetMask != page.defaultValues.NetMask {
			changed = true
		}

		if DHCP != page.defaultValues.DHCP {
			changed = true
		}

		if Gateway != page.defaultValues.Gateway {
			changed = true
		}

		if DNS != page.defaultValues.DNS {
			changed = true
		}

		if changed {
			sel := page.getSelectedInterface()
			if !sel.HasIPv4Addr() {
				sel.AddAddr(IP, NetMask, network.IPv4)
			} else {
				for _, addr := range sel.Addrs {
					if addr.Version != network.IPv4 {
						continue
					}

					addr.IP = IP
					addr.NetMask = NetMask
					break
				}
			}

			sel.DHCP = DHCP
			sel.Gateway = Gateway
			sel.DNS = DNS
			page.getModel().AddNetworkInterface(sel)
		}

		if dialog, err := CreateNetworkTestDialogBox(page.tui.model); err == nil {
			dialog.OnClose(func() {
				page.GotoPage(TuiPageNetwork)
			})
			if dialog.RunNetworkTest() {
				page.tui.getPage(TuiPageNetwork).SetDone(true)

				// Automatically close if it worked
				clui.RefreshScreen()
				time.Sleep(time.Second)
				dialog.Close()
			} else {
				page.tui.getPage(TuiPageNetwork).SetDone(false)
			}
		}
	})

	return page, nil
}
