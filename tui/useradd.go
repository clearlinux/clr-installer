// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"strings"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/user"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
)

// UseraddPage is the Page implementation for the user add configuration page
type UseraddPage struct {
	BasePage
	user            *user.User
	definedUsers    []string
	addMode         bool
	titleLabel      *clui.Label
	loginEdit       *clui.EditField
	usernameEdit    *clui.EditField
	passwordEdit    *clui.EditField
	pwConfirmEdit   *clui.EditField
	adminCheck      *clui.CheckBox
	deleteBtn       *SimpleButton
	changedPwd      bool
	changedLogin    bool
	loginWarning    *clui.Label
	usernameWarning *clui.Label
	passwordWarning *clui.Label
	confirmBtn      *SimpleButton
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *UseraddPage) GetConfiguredValue() string {
	users := page.getModel().Users
	res := []string{}

	if len(users) == 0 {
		return "No users added"
	}

	for _, curr := range users {
		tks := []string{curr.Login}

		if curr.Admin {
			tks = append(tks, "admin")
		}

		res = append(res, strings.Join(tks, ":"))
	}

	return strings.Join(res, ", ")
}

// Activate updates the UI elements
func (page *UseraddPage) Activate() {
	title := "Modify User"

	page.clearForm()

	prevPage := page.tui.getPage(TuiPageUserManager)
	sel, ok := prevPage.GetData().(*SelectedUser)
	if ok {
		page.user = sel.user
		page.definedUsers = sel.definedUsers
		page.addMode = sel.addMode

		if sel.addMode {
			title = "Add New User"
		} else {
			page.resetForm()
		}
	}

	page.titleLabel.SetTitle(title)

	page.setConfirmButton()
}

// SetDone copies the edited user data into the cache and sets the page as done
func (page *UseraddPage) SetDone(done bool) bool {

	// SetDone can only be called if the Confirm Button is enable;
	// hence we know the data is valid
	page.user.UserName = page.usernameEdit.Title()
	page.user.Login = page.loginEdit.Title()
	if page.addMode || page.changedPwd {
		if err := page.user.SetPassword(page.passwordEdit.Title()); err != nil {
			log.Warning("Failed to encrypt password: %v", err)
			page.clearForm()
			page.GotoPage(TuiPageUserManager)
		}
	}

	if page.adminCheck.State() != 0 {
		page.user.Admin = true
	} else {
		page.user.Admin = false
	}

	page.GotoPage(TuiPageUserManager)

	return false
}

func (page *UseraddPage) setConfirmButton() {
	if page.usernameWarning.Title() == "" &&
		page.loginWarning.Title() == "" &&
		page.passwordWarning.Title() == "" &&
		page.loginEdit.Title() != "" &&
		page.passwordEdit.Title() != "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

func (page *UseraddPage) validateUsername() {
	username := page.usernameEdit.Title()
	if ok, msg := user.IsValidUsername(username); !ok {
		page.usernameWarning.SetTitle(msg)
	} else {
		page.usernameWarning.SetTitle("")
	}

	page.setConfirmButton()
}

func (page *UseraddPage) validateLogin() {
	page.loginWarning.SetTitle("")

	login := page.loginEdit.Title()
	if ok, msg := user.IsValidLogin(login); !ok {
		page.loginWarning.SetTitle(msg)
	}

	if notok, err := user.IsSysDefaultUser(login); notok || err != nil {
		if err != nil {
			page.Panic(err)
		}

		page.loginWarning.SetTitle("Specified login is a system default user")
	}

	for _, curr := range page.definedUsers {
		if curr == page.loginEdit.Title() {
			page.loginWarning.SetTitle("User must be unique")
			break
		}
	}

	page.setConfirmButton()
}

func (page *UseraddPage) validatePassword() {
	if !page.changedPwd {
		return
	}

	if ok, msg := user.IsValidPassword(page.passwordEdit.Title()); !ok {
		page.passwordWarning.SetTitle(msg)
	} else if page.passwordEdit.Title() != page.pwConfirmEdit.Title() {
		page.passwordWarning.SetTitle("Passwords do not match")
	} else {
		page.passwordWarning.SetTitle("")
	}

	page.setConfirmButton()
}

func newUseraddPage(tui *Tui) (Page, error) {
	page := &UseraddPage{}
	page.setup(tui, TuiPageUseradd, NoButtons, TuiPageUserManager)

	page.titleLabel = clui.CreateLabel(page.content, AutoSize, 2, "", clui.Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)
	lblFrm := clui.CreateFrame(frm, 10, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	newFieldLabel(lblFrm, "User Name:")
	newFieldLabel(lblFrm, "Login:")
	newFieldLabel(lblFrm, "Password:")
	newFieldLabel(lblFrm, "Confirm:")

	fldFrm := clui.CreateFrame(frm, 50, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	page.usernameEdit, page.usernameWarning = newEditField(fldFrm, true, nil)
	page.usernameEdit.OnChange(func(ev clui.Event) {
		if len(page.loginEdit.Title()) == 0 {
			page.changedLogin = false
		}
		page.validateUsername()
		if !page.changedLogin {
			name := strings.Split(page.usernameEdit.Title(), ",")[0]
			// Remove the invalid characters
			name = strings.Replace(name, " ", "", -1)
			name = strings.Replace(name, "'", "", -1)
			name = strings.Replace(name, ".", "", -1)
			name = strings.ToLower(name)
			if len(name) > user.MaxLoginLength {
				name = name[0:(user.MaxLoginLength - 1)]
			}
			page.loginEdit.SetTitle(name)
		}
	})
	page.usernameWarning.SetVisible(true)

	page.loginEdit, page.loginWarning = newEditField(fldFrm, true, nil)
	page.loginEdit.OnChange(func(ev clui.Event) {
		if len(page.loginEdit.Title()) == 0 {
			page.changedLogin = false
		}
		page.validateLogin()
	})
	// If the user types in the Login field, no longer auto-generate
	// the login value based on the UserName
	page.loginEdit.OnKeyPress(func(k term.Key, ch rune) bool {
		page.changedLogin = true
		return false
	})

	page.loginEdit.OnActive(func(active bool) {
		if page.loginEdit.Active() {
			page.validateLogin()
		}
	})

	page.loginWarning.SetVisible(true)

	page.passwordEdit, _ = newEditField(fldFrm, false, nil)
	page.passwordEdit.SetPasswordMode(true)

	page.passwordEdit.OnChange(func(ev clui.Event) {
		page.validatePassword()
	})

	page.passwordEdit.OnActive(func(active bool) {
		if page.passwordEdit.Active() {
			page.validatePassword()
		}
	})

	page.passwordEdit.OnKeyPress(func(k term.Key, ch rune) bool {
		if k == term.KeyCtrlU {
			page.revealPassword()
			return true
		}
		if k == term.KeyArrowUp || k == term.KeyArrowDown {
			return false
		}
		if !page.changedPwd {
			page.changedPwd = true
			page.passwordEdit.SetTitle("")
			page.pwConfirmEdit.SetTitle("")
		}
		return false
	})

	page.pwConfirmEdit, page.passwordWarning = newEditField(fldFrm, true, nil)
	page.pwConfirmEdit.SetPasswordMode(true)
	page.passwordWarning.SetVisible(true)

	page.pwConfirmEdit.OnChange(func(ev clui.Event) {
		page.validatePassword()
	})

	page.pwConfirmEdit.OnActive(func(active bool) {
		if page.pwConfirmEdit.Active() {
			page.validatePassword()
		}
	})

	page.pwConfirmEdit.OnKeyPress(func(k term.Key, ch rune) bool {
		if k == term.KeyCtrlU {
			page.revealPassword()
			return true
		}
		if k == term.KeyArrowUp || k == term.KeyArrowDown {
			return false
		}
		if !page.changedPwd {
			page.changedPwd = true
			page.passwordEdit.SetTitle("")
			page.pwConfirmEdit.SetTitle("")
		}
		return false
	})

	adminFrm := clui.CreateFrame(fldFrm, 5, 2, BorderNone, Fixed)
	adminFrm.SetPack(clui.Vertical)

	page.adminCheck = clui.CreateCheckBox(adminFrm, 1, "Administrator", Fixed)

	cancelBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.clearForm()
		page.GotoPage(TuiPageUserManager)
	})

	page.confirmBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Confirm", Fixed)
	page.confirmBtn.OnClick(func(ev clui.Event) {
		page.SetDone(true)
	})

	page.deleteBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Delete", Fixed)
	page.deleteBtn.SetEnabled(false)
	page.deleteBtn.OnClick(func(ev clui.Event) {
		// Clear the form and user data; with Login being empty,
		// this will result in the user not being re-added, aka deleted.
		page.user.UserName = ""
		page.user.Login = ""
		page.user.Password = ""
		page.user.Admin = false
		page.clearForm()
		page.GotoPage(TuiPageUserManager)
	})

	resetBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Reset", Fixed)
	resetBtn.OnClick(func(ev clui.Event) {
		if page.addMode {
			page.clearForm()
		} else {
			page.resetForm()
		}

		page.setConfirmButton()
	})

	page.activated = page.usernameEdit

	return page, nil
}

func (page *UseraddPage) revealPassword() {
	if !page.addMode {
		return
	}

	if page.passwordEdit.PasswordMode() {
		page.passwordEdit.SetPasswordMode(false)
		page.pwConfirmEdit.SetPasswordMode(false)
	} else {
		page.passwordEdit.SetPasswordMode(true)
		page.pwConfirmEdit.SetPasswordMode(true)
	}
}

func (page *UseraddPage) resetForm() {
	page.usernameEdit.SetTitle(page.user.UserName)
	page.loginEdit.SetTitle(page.user.Login)
	page.changedLogin = true // Assume the user wants to keep this
	page.changedPwd = false
	if !page.addMode {
		page.passwordEdit.SetTitle("************")
		page.pwConfirmEdit.SetTitle("************")
		page.passwordWarning.SetTitle("")
	}
	if page.user.Admin {
		page.adminCheck.SetState(1)
	}

	page.deleteBtn.SetEnabled(true)

	clui.ActivateControl(page.tui.currPage.GetWindow(), page.usernameEdit)
}

func (page *UseraddPage) clearForm() {
	page.usernameEdit.SetTitle("")
	// Need to ensure there is a change in the title so
	// that the validation code executes
	page.loginEdit.SetTitle(" ")
	page.loginEdit.SetTitle("")
	page.changedLogin = false
	// Need to ensure there is a change in the title and
	// change flag is set so that the validation code executes
	page.changedPwd = true
	page.passwordEdit.SetTitle(" ")
	page.passwordEdit.SetTitle("")
	page.changedPwd = false
	page.pwConfirmEdit.SetTitle("")
	page.passwordEdit.SetPasswordMode(true)
	page.pwConfirmEdit.SetPasswordMode(true)
	page.adminCheck.SetState(0)
	page.deleteBtn.SetEnabled(false)
	page.confirmBtn.SetEnabled(false)
	clui.ActivateControl(page.tui.currPage.GetWindow(), page.usernameEdit)
}
