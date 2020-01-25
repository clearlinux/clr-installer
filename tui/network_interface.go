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
	IPEdit           *clui.EditField
	IPWarning        *clui.Label
	NetMaskEdit      *clui.EditField
	NetMaskWarning   *clui.Label
	GatewayEdit      *clui.EditField
	GatewayWarning   *clui.Label
	DNSServerEdit    *clui.EditField
	DNSServerWarning *clui.Label
	DNSDomainEdit    *clui.EditField
	DNSDomainWarning *clui.Label
	ifaceLbl         *clui.Label
	DHCPCheck        *clui.CheckBox
	confirmBtn       *SimpleButton

	defaultValues struct {
		IP        string
		NetMask   string
		Gateway   string
		DNSServer string
		DNSDomain string
		DHCP      bool
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
	page.DNSServerWarning.SetTitle("")
	page.DNSDomainWarning.SetTitle("")

	page.setConfirmButton()
}

// Activate will set the fields with the selected interface info
func (page *NetworkInterfacePage) Activate() {
	sel := page.getSelectedInterface()

	page.ifaceLbl.SetTitle(sel.Name)
	page.IPEdit.SetTitle("")
	page.NetMaskEdit.SetTitle("")
	page.GatewayEdit.SetTitle(sel.Gateway)
	page.DNSServerEdit.SetTitle(sel.DNSServer)
	page.DNSDomainEdit.SetTitle(sel.DNSDomain)
	page.clearAllWarnings()

	page.defaultValues.Gateway = sel.Gateway
	page.defaultValues.DNSServer = sel.DNSServer
	page.defaultValues.DNSDomain = sel.DNSDomain
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
		page.GatewayWarning.Title() == "" &&
		page.DNSServerWarning.Title() == "" && page.DNSDomainWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

func (page *NetworkInterfacePage) validateIPField(editField *clui.EditField, warnLabel *clui.Label) {

	warnLabel.SetTitle(network.IsValidIP(editField.Title()))

	page.setConfirmButton()
}

func (page *NetworkInterfacePage) validateIPOrHostField(editField *clui.EditField, warnLabel *clui.Label) {

	warning := network.IsValidIP(editField.Title())

	if warning != "" {
		hostWarning := network.IsValidDomainName(editField.Title())
		if hostWarning != "" {
			warning = hostWarning // + " OR " + warning
		} else {
			warning = hostWarning // empty string
		}
	}

	warnLabel.SetTitle(warning)
	page.setConfirmButton()
}

func (page *NetworkInterfacePage) validateDomainField(editField *clui.EditField, warnLabel *clui.Label) {

	warnLabel.SetTitle(network.IsValidDomainName(editField.Title()))

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

func validateIPEdit(k term.Key, ch rune) bool {
	validKeys := []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.'}

	if k == term.KeyBackspace || k == term.KeyBackspace2 {
		return false
	}

	if k == term.KeyArrowUp || k == term.KeyArrowDown ||
		k == term.KeyArrowLeft || k == term.KeyArrowRight {
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

	lblFrm := clui.CreateFrame(frm, 10, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "Interface:")
	newFieldLabel(lblFrm, "Ip address:")
	newFieldLabel(lblFrm, "Subnet mask:")
	newFieldLabel(lblFrm, "Gateway:")
	newFieldLabel(lblFrm, "DNS Server:")
	newFieldLabel(lblFrm, "DNS Domain:")

	fldFrm := clui.CreateFrame(frm, 50, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	ifaceFrm := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	ifaceFrm.SetPack(clui.Vertical)

	page.ifaceLbl = clui.CreateLabel(ifaceFrm, AutoSize, 2, "", Fixed)
	page.ifaceLbl.SetAlign(AlignLeft)

	page.IPEdit, page.IPWarning = newEditField(fldFrm, true, validateIPEdit)
	page.NetMaskEdit, page.NetMaskWarning = newEditField(fldFrm, true, validateIPEdit)
	page.GatewayEdit, page.GatewayWarning = newEditField(fldFrm, true, nil)
	page.DNSServerEdit, page.DNSServerWarning = newEditField(fldFrm, true, nil)
	page.DNSDomainEdit, page.DNSDomainWarning = newEditField(fldFrm, true, nil)

	page.IPEdit.OnChange(func(ev clui.Event) {
		page.validateIPField(page.IPEdit, page.IPWarning)
	})
	page.IPEdit.OnActive(func(active bool) {
		if page.IPEdit.Active() {
			page.validateIPField(page.IPEdit, page.IPWarning)
		}
	})
	page.IPWarning.SetVisible(true)
	page.NetMaskEdit.OnChange(func(ev clui.Event) {
		page.validateIPField(page.NetMaskEdit, page.NetMaskWarning)
	})
	page.NetMaskEdit.OnActive(func(active bool) {
		if page.NetMaskEdit.Active() {
			page.validateIPField(page.NetMaskEdit, page.NetMaskWarning)
		}
	})
	page.NetMaskWarning.SetVisible(true)
	page.GatewayEdit.OnChange(func(ev clui.Event) {
		page.validateIPOrHostField(page.GatewayEdit, page.GatewayWarning)
	})
	page.GatewayEdit.OnActive(func(active bool) {
		if page.GatewayEdit.Active() {
			page.validateIPOrHostField(page.GatewayEdit, page.GatewayWarning)
		}
	})
	page.GatewayWarning.SetVisible(true)
	page.DNSServerEdit.OnChange(func(ev clui.Event) {
		page.validateIPOrHostField(page.DNSServerEdit, page.DNSServerWarning)
	})
	page.DNSServerEdit.OnActive(func(active bool) {
		if page.DNSServerEdit.Active() {
			page.validateIPOrHostField(page.DNSServerEdit, page.DNSServerWarning)
		}
	})
	page.DNSServerWarning.SetVisible(true)
	page.DNSDomainEdit.OnChange(func(ev clui.Event) {
		page.validateDomainField(page.DNSDomainEdit, page.DNSDomainWarning)
	})
	page.DNSDomainEdit.OnActive(func(active bool) {
		if page.DNSDomainEdit.Active() {
			page.validateDomainField(page.DNSDomainEdit, page.DNSDomainWarning)
		}
	})
	page.DNSDomainWarning.SetVisible(true)

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
			page.validateIPOrHostField(page.GatewayEdit, page.GatewayWarning)
			page.validateIPOrHostField(page.DNSServerEdit, page.DNSServerWarning)
			page.validateDomainField(page.DNSDomainEdit, page.DNSDomainWarning)
		}

		page.IPEdit.SetEnabled(enable)
		page.NetMaskEdit.SetEnabled(enable)
		page.GatewayEdit.SetEnabled(enable)
		page.DNSServerEdit.SetEnabled(enable)
		page.DNSDomainEdit.SetEnabled(enable)
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
		DNSServer := page.DNSServerEdit.Title()
		DNSDomain := page.DNSDomainEdit.Title()
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

		if DNSServer != page.defaultValues.DNSServer {
			changed = true
		}

		if DNSDomain != page.defaultValues.DNSDomain {
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
			sel.DNSServer = DNSServer
			sel.DNSDomain = DNSDomain
			page.getModel().AddNetworkInterface(sel)
		}

		networkCancel := make(chan bool)
		if dialog, err := CreateNetworkTestDialogBox(page.tui.model, networkCancel); err == nil {
			dialog.OnClose(func() {
				page.GotoPage(TuiPageNetwork)
			})
			if dialog.RunNetworkTest(networkCancel) {
				page.getModel().CopyNetwork = true
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
