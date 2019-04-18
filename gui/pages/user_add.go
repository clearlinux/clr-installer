// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/user"
	"github.com/clearlinux/clr-installer/utils"
)

// CommonMargin is the margin reference used by widgets
const CommonMargin int = 150

// UserAddPage is a simple page to add/modify/delete the user
type UserAddPage struct {
	controller   Controller
	model        *model.SystemInstall
	box          *gtk.Box
	user         *user.User
	definedUsers []string

	name        *gtk.Entry
	nameWarning *gtk.Label

	login        *gtk.Entry
	loginWarning *gtk.Label

	password        *gtk.Entry
	passwordConfirm *gtk.Entry
	passwordWarning *gtk.Label

	adminCheck   *gtk.CheckButton
	deleteButton *gtk.Button

	changedPassword bool
	changedLogin    bool
	addMode         bool
}

// NewUserAddPage returns a new User Add page
func NewUserAddPage(controller Controller, model *model.SystemInstall) (Page, error) {
	page := &UserAddPage{
		controller: controller,
		model:      model,
	}
	var err error

	// TODO: Remove when multi user is implemented
	page.user = &user.User{}
	if len(page.model.Users) != 0 {
		page.user = page.model.Users[0] // Just get the first user
	}

	// Page Box
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page")
	if err != nil {
		return nil, err
	}

	// Name
	page.name, page.nameWarning, err = page.setSimilarWidgets(utils.Locale.Get("User Name: "),
		utils.Locale.Get("Must start with letter. Can use numbers, hyphens and underscores. Max %d characters.", user.MaxUsernameLength),
		user.MaxUsernameLength)
	if err != nil {
		return nil, err
	}

	// Login
	page.login, page.loginWarning, err = page.setSimilarWidgets(utils.Locale.Get("Login: "),
		utils.Locale.Get("Must start with letter. Can use numbers, hyphens and underscores. Max %d characters.", user.MaxLoginLength),
		user.MaxLoginLength)
	if err != nil {
		return nil, err
	}

	// Password
	page.password, page.passwordConfirm, page.passwordWarning, err =
		page.setPasswordWidgets(utils.Locale.Get("Max %d characters.", user.MaxPasswordLength),
			user.MaxPasswordLength)
	if err != nil {
		return nil, err
	}

	// Admin Check
	page.adminCheck, err = gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}
	page.adminCheck.SetLabel("   " + utils.Locale.Get("Administrative"))
	sc, err := page.adminCheck.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass("label-entry")
	}
	page.adminCheck.SetHAlign(gtk.ALIGN_CENTER)
	page.box.PackStart(page.adminCheck, false, false, 0)

	// Generate signal on Name change
	if _, err := page.name.Connect("changed", page.onNameChange); err != nil {
		return nil, err
	}

	// Generate signal on Login change
	if _, err := page.login.Connect("changed", page.onLoginChange); err != nil {
		return nil, err
	}

	// Generate signal on Password change
	if _, err := page.password.Connect("changed", page.onPasswordChange); err != nil {
		return nil, err
	}

	// Generate signal on Password Confirm change
	if _, err := page.passwordConfirm.Connect("changed", page.onPasswordConfirmChange); err != nil {
		return nil, err
	}

	return page, nil
}

func (page *UserAddPage) onNameChange(entry *gtk.Entry) {
	if ok, msg := user.IsValidUsername(getTextFromEntry(page.name)); !ok {
		page.nameWarning.SetText(msg)
	} else {
		page.nameWarning.SetText("")
	}

	page.setConfirmButton()
}

func (page *UserAddPage) onLoginChange(entry *gtk.Entry) error {
	page.loginWarning.SetText("")

	login := getTextFromEntry(page.login)

	if ok, msg := user.IsValidLogin(getTextFromEntry(page.login)); !ok {
		page.loginWarning.SetText(msg)
	}

	isDefaultUser, err := user.IsSysDefaultUser(login)
	if err != nil {
		return err
	}

	if isDefaultUser {
		page.loginWarning.SetText(utils.Locale.Get("Specified login is a system default user"))
	}

	// TODO: Remove this until multi user is implemented
	for _, curr := range page.definedUsers {
		if curr == login {
			page.loginWarning.SetText(utils.Locale.Get("User must be unique"))
			break
		}
	}

	page.setConfirmButton()

	return nil
}

func (page *UserAddPage) onPasswordChange(entry *gtk.Entry) {
	if !page.changedPassword {
		page.changedPassword = true
	}
}

func (page *UserAddPage) onPasswordConfirmChange(entry *gtk.Entry) {
	if !page.changedPassword {
		return
	}

	password := getTextFromEntry(page.password)
	passwordConfirm := getTextFromEntry(page.passwordConfirm)
	if ok, msg := user.IsValidPassword(password); !ok {
		page.passwordWarning.SetText(msg)
	} else if password != passwordConfirm {
		page.passwordWarning.SetText(utils.Locale.Get("Passwords do not match"))
	} else {
		page.passwordWarning.SetText("")
	}

	page.setConfirmButton()
}

// IsRequired will return false as we have default values
func (page *UserAddPage) IsRequired() bool {
	return false
}

// IsDone checks if all the steps are completed
func (page *UserAddPage) IsDone() bool {
	return page.GetConfiguredValue() != ""
}

// GetID returns the ID for this page
func (page *UserAddPage) GetID() int {
	return PageIDUserAdd
}

// GetIcon returns the icon for this page
func (page *UserAddPage) GetIcon() string {
	return "avatar-default-symbolic"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *UserAddPage) GetRootWidget() gtk.IWidget {
	return page.box
}

// GetSummary will return the summary for this page
func (page *UserAddPage) GetSummary() string {
	return utils.Locale.Get("Add User")
}

// GetTitle will return the title for this page
func (page *UserAddPage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *UserAddPage) StoreChanges() {
	page.user.UserName = getTextFromEntry(page.name)
	page.user.Login = getTextFromEntry(page.login)
	page.user.Password = getTextFromEntry(page.password)
	page.user.Admin = page.adminCheck.GetActive()
	if len(page.model.Users) != 0 {
		page.model.Users[0] = page.user
	} else {
		page.model.Users = append(page.model.Users, page.user)
	}

	/*

		if page.addMode || page.changedPassword {
			if err := page.user.SetPassword(getTextFromEntry(page.password)); err != nil {
				log.Warning("Failed to encrypt password: %v", err)
				TODO: Add during multi user implementation
				page.clearForm()
				page.GotoPage(TuiPageUserManager)

			}
		}
	*/

	/* TODO: Add during multi user implementation
	page.GotoPage(TuiPageUserManager)
	return false
	*/
}

// ResetChanges will reset this page to match the model
func (page *UserAddPage) ResetChanges() {
	if page.model.Users != nil {
		setTextInEntry(page.name, page.model.Users[0].UserName)
		setTextInEntry(page.login, page.model.Users[0].Login)
		setTextInEntry(page.password, page.model.Users[0].Password)
		page.adminCheck.SetActive(page.model.Users[0].Admin)
	}
}

// GetConfiguredValue returns our current config
func (page *UserAddPage) GetConfiguredValue() string {
	users := page.model.Users
	res := []string{}

	if len(users) == 0 {
		return utils.Locale.Get("No users added")
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

func (page *UserAddPage) setConfirmButton() {
	userWarning, _ := page.nameWarning.GetText()
	loginWarning, _ := page.loginWarning.GetText()
	passwordWarning, _ := page.passwordWarning.GetText()
	login, _ := page.login.GetText()
	password, _ := page.password.GetText()

	if userWarning == "" && loginWarning == "" && passwordWarning == "" && login != "" && password != "" {
		page.controller.SetButtonState(ButtonConfirm, true)
	} else {
		page.controller.SetButtonState(ButtonConfirm, false)
	}
}

// Clear the form and user data; with Login being empty,
// this will result in the user not being re-added, aka deleted.
func (page *UserAddPage) onDeleteClicked() {
	page.user.UserName = ""
	page.user.Login = ""
	page.user.Password = ""
	page.user.Admin = false
	page.clearForm()
	//page.GotoPage(TuiPageUserManager)
}

func (page *UserAddPage) clearForm() {
	setTextInEntry(page.name, "")

	// Need to ensure there is a change in the login so that the validation code executes
	setTextInEntry(page.login, " ")
	setTextInEntry(page.login, "")
	page.changedLogin = false

	// Need to ensure there is a change in the password and change flag is set so that the validation code executes
	page.changedPassword = true
	setTextInEntry(page.password, " ")
	setTextInEntry(page.password, "")
	page.changedPassword = false

	setTextInEntry(page.passwordConfirm, "")
	page.password.SetVisibility(true)
	page.passwordConfirm.SetVisibility(true)
	page.adminCheck.SetActive(false)
	page.deleteButton.SetSensitive(false)
	page.controller.SetButtonState(ButtonConfirm, false)
	//clui.ActivateControl(page.tui.currPage.GetWindow(), page.usernameEdit) // TODO
}

func setLabelAndEntry(entryText string, maxSize int) (*gtk.Box, *gtk.Entry, error) {
	// Box
	boxEntry, err := setBox(gtk.ORIENTATION_HORIZONTAL, 0, "")
	if err != nil {
		return nil, nil, err
	}

	// Label
	labelEntry, err := setLabel(entryText, "label-entry", 0)
	if err != nil {
		return nil, nil, err
	}
	labelEntry.SetSizeRequest(150, -1)
	boxEntry.PackStart(labelEntry, false, false, 0)

	// Entry
	entry, err := setEntry("entry")
	if err != nil {
		return nil, nil, err
	}
	entry.SetMaxLength(maxSize)
	entry.SetMarginEnd(18)
	boxEntry.PackStart(entry, true, true, 0)

	return boxEntry, entry, nil
}

func (page *UserAddPage) setSimilarWidgets(entryText, rulesText string, maxSize int) (*gtk.Entry, *gtk.Label, error) {
	boxEntry, entry, err := setLabelAndEntry(entryText, maxSize)
	if err != nil {
		return nil, nil, err
	}
	page.box.PackStart(boxEntry, false, false, 0)

	// Rules
	rulesLabel, err := setLabel(rulesText, "label-rules", 0.0)
	if err != nil {
		return nil, nil, err
	}
	rulesLabel.SetMarginStart(CommonMargin)
	page.box.PackStart(rulesLabel, false, false, 0)

	// Warning
	warningLabel, err := setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, nil, err
	}
	warningLabel.SetMarginStart(CommonMargin)
	page.box.PackStart(warningLabel, false, false, 0)

	return entry, warningLabel, err
}

func (page *UserAddPage) setPasswordWidgets(rulesText string, maxSize int) (*gtk.Entry, *gtk.Entry, *gtk.Label, error) {
	boxPassword, password, err := setLabelAndEntry(utils.Locale.Get("Password: "), maxSize)
	if err != nil {
		return nil, nil, nil, err
	}
	password.SetVisibility(false)
	page.box.PackStart(boxPassword, false, false, 0)

	// Rules
	rulesLabel, err := setLabel(rulesText, "label-rules", 0.0)
	if err != nil {
		return nil, nil, nil, err
	}
	rulesLabel.SetMarginStart(CommonMargin)
	page.box.PackStart(rulesLabel, false, false, 0)

	boxPasswordConfirm, passwordConfirm, err := setLabelAndEntry("Confirm Password: ", maxSize)
	if err != nil {
		return nil, nil, nil, err
	}
	passwordConfirm.SetVisibility(false)
	page.box.PackStart(boxPasswordConfirm, false, false, 0)

	// Warning
	warningLabel, err := setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, nil, nil, err
	}
	warningLabel.SetMarginStart(CommonMargin)
	page.box.PackStart(warningLabel, false, false, 0)

	return password, passwordConfirm, warningLabel, err
}
