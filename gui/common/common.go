package common

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/log"
)

// CreateDialog creates a gtk dialog with no buttons
func CreateDialog(contentBox *gtk.Box, title string) (*gtk.Dialog, error) {
	var err error
	widget, err := gtk.DialogNew()
	if err != nil {
		return nil, err
	}
	widget.SetModal(true)

	widget.SetDefaultSize(200, 100)
	widget.SetTitle(title)
	sc, err := widget.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass("dialog")
	}

	if contentBox != nil {
		contentBox.SetMarginStart(10)
		contentBox.SetMarginEnd(10)
		contentBox.SetMarginTop(10)
		contentBox.SetMarginBottom(10)
		contentArea, err := widget.GetContentArea()
		if err != nil {
			log.Warning("Error getting content area: ", err)
		}
		contentArea.Add(contentBox)
	}

	widget.ShowAll()

	return widget, nil
}

// CreateDialogOkCancel creates a gtk dialog with OK and Cancel buttons
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
	buttonCancel.SetMarginStart(20)
	buttonCancel.SetMarginEnd(20)
	widget.AddActionWidget(buttonCancel, gtk.RESPONSE_CANCEL)

	buttonOK, err := SetButton(ok, "button-confirm")
	if err != nil {
		return nil, err
	}
	buttonOK.SetMarginStart(20)
	buttonOK.SetMarginEnd(20)
	widget.AddActionWidget(buttonOK, gtk.RESPONSE_OK)

	widget.ShowAll()

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
