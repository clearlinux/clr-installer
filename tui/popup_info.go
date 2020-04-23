// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
)

// InfoDialog is dialog window use to stop all other user
// interaction and present them with an informational message
type InfoDialog struct {
	DialogBox *clui.Window
	onClose   func()

	infoLabel  *clui.Label
	okayButton *SimpleButton
}

// OnClose sets the callback that is called when the
// dialog is closed
func (dialog *InfoDialog) OnClose(fn func()) {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	dialog.onClose = fn
}

// Close closes the dialog window and executes a callback if registered
func (dialog *InfoDialog) Close() {
	clui.WindowManager().DestroyWindow(dialog.DialogBox)
	clui.WindowManager().BeginUpdate()
	closeFn := dialog.onClose
	_ = term.Flush() // This might be dropped once clui is fixed
	clui.WindowManager().EndUpdate()
	if closeFn != nil {
		closeFn()
	}
}

func initInfoDiaglogWindow(dialog *InfoDialog) error {
	const title = "Informational"
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

	dialog.infoLabel = clui.CreateLabel(borderFrame, 1, AutoSize, "", 1)
	dialog.infoLabel.SetMultiline(true)

	buttonFrame := clui.CreateFrame(borderFrame, AutoSize, 1, clui.BorderNone, clui.Fixed)
	buttonFrame.SetPack(clui.Horizontal)
	buttonFrame.SetGaps(1, 0)
	dialog.okayButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, " OK ", Fixed)
	dialog.okayButton.SetEnabled(true)
	dialog.okayButton.SetActive(true)

	return nil
}

// CreateInfoDialogBox creates the Network PopUp
func CreateInfoDialogBox(message string) (*InfoDialog, error) {
	dialog := new(InfoDialog)

	if dialog == nil {
		return nil, fmt.Errorf("Failed to allocate a Info Dialog")
	}

	if err := initInfoDiaglogWindow(dialog); err != nil {
		return nil, fmt.Errorf("Failed to create Info Dialog: %v", err)
	}

	dialog.okayButton.OnClick(func(ev clui.Event) {
		dialog.Close()
	})

	dialog.infoLabel.SetTitle(message)
	clui.ActivateControl(dialog.DialogBox, dialog.okayButton)
	clui.RefreshScreen()

	return dialog, nil
}
