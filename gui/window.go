// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/gui/pages"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	// WindowWidth specifies the default width of the window
	WindowWidth int = 950

	// WindowHeight specifies the default height of the window
	WindowHeight int = 600
)

// PageConstructor is a typedef of the constructors for our pages
type PageConstructor func(controller pages.Controller, model *model.SystemInstall) (pages.Page, error)

// Window provides management of the underlying GtkWindow and
// associated windows to provide a level of OOP abstraction.
type Window struct {
	handle        *gtk.Window // Abstract the underlying GtkWindow
	mainLayout    *gtk.Box    // Content layout (horizontal)
	banner        *Banner     // Banner
	contentLayout *gtk.Box    // Content Layout
	rootStack     *gtk.Stack  // Root-level stack

	model   *model.SystemInstall // model
	options args.Args            // installer args
	rootDir string               // root directory

	// Menu
	menu struct {
		switcher     *Switcher             // Allow switching between main menu
		stack        *gtk.Stack            // Menu switching
		screens      map[bool]*ContentView // Mapping to content views
		welcomePage  pages.Page            // Pointer to the welcome page
		preCheckPage pages.Page            // Pointer to the pre-check page
		currentPage  pages.Page            // Pointer to the currently open page
		installPage  pages.Page            // Pointer to the installer page
	}

	// Buttons
	buttons struct {
		stack        *gtk.Stack // Storage for buttons
		boxWelcome   *gtk.Box   // Storage for welcome buttons
		boxPrimary   *gtk.Box   // Storage for primary buttons
		boxSecondary *gtk.Box   // Storage for secondary buttons

		// Welcome buttons
		next *gtk.Button // Launch primary view
		exit *gtk.Button // Exit installer

		// Primary buttons
		install *gtk.Button // Install Clear Linux
		quit    *gtk.Button // Exit installer
		back    *gtk.Button // Back to welcome page

		// Secondary buttons
		confirm *gtk.Button // Confirm changes
		cancel  *gtk.Button // Cancel changes
	}

	didInit      bool                // Whether initialized the view animation
	pages        map[int]gtk.IWidget // Mapping to each root page
	scanInfo     pages.ScanInfo      // Information related to scanning the media
	preCheckDone bool                // Whether pre-check was done
}

// CreateHeaderBar creates invisible header bar
func (window *Window) CreateHeaderBar() error {
	box, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return err
	}

	st, err := box.GetStyleContext()
	if err != nil {
		return err
	}

	window.handle.SetTitlebar(box)
	st.RemoveClass("titlebar")
	st.RemoveClass("headerbar")
	st.RemoveClass("header")
	st.AddClass("invisible-titlebar")

	return nil
}

// NewWindow creates a new instance of the welcome page
func NewWindow(model *model.SystemInstall, rootDir string, options args.Args) (*Window, error) {
	var err error

	// Create basic window
	window := &Window{
		didInit: false,
		pages:   make(map[int]gtk.IWidget),
		model:   model,
		rootDir: rootDir,
		options: options,
	}

	// Default Icon the application
	gtk.WindowSetDefaultIconName("system-software-install")

	// Construct main window
	window.handle, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return nil, err
	}
	window.handle.SetPosition(gtk.WIN_POS_CENTER)
	window.handle.SetDefaultSize(WindowWidth, WindowHeight)
	window.handle.SetResizable(false)

	// Create invisible header bar
	if err = window.CreateHeaderBar(); err != nil {
		return nil, err
	}

	// Set up the content layout within main layout
	window.mainLayout, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	window.handle.Add(window.mainLayout)

	// Set locale
	utils.SetLocale(model.Language.Code)

	// Create welcome page
	window, err = window.createWelcomePage()
	if err != nil {
		return nil, err
	}

	window.scanInfo.Channel = make(chan bool)
	go func() {
		log.Debug("Scanning media")
		window.scanInfo.Media, err = storage.RescanBlockDevices(window.model.TargetMedias)
		if err != nil {
			log.Warning("Error scanning media %s", err.Error())
		}
		window.scanInfo.Channel <- true
	}()

	return window, nil
}

// createWelcomePage creates the welcome page
func (window *Window) createWelcomePage() (*Window, error) {
	var err error
	// Create banner and add to main layout
	if window.banner, err = NewBanner(); err != nil {
		return nil, err
	}
	window.mainLayout.PackStart(window.banner.GetRootWidget(), false, false, 0)

	// Set up content layout and add to main layout
	window.contentLayout, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	window.mainLayout.PackStart(window.contentLayout, true, true, 0)

	// Set up the root stack and add to content layout
	window.rootStack, err = gtk.StackNew()
	window.rootStack.SetTransitionType(gtk.STACK_TRANSITION_TYPE_CROSSFADE)
	if err != nil {
		return nil, err
	}
	window.contentLayout.PackStart(window.rootStack, true, true, 0)

	// Set up the menu stack and add to root stack
	window.menu.stack, err = gtk.StackNew()
	if err != nil {
		return nil, err
	}
	window.menu.stack.SetTransitionType(gtk.STACK_TRANSITION_TYPE_SLIDE_LEFT_RIGHT)
	window.rootStack.AddTitled(window.menu.stack, "menu", "Menu")

	// Create the welcome page
	pageCreators := []PageConstructor{
		pages.NewLanguagePage,
	}

	for _, f := range pageCreators {
		page, err := f(window, window.model)
		if err != nil {
			return nil, err
		}
		if err = window.AddPage(page); err != nil {
			return nil, err
		}
	}

	// TODO: Remove this temporary code after development phase
	_, err = window.handle.Connect("destroy", func() {
		gtk.MainQuit()
	})
	if err != nil {
		return nil, err
	}

	// Create footer area now
	if err = window.CreateFooter(window.contentLayout); err != nil {
		return nil, err
	}

	window.handle.ShowAll()
	window.ActivatePage(window.menu.welcomePage)

	return window, nil
}

// createPreCheckPage creates the pre-check page
func (window *Window) createPreCheckPage() (*Window, error) {
	window.banner.labelText.SetMarkup(GetWelcomeMessage())

	window.contentLayout.Remove(window.rootStack)
	window.contentLayout.PackStart(window.rootStack, true, true, 0)

	// Our pages
	pageCreators := []PageConstructor{
		// required
		pages.NewPreCheckPage,
	}

	for _, f := range pageCreators {
		page, err := f(window, window.model)
		if err != nil {
			return nil, err
		}
		if err = window.AddPage(page); err != nil {
			return nil, err
		}
	}

	// Show the whole window now
	window.handle.ShowAll()
	window.ActivatePage(window.menu.preCheckPage)

	return window, nil
}

// createMenuPages creates the menu pages
func (window *Window) createMenuPages() (*Window, error) {
	var err error
	window.banner.labelText.SetMarkup(GetWelcomeMessage())
	window.menu.screens = make(map[bool]*ContentView)

	// Set up the stack switcher
	window.menu.switcher, err = NewSwitcher(nil)
	if err != nil {
		return nil, err
	}
	window.menu.switcher.SetStack(window.menu.stack)

	if err = window.InitScreens(); err != nil {
		return nil, err
	}
	window.contentLayout.Remove(window.rootStack)
	window.contentLayout.PackStart(window.menu.switcher.GetRootWidget(), false, false, 0)
	window.contentLayout.PackStart(window.rootStack, true, true, 0)

	// Create footer area
	if err = window.UpdateFooter(window.contentLayout); err != nil {
		return nil, err
	}

	// Create rest of the pages
	pageCreators := []PageConstructor{
		// required
		pages.NewTimezonePage,
		pages.NewKeyboardPage,
		pages.NewDiskConfigPage,
		pages.NewUserAddPage,
		pages.NewTelemetryPage,

		// advanced
		pages.NewBundlePage,
		pages.NewHostnamePage,
		pages.NewConfigKernelPage,

		// always last
		pages.NewInstallPage,
	}

	// Create all pages
	for _, f := range pageCreators {
		page, err := f(window, window.model)
		if err != nil {
			return nil, err
		}
		if err = window.AddPage(page); err != nil {
			return nil, err
		}
	}

	// Show the whole window now
	window.handle.ShowAll()

	// Show the menu view
	window.ShowMenuView()

	return window, nil
}

// ShowMenuView displays the menu view
func (window *Window) ShowMenuView() {
	done := window.menu.screens[ContentViewRequired].IsDone()
	window.buttons.install.SetSensitive(done)

	window.banner.Show()
	window.menu.switcher.Show()

	window.rootStack.SetVisibleChildName("menu")
	window.menu.stack.SetVisibleChildName("required")
	window.buttons.stack.SetVisibleChildName("primary")
}

// InitScreens initializes the switcher screens
func (window *Window) InitScreens() error {
	var err error

	// Set up required screen
	if window.menu.screens[true], err = NewContentView(window); err != nil {
		return err
	}
	window.menu.stack.AddNamed(window.menu.screens[ContentViewRequired].GetRootWidget(), "required")

	// Set up non required screen
	if window.menu.screens[false], err = NewContentView(window); err != nil {
		return err
	}
	window.menu.stack.AddNamed(window.menu.screens[ContentViewAdvanced].GetRootWidget(), "advanced")

	return nil
}

// AddPage adds the page to the relevant switcher screen
func (window *Window) AddPage(page pages.Page) error {
	var (
		err error
		id  int
	)

	id = page.GetID()
	switch id {
	case pages.PageIDWelcome:
		window.menu.welcomePage = page
	case pages.PageIDPreCheck:
		window.menu.preCheckPage = page
	case pages.PageIDInstall:
		window.menu.installPage = page
	default: // Add to the required or advanced (optional) screen
		err := window.menu.screens[page.IsRequired()].AddPage(page)
		if err != nil {
			return err
		}
	}

	// Create a header page wrap for the page
	header, err := PageHeaderNew(page)
	if err != nil {
		return err
	}
	root := header.GetRootWidget()

	// Make available via root stack
	window.pages[id] = root
	window.rootStack.AddNamed(root, "page:"+string(id))

	return nil
}

// createNavButton creates specialised navigation button
func createNavButton(label, style string) (*gtk.Button, error) {
	button, err := gtk.ButtonNewWithLabel(label)
	if err != nil {
		return nil, err
	}

	sc, err := button.GetStyleContext()
	if err != nil {
		return nil, err
	}
	// Not able to set size using CSS
	button.SetSizeRequest(100, 36)
	sc.AddClass(style)

	return button, nil
}

// CreateFooter creates the navigation footer area
func (window *Window) CreateFooter(store *gtk.Box) error {
	var err error

	// Create stack for buttons
	if window.buttons.stack, err = gtk.StackNew(); err != nil {
		return err
	}

	// Set alignment up
	window.buttons.stack.SetMarginTop(4)
	window.buttons.stack.SetMarginBottom(6)
	window.buttons.stack.SetMarginEnd(18)
	window.buttons.stack.SetHAlign(gtk.ALIGN_FILL)
	window.buttons.stack.SetTransitionType(gtk.STACK_TRANSITION_TYPE_CROSSFADE)
	store.PackEnd(window.buttons.stack, false, false, 0)

	// Create box for welcome page buttons
	if window.buttons.boxWelcome, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0); err != nil {
		return err
	}

	// Install button
	if window.buttons.next, err = createNavButton(utils.Locale.Get("NEXT"), "button-confirm"); err != nil {
		return err
	}
	if _, err = window.buttons.next.Connect("clicked", func() { window.pageNext() }); err != nil {
		return err
	}

	// Exit button
	if window.buttons.exit, err = createNavButton(utils.Locale.Get("EXIT"), "button-cancel"); err != nil {
		return err
	}
	if _, err = window.buttons.exit.Connect("clicked", func() { gtk.MainQuit() }); err != nil {
		return err
	}

	// Pack the buttons
	window.buttons.boxWelcome.PackEnd(window.buttons.next, false, false, 4)
	window.buttons.boxWelcome.PackEnd(window.buttons.exit, false, false, 4)

	// Add the boxes
	window.buttons.stack.AddNamed(window.buttons.boxWelcome, "welcome")

	return nil
}

// UpdateFooter updates the navigation footer area
func (window *Window) UpdateFooter(store *gtk.Box) error {
	var err error

	// Install button
	if window.buttons.install, err = createNavButton(utils.Locale.Get("INSTALL"), "button-confirm"); err != nil {
		return err
	}
	if _, err = window.buttons.install.Connect("clicked", func() { window.confirmInstall() }); err != nil {
		return err
	}

	// Exit button
	if window.buttons.quit, err = createNavButton(utils.Locale.Get("EXIT"), "button-cancel"); err != nil {
		return err
	}
	if _, err = window.buttons.quit.Connect("clicked", func() { gtk.MainQuit() }); err != nil {
		return err
	}

	// Back button
	if window.buttons.back, err = createNavButton(utils.Locale.Get("CHANGE LANGUAGE"), "button-cancel"); err != nil {
		return err
	}
	if _, err = window.buttons.back.Connect("clicked", func() { window.launchWelcomeView() }); err != nil {
		return err
	}

	width, _ := window.handle.GetSize() // get current size
	marginEnd := width * 35 / 100
	window.buttons.back.SetMarginEnd(marginEnd) // TODO: MarginStart would be ideal but does not work

	// Confirm button
	if window.buttons.confirm, err = createNavButton(utils.Locale.Get("CONFIRM"), "button-confirm"); err != nil {
		return err
	}
	if _, err = window.buttons.confirm.Connect("clicked", func() { window.pageClosed(true) }); err != nil {
		return err
	}

	// Cancel button
	if window.buttons.cancel, err = createNavButton(utils.Locale.Get("CANCEL"), "button-cancel"); err != nil {
		return err
	}
	if _, err = window.buttons.cancel.Connect("clicked", func() { window.pageClosed(false) }); err != nil {
		return err
	}

	// Create box for primary buttons
	if window.buttons.boxPrimary, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0); err != nil {
		return err
	}
	window.buttons.boxPrimary.PackEnd(window.buttons.install, false, false, 4)
	window.buttons.boxPrimary.PackEnd(window.buttons.quit, false, false, 4)
	window.buttons.boxPrimary.PackEnd(window.buttons.back, false, false, 4)

	// Create box for secondary buttons
	if window.buttons.boxSecondary, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0); err != nil {
		return err
	}
	window.buttons.boxSecondary.PackEnd(window.buttons.confirm, false, false, 4)
	window.buttons.boxSecondary.PackEnd(window.buttons.cancel, false, false, 4)

	// Add the boxes
	window.buttons.stack.AddNamed(window.buttons.boxPrimary, "primary")
	window.buttons.stack.AddNamed(window.buttons.boxSecondary, "secondary")

	return nil
}

// pageNext handles next page.
func (window *Window) pageNext() {
	if !window.preCheckDone { // If pre-check has not been done at least once, launch the pre-check view first
		window.launchPreCheckView()
		window.preCheckDone = true
	} else {
		window.launchMenuView()
	}
}

// pageClosed handles closure of a page.
func (window *Window) pageClosed(applied bool) {
	// If applied, tell page to stash in model
	// otherwise, reset from existing model
	if applied {
		window.menu.currentPage.StoreChanges()
	} else {
		window.menu.currentPage.ResetChanges()
	}

	// Let installation continue if possible
	done := window.menu.screens[ContentViewRequired].IsDone()
	window.buttons.install.SetSensitive(done)

	// Reset the SummaryWidget for responsible controller
	window.menu.screens[window.menu.currentPage.IsRequired()].UpdateView(window.menu.currentPage)

	// Reset currentPage
	window.menu.currentPage = nil

	// Switch UI back to primary view
	window.rootStack.SetVisibleChildName("menu")
	window.banner.Show()
	window.menu.switcher.Show()
	window.buttons.stack.SetVisibleChildName("primary")
}

// ActivatePage customizes common widgets and displays the page
func (window *Window) ActivatePage(page pages.Page) {
	window.menu.currentPage = page
	id := page.GetID()

	// Customize common widgets based on the page being loaded
	switch id {
	case pages.PageIDWelcome:
		window.banner.Show()
		window.buttons.stack.SetVisibleChildName("welcome")
	case pages.PageIDPreCheck:
		window.banner.Show()
		window.buttons.stack.SetVisibleChildName("welcome")
		window.buttons.next.SetLabel(utils.Locale.Get("NEXT")) // This is done just to translate label based on localization
		window.buttons.exit.SetLabel(utils.Locale.Get("EXIT")) // This is done just to translate label based on localization
	case pages.PageIDInstall:
		window.menu.switcher.Hide()
		window.banner.Show()
		window.banner.labelText.SetMarkup(GetThankYouMessage())
		window.buttons.stack.SetVisibleChildName("primary")
		window.buttons.install.Hide()
		window.buttons.back.Hide()
		sc, err := window.buttons.quit.GetStyleContext()
		if err != nil {
			log.Warning("Error getting style context: ", err) // Just log trivial error
		} else {
			sc.RemoveClass("button-cancel")
			sc.AddClass("button-confirm")
		}
		window.buttons.quit.SetSensitive(false)
	case pages.PageIDTelemetry:
		window.menu.switcher.Hide()
		window.banner.Hide()
		window.buttons.stack.SetVisibleChildName("secondary")
		window.buttons.confirm.SetSensitive(true)
		window.buttons.confirm.SetCanDefault(true)
		window.buttons.confirm.GrabDefault()
		window.buttons.confirm.SetLabel(utils.Locale.Get("YES"))
		window.buttons.cancel.SetLabel(utils.Locale.Get("NO"))
	default:
		window.menu.switcher.Hide()
		window.banner.Hide()
		window.buttons.stack.SetVisibleChildName("secondary")
		window.buttons.confirm.SetSensitive(false)
		window.buttons.confirm.SetLabel(utils.Locale.Get("CONFIRM"))
		window.buttons.cancel.SetLabel(utils.Locale.Get("CANCEL"))
	}
	page.ResetChanges()                                // Allow page to take control now
	window.rootStack.SetVisibleChild(window.pages[id]) // Set the root stack to show the new page
}

// SetButtonState is called by the pages to enable/disable certain buttons.
func (window *Window) SetButtonState(button pages.Button, enabled bool) {
	switch button {
	case pages.ButtonCancel:
		window.buttons.cancel.SetSensitive(enabled)
	case pages.ButtonConfirm:
		window.buttons.confirm.SetSensitive(enabled)
	case pages.ButtonQuit:
		window.buttons.quit.SetSensitive(enabled)
	case pages.ButtonBack:
		window.buttons.back.SetSensitive(enabled)
	case pages.ButtonNext:
		window.buttons.next.SetSensitive(enabled)
	case pages.ButtonExit:
		window.buttons.exit.SetSensitive(enabled)
	default:
		log.Error("Undefined button")
	}
}

// SetButtonVisible is called by the pages to view/hide certain buttons.
func (window *Window) SetButtonVisible(button pages.Button, visible bool) {
	switch button {
	case pages.ButtonCancel:
		window.buttons.cancel.SetVisible(visible)
	case pages.ButtonConfirm:
		window.buttons.confirm.SetVisible(visible)
	case pages.ButtonQuit:
		window.buttons.quit.SetVisible(visible)
	case pages.ButtonBack:
		window.buttons.back.SetVisible(visible)
	case pages.ButtonNext:
		window.buttons.next.SetVisible(visible)
	case pages.ButtonExit:
		window.buttons.exit.SetVisible(visible)
	default:
		log.Error("Undefined button")
	}
}

// launchWelcomeView launches the welcome view
func (window *Window) launchWelcomeView() {
	window.mainLayout.Remove(window.contentLayout)
	window.mainLayout.Remove(window.banner.GetRootWidget())
	if _, err := window.createWelcomePage(); err != nil {
		window.Panic(err)
	}
}

// launchPreCheckView launches the pre-check view view
func (window *Window) launchPreCheckView() {
	log.Debug("Launching PreCheckView")
	window.menu.currentPage.StoreChanges()

	if _, err := window.createPreCheckPage(); err != nil {
		window.Panic(err)
	}
}

// launchMenuView launches the menu view
func (window *Window) launchMenuView() {
	log.Debug("Launching MenuView")
	window.menu.currentPage.StoreChanges()

	if _, err := window.createMenuPages(); err != nil {
		window.Panic(err)
	}
}

// confirmInstall prompts the user for confirmation before installing
func (window *Window) confirmInstall() {
	var text, primaryText, secondaryText string
	var err error

	if window.model.InstallSelected.EraseDisk {
		primaryText = utils.Locale.Get(storage.DestructiveWarning)
	} else if window.model.InstallSelected.DataLoss {
		primaryText = utils.Locale.Get(storage.DataLossWarning)
	} else if window.model.InstallSelected.WholeDisk {
		primaryText = utils.Locale.Get(storage.SafeWholeWarning)
	} else {
		primaryText = utils.Locale.Get(storage.SafePartialWarning)
	}

	// Build the string with the media being modified
	targets := []string{}
	if len(window.model.TargetMedias) == 0 {
		targets = append(targets, utils.Locale.Get("None"))
	} else {
		for _, media := range window.model.TargetMedias {
			targets = append(targets, media.GetDeviceFile())
		}
	}
	secondaryText = utils.Locale.Get("Target Media") + ": " + strings.Join(targets, ", ")

	title := utils.Locale.Get(storage.ConfirmInstallation)
	text = primaryText + "\n" + "<small>" + secondaryText + "</small>"

	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	contentBox.SetHAlign(gtk.ALIGN_FILL)
	contentBox.SetMarginBottom(common.TopBottomMargin)
	if err != nil {
		log.Error("Error creating box", err)
		return
	}

	label, err := gtk.LabelNew(text)
	if err != nil {
		log.Error("Error creating label", err)
		return
	}
	label.SetUseMarkup(true)
	label.SetHAlign(gtk.ALIGN_START)
	contentBox.PackStart(label, false, true, 0)

	dialog, err := common.CreateDialogOkCancel(contentBox, title, utils.Locale.Get("CONFIRM"), utils.Locale.Get("CANCEL"))
	if err != nil {
		log.Error("Error creating dialog", err)
		return
	}
	_, err = dialog.Connect("response", window.dialogResponse)
	if err != nil {
		log.Error("Error connecting to dialog", err)
		return
	}
	dialog.ShowAll()
	dialog.Run()
}

// dialogResponse handles the response from the dialog message
func (window *Window) dialogResponse(msgDialog *gtk.Dialog, responseType gtk.ResponseType) {
	if responseType == gtk.RESPONSE_OK {
		window.ActivatePage(window.menu.installPage)
	}
	msgDialog.Destroy()
}

// GetOptions returns the options given to the window
func (window *Window) GetOptions() args.Args {
	return window.options
}

// GetRootDir returns the root dir
func (window *Window) GetRootDir() string {
	return window.rootDir
}

// GetScanChannel is the getter for ScanInfo Channel
func (window *Window) GetScanChannel() chan bool {
	return window.scanInfo.Channel
}

// GetScanDone is the getter for ScanInfo Done
func (window *Window) GetScanDone() bool {
	return window.scanInfo.Done
}

// SetScanDone is the setter for ScanInfo Done
func (window *Window) SetScanDone(done bool) {
	window.scanInfo.Done = done
}

// GetScanMedia is the getter for ScanInfo Media
func (window *Window) GetScanMedia() []*storage.BlockDevice {
	return window.scanInfo.Media
}

// SetScanMedia is the setter for ScanInfo Media
func (window *Window) SetScanMedia(scannedMedia []*storage.BlockDevice) {
	window.scanInfo.Media = scannedMedia
}

// Panic handles the gui crashes
func (window *Window) Panic(err error) {
	log.Debug("Panic")
	if errLog := window.model.Telemetry.LogRecord("guipanic", 3, err.Error()); errLog != nil {
		log.Error("Failed to log Telemetry fail record: %s", "guipanic")
	}
	log.RequestCrashInfo()
	displayErrorDialog(err)
}

// GetWelcomeMessage gets the welcome message
func GetWelcomeMessage() string {
	text := "<span font-size='xx-large'>" + utils.Locale.Get("Welcome to Clear Linux* OS Desktop Installation") + "</span>"
	if model.Version != model.DemoVersion {
		text += "\n\n<small>" + utils.Locale.Get("VERSION %s", model.Version) + "</small>"
	}

	return text
}

// GetThankYouMessage gets the thank you message
func GetThankYouMessage() string {
	text := "<span font-size='xx-large'>" + utils.Locale.Get("Thank you for choosing Clear Linux* OS") + "</span>"
	if model.Version != model.DemoVersion {
		text += "\n\n<small>" + utils.Locale.Get("VERSION %s", model.Version) + "</small>"
	}

	return text
}

func displayErrorDialog(err error) {
	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	contentBox.SetHAlign(gtk.ALIGN_FILL)
	if err != nil {
		log.Error("Error creating box", err)
		return
	}

	label, err := common.SetLabel(log.GetCrashInfoMsg(), "label-error", 0.0)
	if err != nil {
		log.Error("Error creating label", err)
		return
	}
	label.SetHAlign(gtk.ALIGN_CENTER)
	label.SetMarginBottom(common.TopBottomMargin)
	label.SetSelectable(true)
	contentBox.PackStart(label, true, true, 0)

	title := utils.Locale.Get("Something went wrong...")
	dialog, err := common.CreateDialogOneButton(contentBox, title, utils.Locale.Get("OK"), "button-confirm")
	if err != nil {
		log.Error("Error creating dialog", err)
		return
	}
	_, err = dialog.Connect("response", func() {
		dialog.Destroy()
		gtk.MainQuit() // Exit Installer
	})
	if err != nil {
		log.Error("Error connecting to dialog", err)
		return
	}
	dialog.ShowAll()
	dialog.Run()
}
