// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/VladimirMarkelov/clui"
	xs "github.com/huandu/xstrings"
	term "github.com/nsf/termbox-go"
	"sync/atomic"
	"time"
)

// SimpleButton is the implementation of a clui button with simpler
// visual elements (i.e no shadows).
type SimpleButton struct {
	clui.BaseControl
	pressed int32
	onClick func(clui.Event)
}

// CreateSimpleButton returns an instance of SimpleButton
// parent - is the parent control this button is attached to
// width - the minimum width size
// height - the minimum height size
// title - the button's label
// scale - the amount of space to use whenever used in an adjustable layout
func CreateSimpleButton(parent clui.Control, width, height int, title string, scale int) *SimpleButton {
	b := new(SimpleButton)
	b.BaseControl = clui.NewBaseControl()

	b.SetParent(parent)
	b.SetAlign(clui.AlignCenter)

	if height == clui.AutoSize {
		height = 1
	}

	if width == clui.AutoSize {
		width = xs.Len(title) + 2
	}

	b.SetTitle(title)
	b.SetSize(width, height)
	b.SetConstraints(width, height)
	b.SetScale(scale)

	if parent != nil {
		parent.AddChild(b)
	}

	return b
}

// Draw paints the button in the screen and adjust colors depending on the button state
func (b *SimpleButton) Draw() {
	if !b.Visible() {
		return
	}

	clui.PushAttributes()
	defer clui.PopAttributes()

	x, y := b.Pos()
	w, h := b.Size()

	fg, bg := b.TextColor(), b.BackColor()

	if !b.Enabled() {
		fg = clui.RealColor(fg, b.Style(), "ButtonDisabledText")
		bg = clui.RealColor(bg, b.Style(), "ButtonDisabledBack")
	} else if b.Active() {
		fgActive, bgActive := b.ActiveColors()

		fg = clui.RealColor(fgActive, b.Style(), "ButtonActiveText")
		bg = clui.RealColor(bgActive, b.Style(), "ButtonActiveBack")
	} else {
		fg = clui.RealColor(fg, b.Style(), "ButtonText")
		bg = clui.RealColor(bg, b.Style(), "ButtonBack")
	}

	clui.SetTextColor(fg)
	shift, text := clui.AlignColorizedText(b.Title(), w, b.Align())

	if b.isPressed() == 0 {
		clui.SetBackColor(bg)
		clui.FillRect(x, y, w, h, ' ')
		clui.DrawText(x+shift, y, text)
	} else {
		clui.SetBackColor(bg)
		clui.FillRect(x, y, w, h, ' ')
		clui.DrawText(x+shift, y, b.Title())
	}
}

func (b *SimpleButton) isPressed() int32 {
	return atomic.LoadInt32(&b.pressed)
}

func (b *SimpleButton) setPressed(pressed int32) {
	atomic.StoreInt32(&b.pressed, pressed)
}

func (b *SimpleButton) processKeyEvent(event clui.Event) bool {
	ekey := event.Key

	if (ekey == term.KeySpace || ekey == term.KeyEnter) && b.isPressed() == 0 {
		b.setPressed(1)
		ev := clui.Event{Type: clui.EventRedraw}

		go func() {
			clui.PutEvent(ev)
			time.Sleep(100 * time.Millisecond)
			b.setPressed(0)
			clui.PutEvent(ev)
		}()

		if b.onClick != nil {
			b.onClick(event)
		}
		return true
	} else if ekey == term.KeyEsc && b.isPressed() != 0 {
		b.setPressed(0)
		clui.ReleaseEvents()
		return true
	}

	return false
}

func (b *SimpleButton) processMouseEvent(event clui.Event) bool {
	if event.Key == term.MouseLeft {
		b.setPressed(1)
		clui.GrabEvents(b)
		return true
	} else if event.Key == term.MouseRelease && b.isPressed() != 0 {
		clui.ReleaseEvents()
		bX, bY := b.Pos()
		bw, bh := b.Size()

		if event.X >= bX && event.Y >= bY && event.X < bX+bw && event.Y < bY+bh {
			if b.onClick != nil {
				b.onClick(event)
			}
		}
		b.setPressed(0)
		return true
	}

	return false
}

// ProcessEvent will process the events triggered by clui mainloop
func (b *SimpleButton) ProcessEvent(event clui.Event) bool {
	if !b.Enabled() {
		return false
	}

	if event.Type == clui.EventKey {
		return b.processKeyEvent(event)
	} else if event.Type == clui.EventMouse {
		return b.processMouseEvent(event)
	}

	return false
}

// OnClick sets the button's onClick callback
func (b *SimpleButton) OnClick(fn func(clui.Event)) {
	b.onClick = fn
}
