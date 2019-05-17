package common

import (
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

const (
	// ContentTypeInfo is the id for info content
	ContentTypeInfo = iota

	// ContentTypeError is the id for error content
	ContentTypeError = iota
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

// CreateDialogOneButton creates a gtk dialog with a single button
func CreateDialogOneButton(contentBox *gtk.Box, title, buttonLabel, buttonStyle string) (*gtk.Dialog, error) {
	var err error
	widget, err := CreateDialog(contentBox, title)
	if err != nil {
		return nil, err
	}
	widget.SetSkipTaskbarHint(false)
	widget.SetResizable(false)

	buttonExit, err := SetButton(buttonLabel, buttonStyle)
	if err != nil {
		return nil, err
	}
	buttonExit.SetMarginEnd(ButtonSpacing)
	widget.AddActionWidget(buttonExit, gtk.RESPONSE_CANCEL)

	return widget, nil
}

// CreateDialogOkCancel creates a gtk dialog with Ok and Cancel buttons
func CreateDialogOkCancel(contentBox *gtk.Box, title, ok, cancel string) (*gtk.Dialog, error) {
	//parentWindow := GetWinHandle()
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

// CreateDialogContent creates a gtk box that can be used as dialog content
func CreateDialogContent(message string, contentType int) (*gtk.Box, error) {
	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	contentBox.SetHAlign(gtk.ALIGN_FILL)
	contentBox.SetMarginBottom(TopBottomMargin)
	if err != nil {
		log.Warning("Error creating box")
		return nil, err
	}

	if contentType == ContentTypeError {
		st, err := contentBox.GetStyleContext()
		if err != nil {
			log.Warning("Error getting style context: ", err) // Just log trivial error
		} else {
			st.AddClass("dialog-error")
		}

		icon, err := gtk.ImageNewFromIconName("dialog-error-symbolic", gtk.ICON_SIZE_DIALOG)
		if err != nil {
			log.Warning("gtk.ImageNewFromIconName failed for icon dialog-error-symbolic") // Just log trivial error
		} else {
			icon.SetMarginEnd(12)
			icon.SetHAlign(gtk.ALIGN_START)
			icon.SetVAlign(gtk.ALIGN_START)
			contentBox.PackStart(icon, false, true, 0)
		}
	}

	label, err := gtk.LabelNew(message)
	if err != nil {
		log.Warning("Error creating label") // Just log trivial error
		return nil, err
	}

	label.SetUseMarkup(true)
	label.SetHAlign(gtk.ALIGN_END)
	contentBox.PackStart(label, false, true, 0)

	return contentBox, nil
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
