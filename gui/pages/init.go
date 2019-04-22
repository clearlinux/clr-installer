// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"math"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/args"
	"github.com/clearlinux/clr-installer/log"
)

// Button allows us to flag up different buttons
type Button uint

const (
	// ButtonCancel enables the cancel button
	ButtonCancel Button = 1 << iota

	// ButtonConfirm enables the confirm button
	ButtonConfirm Button = 1 << iota

	// ButtonQuit enables the quit button
	ButtonQuit Button = 1 << iota

	// ButtonBack enables the back button
	ButtonBack Button = 1 << iota

	// ButtonNext enables the next button
	ButtonNext Button = 1 << iota

	// ButtonExit enables the exit button
	ButtonExit Button = 1 << iota
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
	// PageIDWelcome is the language page key
	PageIDWelcome = iota

	// PageIDTimezone is the timezone page key
	PageIDTimezone = iota

	// PageIDKeyboard is the keyboard page key
	PageIDKeyboard = iota

	// PageIDBundle is the bundle page key
	PageIDBundle = iota

	// PageIDTelemetry is the telemetry page key
	PageIDTelemetry = iota

	// PageIDUserAdd is the user add page key
	PageIDUserAdd = iota

	// PageIDDiskConfig is the disk configuration page key
	PageIDDiskConfig = iota

	// PageIDHostname is the hostname page key
	PageIDHostname = iota

	// PageIDInstall is the special installation page key
	PageIDInstall = iota
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
		log.Warning("Error adjusting scroll: ", err) // Just log trivial error
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
func getTextFromEntry(entry *gtk.Entry) string {
	text := ""
	buffer, err := getBufferFromEntry(entry)
	if err != nil {
		log.Warning("Error getting buffer: ", err) // Just log trivial error
	} else {
		text, err = buffer.GetText()
		if err != nil {
			log.Warning("Error reading buffer: ", err) // Just log trivial error
		}
	}
	return text
}

// setTextInEntry writes the text to an Entry buffer
func setTextInEntry(entry *gtk.Entry, text string) {
	buffer, err := getBufferFromEntry(entry)
	if err != nil {
		log.Warning("Error getting buffer: ", err) // Just log trivial error
	} else {
		buffer.SetText(text)
	}
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
func getTextFromSearchEntry(entry *gtk.SearchEntry) string {
	text := ""
	buffer, err := getBufferFromSearchEntry(entry)
	if err != nil {
		log.Warning("Error getting buffer: ", err) // Just log trivial error
		text, err = buffer.GetText()
		if err != nil {
			log.Warning("Error reading buffer: ", err) // Just log trivial error
		}
	}
	return text
}

// setBox creates and styles a new gtk Box
func setBox(orient gtk.Orientation, spacing int, style string) (*gtk.Box, error) {
	widget, err := gtk.BoxNew(orient, spacing)
	if err != nil {
		return nil, err
	}

	sc, err := widget.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass(style)
	}

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
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass(style)
	}

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
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass(style)
	}

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
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass(style)
	}

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
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass(style)
	}

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
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass(style)
	}
	widget.SetXAlign(x)

	return widget, nil
}

// setButton creates and styles a new gtk Button
func setButton(text, style string) (*gtk.Button, error) {
	widget, err := gtk.ButtonNewWithLabel(text)
	if err != nil {
		return nil, err
	}

	sc, err := widget.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass(style)
	}

	return widget, nil
}
