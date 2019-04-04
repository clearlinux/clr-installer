// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"github.com/gotk3/gotk3/gtk"
)

// InstallWidget provides a description with tickes/crosses to let the user
// know which parts of the install have been completed
type InstallWidget struct {
	layout *gtk.Box
	label  *gtk.Label
	image  *gtk.Image
}

// NewInstallWidget will return a new install widget for display
func NewInstallWidget(desc string) (*InstallWidget, error) {
	var err error

	widget := &InstallWidget{}

	// Create layout
	widget.layout, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}

	// Create label
	widget.label, err = gtk.LabelNew(desc)
	if err != nil {
		return nil, err
	}

	// Create image
	widget.image, err = gtk.ImageNewFromIconName("appointment-soon-symbolic", gtk.ICON_SIZE_BUTTON)
	if err != nil {
		return nil, err
	}

	// Pack it all
	widget.layout.PackStart(widget.label, true, true, 0)
	widget.label.SetHAlign(gtk.ALIGN_START)
	widget.image.SetVAlign(gtk.ALIGN_CENTER)
	widget.layout.PackEnd(widget.image, false, false, 0)
	if err := widget.layout.SetProperty("margin", 4); err != nil {
		return nil, err
	}
	widget.layout.ShowAll()

	return widget, nil
}

// MarkStatus will mark the section
func (widget *InstallWidget) MarkStatus(success bool) {
	if success {
		widget.image.SetFromIconName("object-select-symbolic", gtk.ICON_SIZE_BUTTON)
		return
	}

	widget.image.SetFromIconName("window-close-symbolic", gtk.ICON_SIZE_BUTTON)
	// Make it red.
	st, err := widget.image.GetStyleContext()
	if err == nil {
		st.AddClass("destructive-action")
	}
}

// Completed will mark the widget as completed (no longer active)
func (widget *InstallWidget) Completed() {
	if st, err := widget.layout.GetStyleContext(); err == nil {
		st.AddClass("dim-label")
	}
}

// GetRootWidget will return the root embeddable widget
func (widget *InstallWidget) GetRootWidget() gtk.IWidget {
	return widget.layout
}
