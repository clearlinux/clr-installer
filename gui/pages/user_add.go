// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/user"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// CommonSetting is a common setting used by widgets
	CommonSetting int = 150
)

// UserAddPage is a simple page to add/modify the user
type UserAddPage struct {
	controller   Controller
	model        *model.SystemInstall
	box          *gtk.Box
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
	fakePassword    bool

	adminCheck   *gtk.CheckButton
	adminChanged bool

	justLoaded bool

	addMode bool
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
	if len(page.model.Users) > 0 {
		page.user = page.model.Users[0] // Just get the first user
	}

	// Page Box
	page.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page-new")
	if err != nil {
		return nil, err
	}

	// Name
	page.name, page.nameWarning, err = page.setSimilarWidgets(utils.Locale.Get("User Name"),
		utils.Locale.Get(
			"Must start with letter. Can use numbers, commas, - and _. Max %d characters.", user.MaxUsernameLength),
		user.MaxUsernameLength)
	if err != nil {
		return nil, err
	}

	// Login
	page.login, page.loginWarning, err = page.setSimilarWidgets(utils.Locale.Get("Login")+" *",
		utils.Locale.Get(
			"Must begin with a letter. You can use letters, numbers, hyphens, underscores, and periods."+
				" "+"Up to %d characters.",
			user.MaxLoginLength),
		user.MaxLoginLength)
	if err != nil {
		return nil, err
	}

	// Password
	page.password, page.passwordConfirm, page.passwordWarning, err =
		page.setPasswordWidgets(
			utils.Locale.Get("Min %d and Max %d characters.", user.MinPasswordLength, user.MaxPasswordLength),
			user.MaxPasswordLength)
	if err != nil {
		return nil, err
	}

	// Admin
	page.adminCheck, err = gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}
	page.adminCheck.SetLabel("   " + utils.Locale.Get("Administrator"))
	sc, err := page.adminCheck.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass("label-entry")
	}
	page.adminCheck.SetMarginStart(CommonSetting + common.StartEndMargin)
	page.adminCheck.SetMarginEnd(common.StartEndMargin)
	page.adminCheck.SetSensitive(false) // MUST have an admin user
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
	if _, err := page.passwordConfirm.Connect("changed", page.onPasswordChange); err != nil {
		return nil, err
	}

	// Generate signal on AdminCheck button click
	if _, err := page.adminCheck.Connect("clicked", page.onAdminClick); err != nil {
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
	if !page.addMode && page.fakePassword {
		setTextInEntry(page.password, "")
		setTextInEntry(page.passwordConfirm, "")
		page.fakePassword = false
		page.passwordChanged = true
		page.setConfirmButton()
		return
	}

	password := getTextFromEntry(page.password)
	if password != page.user.Password {
		page.passwordChanged = true
	} else {
		page.passwordChanged = false
	}

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
	rawPassword := getTextFromEntry(page.password)

	if page.addMode {
		newUser := &user.User{
			UserName: getTextFromEntry(page.name),
			Login:    getTextFromEntry(page.login),
			Admin:    page.adminCheck.GetActive(),
		}

		page.model.AddUser(newUser)
	} else {
		if len(page.model.Users) < 1 {
			log.Warning("New user is missing")
			return
		}

		page.model.Users[0].UserName = getTextFromEntry(page.name)
		page.model.Users[0].Login = getTextFromEntry(page.login)
		page.model.Users[0].Admin = page.adminCheck.GetActive()
	}

	log.Debug("page.model.Users[0]: %+v", page.model.Users[0]) // RemoveMe

	// TODO: Modify when multi user is implemented
	// Do not set the encrypted password until after we have
	// added the user so we are updating the right memory
	if page.addMode || page.passwordChanged {
		// TODO: Fix thread issue?
		// Talk to John Andersen if there is a golang native function to use
		// This c-lang encryption function doesn't appear to be
		// safe to use with GTK -- thread issue?
		if err := page.model.Users[0].SetPassword(rawPassword); err != nil {
			log.Warning("Failed to encrypt password: %v", err)
			return
		}
	}

	page.clearForm()
}

// ResetChanges will reset this page to match the model
func (page *UserAddPage) ResetChanges() {
	page.clearForm()

	if len(page.model.Users) > 0 {
		page.user = page.model.Users[0] // Just get the first user
	} else {
		page.user = &user.User{}
	}

	if page.user.Login == "" {
		page.addMode = true
	}

	setTextInEntry(page.name, page.user.UserName)
	setTextInEntry(page.login, page.user.Login)

	if page.addMode {
		log.Debug("Starting in addMode")
		setTextInEntry(page.password, page.user.Password)
		setTextInEntry(page.passwordConfirm, page.user.Password)

		page.adminCheck.SetActive(true)
	} else {
		log.Debug("Starting in changeMode")
		// The password is encrypted, so fake it with stars
		setTextInEntry(page.password, "************")
		setTextInEntry(page.passwordConfirm, "************")
		page.passwordChanged = false
		page.fakePassword = true

		page.adminCheck.SetActive(page.user.Admin)
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
			text = append(text, utils.Locale.Get("admin"))
		}
		result = append(result, strings.Join(text, ": "))
	}

	return strings.Join(result, ", ")
}

func (page *UserAddPage) setConfirmButton() {
	page.controller.SetButtonState(ButtonConfirm, false)

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
	page.adminCheck.SetActive(true)

	page.nameChanged = false
	page.loginChanged = false
	page.passwordChanged = false
	page.fakePassword = false
	page.adminChanged = false
	page.addMode = false
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
	entry.SetMaxLength(maxSize)
	boxEntry.PackStart(entry, true, true, 0)

	return boxEntry, entry, nil
}

func (page *UserAddPage) setSimilarWidgets(entryText, rulesText string, maxSize int) (*gtk.Entry, *gtk.Label, error) {
	boxEntry, entry, err := setLabelAndEntry(entryText, maxSize)
	if err != nil {
		return nil, nil, err
	}
	boxEntry.SetMarginStart(common.StartEndMargin)
	boxEntry.SetMarginEnd(common.StartEndMargin)
	page.box.PackStart(boxEntry, false, false, 0)

	// Rules
	rulesLabel, err := setLabel(rulesText, "label-rules", 0.0)
	if err != nil {
		return nil, nil, err
	}
	rulesLabel.SetMarginStart(CommonSetting + common.StartEndMargin)
	rulesLabel.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	rulesLabel.SetLineWrap(true)
	page.box.PackStart(rulesLabel, false, false, 0)

	// Warning
	warningLabel, err := setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, nil, err
	}
	warningLabel.SetMarginStart(CommonSetting + common.StartEndMargin)
	warningLabel.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	warningLabel.SetLineWrap(true)
	page.box.PackStart(warningLabel, false, false, 0)

	return entry, warningLabel, err
}

func (page *UserAddPage) setPasswordWidgets(rulesText string, maxSize int) (*gtk.Entry, *gtk.Entry, *gtk.Label, error) {
	boxPassword, password, err := setLabelAndEntry(utils.Locale.Get("Password")+" *", maxSize)
	if err != nil {
		return nil, nil, nil, err
	}
	boxPassword.SetMarginStart(common.StartEndMargin)
	boxPassword.SetMarginEnd(common.StartEndMargin)
	password.SetVisibility(false)
	page.box.PackStart(boxPassword, false, false, 0)

	// Rules
	rulesLabel, err := setLabel(rulesText, "label-rules", 0.0)
	if err != nil {
		return nil, nil, nil, err
	}
	rulesLabel.SetMarginStart(CommonSetting + common.StartEndMargin)
	rulesLabel.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	rulesLabel.SetLineWrap(true)
	page.box.PackStart(rulesLabel, false, false, 0)

	boxPasswordConfirm, passwordConfirm, err := setLabelAndEntry(utils.Locale.Get("Confirm")+" *", maxSize)
	if err != nil {
		return nil, nil, nil, err
	}
	boxPasswordConfirm.SetMarginStart(common.StartEndMargin)
	boxPasswordConfirm.SetMarginEnd(common.StartEndMargin)
	passwordConfirm.SetVisibility(false)
	page.box.PackStart(boxPasswordConfirm, false, false, 0)

	// Warning
	warningLabel, err := setLabel("", "label-warning", 0.0)
	if err != nil {
		return nil, nil, nil, err
	}
	warningLabel.SetMarginStart(CommonSetting + common.StartEndMargin)
	warningLabel.SetMaxWidthChars(1) // The value does not matter but its required for LineWrap to work
	warningLabel.SetLineWrap(true)
	page.box.PackStart(warningLabel, false, false, 0)

	return password, passwordConfirm, warningLabel, err
}
