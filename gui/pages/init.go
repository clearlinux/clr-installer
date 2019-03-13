// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/clearlinux/clr-installer/args"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"log"
	"math"
)

// Button allows us to flag up different buttons
type Button uint

const (
	// ButtonCancel enables the cancel button
	ButtonCancel Button = 1 << iota

	// ButtonConfirm enables the confirm button
	ButtonConfirm Button = 1 << iota

	// ButtonQuit is only visible on the main view and install page
	// Normal pages should not modify this button!
	ButtonQuit Button = 1 << iota
)

// Page interface provides a common definition that other
// pages can share to give a standard interface for the
// main controller, the Window
type Page interface {
	IsRequired() bool
	IsDone() bool
	GetID() int
	GetSummary() string
	GetTitle() string
	GetIcon() string
	GetConfiguredValue() string
	GetRootWidget() gtk.IWidget
	StoreChanges() // Store changes in the model
	ResetChanges() // Reset data to model
}

// Controller is implemented by the Window struct, and
// is used by pages and ContentView to exert some control
// over workflow.
type Controller interface {
	ActivatePage(Page)
	SetButtonState(flags Button, enabled bool)
	GetRootDir() string
	GetOptions() args.Args
}

const (
	// PageIDTimezone is the timezone page key
	PageIDTimezone = iota

	// PageIDLanguage is the language page key
	PageIDLanguage = iota

	// PageIDKeyboard is the keyboard page key
	PageIDKeyboard = iota

	// PageIDBundle is the bundle page key
	PageIDBundle = iota

	// PageIDTelemetry is the telemetry page key
	PageIDTelemetry = iota

	// PageIDDiskConfig is the disk configuration page key
	PageIDDiskConfig = iota

	// PageIDInstall is the special installation page key
	PageIDInstall = iota

	// PageIDHostname is the hostname page key
	PageIDHostname = iota
)

// Private helper to assist in the ugliness of forcibly scrolling a GtkListBox
// to the selected row
//
// Note this must be performed on the idle loop in glib to ensure selection
// is correctly performed, and that we have valid constraints in which to
// scroll.
func scrollToView(scroll *gtk.ScrolledWindow, container gtk.IWidget, widget *gtk.Widget) {
	_, err := glib.TimeoutAdd(100, func() bool {
		adjustment := scroll.GetVAdjustment()
		_, y, err := widget.TranslateCoordinates(container, 0, 0)
		if err != nil {
			return false
		}
		maxSize := adjustment.GetUpper() - adjustment.GetPageSize()
		adjustment.SetValue(math.Min(float64(y), maxSize))
		return false
	})
	if err != nil {
		log.Fatal(err)
	}
}
