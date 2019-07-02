package common

import (
	"time"

	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/log"
)

const (
	// StartEndMargin is the start and end margin
	StartEndMargin int = 18

	// TopBottomMargin is the top and bottom margin
	TopBottomMargin int = 10

	// ButtonSpacing is generic spacing between buttons
	ButtonSpacing int = 4
)

var (
	// LoopWaitDuration is a common loop wait duration used in pages
	LoopWaitDuration = 200 * time.Millisecond

	// LoopTimeOutDuration is a common loop timeout duration used in pages
	LoopTimeOutDuration = 10000 * time.Millisecond // 10 seconds
)

// CreateDialog creates a gtk dialog with no buttons
func CreateDialog(contentBox *gtk.Box, title string) (*gtk.Dialog, error) {
	var err error
	widget, err := gtk.DialogNew()
	if err != nil {
		return nil, err
	}
	widget.SetModal(true)

	widget.SetDefaultSize(350, 100)
	widget.SetTitle(title)
	sc, err := widget.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass("dialog")
	}

	if contentBox != nil {
		contentBox.SetMarginStart(StartEndMargin)
		contentBox.SetMarginEnd(StartEndMargin)
		contentBox.SetMarginTop(TopBottomMargin)
		contentBox.SetMarginBottom(TopBottomMargin)
		contentArea, err := widget.GetContentArea()
		if err != nil {
			log.Warning("Error getting content area: ", err)
			return nil, err
		}
		contentArea.Add(contentBox)
	}

	return widget, nil
}

// CreateDialogOneButton creates a gtk dialog with one button
func CreateDialogOneButton(contentBox *gtk.Box, title, buttonLabel, buttonStyle string) (*gtk.Dialog, error) {
	var err error
	widget, err := CreateDialog(contentBox, title)
	if err != nil {
		return nil, err
	}

	button, err := SetButton(buttonLabel, buttonStyle)
	if err != nil {
		return nil, err
	}
	button.SetMarginEnd(StartEndMargin)
	widget.AddActionWidget(button, gtk.RESPONSE_OK)

	return widget, nil
}

// CreateDialogOkCancel creates a gtk dialog with Ok and Cancel buttons
func CreateDialogOkCancel(contentBox *gtk.Box, title, ok, cancel string) (*gtk.Dialog, error) {
	var err error
	widget, err := CreateDialog(contentBox, title)
	if err != nil {
		return nil, err
	}

	buttonCancel, err := SetButton(cancel, "button-cancel")
	if err != nil {
		return nil, err
	}
	buttonCancel.SetMarginEnd(ButtonSpacing)
	widget.AddActionWidget(buttonCancel, gtk.RESPONSE_CANCEL)

	buttonOK, err := SetButton(ok, "button-confirm")
	if err != nil {
		return nil, err
	}
	buttonOK.SetMarginEnd(StartEndMargin)
	widget.AddActionWidget(buttonOK, gtk.RESPONSE_OK)

	return widget, nil
}

// SetButton creates and styles a new gtk Button
func SetButton(text, style string) (*gtk.Button, error) {
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

// SetLabel creates and styles a new gtk Label
func SetLabel(text, style string, x float64) (*gtk.Label, error) {
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
