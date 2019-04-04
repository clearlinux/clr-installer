// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/utils"
)

const (
	clearLinuxImage = "clr.png"
)

// Banner is used to add a nice banner widget to the front of the installer
type Banner struct {
	revealer  *gtk.Revealer // For animations
	ebox      *gtk.EventBox // To allow styling
	box       *gtk.Box      // Main mainLayout
	img       *gtk.Image    // Our image widget
	labelText *gtk.Label    // Display label
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
	st.AddClass("ebox-banner")
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
	banner.box.SetMarginStart(20)
	banner.box.SetMarginEnd(14)

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

	banner.labelText, err = gtk.LabelNew(GetWelcomeMessage())
	if err != nil {
		return nil, err
	}
	banner.labelText.SetUseMarkup(true)
	banner.labelText.SetHAlign(gtk.ALIGN_START)
	banner.labelText.SetVAlign(gtk.ALIGN_CENTER)
	banner.labelText.SetLineWrap(true)
	banner.labelText.SetMaxWidthChars(22)
	banner.labelText.SetHExpand(false)
	banner.box.PackStart(banner.labelText, false, false, 0)

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
