// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/gui/network"
	"github.com/clearlinux/clr-installer/gui/pages"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/swupd"
	"github.com/clearlinux/clr-installer/syscheck"
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

	scroller  *gtk.ScrolledWindow
	useScroll bool // need to embed content layout to scrolled window

	contentContainer *gtk.Container // main container for content

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

	didInit  bool                // Whether initialized the view animation
	pages    map[int]gtk.IWidget // Mapping to each root page
	scanInfo pages.ScanInfo      // Information related to scanning the media
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

// FitToMonitorSize sets a proper size for the installer if it does
// not fit to the monitor size
func FitToMonitorSize(win *Window, w float32, h float32) error {
	screen, err := gdk.ScreenGetDefault()
	if err != nil {
		return err
	}

	dpy, err := screen.GetDisplay()
	if err != nil {
		return err
	}

	mon, err := dpy.GetPrimaryMonitor()
	if err != nil {
		return err
	}

	rect := mon.GetGeometry()

	monW := rect.GetWidth()
	monH := rect.GetHeight()

	// fit to monitor
	if WindowWidth > monW || WindowHeight > monH {
		relW := int(w * float32(monW))
		relH := int(h * float32(monH))
		win.handle.SetDefaultSize(relW, relH)
		win.useScroll = true
	}

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
	window.useScroll = false

	err = FitToMonitorSize(window, 0.9, 0.8)
	if err != nil {
		log.Warning("Could not query monitor size %s", err.Error())
	}

	// Create invisible header bar
	if err = window.CreateHeaderBar(); err != nil {
		return nil, err
	}

	window.mainLayout, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	window.handle.Add(window.mainLayout)

	// Set locale
	utils.SetLocale(model.Language.Code)

	// Check if we can boot EFI
	if !utils.HostHasEFI() {
		log.Warning("Failed to find EFI firmware, falling back to legacy BIOS for installation.")
		model.MediaOpts.LegacyBios = true
	}

	// Create welcome page
	window, err = window.createWelcomePage()
	if err != nil {
		return nil, err
	}

	// Launch the first page
	// If pre-check has been done at least once, start on the menu page
	if window.model.PreCheckDone {
		window.launchMenuView()
	} else {
		window.model.PreCheckDone = true
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

	// default container
	window.contentContainer = &window.contentLayout.Container

	// useScroll is set, pack default container to a scrolledwindow
	if window.useScroll {
		window.scroller, err = gtk.ScrolledWindowNew(nil, nil)
		if err != nil {
			return nil, err
		}
		window.contentContainer = &window.scroller.Bin.Container
		window.scroller.Add(window.contentLayout)
	}

	window.mainLayout.PackStart(window.contentContainer, true, true, 0)

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
	_ = window.handle.Connect("destroy", func() { gtk.MainQuit() })

	// Create footer area now
	if err = window.CreateFooter(); err != nil {
		return nil, err
	}

	window.handle.ShowAll()
	window.ActivatePage(window.menu.welcomePage)

	// Create syscheck pop-up when system check fails
	if syscheckErr := syscheck.RunSystemCheck(true); syscheckErr != nil {
		_ = glib.IdleAdd(func() { displaySyscheckDialog(syscheckErr) })
	}

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
	if err = window.UpdateFooter(); err != nil {
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
		pages.NewSwupdConfigPage,
		pages.NewNetworkPage,

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
	window.rootStack.AddNamed(root, "page:"+fmt.Sprint(id))

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
func (window *Window) CreateFooter() error {
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
	window.contentLayout.PackEnd(window.buttons.stack, false, false, 0)

	// Create box for welcome page buttons
	if window.buttons.boxWelcome, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0); err != nil {
		return err
	}

	// Install button
	if window.buttons.next, err = createNavButton(utils.Locale.Get("NEXT"), "button-confirm"); err != nil {
		return err
	}
	_ = window.buttons.next.Connect("clicked", func() { window.onNextClick() })

	// Exit button
	if window.buttons.exit, err = createNavButton(utils.Locale.Get("EXIT"), "button-cancel"); err != nil {
		return err
	}
	_ = window.buttons.exit.Connect("clicked", func() { gtk.MainQuit() })

	// Pack the buttons
	window.buttons.boxWelcome.PackEnd(window.buttons.next, false, false, 4)
	window.buttons.boxWelcome.PackEnd(window.buttons.exit, false, false, 4)

	// Add the boxes
	window.buttons.stack.AddNamed(window.buttons.boxWelcome, "welcome")

	return nil
}

// UpdateFooter updates the navigation footer area
func (window *Window) UpdateFooter() error {
	var err error

	// Install button
	if window.buttons.install, err = createNavButton(utils.Locale.Get("INSTALL"), "button-confirm"); err != nil {
		return err
	}
	_ = window.buttons.install.Connect("clicked", func() { window.confirmInstall() })

	// Exit button
	if window.buttons.quit, err = createNavButton(utils.Locale.Get("EXIT"), "button-cancel"); err != nil {
		return err
	}
	_ = window.buttons.quit.Connect("clicked", func() { gtk.MainQuit() })

	// Back button
	if window.buttons.back, err = createNavButton(utils.Locale.Get("CHANGE LANGUAGE"), "button-cancel"); err != nil {
		return err
	}
	_ = window.buttons.back.Connect("clicked", func() { window.launchWelcomeView() })

	window.buttons.back.SetMarginStart(common.StartEndMargin)

	// Confirm button
	if window.buttons.confirm, err = createNavButton(utils.Locale.Get("CONFIRM"), "button-confirm"); err != nil {
		return err
	}
	_ = window.buttons.confirm.Connect("clicked", func() { window.onConfirmClick() })

	// Cancel button
	if window.buttons.cancel, err = createNavButton(utils.Locale.Get("CANCEL"), "button-cancel"); err != nil {
		return err
	}
	_ = window.buttons.cancel.Connect("clicked", func() { window.onCancelClick() })

	// Create box for primary buttons
	if window.buttons.boxPrimary, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0); err != nil {
		return err
	}
	window.buttons.boxPrimary.PackEnd(window.buttons.install, false, false, 4)
	window.buttons.boxPrimary.PackEnd(window.buttons.quit, false, false, 4)
	window.buttons.boxPrimary.PackStart(window.buttons.back, false, false, 4)

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

// onNextClick handles the Next button click
func (window *Window) onNextClick() {
	window.launchMenuView()
}

// onConfirmClick handles the Confirm button click.
func (window *Window) onConfirmClick() {
	window.menu.currentPage.StoreChanges()

	// Close the page only if the page is done.
	if window.menu.currentPage.IsDone() {
		window.closePage()
	}
}

// onCancelClick handles the Cancel button click.
func (window *Window) onCancelClick() {
	window.menu.currentPage.ResetChanges()

	window.closePage()
}

// closePage closes the current page and displays the Menu page
func (window *Window) closePage() {
	// Reset the SummaryWidget for responsible controller
	window.menu.screens[window.menu.currentPage.IsRequired()].UpdateView(window.menu.currentPage)

	// Reset currentPage
	window.menu.currentPage = nil

	// Switch UI back to primary view
	window.rootStack.SetVisibleChildName("menu")
	window.banner.Show()
	window.menu.switcher.Show()
	window.buttons.stack.SetVisibleChildName("primary")

	// Enable installation if all required pages are done
	requiredDone := window.menu.screens[ContentViewRequired].IsDone()
	window.buttons.install.SetSensitive(requiredDone)
}

// ActivatePage customizes common widgets and displays the page
func (window *Window) ActivatePage(page pages.Page) {
	id := page.GetID()

	// Customize common widgets based on the page being loaded
	switch id {
	case pages.PageIDWelcome:
		window.banner.Show()
		window.buttons.stack.SetVisibleChildName("welcome")
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
	case pages.PageIDNetwork:
		// Launches network check pop-up without changing page
		if _, err := network.RunNetworkTest(window.model); err != nil {
			log.Warning("Error running network test: ", err)
		}
		return
	case pages.PageIDBundle:
		window.menu.switcher.Hide()
		window.banner.Hide()
		window.buttons.stack.SetVisibleChildName("secondary")
		window.buttons.confirm.SetSensitive(false)
		window.buttons.confirm.SetLabel(utils.Locale.Get("CONFIRM"))
		window.buttons.cancel.SetLabel(utils.Locale.Get("CANCEL"))
		page.ResetChanges()
	default:
		window.menu.switcher.Hide()
		window.banner.Hide()
		window.buttons.stack.SetVisibleChildName("secondary")
		window.buttons.confirm.SetSensitive(false)
		window.buttons.confirm.SetLabel(utils.Locale.Get("CONFIRM"))
		window.buttons.cancel.SetLabel(utils.Locale.Get("CANCEL"))
	}
	window.menu.currentPage = page
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
	window.mainLayout.Remove(window.contentContainer)

	window.mainLayout.Remove(window.banner.GetRootWidget())
	if _, err := window.createWelcomePage(); err != nil {
		window.Panic(err)
	}
}

// launchMenuView launches the menu view
func (window *Window) launchMenuView() {
	log.Debug("Launching MenuView")
	if window.menu.currentPage != nil {
		window.menu.currentPage.StoreChanges()
	}

	if _, err := window.createMenuPages(); err != nil {
		window.Panic(err)
	}
}

// writeToConfirmInstallDialog is a helper function to write to dialog for confirm installation window
func writeToConfirmInstallDialog(buffer *gtk.TextBuffer, dryRunResults *storage.DryRunType) {
	for _, media := range *dryRunResults.UnPlannedDestructiveResults {
		log.Debug("OtherMediaChange: %s", media)
		buffer.InsertMarkup(buffer.GetEndIter(),
			"<b><span foreground=\"#FDB814\">"+media+"</span></b>\n")
	}

	for _, media := range *dryRunResults.TargetResults {
		log.Debug("MediaChange: %s", media)
		buffer.Insert(buffer.GetEndIter(), media+"\n")
	}
}

func setConfirmButtonState(dialog *gtk.Dialog, window *Window) error {
	var err error
	if storage.GetImpactOnOtherDisks() && !window.model.MediaOpts.ForceDestructive {
		if buttonIWidget, err := dialog.GetWidgetForResponse(gtk.RESPONSE_OK); err == nil {
			confirmButton := buttonIWidget.ToWidget()
			confirmButton.SetSensitive(false)
			return nil
		}
		return err
	}
	return nil
}

// confirmInstall prompts the user for confirmation before installing
func (window *Window) confirmInstall() {
	var primaryText, secondaryText string

	eraseDisk := false
	dataLoss := false
	wholeDisk := false

	// Build the string with the media being modified
	targets := []string{}
	if len(window.model.TargetMedias) == 0 {
		targets = append(targets, utils.Locale.Get("None"))
	} else {
		for _, media := range window.model.TargetMedias {
			targets = append(targets, media.GetDeviceFile())
			if val, ok := window.model.InstallSelected[media.Name]; ok {
				eraseDisk = eraseDisk || val.EraseDisk
				dataLoss = dataLoss || val.DataLoss
				wholeDisk = wholeDisk || val.WholeDisk
			}
		}
	}
	sort.Strings(targets)

	if eraseDisk {
		primaryText = utils.Locale.Get(storage.DestructiveWarning)
	} else if dataLoss {
		primaryText = utils.Locale.Get(storage.DataLossWarning)
	} else if wholeDisk {
		primaryText = utils.Locale.Get(storage.SafeWholeWarning)
	} else {
		primaryText = utils.Locale.Get(storage.SafePartialWarning)
	}

	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	contentBox.SetHAlign(gtk.ALIGN_FILL)
	contentBox.SetMarginBottom(common.TopBottomMargin)
	if err != nil {
		log.Error("Error creating box", err)
		return
	}

	secondaryText = utils.Locale.Get("Target Media") + ": " + strings.Join(targets, ", ")

	title := utils.Locale.Get(storage.ConfirmInstallation)

	style := ""
	if eraseDisk {
		style = "label-error"
	}
	pLabel, err := common.SetLabel(primaryText, style, 0)
	if err != nil {
		log.Error("Error creating pLabel", err)
		return
	}
	pLabel.SetUseMarkup(true)
	pLabel.SetHAlign(gtk.ALIGN_START)
	contentBox.PackStart(pLabel, false, true, 2)

	sLabel, err := common.SetLabel(secondaryText, "", 0)
	if err != nil {
		log.Error("Error creating sLabel", err)
		return
	}
	sLabel.SetUseMarkup(true)
	sLabel.SetHAlign(gtk.ALIGN_START)
	contentBox.PackStart(sLabel, false, true, 0)

	textArea, err := gtk.TextViewNew()
	if err != nil {
		log.Error("Error creating textArea", err)
		return
	}
	textArea.SetEditable(false)

	buffer, err := textArea.GetBuffer()
	if err != nil {
		log.Error("Error getter textArea buffer", err)
		return
	}
	// Set up the scroller
	scroll, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		log.Error("Error creating ScrolledWindow", err)
		return
	}
	scroll.SetMarginTop(20)
	// Set the scroll policy
	scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	// Set shadow type
	scroll.SetShadowType(gtk.SHADOW_NONE)

	scroll.SetSizeRequest(300, 200)
	scroll.Add(textArea)
	contentBox.PackStart(scroll, false, true, 0)

	dialog, err := common.CreateDialogOkCancel(contentBox, title,
		utils.Locale.Get("CONFIRM"), utils.Locale.Get("CANCEL"))
	if err != nil {
		log.Error("Error creating dialog", err)
		return
	}

	_ = dialog.Connect("response", window.dialogResponse)

	// Valid network is required to install without offline content or additional bundles
	if (!swupd.OfflineIsUsable(utils.VersionUintString(window.model.Version), window.options) ||
		len(window.model.UserBundles) != 0) && !controller.NetworkPassing {
		if ret, err := network.RunNetworkTest(window.model); ret == network.NetTestErr {
			log.Warning("Error running network test: ", err)
			return
		}
	}

	if !controller.NetworkPassing {
		if !swupd.OfflineIsUsable(utils.VersionUintString(window.model.Version), window.options) {
			// Cannot install without network or offline content
			return
		} else if len(window.model.UserBundles) != 0 {
			// Cannot install without network and additional bundles. Allow user to remove bundles
			// and continue the install
			offlineMsg := utils.Locale.Get("Offline Install: Removing additional bundles")
			buffer.Insert(buffer.GetEndIter(), offlineMsg+"\n")

			buttonIWidget, err := dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
			if err != nil {
				log.Error("Error getting confirm button", err)
				return
			}
			confirmButton := buttonIWidget.ToWidget()
			_ = confirmButton.Connect("button-press-event", func() {
				window.model.UserBundles = nil
			})
		}
	}

	dryRunResults := storage.GetPlannedMediaChanges(window.model.InstallSelected, window.model.TargetMedias,
		window.model.MediaOpts)

	writeToConfirmInstallDialog(buffer, dryRunResults)

	if err = setConfirmButtonState(dialog, window); err != nil {
		log.Error("Error setting Confirm button state", err)
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
	text := "<span font-size='xx-large'>" +
		utils.Locale.Get("Welcome to Clear Linux* OS Desktop Installation") + "</span>"
	if model.Version != model.DemoVersion {
		text += "\n\n<small>" + utils.Locale.Get("VERSION %s", model.Version) + "</small>"
	}

	return text
}

// GetThankYouMessage gets the thank you message
func GetThankYouMessage() string {
	text := "<span font-size='xx-large'>" +
		utils.Locale.Get("Thank you for choosing Clear Linux* OS") + "</span>"
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
	_ = dialog.Connect("response", func() {
		dialog.Destroy()
		gtk.MainQuit() // Exit Installer
	})
	dialog.ShowAll()
	dialog.Run()
}

// displaySyscheckDialog creates a pop-up for system check failures
func displaySyscheckDialog(syscheckErr error) {
	log.Error("System check failed: %s", syscheckErr.Error())

	// Create box
	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		log.Error("Error creating box", err)
		return
	}
	contentBox.SetHAlign(gtk.ALIGN_FILL)
	contentBox.SetMarginBottom(common.TopBottomMargin)

	// Create label
	text := utils.Locale.Get("System failed to pass pre-install checks.")
	label, err := gtk.LabelNew(text)
	if err != nil {
		log.Error("Error creating label", err)
		return
	}
	label.SetUseMarkup(true)
	label.SetHAlign(gtk.ALIGN_START)
	contentBox.PackStart(label, false, true, 0)
	// Fail message label
	text = syscheckErr.Error()
	label, err = gtk.LabelNew(text)
	if err != nil {
		log.Error("Error creating specific label", err)
		return
	}
	label.SetUseMarkup(true)
	label.SetHAlign(gtk.ALIGN_START)
	contentBox.PackStart(label, false, true, 0)

	// Create dialog
	title := utils.Locale.Get("Warning")
	dialog, err := common.CreateDialogOneButton(contentBox, title, utils.Locale.Get("OK"), "button-confirm")
	if err != nil {
		log.Error("Error creating dialog", err)
		return
	}
	dialog.SetDeletable(false)

	// Configure button
	buttonIWidget, err := dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
	if err != nil {
		log.Error("Error getting confirm button", err)
		return
	}
	confirmButton := buttonIWidget.ToWidget()
	_ = confirmButton.Connect("button-press-event", func() {
		dialog.Destroy()
		gtk.MainQuit() // Exit Installer
	})

	dialog.ShowAll()
	dialog.Run()
}
