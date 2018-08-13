// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"time"

	"github.com/clearlinux/clr-installer/controller"
	"github.com/clearlinux/clr-installer/progress"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
)

// NetworkValidatePage is the Page implementation for network config validation, it also
// implements the progress.Client interface
type NetworkValidatePage struct {
	BasePage
	prgLabel *clui.Label
	prgBar   *clui.ProgressBar
	prgMax   int
	doneBtn  *SimpleButton
}

const (
	networkTestDesc = `By pressing the "Test" button we'll attempt to apply the specified
network settings and try to reach the required network servers.`
)

// Success is part of the progress.Client implementation and represents the
// successful progress completion of a task by setting
// the progress bar to "full"
func (page *NetworkValidatePage) Success() {
	page.prgBar.SetValue(page.prgMax)
	clui.RefreshScreen()
}

// Failure is part of the progress.Client implementation and represents the
// unsuccessful progress completion of a task by setting
// the progress bar to "fail"
func (page *NetworkValidatePage) Failure() {
	bg := page.prgBar.BackColor()
	page.prgBar.SetValue(0)
	for i := 1; i <= 5; i++ {
		page.prgBar.SetBackColor(term.ColorRed)
		clui.RefreshScreen()
		time.Sleep(100 * time.Millisecond)
		page.prgBar.SetBackColor(bg)
		clui.RefreshScreen()
		time.Sleep(100 * time.Millisecond)
	}
}

// Step is part of the progress.Client implementation and moves the progress bar one step
// case it becomes full it starts again
func (page *NetworkValidatePage) Step() {
	if page.prgBar.Value() == page.prgMax {
		page.prgBar.SetValue(0)
	} else {
		page.prgBar.Step()
	}
	clui.RefreshScreen()
}

// Desc is part of the progress.Client implementation and sets the progress bar label
func (page *NetworkValidatePage) Desc(desc string) {
	page.prgLabel.SetTitle(desc)
	clui.RefreshScreen()
}

// Partial is part of the progress.Client implementation and adjusts the progress bar to the
// current completion percentage
func (page *NetworkValidatePage) Partial(total int, step int) {
}

// LoopWaitDuration is part of the progress.Client implementation and returns the time duration
// each step should wait until calling Step again
func (page *NetworkValidatePage) LoopWaitDuration() time.Duration {
	return 1 * time.Second
}

// Activate resets the page state
func (page *NetworkValidatePage) Activate() {
	page.doneBtn.SetVisible(false)
	page.prgLabel.SetTitle("")
	page.prgBar.SetValue(0)
}

func newNetworkValidatePage(tui *Tui) (Page, error) {
	page := &NetworkValidatePage{}
	page.setupMenu(tui, TuiPageNetworkValidate, "Test Network Settings",
		NoButtons, TuiPageAdvancedMenu)

	lbl := clui.CreateLabel(page.content, 2, 2, "Test Network Settings", clui.Fixed)
	lbl.SetPaddings(0, 2)

	lbl = clui.CreateLabel(page.content, 70, 3, networkTestDesc, Fixed)
	lbl.SetMultiline(true)

	progressFrame := clui.CreateFrame(page.content, AutoSize, 3, BorderNone, clui.Fixed)
	progressFrame.SetPack(clui.Vertical)

	page.prgBar = clui.CreateProgressBar(progressFrame, AutoSize, AutoSize, clui.Fixed)

	page.prgLabel = clui.CreateLabel(progressFrame, 1, 1, "", Fixed)
	page.prgLabel.SetPaddings(0, 3)

	page.prgMax, _ = page.prgBar.Size()
	page.prgBar.SetLimits(0, page.prgMax)

	cancelBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Cancel", Fixed)
	cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageAdvancedMenu)
	})

	btn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Test", Fixed)
	btn.OnClick(func(ev clui.Event) {
		go func() {
			progress.Set(page)

			if err := controller.ConfigureNetwork(page.getModel()); err != nil {
				page.prgLabel.SetTitle("Failed. Network is not working.")
				page.Failure()
			} else {
				page.Success()
				page.prgLabel.SetTitle("Success.")
				page.doneBtn.SetVisible(true)
				clui.ActivateControl(tui.currPage.GetWindow(), page.doneBtn)
			}

			clui.RefreshScreen()
		}()
	})

	page.doneBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Done", Fixed)
	page.doneBtn.SetVisible(false)

	page.doneBtn.OnClick(func(ev clui.Event) {
		page.SetDone(true)
		page.GotoPage(TuiPageAdvancedMenu)
	})

	page.activated = btn

	return page, nil
}
