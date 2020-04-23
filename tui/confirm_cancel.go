// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
)

// ConfirmCancelDialog is dialog window use to stop all other
// user interaction and have them confirm the possible loss of
// changes due to pressing the Cancel button.
type ConfirmCancelDialog struct {
	DialogBox *clui.Window
	Confirmed bool
	onClose   func()

	message       string
	warningLabel  *clui.Label
	cancelButton  *SimpleButton
	confirmButton *SimpleButton
}

// OnClose sets the callback that is called when the
// dialog is closed
func (dialog *ConfirmCancelDialog) OnClose(fn func()) {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	dialog.onClose = fn
}

// Close closes the dialog window and executes a callback if registered
func (dialog *ConfirmCancelDialog) Close() {
	clui.WindowManager().DestroyWindow(dialog.DialogBox)
	clui.WindowManager().BeginUpdate()
	closeFn := dialog.onClose
	_ = term.Flush() // This might be dropped once clui is fixed
	clui.WindowManager().EndUpdate()
	if closeFn != nil {
		closeFn()
	}
}

func initCancelDiaglogWindow(dialog *ConfirmCancelDialog, title string) error {
	const wBuff = 5
	const hBuff = 5
	const dWidth = 50
	const dHeight = 8

	sw, sh := clui.ScreenSize()

	x := (sw - WindowWidth) / 2
	y := (sh - WindowHeight) / 2

	posX := (WindowWidth - dWidth + wBuff) / 2
	if posX < wBuff {
		posX = wBuff
	}
	posX = x + posX
	posY := (WindowHeight-dHeight+hBuff)/2 - hBuff
	if posY < hBuff {
		posY = hBuff
	}
	posY = y + posY

	dialog.DialogBox = clui.AddWindow(posX, posY, dWidth, dHeight, title)
	dialog.DialogBox.SetTitleButtons(0)
	dialog.DialogBox.SetMovable(false)
	dialog.DialogBox.SetSizable(false)
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	dialog.DialogBox.SetModal(true)
	dialog.DialogBox.SetConstraints(dWidth, dHeight)
	dialog.DialogBox.SetPack(clui.Vertical)
	dialog.DialogBox.SetBorder(clui.BorderAuto)

	borderFrame := clui.CreateFrame(dialog.DialogBox, dWidth, dHeight, clui.BorderNone, clui.Fixed)
	borderFrame.SetPack(clui.Vertical)
	borderFrame.SetGaps(0, 1)
	borderFrame.SetPaddings(1, 1)

	dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 2, dialog.message, 1)
	dialog.warningLabel.SetMultiline(true)

	buttonFrame := clui.CreateFrame(borderFrame, AutoSize, 1, clui.BorderNone, clui.Fixed)
	buttonFrame.SetPack(clui.Horizontal)
	buttonFrame.SetGaps(1, 0)
	dialog.cancelButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, "Cancel", Fixed)
	dialog.cancelButton.SetEnabled(true)
	dialog.cancelButton.SetActive(true)

	dialog.confirmButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, "Confirm", Fixed)
	dialog.confirmButton.SetEnabled(true)
	dialog.confirmButton.SetActive(false)

	return nil
}

// CreateConfirmCancelDialogBox creates the Network PopUp
func CreateConfirmCancelDialogBox(message string, title string) (*ConfirmCancelDialog, error) {
	dialog := new(ConfirmCancelDialog)

	if dialog == nil {
		return nil, fmt.Errorf("Failed to allocate a Confirmation of Cancel Dialog")
	}

	dialog.message = message
	if message == "" {
		dialog.message = "Data has changed and will be lost!\n\nDiscard data changes?"
	}

	if err := initCancelDiaglogWindow(dialog, title); err != nil {
		return nil, fmt.Errorf("Failed to create Confirmation of Cancel Dialog: %v", err)
	}

	dialog.cancelButton.OnClick(func(ev clui.Event) {
		dialog.Confirmed = false
		dialog.Close()
	})

	dialog.confirmButton.OnClick(func(ev clui.Event) {
		dialog.Confirmed = true
		dialog.Close()
	})

	clui.ActivateControl(dialog.DialogBox, dialog.cancelButton)
	clui.RefreshScreen()

	return dialog, nil
}
