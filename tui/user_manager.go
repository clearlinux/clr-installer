// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VladimirMarkelov/clui"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/user"
)

// SelectedUser holds the shared date between the User Manager page
// and the user configuration page
type SelectedUser struct {
	user         *user.User
	definedUsers []string
	addMode      bool
}

const (
	userManagerTitle = "Manage User"
)
const (
	userColumnLogin = iota
	userColumnUserName
	userColumnPassword
	userColumnAdmin
	userColumnCount
)

var (
	userColumns []columnInfo
)

func init() {
	userColumns = make([]columnInfo, userColumnCount)

	userColumns[userColumnLogin].title = "Login"
	userColumns[userColumnLogin].minWidth = 12

	userColumns[userColumnUserName].title = "User Name"
	userColumns[userColumnUserName].minWidth = -1 // This column get all free space

	userColumns[userColumnPassword].title = "Password"
	userColumns[userColumnPassword].minWidth = 8

	userColumns[userColumnAdmin].title = "Admin"
	userColumns[userColumnAdmin].minWidth = 5
}

// UserManagerPage is the Page implementation for the disk partitioning menu page
type UserManagerPage struct {
	BasePage
	scrollingFrame *clui.Frame // content scrolling frame
	warningLabel   *clui.Label // content warning label

	columnFormat string

	users []*user.User

	usersChanged bool

	rowFrames   []*clui.Frame
	activeRow   *clui.Frame
	activeUser  *SimpleButton
	activeLogin string
}

// GetDone Returns true when there is an admin user
func (page *UserManagerPage) GetDone() bool {
	for _, user := range page.getModel().Users {
		if user.Admin {
			return true
		}
	}
	return false
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *UserManagerPage) GetConfiguredValue() string {
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

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *UserManagerPage) GetConfigDefinition() int {
	users := page.getModel().Users
	if len(users) == 0 {
		return ConfigNotDefined
	}

	return ConfigDefinedByUser
}

// SetDone put list of users into the model and sets the page as done
func (page *UserManagerPage) SetDone(done bool) bool {
	page.getModel().RemoveAllUsers()
	for _, user := range page.users {
		page.getModel().AddUser(user)
	}

	page.usersChanged = false
	page.users = nil

	// TODO start using new API page.GotoPage() when finished merging
	// disk pages
	page.tui.gotoPage(TuiPageMenu, page)

	return false
}

// Activate updates the UI elements with the most current list of block devices
func (page *UserManagerPage) Activate() {
	// We have new or updated user data
	if page.data != nil {
		data := page.data.(*SelectedUser)

		// Make sure the data is valid before we add
		if data.user.Login != "" {
			page.addUser(data.user)
			page.activeLogin = data.user.Login
			page.data = nil
		} else {
			page.activeLogin = ""
		}
	} else {
		// We are entering from the main menu and need to copy the existing users
		page.removeAllUsers()
		for _, user := range page.getModel().Users {
			page.addUser(user)
		}
		// Since we are restore, there
		page.usersChanged = false
	}

	page.redrawRows()

	if page.activeUser != nil {
		page.activated = page.activeUser
		page.scrollToRow(page.activeRow)
	}

	adminSet := false
	for _, user := range page.users {
		if user.Admin {
			adminSet = true
		}
	}

	if adminSet {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}

	clui.RefreshScreen()
}

func (page *UserManagerPage) scrollToRow(rowFrame *clui.Frame) {
	_, cy, _, ch := page.scrollingFrame.Clipper()
	vx, vy := page.scrollingFrame.Pos()

	_, ry := rowFrame.Pos()
	_, rh := rowFrame.Size()

	if ry+rh > cy+ch {
		diff := (cy + ch) - (ry + rh)
		ty := vy + diff
		page.scrollingFrame.ScrollTo(vx, ty)
	} else if ry < cy {
		page.scrollingFrame.ScrollTo(vx, cy)
	}
}

func (page *UserManagerPage) redrawRows() {
	for _, curr := range page.rowFrames {
		curr.Destroy()
	}
	page.rowFrames = []*clui.Frame{}
	adminSet := false

	if len(page.users) > 0 {
		for _, user := range page.users {
			if user.Admin {
				adminSet = true
			}
			if err := page.addUserRow(user); err != nil {
				page.Panic(err)
			}
		}

		// Display a warning when no admin user is set
		if !adminSet {
			page.warningLabel.SetTitle("Admin user is required")
		} else {
			page.warningLabel.SetTitle("")
		}
	} else {
		rowFrame := clui.CreateFrame(page.scrollingFrame, 1, AutoSize, clui.BorderNone, clui.Fixed)
		rowFrame.SetPack(clui.Vertical)
		_ = clui.CreateLabel(rowFrame, 2, 1, "*** No Custom Users Defined ***", clui.Fixed)
		page.rowFrames = append(page.rowFrames, rowFrame)
		page.scrollToRow(rowFrame)
	}

	clui.RefreshScreen()
}

// The user manager page gives the user the option define additional accounts
// to be created on the target system
func newUserManagerPage(tui *Tui) (Page, error) {
	page := &UserManagerPage{
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}
	page.setupMenu(tui, TuiPageUserManager, userManagerTitle, CancelButton|ConfirmButton, TuiPageMenu)

	cWidth, cHeight := page.content.Size()
	// Calculate the Scrollable frame area
	sWidth := cWidth - (2 * 2) // Buffer of 2 characters from each side
	sHeight := cHeight + 1     // Add back the blank line from content to menu buttons

	// Top label for the page
	topFrame := clui.CreateFrame(page.content, sWidth, 2, clui.BorderNone, clui.Fixed)
	topFrame.SetPack(clui.Horizontal)

	clui.CreateLabel(topFrame, sWidth/2, AutoSize, userManagerTitle, clui.Fixed)

	page.warningLabel = clui.CreateLabel(topFrame, sWidth/2, AutoSize, "", clui.Fixed)
	page.warningLabel.SetAlign(AlignRight)
	page.warningLabel.SetBackColor(errorLabelBg)
	page.warningLabel.SetTextColor(errorLabelFg)

	_, lHeight := topFrame.Size()
	sHeight -= lHeight // Remove the label from total height

	remainingColumns := sWidth - 2
	allFree := -1
	for i, info := range userColumns {
		// Should this column get all extra space?
		if info.minWidth == -1 {
			if allFree == -1 {
				allFree = i
				continue // Do not format this column
			} else {
				log.Warning("More than one user info column set for all free space: %s", info.title)
				info.minWidth = columnWidthDefault
			}
		}

		l, format := getColumnFormat(info)
		userColumns[i].format = format
		userColumns[i].width = l
		remainingColumns -= l
	}

	// remove the column spacers
	remainingColumns -= ((len(userColumns) - 1) * len(columnSpacer))

	// If we had a column which get the remaining space
	if allFree != -1 {
		userColumns[allFree].minWidth = remainingColumns
		userColumns[allFree].width = remainingColumns
		_, userColumns[allFree].format = getColumnFormat(userColumns[allFree])
	}

	// Build the Header Title and full row format string
	titles := []interface{}{""} // need to use an interface for Sprintf
	formats := []string{}
	dividers := []interface{}{""} // need to use an interface for Sprintf
	for _, info := range userColumns {
		titles = append(titles, info.title)
		formats = append(formats, info.format)
		dividers = append(dividers, strings.Repeat(rowDividor, info.width))
	}
	titles = titles[1:] // pop the first empty string
	page.columnFormat = strings.Join(formats, columnSpacer)
	dividers = dividers[1:] // pop the first empty string

	// Create the frame for the header label
	headerFrame := clui.CreateFrame(page.content, sWidth, 1, clui.BorderNone, clui.Fixed)
	headerFrame.SetPack(clui.Vertical)
	headerFrame.SetPaddings(1, 0)
	columnsTitle := fmt.Sprintf(page.columnFormat, titles...)
	columnsLabel := clui.CreateLabel(headerFrame, AutoSize, 1, columnsTitle, clui.Fixed)
	columnsLabel.SetPaddings(0, 0)
	_, lHeight = columnsLabel.Size()
	sHeight -= lHeight // Remove the label from total height
	columnsDividors := fmt.Sprintf(page.columnFormat, dividers...)
	columnsDividorLabel := clui.CreateLabel(headerFrame, AutoSize, 1, columnsDividors, clui.Fixed)
	columnsDividorLabel.SetPaddings(0, 0)
	_, lHeight = columnsDividorLabel.Size()
	sHeight -= lHeight // Remove the label from total height

	page.scrollingFrame = clui.CreateFrame(page.content, sWidth, sHeight, clui.BorderNone, clui.Fixed)
	page.scrollingFrame.SetPack(clui.Vertical)
	page.scrollingFrame.SetScrollable(true)
	page.scrollingFrame.SetGaps(0, 1)
	page.scrollingFrame.SetPaddings(1, 0)

	// Setup the cancel button with a warning
	page.cancelBtn.OnClick(func(ev clui.Event) {
		if page.usersChanged {
			message := "User data modified and will be lost!\n\nDiscard user changes?"
			if dialog, err := CreateConfirmCancelDialogBox(message); err == nil {
				dialog.OnClose(func() {
					if dialog.Confirmed {
						page.data = nil
						page.GotoPage(TuiPageMenu)
						log.Debug("Warning: DATA LOSS: Cancel button " + message)
					}
				})
			}
		} else {
			page.data = nil
			page.GotoPage(TuiPageMenu)
		}
	})

	// Add an Add User button
	addButton := CreateSimpleButton(page.cFrame, AutoSize, 1, "Add New User", Fixed)
	addButton.OnClick(func(ev clui.Event) {
		user := &user.User{}
		users := []string{}
		for _, cur := range page.users {
			users = append(users, cur.Login)
		}
		page.data = &SelectedUser{user: user, definedUsers: users, addMode: true}
		page.GotoPage(TuiPageUseradd)
	})

	// Add a Revert button
	revertBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Revert", Fixed)
	revertBtn.OnClick(func(ev clui.Event) {
		if page.usersChanged {
			message := "User data modified and will be lost!\n\nDiscard user changes?"
			if dialog, err := CreateConfirmCancelDialogBox(message); err == nil {
				dialog.OnClose(func() {
					if dialog.Confirmed {
						page.removeAllUsers()
						for _, user := range page.getModel().Users {
							page.addUser(user)
						}
						page.usersChanged = false
						page.activeLogin = ""

						// Clear last selected row as it might be removed
						page.data = nil

						log.Debug("Warning: DATA LOSS: Revert button " + message)
						page.GotoPage(TuiPageUserManager)
					}
				})
			}
		} else {
			page.data = nil
			page.activeLogin = ""
			page.GotoPage(TuiPageUserManager)
		}
	})

	page.activated = addButton

	return page, nil
}

func (page *UserManagerPage) addUserRow(user *user.User) error {
	rowFrame := clui.CreateFrame(page.scrollingFrame, 1, 1, clui.BorderNone, clui.Fixed)
	rowFrame.SetPack(clui.Vertical)

	password := ""
	if user.Password != "" {
		password = "********"
	}
	admin := ""
	if user.Admin {
		admin = "  X"
	}
	userTitle := fmt.Sprintf(page.columnFormat,
		user.Login, user.UserName, password, admin)

	userButton := CreateSimpleButton(rowFrame, 1, 1, userTitle, Fixed)
	userButton.SetAlign(AlignLeft)

	userButton.OnClick(func(ev clui.Event) {
		// Remove the user to be modified as any part of the data could change
		page.removeUser(user)

		users := []string{}
		for _, cur := range page.users {
			users = append(users, cur.Login)
		}

		page.data = &SelectedUser{user: user, definedUsers: users, addMode: false}

		page.GotoPage(TuiPageUseradd)
	})

	page.rowFrames = append(page.rowFrames, rowFrame)

	userButton.OnActive(func(active bool) {
		if active {
			page.scrollToRow(rowFrame)
		}
	})
	rowFrame.OnActive(func(active bool) {
		if active {
			page.scrollToRow(rowFrame)
		}
	})

	// We do not have an active user, so default to the current
	if page.activeLogin == "" {
		page.activeLogin = user.Login
		page.activeUser = userButton
		page.activeRow = rowFrame
	} else if user.Login == page.activeLogin {
		// Set Active User button and Row Frame
		page.activeUser = userButton
		page.activeRow = rowFrame
	}

	return nil
}

// Sorting Interface

// Users is a list of User
type Users []*user.User

// Len is require to find the length of the Users list
func (users Users) Len() int { return len(users) }

// Swap is require for swapping two positions in Users list
func (users Users) Swap(a, b int) { users[a], users[b] = users[b], users[a] }

// ByLogin implements sort.Interface for sorting by the for Login field
type ByLogin struct{ Users }

// Less is require comparison function of the Users list for ByLogin
func (users ByLogin) Less(a, b int) bool { return users.Users[a].Login < users.Users[b].Login }

// ByUserName implements sort.Interface for sorting by the for UserName field
type ByUserName struct{ Users }

// Less is require comparison function of the Users list for ByUserName
func (users ByUserName) Less(a, b int) bool { return users.Users[a].UserName < users.Users[b].UserName }

func (page *UserManagerPage) addUser(addUser *user.User) {
	page.usersChanged = true

	newUser := &user.User{
		Login:    addUser.Login,
		UserName: addUser.UserName,
		Password: addUser.Password,
		Admin:    addUser.Admin,
	}

	page.users = append(page.users, newUser)

	// Now sort by Login
	sort.Sort(ByLogin{page.users})
}

func (page *UserManagerPage) removeUser(user *user.User) {
	page.usersChanged = true

	if page.users == nil {
		return
	}

	deleteIndex := -1

	for i, cur := range page.users {
		if cur.Equals(user) {
			deleteIndex = i
			break
		}
	}

	if deleteIndex >= 0 {
		copy(page.users[deleteIndex:], page.users[deleteIndex+1:])
		page.users[len(page.users)-1] = nil
		page.users = page.users[:len(page.users)-1]
	} else {
		log.Warning("Attempting to remove user '%s', but not found in active list.", user.Login)
	}
}
func (page *UserManagerPage) removeAllUsers() {
	page.users = []*user.User{}
}
