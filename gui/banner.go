// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	clearLinuxImage = "clr.png"
)

// Banner is used to add a nice banner widget to the front of the installer
type Banner struct {
	revealer *gtk.Revealer // For animations
	ebox     *gtk.EventBox // To allow styling
	box      *gtk.Box      // Main layout
	img      *gtk.Image    // Our image widget
	label    *gtk.Label    // Display label
}

// NewBanner constructs the header component
func NewBanner() (*Banner, error) {
	var err error
	var pbuf *gdk.Pixbuf
	banner := &Banner{}
	var st *gtk.StyleContext

	// Create the "holder" (revealer)
	if banner.revealer, err = gtk.RevealerNew(); err != nil {
		return nil, err
	}

	// Create eventbox for styling
	if banner.ebox, err = gtk.EventBoxNew(); err != nil {
		return nil, err
	}
	if st, err = banner.ebox.GetStyleContext(); err != nil {
		return nil, err
	}
	st.AddClass("installer-welcome-banner")
	banner.revealer.Add(banner.ebox)

	// Create the root box
	if banner.box, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0); err != nil {
		return nil, err
	}
	banner.ebox.Add(banner.box)
	banner.revealer.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_CROSSFADE)

	// Set the margins up
	banner.box.SetMarginTop(40)
	banner.box.SetMarginBottom(40)
	banner.box.SetMarginEnd(24)
	banner.box.SetMarginStart(40)

	themeDir, err := utils.LookupThemeDir()
	if err != nil {
		return nil, err
	}

	// Construct the image
	if banner.img, err = gtk.ImageNew(); err != nil {
		return nil, err
	}

	if pbuf, err = gdk.PixbufNewFromFileAtSize(
		filepath.Join(themeDir, clearLinuxImage),
		128, 128); err != nil {
		return nil, err
	}
	banner.img.SetFromPixbuf(pbuf)
	banner.img.SetPixelSize(64)
	banner.img.SetMarginTop(12)
	banner.img.SetMarginBottom(24)
	banner.img.SetHAlign(gtk.ALIGN_CENTER)
	banner.img.SetVAlign(gtk.ALIGN_CENTER)
	banner.box.PackStart(banner.img, false, false, 0)
	banner.box.SetHAlign(gtk.ALIGN_CENTER)

	// Sort the label out
	labelText := "<span font-size='xx-large'>Welcome to\nClear Linux\nDesktop\nInstallation</span>"
	labelText += "\n\n<small>VERSION " + model.Version + "</small>"
	if banner.label, err = gtk.LabelNew(labelText); err != nil {
		return nil, err
	}
	banner.label.SetUseMarkup(true)
	banner.label.SetHAlign(gtk.ALIGN_START)
	banner.label.SetVAlign(gtk.ALIGN_CENTER)
	banner.box.PackStart(banner.label, false, false, 0)

	return banner, nil
}

// GetRootWidget returns the embeddable root widget
func (banner *Banner) GetRootWidget() gtk.IWidget {
	return banner.revealer
}

// ShowFirst will display the banner for the first time during an intro sequence
func (banner *Banner) ShowFirst() {
	banner.revealer.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_CROSSFADE)
	banner.revealer.SetTransitionDuration(3000)
	banner.revealer.SetRevealChild(true)
}

// Show will animate the banner into view, showing the content
func (banner *Banner) Show() {
	banner.revealer.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_SLIDE_RIGHT)
	banner.revealer.SetTransitionDuration(250)
	banner.revealer.SetRevealChild(true)
}

// Hide will animate the banner out of view, hiding the content
func (banner *Banner) Hide() {
	banner.revealer.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_SLIDE_LEFT)
	banner.revealer.SetTransitionDuration(250)
	banner.revealer.SetRevealChild(false)
}

// InstallMode updates the text for install mode
func (banner *Banner) InstallMode() {
	labelText := "<span font-size='xx-large'>Thank you\nfor choosing\nClear Linux</span>"
	labelText += "\n\n<small>VERSION " + model.Version + "</small>"
	banner.label.SetMarkup(labelText)
}
