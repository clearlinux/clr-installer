// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/clearlinux/clr-installer/model"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
)

// BasePage is the common implementation for the TUI frontend
// other pages will inherit this base page behaviours
type BasePage struct {
	tui        *Tui          // the Tui frontend reference
	window     *clui.Window  // the page window
	content    *clui.Frame   // the main content frame
	cFrame     *clui.Frame   // control frame
	cancelBtn  *SimpleButton // cancel button
	backBtn    *SimpleButton // back button
	confirmBtn *SimpleButton // confirm button
	activated  clui.Control  // activated control
	menuTitle  string        // the title used to show on main menu
	done       bool          // marks if an item is completed
	id         int           // the page id
	data       interface{}   // arbitrary page context data
	action     int           // indicates if the user has performed a navigation action
	required   bool          // marks if an item is required for the install
	menuButton *MenuButton
}

// Page defines the methods a Page must implement
type Page interface {
	GetID() int
	IsRequired() bool
	GetWindow() *clui.Window
	GetActivated() clui.Control
	GetMenuTitle() string
	SetData(data interface{})
	GetData() interface{}
	SetDone(done bool) bool
	GetDone() bool
	Activate()
	DeActivate()
	GetConfigDefinition() int
	GetConfiguredValue() string
	SetMenuButton(mb *MenuButton)
	GetMenuButton() *MenuButton
}

const (
	// WindowWidth is our desired terminal width
	WindowWidth = 80
	// WindowHeight is our desired terminal width
	WindowHeight = 24

	// ContentHeight is content frame height
	ContentHeight = 15

	// AutoSize is shortcut for clui.AutoSize flag
	AutoSize = clui.AutoSize

	// Fixed is shortcut for clui.Fixed flag
	Fixed = clui.Fixed

	// BorderNone is shortcut for clui.BorderNone flag
	BorderNone = clui.BorderNone

	// AlignLeft is shortcut for clui.AlignLeft flag
	AlignLeft = clui.AlignLeft

	// AlignRight is shortcut for clui.AlignRight flag
	AlignRight = clui.AlignRight

	// NoButtons mask defines a common Page will not set any control button
	NoButtons = 0

	// BackButton mask defines a common Page will have a back button
	BackButton = 1 << 1

	// ConfirmButton mask defines a common Page will have Confirm button
	ConfirmButton = 1 << 2

	// CancelButton mask defines a common Page will have a cancel button
	CancelButton = 1 << 3

	// AllButtons mask defines a common Page will have both Back and Confirm buttons
	AllButtons = BackButton | ConfirmButton

	// TuiPageMenu is the id for menu page
	TuiPageMenu = iota

	// TuiPageInstall is the id for install page
	TuiPageInstall

	// TuiPageLanguage is the id for language page
	TuiPageLanguage

	// TuiPageKeyboard is the id for keyboard page
	TuiPageKeyboard

	// TuiPageMediaConfig is the id for media configuration menu
	TuiPageMediaConfig

	// TuiPageNetwork is the id for network configuration page
	TuiPageNetwork

	// TuiPageProxy is the id for the proxy configuration page
	TuiPageProxy

	// TuiPageNetworkValidate is the id for the network validation page
	TuiPageNetworkValidate

	// TuiPageInterface is the id for the network interface configuration page
	TuiPageInterface

	// TuiPageBundle is the id for the bundle selection page
	TuiPageBundle

	// TuiPageTelemetry is the id for the telemetry enabling screen
	TuiPageTelemetry

	// TuiPageTimezone is the id for the timezone selection page
	TuiPageTimezone

	// TuiPageUserManager is the id for the user management page
	TuiPageUserManager

	// TuiPageUseradd is the id for the user add page
	TuiPageUseradd

	// TuiPageKernelCMDLine is the id for the kernel command line page
	TuiPageKernelCMDLine

	// TuiPageKernel is the id for the kernel selection page
	TuiPageKernel

	// TuiPageSwupdMirror is the id for the swupd mirror page
	TuiPageSwupdMirror

	// TuiPageHostname is the id for the hostname page
	TuiPageHostname

	// TuiPageAutoUpdate is the id for the Auto Update Enablement page
	TuiPageAutoUpdate

	// TuiPageSaveConfig is the id for the save YAML configuration file page
	TuiPageSaveConfig

	// ConfigDefinedByUser is used to determine a configuration was interactively
	// defined by the user
	ConfigDefinedByUser = iota

	// ConfigDefinedByConfig is used to determine a configuration was defined by
	// a configuration file
	ConfigDefinedByConfig

	// ConfigNotDefined is used to determine no configuration was provided yet
	ConfigNotDefined

	// ActionBackButton indicates the user has pressed back button
	ActionBackButton = iota

	// ActionConfirmButton indicates the user has pressed confirm button
	ActionConfirmButton

	// ActionCancelButton indicates the user has pressed cancel button
	ActionCancelButton

	// ActionNone indicates no action has been performed
	ActionNone
)
const (
	columnSpacer       = `  `
	columnWidthDefault = 10
	rowDividor         = `_`
)

type columnInfo struct {
	title        string
	rightJustify bool
	minWidth     int
	format       string
	width        int
}

// given the columnInfo type, return the length and fmt string
func getColumnFormat(info columnInfo) (int, string) {
	l := len(info.title)
	if info.minWidth > l {
		l = info.minWidth
	}
	justify := "-"
	if info.rightJustify {
		justify = ""
	}

	// Ensure we only write the max width
	return l, fmt.Sprintf("%%%s%d.%ds", justify, l, l)
}

// Is the page id a PopUp page
func isPopUpPage(id int) bool {
	if id == TuiPageNetworkValidate || id == TuiPageSaveConfig {
		return true
	}

	return false
}

// SetMenuButton sets the page's menu control
func (page *BasePage) SetMenuButton(mb *MenuButton) {
	page.menuButton = mb
}

// GetMenuButton returns the page's menu control
func (page *BasePage) GetMenuButton() *MenuButton {
	return page.menuButton
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *BasePage) GetConfiguredValue() string {
	return "Unknown value"
}

// GetConfigDefinition is a stub implementation
// the real implementation must check with the model and return:
//    + ConfigDefinedByUser: if the configuration was interactively defined by the user
//    + ConfigDefinedByConfig: if the configuration was provided by a config file
//    + ConfigNotDefined: if none of the above apply
func (page *BasePage) GetConfigDefinition() int {
	return ConfigNotDefined
}

// DeActivate is a stub implementation for the DeActivate method of Page interface
func (page *BasePage) DeActivate() {}

// Activate is a stub implementation for the Activate method of Page interface
func (page *BasePage) Activate() {}

// SetDone sets the page's done flag
func (page *BasePage) SetDone(done bool) bool {
	page.done = done
	return true
}

// GotoPage transitions between 2 pages
func (page *BasePage) GotoPage(id int) {
	page.tui.gotoPage(id, page.tui.currPage)
}

// Panic write an error to the tui panicked channel - we'll deal the error, stop clui
// mainloop and nicely panic() the application
func (page *BasePage) Panic(err error) {
	page.tui.paniced <- err
}

// GetDone returns the current value of a page's done flag
func (page *BasePage) GetDone() bool {
	return page.done
}

// GetData returns the current value of a page's data member
func (page *BasePage) GetData() interface{} {
	return page.data
}

// SetData set the current value for the page's data member
func (page *BasePage) SetData(data interface{}) {
	page.data = data
}

// GetMenuTitle returns the current page's title string
func (page *BasePage) GetMenuTitle() string {
	return page.menuTitle
}

// GetActivated returns the control set as activated for a page
func (page *BasePage) GetActivated() clui.Control {
	return page.activated
}

// GetWindow returns the current page's window control
func (page *BasePage) GetWindow() *clui.Window {
	return page.window
}

// GetID returns the current page's identifier
func (page *BasePage) GetID() int {
	return page.id
}

// IsRequired returns if this Page is required to be completed for the Install
func (page *BasePage) IsRequired() bool {
	return page.required
}

// GetMenuStatus returns the menu button status id
func GetMenuStatus(item Page) int {
	res := MenuButtonStatusDefault

	if item.GetDone() {
		res = MenuButtonStatusUserDefined
	} else if item.GetConfigDefinition() == ConfigDefinedByConfig {
		res = MenuButtonStatusAutoDetect
	}

	return res
}

func (page *BasePage) setupMenu(tui *Tui, id int, menuTitle string, btns int, returnID int) {
	page.setup(tui, id, btns, returnID)
	page.menuTitle = menuTitle
}

func (page *BasePage) setup(tui *Tui, id int, btns int, returnID int) {
	page.action = ActionNone
	page.id = id
	page.tui = tui

	page.newWindow()
	page.window.SetPack(clui.Vertical)

	page.content = clui.CreateFrame(page.window, AutoSize, ContentHeight,
		BorderNone, clui.Fixed)
	page.content.SetPack(clui.Vertical)
	page.content.SetPaddings(2, 1)

	page.cFrame = clui.CreateFrame(page.window, AutoSize, 1, BorderNone, Fixed)
	page.cFrame.SetPack(clui.Horizontal)
	page.cFrame.SetGaps(1, 1)
	page.cFrame.SetPaddings(3, 0)

	if btns&CancelButton == CancelButton {
		page.newCancelButton(returnID)
	}

	if btns&BackButton == BackButton {
		page.newBackButton(returnID)
	}

	if btns&ConfirmButton == ConfirmButton {
		page.newConfirmButton(tui, returnID)
	}

	frm := clui.CreateFrame(page.window, AutoSize, 1, BorderNone, Fixed)
	frm.SetPaddings(3, 1)

	clui.CreateLabel(frm, AutoSize, 1,
		"Use [Tab] or the arrow keys [Up and Down] to navigate", Fixed)

	page.window.SetVisible(false)

	// Escape-key cancel's this screen and returns
	// same as the default 'Cancel' button
	page.window.OnKeyDown(func(ev clui.Event, data interface{}) bool {
		if ev.Key == term.KeyEsc {
			if page.cancelBtn != nil {
				page.cancelBtn.ProcessEvent(clui.Event{Type: clui.EventKey, Key: term.KeyEnter})
			} else {
				page.action = ActionCancelButton
				page.GotoPage(returnID)
				page.action = ActionNone
				return true
			}
		}

		return false
	}, nil)
}

func (page *BasePage) newWindow() {
	sw, sh := clui.ScreenSize()

	x := (sw - WindowWidth) / 2
	y := (sh - WindowHeight) / 2

	// Default all the windows to borderless
	clui.WindowManager().SetBorder(clui.BorderNone)
	title := " [Clear Linux* OS Installer"
	if model.Version != model.DemoVersion {
		title = title + " (" + model.Version + ")"
	}
	title = title + "] "
	page.window = clui.AddWindow(x, y, WindowWidth, WindowHeight, title)

	page.window.SetTitleButtons(0)
	page.window.SetSizable(false)
	page.window.SetMovable(false)

	page.window.OnScreenResize(func(evt clui.Event) {
		ww, wh := page.window.Size()

		x := (evt.Width - ww) / 2
		y := (evt.Height - wh) / 2

		page.window.SetPos(x, y)
		page.window.ResizeChildren()
		page.window.PlaceChildren()
	})
}

func (page *BasePage) newBackButton(pageID int) {
	btn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "< Main Menu", Fixed)

	btn.OnClick(func(ev clui.Event) {
		page.action = ActionBackButton
		page.GotoPage(pageID)
		page.action = ActionNone
	})

	page.backBtn = btn
}

func (page *BasePage) newCancelButton(pageID int) {
	btn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Cancel", Fixed)

	btn.OnClick(func(ev clui.Event) {
		page.action = ActionCancelButton
		page.GotoPage(pageID)
		page.action = ActionNone
	})

	page.cancelBtn = btn
}

func (page *BasePage) newConfirmButton(tui *Tui, pageID int) {
	btn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Confirm", Fixed)

	btn.OnClick(func(ev clui.Event) {
		if tui.currPage.SetDone(true) {
			page.action = ActionConfirmButton
			page.GotoPage(pageID)
			page.action = ActionNone
		}
	})
	page.confirmBtn = btn
}

func (page *BasePage) getModel() *model.SystemInstall {
	return page.tui.model
}

func newEditField(frame *clui.Frame, validation bool, cb func(k term.Key, ch rune) bool) (*clui.EditField, *clui.Label) {
	var label *clui.Label

	height := 2
	if validation {
		height = 1
	}

	iframe := clui.CreateFrame(frame, 5, height, BorderNone, Fixed)
	iframe.SetPack(clui.Vertical)
	edit := clui.CreateEditField(iframe, 1, "", Fixed)

	if validation {
		label = clui.CreateLabel(iframe, AutoSize, 1, "", Fixed)
		label.SetVisible(false)
		label.SetBackColor(errorLabelBg)
		label.SetTextColor(errorLabelFg)
	}

	if cb != nil {
		edit.OnKeyPress(cb)
	}

	return edit, label
}
