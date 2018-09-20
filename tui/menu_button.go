// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/VladimirMarkelov/clui"
	xs "github.com/huandu/xstrings"
	term "github.com/nsf/termbox-go"
	"sync/atomic"
	"time"
)

// MenuButton is the implementation of a clui button with simpler
// visual elements (i.e no shadows).
type MenuButton struct {
	clui.BaseControl
	itemValue string
	pressed   int32
	onClick   func(clui.Event)
	status    int
	sWdith    int // splitter width
}

const (
	textPadding = 3

	// MenuButtonStatusDefault means: not the auto detect,
	// not the user defined, not even failed
	MenuButtonStatusDefault = iota

	// MenuButtonStatusAutoDetect means: the current value was autodetected
	MenuButtonStatusAutoDetect

	// MenuButtonStatusUserDefined means: the user actively changed the value
	MenuButtonStatusUserDefined

	//MenuButtonStatusFailure means: the label displayed is an error message
	MenuButtonStatusFailure
)

var (
	// by now we're using the same char for everything
	statusSymbol = map[int]rune{
		MenuButtonStatusDefault:     '»',
		MenuButtonStatusUserDefined: '»',
		MenuButtonStatusFailure:     '»',
		MenuButtonStatusAutoDetect:  '»',
	}
)

// SetStatus sets the status attribute for a menu button
func (mb *MenuButton) SetStatus(status int) {
	mb.status = status
}

// Status returns the currently set status for a menu button
func (mb *MenuButton) Status() int {
	return mb.status
}

// SetMenuItemValue sets the value for the itemValue (configured value for a menu item)
func (mb *MenuButton) SetMenuItemValue(sv string) {
	mb.itemValue = sv
}

// MenuItemValue returns the value set for itemValue (configured value for a menu item)
func (mb *MenuButton) MenuItemValue() string {
	return mb.itemValue
}

// CreateMenuButton returns an instance of MenuButton
// parent - is the parent control this button is attached to
// status - one of:	MenuButtonStatusDefault, MenuButtonStatusUserDefined, MenuButtonStatusFailure
// title - the button's label
func CreateMenuButton(parent clui.Control, status int, title string, sWidth int) *MenuButton {
	mb := new(MenuButton)
	mb.sWdith = sWidth
	mb.BaseControl = clui.NewBaseControl()

	mb.SetParent(parent)
	mb.SetAlign(clui.AlignLeft)
	mb.status = status

	height := 4
	width := xs.Len(title) + 2

	mb.SetTitle(title)

	mb.SetSize(width, height)

	mb.SetConstraints(width, height)
	mb.SetScale(Fixed)

	if parent != nil {
		parent.AddChild(mb)
	}

	return mb
}

// Draw paints the button in the screen and adjust colors depending on the button state
func (mb *MenuButton) Draw() {

	clui.PushAttributes()
	defer clui.PopAttributes()

	x, y := mb.Pos()
	w, h := mb.Size()

	fg, bg := mb.TextColor(), mb.BackColor()
	sfg, sbg := mb.TextColor(), mb.BackColor()

	if mb.Active() {
		fgActive, bgActive := mb.ActiveColors()

		fg = clui.RealColor(fgActive, mb.Style(), "MenuActiveText")
		bg = clui.RealColor(bgActive, mb.Style(), "MenuActiveBack")

		sfg = clui.RealColor(sfg, mb.Style(), "MenuContentActiveText")
		sbg = clui.RealColor(sbg, mb.Style(), "MenuContentActiveBack")
	} else {
		fg = clui.RealColor(fg, mb.Style(), "MenuText")
		bg = clui.RealColor(bg, mb.Style(), "MenuBack")

		sfg = clui.RealColor(sfg, mb.Style(), "MenuContentText")
		sbg = clui.RealColor(sbg, mb.Style(), "MenuContentBack")
	}

	clui.SetTextColor(fg)

	shift, text := clui.AlignColorizedText(mb.Title(), w, mb.Align())

	clui.SetBackColor(bg)
	clui.FillRect(x, y, w, h, ' ')
	clui.DrawText(x+shift+textPadding, y+1, text)

	clui.PopAttributes()
	clui.PushAttributes()

	itemValue := fmt.Sprintf("%c %s", statusSymbol[mb.status], mb.itemValue)

	if len(itemValue) >= w {
		ellipsesSize := 3
		scrollbarPadding := 1
		ppx, _ := mb.Parent().Paddings()

		maxWidth := w - (ellipsesSize + ppx + scrollbarPadding)
		itemValue = fmt.Sprintf("%s...", strings.TrimSpace(clui.CutText(itemValue, maxWidth)))
	}

	shift, itemValue = clui.AlignColorizedText(itemValue, w, mb.Align())

	clui.SetBackColor(sbg)
	clui.SetTextColor(sfg)
	clui.DrawText(x+shift+textPadding, y+2, itemValue)

	if !mb.Active() {
		for i := 0; i < mb.sWdith; i++ {
			clui.DrawText(x+shift+textPadding+i, y+3, "_")
		}
	}
}

func (mb *MenuButton) isPressed() int32 {
	return atomic.LoadInt32(&mb.pressed)
}

func (mb *MenuButton) setPressed(pressed int32) {
	atomic.StoreInt32(&mb.pressed, pressed)
}

func (mb *MenuButton) processKeyEvent(event clui.Event) bool {
	ekey := event.Key

	if (ekey == term.KeySpace || ekey == term.KeyEnter) && mb.isPressed() == 0 {
		mb.setPressed(1)
		ev := clui.Event{Type: clui.EventRedraw}

		go func() {
			clui.PutEvent(ev)
			time.Sleep(100 * time.Millisecond)
			mb.setPressed(0)
			clui.PutEvent(ev)
		}()

		if mb.onClick != nil {
			mb.onClick(event)
		}
		return true
	} else if ekey == term.KeyEsc && mb.isPressed() != 0 {
		mb.setPressed(0)
		clui.ReleaseEvents()
		return true
	}

	return false
}

func (mb *MenuButton) processMouseEvent(event clui.Event) bool {
	if event.Key == term.MouseLeft {
		mb.setPressed(1)
		clui.GrabEvents(mb)
		return true
	} else if event.Key == term.MouseRelease && mb.isPressed() != 0 {
		clui.ReleaseEvents()
		bX, bY := mb.Pos()
		bw, bh := mb.Size()

		if event.X >= bX && event.Y >= bY && event.X < bX+bw && event.Y < bY+bh {
			if mb.onClick != nil {
				mb.onClick(event)
			}
		}
		mb.setPressed(0)
		return true
	}

	return false
}

// ProcessEvent will process the events triggered by clui mainloop
func (mb *MenuButton) ProcessEvent(event clui.Event) bool {
	if !mb.Enabled() {
		return false
	}

	if event.Type == clui.EventKey {
		return mb.processKeyEvent(event)
	} else if event.Type == clui.EventMouse {
		return mb.processMouseEvent(event)
	}

	return false
}

// OnClick sets the button's onClick callback
func (mb *MenuButton) OnClick(fn func(clui.Event)) {
	mb.onClick = fn
}
