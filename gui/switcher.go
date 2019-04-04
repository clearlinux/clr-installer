// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package gui

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/utils"
)

// Switcher is used to switch between main installer sections
type Switcher struct {
	revealer *gtk.Revealer // root widget
	box      *gtk.Box      // Main mainLayout
	stack    *gtk.Stack    // Stack to control

	buttons struct {
		required *gtk.RadioButton
		advanced *gtk.RadioButton
	}
}

// NewSwitcher constructs the header component
func NewSwitcher(stack *gtk.Stack) (*Switcher, error) {
	var err error
	var st *gtk.StyleContext

	// Create switcher
	switcher := &Switcher{
		stack: stack,
	}

	// root revealer
	switcher.revealer, err = gtk.RevealerNew()
	if err != nil {
		return nil, err
	}
	switcher.revealer.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_SLIDE_DOWN)

	// Create main mainLayout
	switcher.box, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	switcher.revealer.Add(switcher.box)

	// Set styling
	st, err = switcher.box.GetStyleContext()
	if err != nil {
		return nil, err
	}
	st.AddClass("box-switcher")

	// Required options
	switcher.buttons.required, err = createFancyButton(utils.Locale.Get("REQUIRED OPTIONS"), "label-switcher")
	if err != nil {
		return nil, err
	}
	switcher.buttons.required.SetActive(true)
	if _, err := switcher.buttons.required.Connect("toggled", func() { switcher.switchTo(switcher.buttons.required, "required") }); err != nil {
		return nil, err
	}
	switcher.box.PackStart(switcher.buttons.required, true, true, 0)

	// Advanced options
	switcher.buttons.advanced, err = createFancyButton(utils.Locale.Get("ADVANCED OPTIONS"), "label-switcher")
	if err != nil {
		return nil, err
	}
	switcher.buttons.advanced.JoinGroup(switcher.buttons.required)
	if _, err := switcher.buttons.advanced.Connect("toggled", func() { switcher.switchTo(switcher.buttons.advanced, "advanced") }); err != nil {
		return nil, err
	}
	switcher.box.PackStart(switcher.buttons.advanced, true, true, 0)

	return switcher, nil
}

// handle switching to another view
func (switcher *Switcher) switchTo(button *gtk.RadioButton, id string) {
	if switcher.stack == nil {
		return
	}
	if !button.GetActive() {
		return
	}
	switcher.stack.SetVisibleChildName(id)
}

func createFancyButton(text, style string) (*gtk.RadioButton, error) {
	button, err := gtk.RadioButtonNew(nil)
	if err != nil {
		return nil, err
	}
	button.SetMode(false)
	label, err := gtk.LabelNew(text)
	if err != nil {
		return nil, err
	}
	sc, err := label.GetStyleContext()
	if err != nil {
		return nil, err
	}
	sc.AddClass(style)
	button.Add(label)
	return button, nil
}

// GetRootWidget returns the embeddable root widget
func (switcher *Switcher) GetRootWidget() gtk.IWidget {
	return switcher.revealer
}

// SetStack updates the associated stack
func (switcher *Switcher) SetStack(stack *gtk.Stack) {
	switcher.stack = stack
}

// Show will tween in the switcher
func (switcher *Switcher) Show() {
	switcher.revealer.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_SLIDE_DOWN)
	switcher.revealer.SetRevealChild(true)
}

// Hide will tween the switcher out
func (switcher *Switcher) Hide() {
	switcher.revealer.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_SLIDE_UP)
	switcher.revealer.SetRevealChild(false)
}
