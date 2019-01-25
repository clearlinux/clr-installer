// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"fmt"
	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/gui/pages"
	"github.com/clearlinux/clr-installer/model"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// PageConstructor is a typedef of the constructors for our pages
type PageConstructor func(controller pages.Controller, model *model.SystemInstall) (pages.Page, error)

// Window provides management of the underlying GtkWindow and
// associated windows to provide a level of OOP abstraction.
type Window struct {
	handle        *gtk.Window // Abstract the underlying GtkWindow
	rootStack     *gtk.Stack  // Root-level stack
	layout        *gtk.Box    // Main layout (vertical)
	contentLayout *gtk.Box    // content layout (horizontal)
	banner        *Banner     // Top banner

	options args.Args // installer args
	rootDir string    // Root directory

	// Menus
	menu struct {
		stack       *gtk.Stack            // Menu switching
		switcher    *Switcher             // Allow switching between main menu
		screens     map[bool]*ContentView // Mapping to content views
		currentPage pages.Page            // Pointer to the currently open page
		install     pages.Page            // Pointer to the installer page
	}

	// Buttons

	buttons struct {
		stack        *gtk.Stack // Storage for buttons
		boxPrimary   *gtk.Box   // Storage for main buttons (install/quit)
		boxSecondary *gtk.Box   // Storage for secondary buttons (confirm/cancel)

		confirm *gtk.Button // Apply changes
		cancel  *gtk.Button // Cancel changes
		install *gtk.Button // Install Clear Linux
		quit    *gtk.Button // Quit the installer
	}

	didInit bool                 // Whether we've inited the view animation
	model   *model.SystemInstall // Our model
	pages   map[int]gtk.IWidget  // Mapping to each root page
}

// ConstructHeaderBar attempts creation of the headerbar
func (window *Window) ConstructHeaderBar() error {
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

// NewWindow creates a new Window instance
func NewWindow(model *model.SystemInstall, rootDir string, options args.Args) (*Window, error) {
	var err error

	// Construct basic window
	window := &Window{
		didInit: false,
		pages:   make(map[int]gtk.IWidget),
		model:   model,
		rootDir: rootDir,
		options: options,
	}
	window.menu.screens = make(map[bool]*ContentView)

	// Construct main window
	window.handle, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return nil, err
	}

	// Need HeaderBar ?
	if err = window.ConstructHeaderBar(); err != nil {
		return nil, err
	}

	// Set up basic window attributes
	window.handle.SetTitle("Install Clear Linux OS")
	window.handle.SetPosition(gtk.WIN_POS_CENTER)
	window.handle.SetDefaultSize(800, 500)
	window.handle.SetResizable(false)

	// Set up the main layout
	window.layout, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	window.handle.Add(window.layout)

	// Set up the stack switcher
	window.menu.switcher, err = NewSwitcher(nil)
	if err != nil {
		return nil, err
	}

	// To add the *main* content
	window.contentLayout, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	window.layout.PackStart(window.contentLayout, true, true, 0)

	// Create the banner
	if window.banner, err = NewBanner(); err != nil {
		return nil, err
	}
	window.contentLayout.PackStart(window.banner.GetRootWidget(), false, false, 0)

	// Set up the root stack
	window.rootStack, err = gtk.StackNew()
	window.rootStack.SetTransitionType(gtk.STACK_TRANSITION_TYPE_CROSSFADE)
	if err != nil {
		return nil, err
	}

	// We want vertical layout here with buttons above the rootstack
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}
	window.contentLayout.PackStart(vbox, true, true, 0)
	vbox.PackStart(window.menu.switcher.GetRootWidget(), false, false, 0)
	vbox.PackStart(window.rootStack, true, true, 0)

	// Set up the content stack
	window.menu.stack, err = gtk.StackNew()
	if err != nil {
		return nil, err
	}
	window.menu.stack.SetTransitionType(gtk.STACK_TRANSITION_TYPE_SLIDE_LEFT_RIGHT)
	window.menu.switcher.SetStack(window.menu.stack)

	// Add menu stack to root stack
	window.rootStack.AddTitled(window.menu.stack, "menu", "Menu")

	// Temporary for development testing: Close window when asked
	window.handle.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// On map, expose the revealer
	window.handle.Connect("map", window.handleMap)

	// Set up primary content views
	if err = window.InitScreens(); err != nil {
		return nil, err
	}

	// Create footer area now
	if err = window.CreateFooter(vbox); err != nil {
		return nil, err
	}

	// Our pages
	pageCreators := []PageConstructor{
		// required
		pages.NewTimezonePage,
		pages.NewLanguagePage,
		pages.NewKeyboardPage,
		pages.NewDiskConfigPage,
		pages.NewTelemetryPage,

		// advanced
		pages.NewBundlePage,

		// Always last
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

	// Let installation continue if possible
	done := window.menu.screens[ContentViewRequired].IsDone()
	window.buttons.install.SetSensitive(done)

	// Show the whole window now
	window.handle.ShowAll()

	// Show the default view
	window.ShowDefaultView()

	return window, nil
}

// ShowDefaultView will set up the default window view
func (window *Window) ShowDefaultView() {
	// Ensure menu page is set
	window.menu.stack.SetVisibleChildName("required")

	// And root stack
	window.rootStack.SetVisibleChildName("menu")
}

// InitScreens will set up the content views
func (window *Window) InitScreens() error {
	var err error

	// Set up required screen
	if window.menu.screens[true], err = NewContentView(window); err != nil {
		return err
	}
	window.menu.stack.AddTitled(window.menu.screens[ContentViewRequired].GetRootWidget(), "required", "REQUIRED OPTIONS\nTakes approximately 2 minutes")

	// Set up non required screen
	if window.menu.screens[false], err = NewContentView(window); err != nil {
		return err
	}
	window.menu.stack.AddTitled(window.menu.screens[ContentViewAdvanced].GetRootWidget(), "advanced", "ADVANCED OPTIONS\nCustomize setup")

	return nil
}

// AddPage will add the page to the relevant screen
func (window *Window) AddPage(page pages.Page) error {
	var (
		err error
		id  int
	)

	id = page.GetID()

	// Non-installer pages go into the menu..
	if id != pages.PageIDInstall {
		// Add to the required or advanced(optional) screen
		err := window.menu.screens[page.IsRequired()].AddPage(page)
		if err != nil {
			return err
		}
	} else {
		// Cache the special page
		window.menu.install = page
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
func createNavButton(label string) (*gtk.Button, error) {
	var st *gtk.StyleContext
	button, err := gtk.ButtonNewWithLabel(label)
	if err != nil {
		return nil, err
	}

	st, err = button.GetStyleContext()
	if err != nil {
		return nil, err
	}
	st.AddClass("nav-button")
	return button, nil
}

// CreateFooter creates our navigation footer area
func (window *Window) CreateFooter(store *gtk.Box) error {
	var err error

	// Create stack for buttons
	if window.buttons.stack, err = gtk.StackNew(); err != nil {
		return err
	}

	// Set alignment up
	window.buttons.stack.SetMarginTop(4)
	window.buttons.stack.SetMarginBottom(6)
	window.buttons.stack.SetMarginEnd(24)
	window.buttons.stack.SetHAlign(gtk.ALIGN_END)
	window.buttons.stack.SetTransitionType(gtk.STACK_TRANSITION_TYPE_CROSSFADE)
	store.PackEnd(window.buttons.stack, false, false, 0)

	// Create box for primary buttons
	if window.buttons.boxPrimary, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0); err != nil {
		return err
	}

	// Install button
	if window.buttons.install, err = createNavButton("INSTALL"); err != nil {
		return err
	}
	window.buttons.install.Connect("clicked", func() { window.beginInstall() })

	// Exit button
	if window.buttons.quit, err = createNavButton("EXIT"); err != nil {
		return err
	}
	window.buttons.quit.Connect("clicked", func() {
		gtk.MainQuit()
	})

	// Create box for secondary buttons
	if window.buttons.boxSecondary, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0); err != nil {
		return err
	}

	// Confirm button
	if window.buttons.confirm, err = createNavButton("CONFIRM"); err != nil {
		return err
	}
	window.buttons.confirm.Connect("clicked", func() { window.pageClosed(true) })

	// Cancel button
	if window.buttons.cancel, err = createNavButton("CANCEL"); err != nil {
		return err
	}
	window.buttons.cancel.Connect("clicked", func() { window.pageClosed(false) })

	// Pack the buttons
	window.buttons.boxPrimary.PackEnd(window.buttons.install, false, false, 4)
	window.buttons.boxPrimary.PackEnd(window.buttons.quit, false, false, 4)
	window.buttons.boxSecondary.PackEnd(window.buttons.confirm, false, false, 4)
	window.buttons.boxSecondary.PackEnd(window.buttons.cancel, false, false, 4)

	// Add the boxes
	window.buttons.stack.AddNamed(window.buttons.boxPrimary, "primary")
	window.buttons.stack.AddNamed(window.buttons.boxSecondary, "secondary")

	return nil
}

// We've been mapped on screen
func (window *Window) handleMap() {
	if window.didInit {
		return
	}
	glib.TimeoutAdd(200, func() bool {
		if !window.didInit {
			window.banner.ShowFirst()
			window.menu.switcher.Show()
			window.menu.stack.SetVisibleChildName("required")
			window.didInit = true
		}
		return false
	})
}

// pageClosed handles closure of a page. We're interested in whether
// the change was "applied" or not.
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

// ActivatePage will set the view as visible.
func (window *Window) ActivatePage(page pages.Page) {
	fmt.Println("Activating: " + page.GetSummary())
	window.menu.currentPage = page

	id := page.GetID()
	window.menu.switcher.Hide()

	// Install page?
	if id == pages.PageIDInstall {
		// Show banner so we can be pretty
		window.banner.Show()
		window.banner.InstallMode()

		window.buttons.stack.SetVisibleChildName("primary")
		window.buttons.install.Hide()
	} else {
		// Hide banner so we can get more room
		window.banner.Hide()

		// Non-install page
		window.buttons.stack.SetVisibleChildName("secondary")
		// Update the new page
		window.SetButtonState(pages.ButtonConfirm, false)
	}

	// Allow page to take control now
	page.ResetChanges()

	// Set the root stack to show the new page
	window.rootStack.SetVisibleChild(window.pages[id])
}

// SetButtonState is called by the pages to enable/disable certain buttons.
func (window *Window) SetButtonState(flags pages.Button, enabled bool) {
	if flags&pages.ButtonCancel == pages.ButtonCancel {
		window.buttons.cancel.SetSensitive(enabled)
	}
	if flags&pages.ButtonConfirm == pages.ButtonConfirm {
		window.buttons.confirm.SetSensitive(enabled)
	}
	if flags&pages.ButtonQuit == pages.ButtonQuit {
		window.buttons.quit.SetSensitive(enabled)
	}
}

// beginInstall begins the real installation routine
func (window *Window) beginInstall() {
	window.ActivatePage(window.menu.install)
}

// GetOptions will return the options given to the window
func (window *Window) GetOptions() args.Args {
	return window.options
}

// GetRootDir will return the root dir
func (window *Window) GetRootDir() string {
	return window.rootDir
}
