// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/gui/pages"
)

// PageHeader provides a wrapper around a page, with a consistent
// header region and spacing. It embeds the root widget of each page
// within the view.
type PageHeader struct {
	handle *gtk.Box
	page   pages.Page

	ebox   *gtk.EventBox
	layout *gtk.Box
	image  *gtk.Image
	label  *gtk.Label
}

// PageHeaderNew constructs a new page header for the given page
func PageHeaderNew(page pages.Page) (*PageHeader, error) {
	var st *gtk.StyleContext

	// Root widget
	handle, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return nil, err
	}

	// Construct a PageHeader
	header := &PageHeader{
		handle: handle,
		page:   page,
	}

	// Construct eventbox for background styling
	header.ebox, err = gtk.EventBoxNew()
	if err != nil {
		return nil, err
	}
	header.ebox.SetMarginBottom(6)
	header.handle.PackStart(header.ebox, false, false, 0)
	st, err = header.ebox.GetStyleContext()
	if err != nil {
		return nil, err
	}

	// Style the header now
	st.AddClass("box-header")

	// Header mainLayout
	header.layout, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	header.layout.SetBorderWidth(6)
	header.ebox.Add(header.layout)

	// Image for the page
	header.image, err = gtk.ImageNewFromIconName(page.GetIcon()+"-symbolic", gtk.ICON_SIZE_INVALID)
	if err != nil {
		return nil, err
	}
	header.image.SetPixelSize(48)
	header.image.SetMarginStart(6)
	header.image.SetMarginEnd(12)
	header.image.SetMarginTop(4)
	header.image.SetMarginBottom(4)
	header.layout.PackStart(header.image, false, false, 0)

	// Label for the page
	header.label, err = gtk.LabelNew("<big>" + page.GetTitle() + "</big>")
	if err != nil {
		return nil, err
	}
	header.label.SetUseMarkup(true)
	header.layout.PackStart(header.label, false, false, 0)

	// Pack the root widget
	root := page.GetRootWidget()
	if root != nil {
		header.handle.PackStart(root, true, true, 0)
	}

	return header, nil
}

// GetRootWidget returns the root embeddable widget for the PageHeader
func (header *PageHeader) GetRootWidget() gtk.IWidget {
	return header.handle
}
