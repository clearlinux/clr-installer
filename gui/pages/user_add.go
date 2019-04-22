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

const (
	// CommonSetting is a common setting used by widgets
	CommonSetting int = 120

	// StartEndMargin is the start and end margin
	StartEndMargin int = 18
)

// UserAddPage is a simple page to add/modify/delete the user
type UserAddPage struct {
	controller   Controller
	model        *model.SystemInstall
	box          *gtk.Box
	boxButtons   *gtk.Box
	user         *user.User
	definedUsers []string

	name        *gtk.Entry
	nameWarning *gtk.Label
	nameChanged bool

	login        *gtk.Entry
	loginWarning *gtk.Label
	loginChanged bool

	password        *gtk.Entry
	passwordConfirm *gtk.Entry
	passwordWarning *gtk.Label
	passwordChanged bool

	adminCheck   *gtk.CheckButton
	adminChanged bool

	deleteButton  *gtk.Button
	deleteClicked bool

	justLoaded bool
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
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page-new")
	if err != nil {
		return nil, err
	}

	// Name
	page.name, page.nameWarning, err = page.setSimilarWidgets(utils.Locale.Get("User Name"),
		utils.Locale.Get("Must start with letter. Can use numbers, hyphens and underscores. Max %d characters.", user.MaxUsernameLength),
		user.MaxUsernameLength)
	if err != nil {
		return nil, err
	}

	// Login
	page.login, page.loginWarning, err = page.setSimilarWidgets(utils.Locale.Get("Login")+" *",
		utils.Locale.Get("Must start with letter. Can use numbers, hyphens and underscores. Max %d characters.", user.MaxLoginLength),
		user.MaxLoginLength)
	if err != nil {
		return nil, err
	}

	// Password
	page.password, page.passwordConfirm, page.passwordWarning, err =
		page.setPasswordWidgets(utils.Locale.Get("Min %d and Max %d characters.", user.MinPasswordLength, user.MaxPasswordLength),
			user.MaxPasswordLength)
	if err != nil {
		return nil, err
	}

	// Admin
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
	page.adminCheck.SetMarginStart(CommonSetting + StartEndMargin)
	page.adminCheck.SetMarginEnd(StartEndMargin)
	page.box.PackStart(page.adminCheck, false, false, 0)

	// Button box
	page.boxButtons, err = setBox(gtk.ORIENTATION_HORIZONTAL, 0, "box-page")
	if err != nil {
		return nil, err
	}

	// Buttons
	page.deleteButton, err = setButton(utils.Locale.Get("DELETE USER"), "button-page")
	if err != nil {
		return nil, err
	}
	page.boxButtons.PackStart(page.deleteButton, false, false, 0)

	page.box.PackStart(page.boxButtons, false, false, 0)

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

	// Generate signal on AdminCheck button click
	if _, err := page.adminCheck.Connect("clicked", page.onAdminClick); err != nil {
		return nil, err
	}

	// Generate signal on Delete button click
	if _, err := page.deleteButton.Connect("clicked", page.onDeleteClick); err != nil {
		return nil, err
	}

	return page, nil
}

func (page *UserAddPage) onNameChange(entry *gtk.Entry) {
	name := getTextFromEntry(page.name)
	if name != page.user.UserName {
		page.nameChanged = true
	} else {
		page.nameChanged = false
	}

	if ok, msg := user.IsValidUsername(getTextFromEntry(page.name)); !ok {
		page.nameWarning.SetText(msg)
	} else {
		page.nameWarning.SetText("")
	}

	page.setConfirmButton()
}

func (page *UserAddPage) onLoginChange(entry *gtk.Entry) error {
	login := getTextFromEntry(page.login)
	if login != page.user.Login {
		page.loginChanged = true
	} else {
		page.loginChanged = false
	}

	page.loginWarning.SetText("")
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
	password := getTextFromEntry(page.password)
	if password != page.user.Password {
		page.passwordChanged = true
	} else {
		page.passwordChanged = false
	}
}

func (page *UserAddPage) onPasswordConfirmChange(entry *gtk.Entry) {
	if !page.passwordChanged {
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

func (page *UserAddPage) onAdminClick(button *gtk.CheckButton) {
	if page.adminCheck.GetActive() != page.user.Admin {
		page.adminChanged = true
	} else {
		page.adminChanged = false
	}
	page.setConfirmButton()
}

func (page *UserAddPage) onDeleteClick(button *gtk.Button) {
	page.deleteClicked = true
	page.clearForm()
	page.deleteButton.SetSensitive(false)
}

// IsRequired will return false as we have default values
func (page *UserAddPage) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (page *UserAddPage) IsDone() bool {
	return len(page.model.Users) != 0
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
	return utils.Locale.Get("Manage User")
}

// GetTitle will return the title for this page
func (page *UserAddPage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *UserAddPage) StoreChanges() {
	// TODO: Modify when multi user is implemented
	page.user.UserName = getTextFromEntry(page.name)
	page.user.Login = getTextFromEntry(page.login)
	page.user.Password = getTextFromEntry(page.password)
	page.user.Admin = page.adminCheck.GetActive()

	if page.user.Login == "" {
		page.model.Users = make([]*user.User, 0)
	} else {
		if len(page.model.Users) == 0 {
			page.model.Users = append(page.model.Users, page.user)
		} else {
			page.model.Users[0] = page.user
		}
	}
}

// ResetChanges will reset this page to match the model
func (page *UserAddPage) ResetChanges() {
	setTextInEntry(page.name, page.user.UserName)
	setTextInEntry(page.login, page.user.Login)
	setTextInEntry(page.password, page.user.Password)
	setTextInEntry(page.passwordConfirm, page.user.Password)
	page.adminCheck.SetActive(page.user.Admin)

	if page.user.Login != "" {
		page.deleteButton.SetSensitive(true)
	} else {
		page.deleteButton.SetSensitive(false)
	}

	page.justLoaded = true
}

// GetConfiguredValue returns our current config
func (page *UserAddPage) GetConfiguredValue() string {
	users := page.model.Users
	result := []string{}

	if len(users) == 0 {
		return utils.Locale.Get("No users added")
	}

	for _, curr := range users {
		text := []string{curr.Login}
		if curr.Admin {
			text = append(text, "admin")
		}
		result = append(result, strings.Join(text, ": "))
	}

	return strings.Join(result, ", ")
}

func (page *UserAddPage) setConfirmButton() {

	if page.justLoaded {
		page.justLoaded = false
		page.controller.SetButtonState(ButtonConfirm, false)
		return
	}

	if page.deleteClicked {
		page.controller.SetButtonState(ButtonConfirm, true)
		return
	}

	if page.nameChanged || page.loginChanged || page.passwordChanged || page.adminChanged {
		userWarning, _ := page.nameWarning.GetText()
		loginWarning, _ := page.loginWarning.GetText()
		passwordWarning, _ := page.passwordWarning.GetText()
		login := getTextFromEntry(page.login)
		password := getTextFromEntry(page.password)

		if userWarning == "" && loginWarning == "" && passwordWarning == "" && login != "" && password != "" {
			page.controller.SetButtonState(ButtonConfirm, true)
		} else {
			page.controller.SetButtonState(ButtonConfirm, false)
		}
	}
}

func (page *UserAddPage) clearForm() {
	setTextInEntry(page.name, "")
	setTextInEntry(page.login, "")
	setTextInEntry(page.password, "")
	setTextInEntry(page.passwordConfirm, "")
	page.adminCheck.SetActive(false)
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
	labelEntry.SetSizeRequest(CommonSetting, -1)
	boxEntry.PackStart(labelEntry, false, false, 0)

	// Entry
	entry, err := setEntry("entry")
	if err != nil {
		return nil, nil, err
	}
	entry.SetSizeRequest(50, -1)
	entry.SetMaxWidthChars(50)
	entry.SetMaxLength(maxSize)
	boxEntry.PackStart(entry, true, true, 0)

	return boxEntry, entry, nil
}

func (page *UserAddPage) setSimilarWidgets(entryText, rulesText string, maxSize int) (*gtk.Entry, *gtk.Label, error) {
	boxEntry, entry, err := setLabelAndEntry(entryText, maxSize)
	if err != nil {
		return nil, nil, err
	}
	boxEntry.SetMarginStart(StartEndMargin)
	boxEntry.SetMarginEnd(StartEndMargin)
	page.box.PackStart(boxEntry, false, false, 0)

	// Rules
	rulesLabel, err := setLabel(rulesText, "label-rules", 0.0)
	if err != nil {
		return nil, nil, err
	}
	rulesLabel.SetMarginStart(CommonSetting + StartEndMargin)
	page.box.PackStart(rulesLabel, false, false, 0)

	// Warning
	warningLabel, err := setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, nil, err
	}
	warningLabel.SetMarginStart(CommonSetting + StartEndMargin)
	page.box.PackStart(warningLabel, false, false, 0)

	return entry, warningLabel, err
}

func (page *UserAddPage) setPasswordWidgets(rulesText string, maxSize int) (*gtk.Entry, *gtk.Entry, *gtk.Label, error) {
	boxPassword, password, err := setLabelAndEntry(utils.Locale.Get("Password")+" *", maxSize)
	if err != nil {
		return nil, nil, nil, err
	}
	boxPassword.SetMarginStart(StartEndMargin)
	boxPassword.SetMarginEnd(StartEndMargin)
	password.SetVisibility(false)
	page.box.PackStart(boxPassword, false, false, 0)

	// Rules
	rulesLabel, err := setLabel(rulesText, "label-rules", 0.0)
	if err != nil {
		return nil, nil, nil, err
	}
	rulesLabel.SetMarginStart(CommonSetting + StartEndMargin)
	page.box.PackStart(rulesLabel, false, false, 0)

	boxPasswordConfirm, passwordConfirm, err := setLabelAndEntry(utils.Locale.Get("Retype"), maxSize)
	if err != nil {
		return nil, nil, nil, err
	}
	boxPasswordConfirm.SetMarginStart(18)
	boxPasswordConfirm.SetMarginEnd(18)
	passwordConfirm.SetVisibility(false)
	page.box.PackStart(boxPasswordConfirm, false, false, 0)

	// Warning
	warningLabel, err := setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, nil, nil, err
	}
	warningLabel.SetMarginStart(CommonSetting + StartEndMargin)
	page.box.PackStart(warningLabel, false, false, 0)

	return password, passwordConfirm, warningLabel, err
}
