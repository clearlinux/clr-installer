// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/model"
)

// ConfirmInstallDialog is dialog window use to stop all other
// user interaction and have the them confirm the destruction
// of the target media and start the install. Last change to abort.
type ConfirmInstallDialog struct {
	DialogBox *clui.Window
	Confirmed bool
	onClose   func()

	modelSI       *model.SystemInstall
	warningLabel  *clui.Label
	mediaLabel    *clui.Label
	cancelButton  *SimpleButton
	confirmButton *SimpleButton
}

// OnClose sets the callback that is called when the
// dialog is closed
func (dialog *ConfirmInstallDialog) OnClose(fn func()) {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	dialog.onClose = fn
}

// Close closes the dialog window and executes a callback if registered
func (dialog *ConfirmInstallDialog) Close() {
	clui.WindowManager().DestroyWindow(dialog.DialogBox)
	clui.WindowManager().BeginUpdate()
	closeFn := dialog.onClose
	_ = term.Flush() // This might be dropped once clui is fixed
	clui.WindowManager().EndUpdate()
	if closeFn != nil {
		closeFn()
	}
}

func initConfirmDiaglogWindow(dialog *ConfirmInstallDialog) error {

	const title = "Confirm Installation"
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

	// Build the string with the media being modified
	targets := []string{}
	if len(dialog.modelSI.TargetMedias) == 0 {
		targets = append(targets, "None")
	} else {
		for _, media := range dialog.modelSI.TargetMedias {
			targets = append(targets, media.GetDeviceFile())
		}
	}

	if dialog.modelSI.InstallSelected.EraseDisk {
		dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 2,
			"Warning! The following media will be erased and repartitioned for the Clear Linux OS Install:", 1)
	} else if dialog.modelSI.InstallSelected.WholeDisk {
		dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 2,
			"The following media will be partitioned for the Clear Linux OS Install:", 1)
	} else {
		dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 2,
			"The following media will have partitions added  for the Clear Linux OS Install:", 1)
	}
	dialog.warningLabel.SetMultiline(true)

	dialog.mediaLabel = clui.CreateLabel(borderFrame, 1, 1, "Target Media: "+strings.Join(targets, ", "), 1)
	dialog.mediaLabel.SetMultiline(true)
	if dialog.modelSI.InstallSelected.EraseDisk {
		dialog.mediaLabel.SetBackColor(term.ColorRed)
	}

	buttonFrame := clui.CreateFrame(borderFrame, AutoSize, 1, clui.BorderNone, clui.Fixed)
	buttonFrame.SetPack(clui.Horizontal)
	buttonFrame.SetGaps(1, 0)
	dialog.cancelButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, "Cancel", Fixed)
	dialog.cancelButton.SetEnabled(true)
	dialog.cancelButton.SetActive(true)

	dialog.confirmButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, "Confirm Install", Fixed)
	dialog.confirmButton.SetEnabled(true)
	dialog.confirmButton.SetActive(false)

	return nil
}

// CreateConfirmInstallDialogBox creates the Network PopUp
func CreateConfirmInstallDialogBox(modelSI *model.SystemInstall) (*ConfirmInstallDialog, error) {
	dialog := new(ConfirmInstallDialog)

	if dialog == nil {
		return nil, fmt.Errorf("Failed to allocate a Confirmation of Installation Dialog")
	}

	if modelSI == nil {
		return nil, fmt.Errorf("Missing model for Confirmation of Installation Dialog")
	}
	dialog.modelSI = modelSI

	if err := initConfirmDiaglogWindow(dialog); err != nil {
		return nil, fmt.Errorf("Failed to create Confirmation of Installation Dialog: %v", err)
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
