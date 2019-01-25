// Copyright © 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"github.com/clearlinux/clr-installer/gui/pages"
	"github.com/gotk3/gotk3/gtk"
)

// ContentView is used to encapsulate the Required/Advanced options view
// by wrapping them into simple styled lists
type ContentView struct {
	views      map[int]pages.Page     // Mapping of row to page
	widgets    map[int]*SummaryWidget // Mapping of page to header
	controller pages.Controller

	scroll *gtk.ScrolledWindow
	list   *gtk.ListBox
}

// NewContentView will attempt creation of a new ContentView
func NewContentView(controller pages.Controller) (*ContentView, error) {
	var err error

	// Init the struct
	view := &ContentView{
		controller: controller,
		views:      make(map[int]pages.Page),
		widgets:    make(map[int]*SummaryWidget),
	}

	// Set up the scroller
	if view.scroll, err = gtk.ScrolledWindowNew(nil, nil); err != nil {
		return nil, err
	}
	view.scroll.SetMarginTop(20)

	// Set the scroll policy
	view.scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	// Set shadow type
	view.scroll.SetShadowType(gtk.SHADOW_NONE)

	// Set up the list
	if view.list, err = gtk.ListBoxNew(); err != nil {
		return nil, err
	}

	// Remove background
	st, _ := view.list.GetStyleContext()
	st.AddClass("scroller-special")

	// Ensure navigation works properly
	view.list.SetSelectionMode(gtk.SELECTION_SINGLE)
	view.list.SetActivateOnSingleClick(true)
	view.list.Connect("row-activated", view.onRowActivated)
	view.scroll.Add(view.list)

	return view, nil
}

// GetRootWidget will return the root widget for embedding
func (view *ContentView) GetRootWidget() gtk.IWidget {
	return view.scroll
}

// AddPage will add the relevant page to this content view.
// Right now it does nothing.
func (view *ContentView) AddPage(page pages.Page) error {
	widget, err := NewSummaryWidget(page)
	if err != nil {
		return err
	}

	view.list.Add(widget.GetRootWidget())
	view.views[widget.GetRowIndex()] = page
	view.widgets[page.GetID()] = widget

	// Update for first time
	widget.Update()
	return nil
}

func (view *ContentView) onRowActivated(box *gtk.ListBox, row *gtk.ListBoxRow) {
	if row == nil {
		return
	}
	// Go activate this.
	view.controller.ActivatePage(view.views[row.GetIndex()])
}

// UpdateView will update the summary for the given page
func (view *ContentView) UpdateView(page pages.Page) {
	view.widgets[page.GetID()].Update()
}

// IsDone returns true if all components have been completed
func (view *ContentView) IsDone() bool {
	for _, page := range view.views {
		if !page.IsDone() {
			return false
		}
	}
	return true
}
