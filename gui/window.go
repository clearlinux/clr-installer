// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"log"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/gui/pages"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
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
		switcher    *Switcher             // Allow switching between main menu
		stack       *gtk.Stack            // Menu switching
		screens     map[bool]*ContentView // Mapping to content views
		welcomePage pages.Page            // Pointer to the welcome page
		currentPage pages.Page            // Pointer to the currently open page
		installPage pages.Page            // Pointer to the installer page
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

	didInit bool                // Whether initialized the view animation
	pages   map[int]gtk.IWidget // Mapping to each root page
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

	// Construct main window
	window.handle, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return nil, err
	}
	window.handle.SetPosition(gtk.WIN_POS_CENTER)
	window.handle.SetDefaultSize(800, 500)
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

	// Our pages
	pageCreators := []PageConstructor{
		// required
		pages.NewTimezonePage,
		pages.NewKeyboardPage,
		pages.NewDiskConfigPage,
		pages.NewTelemetryPage,

		// advanced
		pages.NewBundlePage,
		pages.NewUserAddPage,
		pages.NewHostnamePage,

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

	if id == pages.PageIDWelcome {
		window.menu.welcomePage = page
	} else if id == pages.PageIDInstall {
		window.menu.installPage = page
	} else { // Add to the required or advanced (optional) screen
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
	if _, err = window.buttons.next.Connect("clicked", func() { window.launchMenuView() }); err != nil {
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
	if _, err = window.buttons.install.Connect("clicked", func() { window.launchInstallView() }); err != nil {
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
	window.buttons.back.SetMarginEnd(250) // TODO: MarginStart would be ideal but does not work

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

// ActivatePage displays the page
func (window *Window) ActivatePage(page pages.Page) {
	window.menu.currentPage = page
	id := page.GetID()

	if id == pages.PageIDWelcome { // Welcome Page
		window.banner.Show()
		window.buttons.stack.SetVisibleChildName("welcome")
	} else if id == pages.PageIDInstall { // Install Page
		window.menu.switcher.Hide()
		window.banner.Show()
		window.banner.labelText.SetMarkup(GetThankYouMessage())
		window.buttons.stack.SetVisibleChildName("primary")
		window.buttons.install.Hide()
		window.buttons.back.Hide()
	} else {
		window.menu.switcher.Hide()
		window.banner.Hide()
		window.buttons.stack.SetVisibleChildName("secondary")
		window.SetButtonState(pages.ButtonConfirm, false)
	}

	// Allow page to take control now
	page.ResetChanges()

	// Set the root stack to show the new page
	window.rootStack.SetVisibleChild(window.pages[id])
}

// SetButtonState is called by the pages to enable/disable certain buttons.
func (window *Window) SetButtonState(flags pages.Button, enabled bool) {
	if window.menu.currentPage.GetID() != pages.PageIDWelcome {
		if flags&pages.ButtonCancel == pages.ButtonCancel {
			window.buttons.cancel.SetSensitive(enabled)
		}
		if flags&pages.ButtonConfirm == pages.ButtonConfirm {
			window.buttons.confirm.SetSensitive(enabled)
		}
		if flags&pages.ButtonQuit == pages.ButtonQuit {
			window.buttons.quit.SetSensitive(enabled)
		}
		if flags&pages.ButtonBack == pages.ButtonBack {
			window.buttons.back.SetSensitive(enabled)
		}
	} else {
		if flags&pages.ButtonNext == pages.ButtonNext {
			window.buttons.next.SetSensitive(enabled)
		}
		if flags&pages.ButtonExit == pages.ButtonExit {
			window.buttons.exit.SetSensitive(enabled)
		}
	}
}

// launchWelcomeView launches the welcome view
func (window *Window) launchWelcomeView() {
	window.mainLayout.Remove(window.contentLayout)
	window.mainLayout.Remove(window.banner.GetRootWidget())
	if _, err := window.createWelcomePage(); err != nil {
		log.Fatal(err) // TODO: Handle error
	}
}

// launchMenuView launches the menu view
func (window *Window) launchMenuView() {
	window.menu.currentPage.StoreChanges()
	if _, err := window.createMenuPages(); err != nil {
		log.Fatal(err) // TODO: Handle error
	}
}

// launchInstallView launches the install view
func (window *Window) launchInstallView() {
	window.ActivatePage(window.menu.installPage)
}

// GetOptions returns the options given to the window
func (window *Window) GetOptions() args.Args {
	return window.options
}

// GetRootDir returns the root dir
func (window *Window) GetRootDir() string {
	return window.rootDir
}

// GetWelcomeMessage gets the welcome message
func GetWelcomeMessage() string {
	text := "<span font-size='xx-large'>" + utils.Locale.Get("Welcome to Clear Linux* Desktop Installation") + "</span>"
	text += "\n\n<small>" + utils.Locale.Get("VERSION %s", model.Version) + "</small>"

	return text
}

// GetThankYouMessage gets the thank you message
func GetThankYouMessage() string {
	text := "<span font-size='xx-large'>" + utils.Locale.Get("Thank you for choosing Clear Linux*") + "</span>"
	text += "\n\n<small>" + utils.Locale.Get("VERSION %s", model.Version) + "</small>"

	return text
}
