// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"time"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/progress"
)

// NetworkTestDialog is dialog window use to stop all other
// user interaction while the network configuration is tested
// for connectivity to the swupd server.
// Check for:
// - Working network interface
// - Use of Proxy (if set)
// - use of Swupd Mirror (if set)
type NetworkTestDialog struct {
	DialogBox *clui.Window
	onClose   func()

	modelSI       *model.SystemInstall
	progressBar   *clui.ProgressBar
	progressMax   int
	resultLabel   *clui.Label
	dismissButton *SimpleButton
}

// Success is part of the progress.Client implementation and represents the
// successful progress completion of a task by setting
// the progress bar to "full"
func (dialog *NetworkTestDialog) Success() {
	dialog.progressBar.SetValue(dialog.progressMax)
	clui.RefreshScreen()
}

// Failure is part of the progress.Client implementation and represents the
// unsuccessful progress completion of a task by setting
// the progress bar to "fail"
func (dialog *NetworkTestDialog) Failure() {
	bg := dialog.progressBar.BackColor()
	flashTime := 100 * time.Millisecond
	//dialog.progressBar.SetValue(0)  // leave the bar where it fails?
	for i := 1; i <= 5; i++ {
		dialog.progressBar.SetBackColor(term.ColorRed)
		clui.RefreshScreen()
		time.Sleep(flashTime)
		dialog.progressBar.SetBackColor(bg)
		clui.RefreshScreen()
		time.Sleep(flashTime)
	}
}

// Step is part of the progress.Client implementation and moves the progress bar one step
// case it becomes full it starts again
func (dialog *NetworkTestDialog) Step() {
	if dialog.progressBar.Value() == dialog.progressMax {
		dialog.progressBar.SetValue(0)
	} else {
		dialog.progressBar.Step()
	}
	clui.RefreshScreen()
}

// Desc is part of the progress.Client implementation and sets the progress bar label
func (dialog *NetworkTestDialog) Desc(desc string) {
	dialog.resultLabel.SetTitle(desc)
	clui.RefreshScreen()
}

// Partial is part of the progress.Client implementation and adjusts the progress bar to the
// current completion percentage
func (dialog *NetworkTestDialog) Partial(total int, step int) {
}

// LoopWaitDuration is part of the progress.Client implementation and returns the time duration
// each step should wait until calling Step again
func (dialog *NetworkTestDialog) LoopWaitDuration() time.Duration {
	return 1 * time.Second
}

// OnClose sets the callback that is called when the
// dialog is closed
func (dialog *NetworkTestDialog) OnClose(fn func()) {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	dialog.onClose = fn
}

// Close closes the dialog window and executes a callback if registered
func (dialog *NetworkTestDialog) Close() {
	clui.WindowManager().DestroyWindow(dialog.DialogBox)
	clui.WindowManager().BeginUpdate()
	closeFn := dialog.onClose
	_ = term.Flush() // This might be dropped once clui is fixed
	clui.WindowManager().EndUpdate()
	if closeFn != nil {
		closeFn()
	}
}

func initDiaglogWindow(dialog *NetworkTestDialog) error {

	const title = "Testing Networking..."
	const wBuff = 5
	const hBuff = 4
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

	borderFrame := clui.CreateFrame(dialog.DialogBox, dWidth, dHeight, clui.BorderNone, clui.Fixed)
	borderFrame.SetPack(clui.Vertical)
	borderFrame.SetGaps(0, 1)
	borderFrame.SetPaddings(1, 1)

	dialog.progressBar = clui.CreateProgressBar(borderFrame, AutoSize, 1, clui.Fixed)
	_, dialog.progressMax = dialog.progressBar.Limits()

	dialog.resultLabel = clui.CreateLabel(borderFrame, 1, 1, "Connecting to the network servers...", 1)
	dialog.resultLabel.SetMultiline(true)

	buttonFrame := clui.CreateFrame(borderFrame, AutoSize, 1, clui.BorderNone, clui.Fixed)
	buttonFrame.SetPack(clui.Horizontal)
	buttonFrame.SetGaps(1, 0)
	dialog.dismissButton = CreateSimpleButton(buttonFrame, AutoSize, AutoSize, "Dismiss", Fixed)
	dialog.dismissButton.SetEnabled(false)

	return nil
}

// CreateNetworkTestDialogBox creates the Network PopUp
func CreateNetworkTestDialogBox(modelSI *model.SystemInstall) (*NetworkTestDialog, error) {
	dialog := new(NetworkTestDialog)

	if dialog == nil {
		return nil, fmt.Errorf("Failed to allocate a Network Test Dialog")
	}

	if err := initDiaglogWindow(dialog); err != nil {
		return nil, fmt.Errorf("Failed to create Network Test Dialog: %v", err)
	}

	if modelSI == nil {
		return nil, fmt.Errorf("Missing model for Network Test Dialog")
	}
	dialog.modelSI = modelSI

	dialog.dismissButton.OnClick(func(ev clui.Event) {
		dialog.Close()
	})

	progress.Set(dialog)
	clui.RefreshScreen()

	return dialog, nil
}

// RunNetworkTest runs the test function
func (dialog *NetworkTestDialog) RunNetworkTest() bool {
	var status bool

	time.Sleep(time.Second)

	var err error
	if err = controller.ConfigureNetwork(dialog.modelSI); err != nil {
		log.Error("Network Testing: %s", err)
		dialog.resultLabel.SetTitle("Failed. Network is not working.")
		dialog.Failure()
		status = false
	} else {
		dialog.resultLabel.SetTitle("Success.")
		dialog.Success()
		status = true
	}

	dialog.dismissButton.SetEnabled(true)
	clui.ActivateControl(dialog.DialogBox, dialog.dismissButton)

	return status
}
