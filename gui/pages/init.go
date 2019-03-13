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

// getBufferFromEntry gets the buffer from a gtk Entry
func getBufferFromEntry(entry *gtk.Entry) (*gtk.EntryBuffer, error) {
	buffer, err := entry.GetBuffer()
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// getTextFromEntry reads the text from an Entry buffer
func getTextFromEntry(entry *gtk.Entry) (string, error) {
	buffer, err := getBufferFromEntry(entry)
	if err != nil {
		return "", err
	}
	text, err := buffer.GetText()
	if err != nil {
		return "", err
	}
	return text, nil
}

// setTextInEntry writes the text to an Entry buffer
func setTextInEntry(entry *gtk.Entry, text string) error {
	buffer, err := getBufferFromEntry(entry)
	if err != nil {
		return err
	}
	buffer.SetText(text)
	return nil
}

// getBufferFromSearchEntry gets the buffer from a gtk Entry
func getBufferFromSearchEntry(entry *gtk.SearchEntry) (*gtk.EntryBuffer, error) {
	buffer, err := entry.GetBuffer()
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// getTextFromSearchEntry reads the text from an SearchEntry buffer
func getTextFromSearchEntry(entry *gtk.SearchEntry) (string, error) {
	buffer, err := getBufferFromSearchEntry(entry)
	if err != nil {
		return "", err
	}
	text, err := buffer.GetText()
	if err != nil {
		return "", err
	}
	return text, nil
}

// setBox creates and styles a new gtk Box
func setBox(orient gtk.Orientation, spacing int, style string) (*gtk.Box, error) {
	widget, err := gtk.BoxNew(orient, spacing)
	if err != nil {
		return nil, err
	}
	sc, err := widget.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass(style)
	return widget, nil
}

// setSearchEntry creates and styles a new gtk SearchEntry
func setSearchEntry(style string) (*gtk.SearchEntry, error) {
	widget, err := gtk.SearchEntryNew()
	if err != nil {
		return nil, err
	}
	sc, err := widget.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass(style)
	return widget, nil
}

// setEntry creates and styles a new gtk Entry
func setEntry(style string) (*gtk.Entry, error) {
	widget, err := gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	sc, err := widget.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass(style)
	return widget, nil
}

// setListBox sets up a new gtk ListBox
func setListBox(mode gtk.SelectionMode, single bool, style string) (*gtk.ListBox, error) {
	widget, err := gtk.ListBoxNew()
	if err != nil {
		return nil, err
	}
	widget.SetSelectionMode(mode)
	widget.SetActivateOnSingleClick(true)
	sc, err := widget.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass(style)
	return widget, nil
}

// setScrolledWindow creates and styles a new gtk ScrolledWindow
func setScrolledWindow(never, auto gtk.PolicyType, style string) (*gtk.ScrolledWindow, error) {
	widget, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	widget.SetPolicy(never, auto)
	sc, err := widget.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass(style)
	return widget, nil
}

// setLabel creates and styles a new gtk Label
func setLabel(text, style string, x float64) (*gtk.Label, error) {
	widget, err := gtk.LabelNew(text)
	if err != nil {
		return nil, err
	}
	sc, err := widget.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass(style)
	widget.SetHAlign(gtk.ALIGN_START)
	widget.SetXAlign(x)
	widget.ShowAll()
	return widget, nil
}
