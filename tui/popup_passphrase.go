// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
)

// EncryptPassphraseDialog is dialog window use to stop all other
// user interaction and have the them confirm the destruction
// of the target media and start the install. Last change to abort.
type EncryptPassphraseDialog struct {
	DialogBox *clui.Window
	Confirmed bool
	onClose   func()

	infoLabel         *clui.Label
	passphraseEdit    *clui.EditField
	ppConfirmEdit     *clui.EditField
	warningLabel      *clui.Label
	changedPassphrase bool
	cancelButton      *SimpleButton
	confirmButton     *SimpleButton
}

// OnClose sets the callback that is called when the
// dialog is closed
func (dialog *EncryptPassphraseDialog) OnClose(fn func()) {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	dialog.onClose = fn
}

// Close closes the dialog window and executes a callback if registered
func (dialog *EncryptPassphraseDialog) Close() {
	clui.WindowManager().DestroyWindow(dialog.DialogBox)
	clui.WindowManager().BeginUpdate()
	closeFn := dialog.onClose
	_ = term.Flush() // This might be dropped once clui is fixed
	clui.WindowManager().EndUpdate()
	if closeFn != nil {
		closeFn()
	}
	clui.RefreshScreen()
}

func (dialog *EncryptPassphraseDialog) validatePassphrase() {
	if !dialog.changedPassphrase {
		return
	}

	if ok, msg := storage.IsValidPassphrase(dialog.passphraseEdit.Title()); !ok {
		dialog.warningLabel.SetTitle(msg)
		dialog.confirmButton.SetEnabled(false)
	} else if dialog.passphraseEdit.Title() != dialog.ppConfirmEdit.Title() {
		dialog.warningLabel.SetTitle("Passphrases do not match")
		dialog.confirmButton.SetEnabled(false)
	} else {
		dialog.warningLabel.SetTitle("")
		dialog.confirmButton.SetEnabled(true)
	}
}

func (dialog *EncryptPassphraseDialog) revealPassphrase() {
	if dialog.passphraseEdit.PasswordMode() {
		dialog.passphraseEdit.SetPasswordMode(false)
		dialog.ppConfirmEdit.SetPasswordMode(false)
	} else {
		dialog.passphraseEdit.SetPasswordMode(true)
		dialog.ppConfirmEdit.SetPasswordMode(true)
	}
}

func initPassphraseDialogWindow(dialog *EncryptPassphraseDialog) error {

	const title = "Encryption Passphrase"
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
	borderFrame.SetPaddings(1, 0)

	dialog.infoLabel = clui.CreateLabel(borderFrame, 1, 1,
		"Encryption requires a Passphrase", 1)
	dialog.infoLabel.SetMultiline(true)

	dialog.passphraseEdit = clui.CreateEditField(borderFrame, 1, "", Fixed)
	dialog.passphraseEdit.SetPasswordMode(true)

	dialog.passphraseEdit.OnChange(func(ev clui.Event) {
		dialog.validatePassphrase()
	})

	dialog.passphraseEdit.OnActive(func(active bool) {
		if dialog.passphraseEdit.Active() {
			dialog.validatePassphrase()
		}
	})

	dialog.passphraseEdit.OnKeyPress(func(k term.Key, ch rune) bool {
		if k == term.KeyCtrlU {
			dialog.revealPassphrase()
			return true
		}
		if k == term.KeyArrowUp || k == term.KeyArrowDown {
			return false
		}
		if !dialog.changedPassphrase {
			dialog.changedPassphrase = true
			dialog.passphraseEdit.SetTitle("")
			dialog.ppConfirmEdit.SetTitle("")
		}
		return false
	})

	dialog.ppConfirmEdit = clui.CreateEditField(borderFrame, 1, "", Fixed)
	dialog.ppConfirmEdit.SetPasswordMode(true)
	dialog.warningLabel = clui.CreateLabel(borderFrame, AutoSize, 1, "", Fixed)
	dialog.warningLabel.SetMultiline(true)
	dialog.warningLabel.SetBackColor(errorLabelBg)
	dialog.warningLabel.SetTextColor(errorLabelFg)

	dialog.ppConfirmEdit.OnChange(func(ev clui.Event) {
		dialog.validatePassphrase()
	})

	dialog.ppConfirmEdit.OnActive(func(active bool) {
		if dialog.ppConfirmEdit.Active() {
			dialog.validatePassphrase()
		}
	})

	dialog.ppConfirmEdit.OnKeyPress(func(k term.Key, ch rune) bool {
		if k == term.KeyCtrlU {
			dialog.revealPassphrase()
			return true
		}
		if k == term.KeyArrowUp || k == term.KeyArrowDown {
			return false
		}
		if !dialog.changedPassphrase {
			dialog.changedPassphrase = true
			dialog.passphraseEdit.SetTitle("")
			dialog.ppConfirmEdit.SetTitle("")
		}
		return false
	})

	buttonFrame := clui.CreateFrame(borderFrame, AutoSize, 1, clui.BorderNone, clui.Fixed)
	buttonFrame.SetPack(clui.Horizontal)
	buttonFrame.SetGaps(1, 0)
	dialog.cancelButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, "Cancel", Fixed)
	dialog.cancelButton.SetEnabled(true)

	dialog.confirmButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, "Confirm", Fixed)

	return nil
}

// CreateEncryptPassphraseDialogBox creates the Network PopUp
func CreateEncryptPassphraseDialogBox(modelSI *model.SystemInstall) (*EncryptPassphraseDialog, error) {
	dialog := new(EncryptPassphraseDialog)

	if dialog == nil {
		return nil, fmt.Errorf("Failed to allocate a Confirmation of Installation Dialog")
	}

	if modelSI == nil {
		return nil, fmt.Errorf("Missing model for Confirmation of Installation Dialog")
	}

	if err := initPassphraseDialogWindow(dialog); err != nil {
		return nil, fmt.Errorf("Failed to create Confirmation of Installation Dialog: %v", err)
	}

	dialog.cancelButton.OnClick(func(ev clui.Event) {
		dialog.Confirmed = false
		dialog.Close()
	})

	dialog.confirmButton.OnClick(func(ev clui.Event) {
		dialog.Confirmed = true
		modelSI.CryptPass = dialog.passphraseEdit.Title()
		dialog.Close()
	})

	if modelSI.CryptPass != "" {
		dialog.passphraseEdit.SetTitle(modelSI.CryptPass)
		dialog.ppConfirmEdit.SetTitle(modelSI.CryptPass)
		dialog.confirmButton.SetEnabled(true)
		clui.ActivateControl(dialog.DialogBox, dialog.confirmButton)
	} else {
		dialog.confirmButton.SetEnabled(false)
		clui.ActivateControl(dialog.DialogBox, dialog.passphraseEdit)
	}

	clui.RefreshScreen()

	return dialog, nil
}
