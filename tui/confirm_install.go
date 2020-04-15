// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/swupd"
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
	mediaDetail   *clui.TextView
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

	const wBuff = 5
	const hBuff = 5
	const dWidth = 55
	const dHeight = 10

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

	dialog.DialogBox = clui.AddWindow(posX, posY, dWidth, dHeight, storage.ConfirmInstallation)
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
	eraseDisk := false
	dataLoss := false
	wholeDisk := false

	if len(dialog.modelSI.TargetMedias) == 0 {
		targets = append(targets, "None")
	} else {
		for _, media := range dialog.modelSI.TargetMedias {
			targets = append(targets, media.GetDeviceFile())
			if val, ok := dialog.modelSI.InstallSelected[media.Name]; ok {
				eraseDisk = eraseDisk || val.EraseDisk
				dataLoss = dataLoss || val.DataLoss
				wholeDisk = wholeDisk || val.WholeDisk
			}
		}
	}
	sort.Strings(targets)

	if eraseDisk {
		dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 1, storage.DestructiveWarning, 1)
	} else if dataLoss {
		dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 1, storage.DataLossWarning, 1)
	} else if wholeDisk {
		dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 1, storage.SafeWholeWarning, 1)
	} else {
		dialog.warningLabel = clui.CreateLabel(borderFrame, 1, 1, storage.SafePartialWarning, 1)
	}
	dialog.warningLabel.SetMultiline(true)

	dialog.mediaLabel = clui.CreateLabel(borderFrame, 1, 1, "Target Media"+": "+strings.Join(targets, ", "), 1)
	dialog.mediaLabel.SetMultiline(true)

	if eraseDisk {
		dialog.mediaLabel.SetStyle("WarningLabel")
	}

	dialog.mediaDetail = clui.CreateTextView(borderFrame, 50, 5, 1)
	dialog.mediaDetail.SetWordWrap(true)
	dialog.mediaDetail.SetStyle("AltEdit")

	medias := storage.GetPlannedMediaChanges(dialog.modelSI.InstallSelected, dialog.modelSI.TargetMedias,
		dialog.modelSI.MediaOpts)
	for _, media := range medias {
		log.Debug("MediaChange: %s", media)
	}

	// Create additional bundle removal warning for offline installs
	if !controller.NetworkPassing && len(dialog.modelSI.UserBundles) != 0 && swupd.IsOfflineContent() {
		medias = append([]string{"Offline Install: Removing additional bundles"}, medias...)
	}

	dialog.mediaDetail.AddText(medias)
	// Add buffer to ensure we see all media changes
	dialog.mediaDetail.AddText([]string{"---", "=-="})

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
