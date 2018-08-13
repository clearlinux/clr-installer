// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/clearlinux/clr-installer/user"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
)

// UseraddPage is the Page implementation for the user add configuration page
type UseraddPage struct {
	BasePage
	loginEdit  *clui.EditField
	pwdEdit    *clui.EditField
	adminCheck *clui.CheckBox
	listFrm    *clui.Frame
	users      []*UserBtn
	deleteBtn  *SimpleButton
	selected   *UserBtn
	changedPwd bool
	loginLabel *clui.Label
	pwdLabel   *clui.Label
	confirmBtn *SimpleButton
}

// UserBtn maps a ui button to a user
type UserBtn struct {
	user *user.User
	btn  *SimpleButton
}

func (page *UseraddPage) validateLogin() {
	showLabel := false
	enableConfirm := true

	login := page.loginEdit.Title()
	if ok, msg := user.IsValidLogin(login); !ok {
		page.loginLabel.SetTitle(msg)
		showLabel = true
		enableConfirm = false
	}

	if notok, err := user.IsSysDefaultUser(login); notok || err != nil {
		if err != nil {
			page.Panic(err)
		}

		page.loginLabel.SetTitle("Specified login is a system default user")
		showLabel = true
		enableConfirm = false
	}

	if page.selected == nil {
		for _, curr := range page.users {
			if curr.user.Login == page.loginEdit.Title() && curr != page.selected {
				page.loginLabel.SetTitle("User must be unique")
				showLabel = true
				break
			}
		}

		validPwd, _ := user.IsValidPassword(page.pwdEdit.Title())
		enableConfirm = !showLabel && validPwd
	}

	page.loginLabel.SetVisible(showLabel)
	page.confirmBtn.SetEnabled(enableConfirm)
}

func (page *UseraddPage) validatePassword() {
	showLabel := false

	if page.selected != nil && !page.changedPwd {
		return
	}

	if ok, msg := user.IsValidPassword(page.pwdEdit.Title()); !ok {
		page.pwdLabel.SetTitle(msg)
		showLabel = true
	}

	page.pwdLabel.SetVisible(showLabel)

	var sysDefUser, validLogin bool
	var err error

	login := page.loginEdit.Title()
	if sysDefUser, err = user.IsSysDefaultUser(login); err != nil {
		page.Panic(err)
	}

	validLogin, _ = user.IsValidLogin(login)

	page.confirmBtn.SetEnabled(!showLabel && (validLogin && !sysDefUser))
}

func newUseraddPage(tui *Tui) (Page, error) {
	page := &UseraddPage{users: []*UserBtn{}}
	page.setupMenu(tui, TuiPageUseradd, "Add Users", BackButton, TuiPageAdvancedMenu)

	clui.CreateLabel(page.content, 2, 2, "Add new users", Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 10, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "Login:")
	newFieldLabel(lblFrm, "Password:")

	fldFrm := clui.CreateFrame(frm, 50, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	page.loginEdit, page.loginLabel = newEditField(fldFrm, true, nil)
	page.loginEdit.OnChange(func(ev clui.Event) {
		page.validateLogin()
	})

	page.loginEdit.OnActive(func(active bool) {
		if page.loginEdit.Active() {
			page.validateLogin()
		}
	})

	page.pwdEdit, page.pwdLabel = newEditField(fldFrm, true, nil)
	page.pwdEdit.SetPasswordMode(true)

	page.pwdEdit.OnChange(func(ev clui.Event) {
		page.validatePassword()
	})

	page.pwdEdit.OnActive(func(active bool) {
		if page.pwdEdit.Active() {
			page.validatePassword()
		}
	})

	page.pwdEdit.OnKeyPress(func(k term.Key, ch rune) bool {
		if page.selected != nil && !page.changedPwd {
			page.changedPwd = true
			page.pwdEdit.SetTitle("")
		}
		return false
	})

	adminFrm := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	adminFrm.SetPack(clui.Vertical)

	page.adminCheck = clui.CreateCheckBox(adminFrm, 1, "Administrative", Fixed)

	btnFrm := clui.CreateFrame(fldFrm, 30, 1, BorderNone, Fixed)
	btnFrm.SetPack(clui.Horizontal)
	btnFrm.SetGaps(1, 1)
	btnFrm.SetPaddings(2, 0)

	page.confirmBtn = CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Confirm", Fixed)

	page.confirmBtn.OnClick(func(ev clui.Event) {
		pwd := page.pwdEdit.Title()

		if !page.changedPwd && page.selected != nil {
			pwd = ""
		}

		page.showUser(page.loginEdit.Title(), pwd, page.adminCheck.State() == 1)
		page.SetDone(true)
	})

	page.deleteBtn = CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Delete", Fixed)
	page.deleteBtn.SetEnabled(false)

	page.deleteBtn.OnClick(func(ev clui.Event) {
		if page.selected == nil {
			return
		}

		btns := []*UserBtn{}
		for _, curr := range page.users {
			if curr == page.selected {
				curr.btn.Destroy()
				continue
			}

			btns = append(btns, curr)
		}

		page.users = btns
		page.resetForm()
	})

	cancelBtn := CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.resetForm()
	})

	page.listFrm = clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	page.listFrm.SetPack(clui.Vertical)
	page.listFrm.SetPaddings(4, 2)

	page.activated = page.loginEdit

	return page, nil
}

func (page *UseraddPage) resetForm() {
	page.loginEdit.SetTitle("")
	page.pwdEdit.SetTitle("")
	page.adminCheck.SetState(0)
	page.selected = nil
	page.deleteBtn.SetEnabled(false)
	clui.ActivateControl(page.tui.currPage.GetWindow(), page.loginEdit)
}

func (page *UseraddPage) updateUser(lbl string, login string, pwd string, admin bool) {
	page.selected.btn.SetTitle(lbl)

	page.selected.user.Login = login
	page.selected.user.Admin = admin

	if pwd != "" {
		_ = page.selected.user.SetPassword(pwd)
	}

	page.selected = nil
}

func (page *UseraddPage) addNewUser(lbl string, login string, pwd string, admin bool) {
	usr, err := user.NewUser(login, pwd, admin)
	if err != nil {
		return
	}

	btn := CreateSimpleButton(page.listFrm, AutoSize, AutoSize, lbl, Fixed)
	btn.SetAlign(AlignLeft)

	usrBtn := &UserBtn{usr, btn}
	page.users = append(page.users, usrBtn)

	btn.OnClick(func(ev clui.Event) {
		state := 0
		page.selected = usrBtn
		page.changedPwd = false

		if usr.Admin {
			state = 1
		}

		page.loginLabel.SetVisible(false)
		page.loginEdit.SetTitle(usr.Login)

		page.pwdLabel.SetVisible(false)
		page.pwdEdit.SetTitle("xxxxxxxx")

		page.adminCheck.SetState(state)
		page.deleteBtn.SetEnabled(true)

		clui.ActivateControl(page.tui.currPage.GetWindow(), page.loginEdit)
	})
}

func (page *UseraddPage) showUser(login string, pwd string, admin bool) {
	lbl := fmt.Sprintf("%-30s admin: %+v", login, admin)

	if page.selected != nil {
		page.updateUser(lbl, login, pwd, admin)
	} else {
		page.addNewUser(lbl, login, pwd, admin)
	}

	page.resetForm()
}

// DeActivate merges the local data with the data model
func (page *UseraddPage) DeActivate() {
	for _, curr := range page.users {
		page.getModel().AddUser(curr.user)
	}
}
